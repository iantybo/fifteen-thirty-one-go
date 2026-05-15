// Package middleware provides Gin middleware components used by the HTTP
// server. The rate limiter implemented here is an in-memory, per-key token
// bucket with periodic GC. It is intentionally dependency-free so it can be
// dropped into any handler chain without external state.
package middleware

import (
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// Limit defines the token-bucket parameters for a single key. Capacity is the
// maximum burst; RefillEvery is how often a single token is added (so the
// sustained rate is 1 token / RefillEvery).
type Limit struct {
	Capacity    int
	RefillEvery time.Duration
}

// KeyFunc derives a stable key from a request. Implementations should return
// an empty string to indicate the request is not subject to rate limiting
// (e.g. health checks).
type KeyFunc func(c *gin.Context) string

// bucket is the internal mutable state for a single rate-limit key.
type bucket struct {
	tokens     int
	lastRefill time.Time
}

// Limiter is a goroutine-safe in-memory rate limiter. Use NewLimiter and pair
// with Middleware. A single Limiter can be shared across multiple routes; if
// you want different policies, instantiate one per policy.
type Limiter struct {
	mu       sync.Mutex
	buckets  map[string]*bucket
	limit    Limit
	stop     chan struct{}
	stopOnce sync.Once
}

// NewLimiter creates a Limiter with the given policy and starts a background
// goroutine that evicts stale buckets every gcInterval. Callers should invoke
// Stop when the limiter is no longer needed (e.g. on graceful shutdown) to
// avoid leaking the GC goroutine.
func NewLimiter(limit Limit, gcInterval time.Duration) *Limiter {
	if limit.Capacity <= 0 {
		limit.Capacity = 30
	}
	if limit.RefillEvery <= 0 {
		limit.RefillEvery = time.Second
	}
	if gcInterval <= 0 {
		gcInterval = 5 * time.Minute
	}
	l := &Limiter{
		buckets: make(map[string]*bucket, 256),
		limit:   limit,
		stop:    make(chan struct{}),
	}
	go l.runGC(gcInterval)
	return l
}

// Stop terminates the GC goroutine. Safe to call multiple times.
func (l *Limiter) Stop() {
	l.stopOnce.Do(func() { close(l.stop) })
}

func (l *Limiter) runGC(interval time.Duration) {
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-l.stop:
			return
		case now := <-t.C:
			l.gc(now)
		}
	}
}

// gc removes buckets that have been idle for at least 4x the refill window.
// The threshold is heuristic: long enough that an active client is unlikely
// to be evicted between requests, short enough that abandoned keys do not
// accumulate.
func (l *Limiter) gc(now time.Time) {
	threshold := 4 * l.limit.RefillEvery * time.Duration(l.limit.Capacity)
	if threshold < time.Minute {
		threshold = time.Minute
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	for k, b := range l.buckets {
		if now.Sub(b.lastRefill) > threshold && b.tokens >= l.limit.Capacity {
			delete(l.buckets, k)
		}
	}
}

// take attempts to consume one token for key. It returns true if a token was
// available, along with the number of tokens remaining and the time at which
// the next token will be available (zero if a token is available now).
func (l *Limiter) take(key string, now time.Time) (allowed bool, remaining int, retryAfter time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()

	b, ok := l.buckets[key]
	if !ok {
		b = &bucket{tokens: l.limit.Capacity, lastRefill: now}
		l.buckets[key] = b
	}

	// Refill based on elapsed time.
	elapsed := now.Sub(b.lastRefill)
	if elapsed > 0 && l.limit.RefillEvery > 0 {
		add := int(elapsed / l.limit.RefillEvery)
		if add > 0 {
			b.tokens += add
			if b.tokens > l.limit.Capacity {
				b.tokens = l.limit.Capacity
			}
			b.lastRefill = b.lastRefill.Add(time.Duration(add) * l.limit.RefillEvery)
		}
	}

	if b.tokens <= 0 {
		// No token available; report when the next one arrives.
		retryAfter = l.limit.RefillEvery - now.Sub(b.lastRefill)
		if retryAfter < 0 {
			retryAfter = 0
		}
		return false, 0, retryAfter
	}

	b.tokens--
	return true, b.tokens, 0
}

// Snapshot is a read-only view of the limiter state for the given key. It is
// primarily useful in tests and for introspection endpoints.
type Snapshot struct {
	Tokens     int
	Capacity   int
	LastRefill time.Time
}

// SnapshotKey returns the current state of a key without consuming a token.
func (l *Limiter) SnapshotKey(key string) (Snapshot, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	b, ok := l.buckets[key]
	if !ok {
		return Snapshot{}, false
	}
	return Snapshot{Tokens: b.tokens, Capacity: l.limit.Capacity, LastRefill: b.lastRefill}, true
}

// Middleware returns a Gin middleware that enforces the limiter against keys
// produced by keyFn. Requests for which keyFn returns an empty string are
// passed through unchecked. The middleware sets standard X-RateLimit-* headers
// on every response.
func (l *Limiter) Middleware(keyFn KeyFunc) gin.HandlerFunc {
	if keyFn == nil {
		keyFn = defaultKeyFn
	}
	return func(c *gin.Context) {
		key := keyFn(c)
		if key == "" {
			c.Next()
			return
		}
		allowed, remaining, retry := l.take(key, time.Now())
		c.Header("X-RateLimit-Limit", strconv.Itoa(l.limit.Capacity))
		c.Header("X-RateLimit-Remaining", strconv.Itoa(remaining))
		if !allowed {
			seconds := int(retry.Seconds())
			if seconds < 1 {
				seconds = 1
			}
			c.Header("Retry-After", strconv.Itoa(seconds))
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error":       "rate limit exceeded",
				"retry_after": seconds,
			})
			return
		}
		c.Next()
	}
}

// defaultKeyFn falls back to the client's IP address. It honours
// X-Forwarded-For when present so deployments behind a trusted proxy work
// out of the box.
func defaultKeyFn(c *gin.Context) string {
	if ip := c.GetHeader("X-Forwarded-For"); ip != "" {
		return ip
	}
	return c.ClientIP()
}

// UserOrIPKey returns a KeyFunc that prefers the authenticated user ID and
// falls back to client IP. Useful for endpoints that allow both authenticated
// and anonymous traffic.
func UserOrIPKey(c *gin.Context) string {
	if v, ok := c.Get("user_id"); ok {
		switch id := v.(type) {
		case int64:
			if id > 0 {
				return "u:" + strconv.FormatInt(id, 10)
			}
		case int:
			if id > 0 {
				return "u:" + strconv.Itoa(id)
			}
		}
	}
	return "ip:" + defaultKeyFn(c)
}
