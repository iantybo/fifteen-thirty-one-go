package middleware

import (
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// RequestIDHeader is the response header carrying the per-request correlation id.
const RequestIDHeader = "X-Request-Id"

// requestIDContextKey is the gin context key holding the correlation id.
const requestIDContextKey = "requestID"

// maxForwardedRequestIDLen bounds an inbound request id so a caller cannot
// push arbitrarily large values into our logs.
const maxForwardedRequestIDLen = 64

// newRequestID returns a random hex correlation id. It falls back to a
// timestamp-derived value if the CSPRNG is unavailable, since a request id is
// for correlation only and must never fail the request.
func newRequestID() string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "ts-" + strconv.FormatInt(time.Now().UnixNano(), 16)
	}
	return hex.EncodeToString(b[:])
}

// safeForwardedRequestID sanitizes a client-supplied request id: it keeps only
// printable ASCII and truncates, so log output cannot be forged or injected
// with newlines.
func safeForwardedRequestID(raw string) string {
	s := strings.TrimSpace(raw)
	if s == "" {
		return ""
	}
	if len(s) > maxForwardedRequestIDLen {
		s = s[:maxForwardedRequestIDLen]
	}
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		// Allow only unambiguous id characters; drop anything else.
		ok := (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') ||
			(c >= '0' && c <= '9') || c == '-' || c == '_'
		if ok {
			b.WriteByte(c)
		}
	}
	return b.String()
}

// RequestIDFromContext returns the correlation id assigned to this request.
func RequestIDFromContext(c *gin.Context) string {
	if v, ok := c.Get(requestIDContextKey); ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// RequestLogger assigns each request a correlation id and logs one structured
// line per completed request: method, path, status, duration and client IP.
//
// Query strings are deliberately omitted: the websocket upgrade path accepts a
// token query parameter, so logging raw queries would write credentials to the
// log.
func RequestLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := safeForwardedRequestID(c.GetHeader(RequestIDHeader))
		if id == "" {
			id = newRequestID()
		}
		c.Set(requestIDContextKey, id)
		c.Writer.Header().Set(RequestIDHeader, id)

		start := time.Now()
		c.Next()
		dur := time.Since(start)

		status := c.Writer.Status()
		attrs := []any{
			"request_id", id,
			"method", c.Request.Method,
			// Use the matched route template where available so logs group by
			// endpoint instead of exploding per id.
			"path", routeForLog(c),
			"status", status,
			"duration_ms", dur.Milliseconds(),
			"client_ip", c.ClientIP(),
		}

		// Surface any errors gin collected during the request.
		if errs := c.Errors.ByType(gin.ErrorTypeAny); len(errs) > 0 {
			attrs = append(attrs, "errors", errs.String())
		}

		switch {
		case status >= 500:
			slog.Error("request", attrs...)
		case status >= 400:
			slog.Warn("request", attrs...)
		default:
			slog.Info("request", attrs...)
		}
	}
}

// routeForLog prefers the matched route template (e.g. /api/decks/:deckId) and
// falls back to the raw path for unmatched routes.
func routeForLog(c *gin.Context) string {
	if fp := c.FullPath(); fp != "" {
		return fp
	}
	return c.Request.URL.Path
}
