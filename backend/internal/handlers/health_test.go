package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	ws "fifteen-thirty-one-go/backend/pkg/websocket"

	"github.com/gin-gonic/gin"
	_ "github.com/mattn/go-sqlite3"
)

func TestMain(m *testing.M) {
	gin.SetMode(gin.TestMode)
	m.Run()
}

// openTestDB returns an open in-memory database that is closed with the test.
func openTestDB(t *testing.T) *sql.DB {
	t.Helper()

	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	if err := db.Ping(); err != nil {
		t.Fatalf("ping test db: %v", err)
	}
	return db
}

// doGet routes a GET request through a bare engine and returns the recorder.
func doGet(t *testing.T, path string, h gin.HandlerFunc) *httptest.ResponseRecorder {
	t.Helper()

	r := gin.New()
	r.GET(path, h)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	return rec
}

func decodeReady(t *testing.T, rec *httptest.ResponseRecorder) readyResponse {
	t.Helper()

	var got readyResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal body %q: %v", rec.Body.String(), err)
	}
	return got
}

// liveHub returns a hub provider backed by a running hub.
func liveHub() func() (*ws.Hub, bool) {
	h := ws.NewHub()
	return func() (*ws.Hub, bool) { return h, true }
}

func TestHealthHandlerReportsOKWithoutDependencies(t *testing.T) {
	rec := doGet(t, "/healthz", HealthHandler())

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var got healthResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal body %q: %v", rec.Body.String(), err)
	}
	if !got.OK {
		t.Error("ok = false, want true")
	}
}

func TestReadyHandlerAllChecksPass(t *testing.T) {
	rec := doGet(t, "/readyz", ReadyHandler(openTestDB(t), liveHub()))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d (body %q)", rec.Code, http.StatusOK, rec.Body.String())
	}

	got := decodeReady(t, rec)
	if !got.OK {
		t.Error("ok = false, want true")
	}
	for _, name := range []string{"database", "websocket_hub"} {
		if got.Checks[name] != "ok" {
			t.Errorf("checks[%q] = %q, want %q", name, got.Checks[name], "ok")
		}
	}
	if got.Version == "" {
		t.Error("version is empty, want a resolved value")
	}
	if got.UptimeSeconds < 0 {
		t.Errorf("uptime_seconds = %d, want >= 0", got.UptimeSeconds)
	}
}

func TestReadyHandlerReturns503WhenDatabaseIsClosed(t *testing.T) {
	db := openTestDB(t)
	if err := db.Close(); err != nil {
		t.Fatalf("close test db: %v", err)
	}

	rec := doGet(t, "/readyz", ReadyHandler(db, liveHub()))

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}

	got := decodeReady(t, rec)
	if got.OK {
		t.Error("ok = true, want false")
	}
	if got.Checks["database"] == "ok" {
		t.Error(`checks["database"] = "ok", want a failure reason`)
	}
	// A failing database must not mask an otherwise healthy hub.
	if got.Checks["websocket_hub"] != "ok" {
		t.Errorf("checks[%q] = %q, want %q", "websocket_hub", got.Checks["websocket_hub"], "ok")
	}
}

func TestReadyHandlerReturns503WhenHubIsStopped(t *testing.T) {
	h := ws.NewHub()
	h.Stop()

	rec := doGet(t, "/readyz", ReadyHandler(openTestDB(t), func() (*ws.Hub, bool) { return h, true }))

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}

	got := decodeReady(t, rec)
	if got.OK {
		t.Error("ok = true, want false")
	}
	if got.Checks["websocket_hub"] != "unavailable: stopped" {
		t.Errorf("checks[%q] = %q, want %q", "websocket_hub", got.Checks["websocket_hub"], "unavailable: stopped")
	}
}

func TestReadyHandlerHandlesMissingDependencies(t *testing.T) {
	tests := []struct {
		name  string
		nilDB bool
		hub   func() (*ws.Hub, bool)
		check string
	}{
		{name: "nil db", nilDB: true, hub: liveHub(), check: "database"},
		{name: "nil hub provider", hub: nil, check: "websocket_hub"},
		{name: "provider returns no hub", hub: func() (*ws.Hub, bool) { return nil, false }, check: "websocket_hub"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var db *sql.DB
			if !tc.nilDB {
				db = openTestDB(t)
			}

			rec := doGet(t, "/readyz", ReadyHandler(db, tc.hub))

			if rec.Code != http.StatusServiceUnavailable {
				t.Fatalf("status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
			}
			got := decodeReady(t, rec)
			if got.Checks[tc.check] == "ok" {
				t.Errorf("checks[%q] = %q, want a failure reason", tc.check, got.Checks[tc.check])
			}
		})
	}
}

func TestHubRunningReflectsStop(t *testing.T) {
	h := ws.NewHub()
	if !h.Running() {
		t.Fatal("Running() = false on a new hub, want true")
	}

	h.Stop()
	if h.Running() {
		t.Error("Running() = true after Stop(), want false")
	}

	// Stop is idempotent, so a second call must not panic or flip the result.
	h.Stop()
	if h.Running() {
		t.Error("Running() = true after a second Stop(), want false")
	}
}
