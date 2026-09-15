package middleware

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestSafeForwardedRequestID(t *testing.T) {
	// A forwarded id must not be able to inject newlines or arbitrary text
	// into log output.
	cases := map[string]string{
		"abc123":                "abc123",
		"  abc-123_x  ":         "abc-123_x",
		"bad\nid status=200":    "badidstatus200",
		"drop;these:chars":      "dropthesechars",
		"":                      "",
		"   ":                   "",
		strings.Repeat("a", 90): strings.Repeat("a", maxForwardedRequestIDLen),
	}
	for in, want := range cases {
		if got := safeForwardedRequestID(in); got != want {
			t.Errorf("safeForwardedRequestID(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestNewRequestIDIsUnique(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 100; i++ {
		id := newRequestID()
		if id == "" {
			t.Fatal("newRequestID returned empty")
		}
		if seen[id] {
			t.Fatalf("duplicate request id %q", id)
		}
		seen[id] = true
	}
}

// newTestRouter returns a router with the logger installed, writing logs to buf.
func newTestRouter(buf *bytes.Buffer) *gin.Engine {
	gin.SetMode(gin.TestMode)
	slog.SetDefault(slog.New(slog.NewTextHandler(buf, nil)))

	r := gin.New()
	r.Use(RequestLogger())
	r.GET("/ok", func(c *gin.Context) { c.Status(http.StatusOK) })
	r.GET("/items/:id", func(c *gin.Context) { c.Status(http.StatusOK) })
	r.GET("/boom", func(c *gin.Context) { c.Status(http.StatusInternalServerError) })
	r.GET("/nope", func(c *gin.Context) { c.Status(http.StatusNotFound) })
	r.GET("/whoami", func(c *gin.Context) { c.String(http.StatusOK, RequestIDFromContext(c)) })
	return r
}

func TestRequestLoggerSetsResponseHeaderAndLogs(t *testing.T) {
	var buf bytes.Buffer
	r := newTestRouter(&buf)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/ok", nil))

	if got := w.Header().Get(RequestIDHeader); got == "" {
		t.Error("response should carry a request id header")
	}
	out := buf.String()
	for _, want := range []string{"method=GET", "path=/ok", "status=200", "request_id="} {
		if !strings.Contains(out, want) {
			t.Errorf("log output missing %q; got %s", want, out)
		}
	}
}

func TestRequestLoggerReusesForwardedID(t *testing.T) {
	var buf bytes.Buffer
	r := newTestRouter(&buf)

	req := httptest.NewRequest(http.MethodGet, "/whoami", nil)
	req.Header.Set(RequestIDHeader, "client-supplied-1")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Body.String() != "client-supplied-1" {
		t.Errorf("handler saw request id %q, want the forwarded one", w.Body.String())
	}
	if w.Header().Get(RequestIDHeader) != "client-supplied-1" {
		t.Errorf("response header = %q", w.Header().Get(RequestIDHeader))
	}
}

func TestRequestLoggerSanitizesForwardedID(t *testing.T) {
	var buf bytes.Buffer
	r := newTestRouter(&buf)

	req := httptest.NewRequest(http.MethodGet, "/ok", nil)
	req.Header.Set(RequestIDHeader, "evil\nlevel=ERROR msg=fake")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if strings.Contains(buf.String(), "msg=fake") {
		t.Errorf("forged log content leaked through: %s", buf.String())
	}
}

func TestRequestLoggerUsesRouteTemplate(t *testing.T) {
	var buf bytes.Buffer
	r := newTestRouter(&buf)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/items/12345", nil))

	// Logging the template keeps per-endpoint grouping usable.
	if !strings.Contains(buf.String(), "path=/items/:id") {
		t.Errorf("expected route template in log, got %s", buf.String())
	}
	if strings.Contains(buf.String(), "/items/12345") {
		t.Errorf("raw path should not be logged for matched routes: %s", buf.String())
	}
}

func TestRequestLoggerSeverityBySatus(t *testing.T) {
	for _, tc := range []struct {
		path  string
		level string
	}{
		{"/ok", "level=INFO"},
		{"/nope", "level=WARN"},
		{"/boom", "level=ERROR"},
	} {
		var buf bytes.Buffer
		r := newTestRouter(&buf)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, tc.path, nil))
		if !strings.Contains(buf.String(), tc.level) {
			t.Errorf("%s: expected %s, got %s", tc.path, tc.level, buf.String())
		}
	}
}

func TestRequestLoggerOmitsQueryString(t *testing.T) {
	var buf bytes.Buffer
	r := newTestRouter(&buf)

	// The websocket path accepts a token query param, so queries must never
	// reach the logs.
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/ok?token=super-secret", nil))

	if strings.Contains(buf.String(), "super-secret") {
		t.Errorf("query string leaked into logs: %s", buf.String())
	}
}
