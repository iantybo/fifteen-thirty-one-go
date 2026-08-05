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
	"fifteen-thirty-one-go/backend/internal/models"
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

	// Bootstrap schemas for feature modules that manage their own tables.
	// These calls are idempotent (CREATE TABLE IF NOT EXISTS) so it's safe to
	// run them every startup.
	bootstrapCtx, cancelBootstrap := context.WithTimeout(context.Background(), 10*time.Second)
	if err := models.EnsureFriendsSchema(bootstrapCtx, db); err != nil {
		cancelBootstrap()
		log.Fatalf("friends schema: %v", err)
	}
	if err := models.EnsureAchievementsSchema(bootstrapCtx, db); err != nil {
		cancelBootstrap()
		log.Fatalf("achievements schema: %v", err)
	}
	if err := handlers.EnsureReactionsSchema(bootstrapCtx, db); err != nil {
		cancelBootstrap()
		log.Fatalf("reactions schema: %v", err)
	}
	cancelBootstrap()

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

	// Stricter rate limit on auth: 10 attempts / minute, burst 10. This
	// dampens credential-stuffing without affecting normal users.
	authLimiter := middleware.NewLimiter(middleware.Limit{Capacity: 10, RefillEvery: 6 * time.Second}, 5*time.Minute)
	defer authLimiter.Stop()
	authGroup := api.Group("")
	authGroup.Use(authLimiter.Middleware(nil))
	handlers.RegisterAuthRoutes(authGroup, db, cfg)

	protected := api.Group("")
	protected.Use(middleware.RequireAuth(cfg))

	// General rate limit on authenticated traffic: 120 req/min, burst 120.
	apiLimiter := middleware.NewLimiter(middleware.Limit{Capacity: 120, RefillEvery: 500 * time.Millisecond}, 10*time.Minute)
	defer apiLimiter.Stop()
	protected.Use(apiLimiter.Middleware(middleware.UserOrIPKey))

	handlers.RegisterLobbyRoutes(protected, db)
	handlers.RegisterGameRoutes(protected, db)
	handlers.RegisterFriendsRoutes(protected, db)
	handlers.RegisterAchievementsRoutes(protected, db)
	handlers.RegisterChatReactionsRoutes(protected, db)

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
