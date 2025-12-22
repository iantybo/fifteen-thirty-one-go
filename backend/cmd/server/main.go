package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"fifteen-thirty-one-go/backend/internal/config"
	"fifteen-thirty-one-go/backend/internal/database"
	"fifteen-thirty-one-go/backend/internal/handlers"
	"fifteen-thirty-one-go/backend/internal/middleware"
	"fifteen-thirty-one-go/backend/pkg/websocket"

	"github.com/gin-gonic/gin"
)

func main() {
	cfg, err := config.LoadFromEnv()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	db, err := database.OpenAndMigrate(cfg.DatabasePath)
	if err != nil {
		log.Fatalf("db open/migrate: %v", err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			log.Printf("db close error: %v", err)
		}
	}()

	hub := websocket.NewHub()
	go hub.Run()

	handlers.SetWebSocketOriginPolicy(cfg.AppEnv == "development", cfg.DevWebSocketsAllowAll, cfg.WSAllowedOrigins)

	r := gin.Default()
	r.GET("/healthz", func(c *gin.Context) { c.JSON(200, gin.H{"ok": true}) })

	api := r.Group("/api")
	handlers.RegisterAuthRoutes(api, db, cfg)

	protected := api.Group("")
	protected.Use(middleware.RequireAuth(cfg))
	handlers.RegisterLobbyRoutes(protected, db)
	handlers.RegisterGameRoutes(protected, db)

	// WebSocket endpoint is auth-gated via token query param or Authorization header.
	r.GET("/ws", handlers.WebSocketHandler(hub, db, cfg))

	addr := cfg.Addr
	if addr == "" {
		if v := os.Getenv("PORT"); v != "" {
			// Address resolution precedence:
			// 1) BACKEND_ADDR (already loaded into cfg.Addr)
			// 2) PORT (when BACKEND_ADDR is not set)
			// 3) Default 127.0.0.1:8080
			addr = "0.0.0.0:" + v
		} else {
			addr = "127.0.0.1:8080"
		}
	}

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

	hub.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("server shutdown error: %v", err)
	}
}


