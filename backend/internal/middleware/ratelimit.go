package middleware

import (
	"math"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

// IPRateLimiter applies a token-bucket limit per client IP (via gin ClientIP).
type IPRateLimiter struct {
	mu           sync.Mutex
	limiters     map[string]*visitorEntry
	lim          rate.Limit
	burst        int
	ttl          time.Duration
	lastSweep    time.Time
	sweepEvery   time.Duration
}

type visitorEntry struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// NewIPRateLimiter creates a limiter where each IP gets an independent bucket.
// Example: NewIPRateLimiter(rate.Every(time.Minute/60), 60) allows ~60 req/min with burst 60.
func NewIPRateLimiter(r rate.Limit, burst int) *IPRateLimiter {
	return &IPRateLimiter{
		limiters:   make(map[string]*visitorEntry),
		lim:        r,
		burst:      burst,
		ttl:        10 * time.Minute,
		lastSweep:  time.Now(),
		sweepEvery: 2 * time.Minute,
	}
}

func (l *IPRateLimiter) maybeSweep(now time.Time) {
	if now.Sub(l.lastSweep) < l.sweepEvery {
		return
	}
	l.lastSweep = now
	for ip, v := range l.limiters {
		if now.Sub(v.lastSeen) > l.ttl {
			delete(l.limiters, ip)
		}
	}
}

func (l *IPRateLimiter) limiterFor(ip string) *rate.Limiter {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	l.maybeSweep(now)
	ent, ok := l.limiters[ip]
	if !ok {
		ent = &visitorEntry{
			limiter:  rate.NewLimiter(l.lim, l.burst),
			lastSeen: now,
		}
		l.limiters[ip] = ent
	} else {
		ent.lastSeen = now
	}
	return ent.limiter
}

func (l *IPRateLimiter) retryAfterSeconds() int {
	if l.lim <= 0 {
		return 60
	}
	// Next token is available after ~1/lim seconds (token bucket refill).
	sec := math.Ceil(1.0 / float64(l.lim))
	if sec < 1 {
		return 1
	}
	if sec > 3600 {
		return 3600
	}
	return int(sec)
}

// GinHandler returns middleware that returns 429 when the IP exceeds its limit.
func (l *IPRateLimiter) GinHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		ip := c.ClientIP()
		if !l.limiterFor(ip).Allow() {
			ra := strconv.Itoa(l.retryAfterSeconds())
			c.Header("Retry-After", ra)
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": "rate limit exceeded"})
			return
		}
		c.Next()
	}
}
