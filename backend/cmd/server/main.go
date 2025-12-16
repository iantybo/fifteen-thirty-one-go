package main

import (
	"log"
	"os"

	"fifteen-thirty-one-go/backend/internal/database"
	"fifteen-thirty-one-go/backend/internal/handlers"
	"fifteen-thirty-one-go/backend/internal/middleware"
	"fifteen-thirty-one-go/backend/pkg/websocket"

	"github.com/gin-gonic/gin"
)

func main() {
	cfg := handlers.LoadConfigFromEnv()

	db, err := database.OpenAndMigrate(cfg.DatabasePath)
	if err != nil {
		log.Fatalf("db open/migrate: %v", err)
	}
	defer db.Close()

	hub := websocket.NewHub()
	go hub.Run()

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
		addr = "127.0.0.1:8080"
	}
	if v := os.Getenv("PORT"); v != "" {
		// Optional: some hosts set PORT. For local dev, BACKEND_ADDR wins.
		addr = "0.0.0.0:" + v
	}

	log.Printf("listening on %s", addr)
	if err := r.Run(addr); err != nil {
		log.Fatalf("server: %v", err)
	}
}


