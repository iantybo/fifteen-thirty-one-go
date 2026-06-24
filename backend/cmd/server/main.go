package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"runtime/debug"
	"syscall"
	"time"

	"fifteen-thirty-one-go/backend/internal/config"
	"fifteen-thirty-one-go/backend/internal/database"
	"fifteen-thirty-one-go/backend/internal/handlers"
	"fifteen-thirty-one-go/backend/internal/middleware"
	"fifteen-thirty-one-go/backend/internal/tracing"
	"fifteen-thirty-one-go/backend/pkg/websocket"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
)

func main() {
	cfg, err := config.LoadFromEnv()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	// Initialize OpenTelemetry tracing
	shutdown := tracing.InitTracer("fifteen-thirty-one-go")
	defer shutdown()

	db, err := database.OpenAndMigrate(cfg.DatabasePath)
	if err != nil {
		log.Fatalf("db open/migrate: %v", err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			log.Printf("db close error: %v", err)
		}
	}()

	hubRef := websocket.NewHubRef(websocket.NewHub())
	go func() {
		for {
			panicked := false
			currentHub, ok := hubRef.Get()
			if !ok || currentHub == nil {
				// Should never happen (we always Store a *Hub), but avoid nil deref.
				time.Sleep(1 * time.Second)
				hubRef.Set(websocket.NewHub())
				continue
			}
			func() {
				defer func() {
					if r := recover(); r != nil {
						panicked = true
						log.Printf("hub.Run panic: %v\n%s", r, debug.Stack())
					}
				}()
				currentHub.Run()
			}()

			// If hub.Run returned normally (e.g., Stop() called), exit.
			// Only restart on panic.
			if !panicked {
				return
			}
			// Ensure any existing clients stop attempting to enqueue work to a dead hub.
			// This makes Register/Join/Unregister/Broadcast no-ops instead of potentially blocking forever.
			currentHub.Stop()
			// Reinitialize hub to ensure clean state.
			hubRef.Set(websocket.NewHub())
			time.Sleep(1 * time.Second)
		}
	}()

	handlers.SetWebSocketOriginPolicy(cfg.AppEnv == "development", cfg.DevWebSocketsAllowAll, cfg.WSAllowedOrigins)
	handlers.SetHubProvider(hubRef.Get)

	r := gin.Default()
	r.Use(otelgin.Middleware("fifteen-thirty-one-go"))
	r.Use(middleware.DevCORS(cfg))
	r.GET("/healthz", func(c *gin.Context) { c.JSON(200, gin.H{"ok": true}) })

	api := r.Group("/api")
	handlers.RegisterAuthRoutes(api, db, cfg)

	protected := api.Group("")
	protected.Use(middleware.RequireAuth(cfg))
	handlers.RegisterLobbyRoutes(protected, db)
	handlers.RegisterGameRoutes(protected, db)

	// WebSocket endpoint is auth-gated via token query param or Authorization header.
	r.GET("/ws", handlers.WebSocketHandler(hubRef.Get, db, cfg))

	// cfg.Addr is fully resolved by config.LoadFromEnv() (BACKEND_ADDR or PORT).
	addr := cfg.Addr

	srv := &http.Server{
		Addr:         addr,
		Handler:      r,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		log.Printf("listening on %s", addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-sigCh:
		log.Printf("shutdown signal received: %v", sig)
	case err := <-errCh:
		log.Printf("server error: %v", err)
	}

	if h, ok := hubRef.Get(); ok && h != nil {
		h.Stop()
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("server shutdown error: %v", err)
	}
}
