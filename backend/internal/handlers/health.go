package handlers

import (
	"context"
	"database/sql"
	"net/http"
	"runtime/debug"
	"sync"
	"time"

	ws "fifteen-thirty-one-go/backend/pkg/websocket"

	"github.com/gin-gonic/gin"
)

// readyzPingTimeout bounds the database check so a wedged DB cannot hold the
// readiness probe open past the server's write timeout.
const readyzPingTimeout = 2 * time.Second

// processStart is the reference point for the uptime reported by /readyz.
var processStart = time.Now()

// buildVersion resolves the running build's version once, lazily, since
// debug.ReadBuildInfo walks the binary's embedded metadata.
var buildVersion = sync.OnceValue(func() string {
	info, ok := debug.ReadBuildInfo()
	if !ok || info.Main.Version == "" {
		return "unknown"
	}
	return info.Main.Version
})

// healthResponse is the liveness payload: it answers "is this process up?" and
// deliberately performs no dependency checks.
type healthResponse struct {
	OK bool `json:"ok"`
}

// readyResponse is the readiness payload: it answers "can this process serve
// traffic?" and reports one status string per checked dependency.
type readyResponse struct {
	OK            bool              `json:"ok"`
	Version       string            `json:"version"`
	UptimeSeconds int64             `json:"uptime_seconds"`
	Checks        map[string]string `json:"checks"`
}

// HealthHandler reports process liveness. It never touches dependencies, so an
// unhealthy database does not cause an orchestrator to restart the process.
func HealthHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, healthResponse{OK: true})
	}
}

// ReadyHandler reports whether the process can serve traffic, checking the
// database and the websocket hub. It returns 503 when any check fails so load
// balancers drain this instance while leaving it running.
func ReadyHandler(db *sql.DB, hub func() (*ws.Hub, bool)) gin.HandlerFunc {
	return func(c *gin.Context) {
		checks := map[string]string{
			"database":      checkDatabase(c.Request.Context(), db),
			"websocket_hub": checkHub(hub),
		}

		ok := true
		for _, status := range checks {
			if status != "ok" {
				ok = false
				break
			}
		}

		code := http.StatusOK
		if !ok {
			code = http.StatusServiceUnavailable
		}

		c.JSON(code, readyResponse{
			OK:            ok,
			Version:       buildVersion(),
			UptimeSeconds: int64(time.Since(processStart).Seconds()),
			Checks:        checks,
		})
	}
}

// checkDatabase pings the database, returning "ok" or a reason. The reason is a
// fixed string rather than the driver error so the endpoint stays unauthenticated
// without leaking connection details.
func checkDatabase(ctx context.Context, db *sql.DB) string {
	if db == nil {
		return "unavailable: not configured"
	}

	ctx, cancel := context.WithTimeout(ctx, readyzPingTimeout)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		return "unavailable: ping failed"
	}
	return "ok"
}

// checkHub reports whether a live websocket hub is available to serve realtime
// gameplay.
func checkHub(hub func() (*ws.Hub, bool)) string {
	if hub == nil {
		return "unavailable: not configured"
	}

	h, ok := hub()
	if !ok || h == nil {
		return "unavailable: no hub"
	}
	if !h.Running() {
		return "unavailable: stopped"
	}
	return "ok"
}
