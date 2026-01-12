package handlers

import (
	"database/sql"

	"fifteen-thirty-one-go/backend/internal/config"
	ws "fifteen-thirty-one-go/backend/pkg/websocket"
	"github.com/gin-gonic/gin"
)

// RegisterAuthRoutes wires auth endpoints. Implemented fully in Phase 1.2.
func RegisterAuthRoutes(rg *gin.RouterGroup, db *sql.DB, cfg config.Config) {
	rg.POST("/auth/register", RegisterHandler(db, cfg))
	rg.POST("/auth/login", LoginHandler(db, cfg))
	rg.GET("/auth/me", MeHandler(db, cfg))
	rg.POST("/auth/logout", LogoutHandler(cfg))
}

// RegisterLobbyRoutes wires lobby endpoints. Implemented fully in Phase 3.
func RegisterLobbyRoutes(rg *gin.RouterGroup, db *sql.DB) {
	rg.GET("/lobbies", ListLobbiesHandler(db))
	rg.POST("/lobbies", CreateLobbyHandler(db))
	rg.POST("/lobbies/:id/join", JoinLobbyHandler(db))
	rg.POST("/lobbies/:id/add_bot", AddBotToLobbyHandler(db))

	// Lobby chat (Yahoo Games inspired)
	rg.GET("/lobbies/:id/chat", GetLobbyChatHistory(db))
	rg.POST("/lobbies/:id/chat", SendLobbyChatMessage(db, getHubProvider))

	// Spectator mode
	rg.POST("/lobbies/:id/spectate", JoinAsSpectator(db, getHubProvider))
	rg.DELETE("/lobbies/:id/spectate", LeaveAsSpectator(db, getHubProvider))
	rg.GET("/lobbies/:id/spectators", GetSpectators(db))

	// User presence
	rg.PUT("/users/presence", UpdatePresence(db, getHubProvider))
	rg.POST("/users/presence/heartbeat", HeartbeatPresence(db))
	rg.GET("/users/:id/presence", GetPresence(db))
}

// getHubProvider returns the current websocket hub and a boolean indicating whether a hub provider
// is configured. When hubProvider is nil, it returns (nil, false).
func getHubProvider() (*ws.Hub, bool) {
	if hubProvider == nil {
		return nil, false
	}
	return hubProvider()
}

// RegisterGameRoutes wires game endpoints. Implemented fully in Phase 3/5.
func RegisterGameRoutes(rg *gin.RouterGroup, db *sql.DB) {
	// Preferences
	rg.GET("/me/preferences", GetPreferencesHandler(db))
	rg.PUT("/me/preferences", PutPreferencesHandler(db))

	rg.GET("/games/:id", GetGameHandler(db))
	rg.GET("/games/:id/moves", GameMovesHandler(db))
	rg.POST("/games/:id/move", MoveHandler(db))
	rg.POST("/games/:id/quit", QuitGameHandler(db))
	rg.POST("/games/:id/next_hand", NextHandHandler(db))
	rg.POST("/games/:id/count", CountHandler(db))
	rg.POST("/games/:id/correct", CorrectHandler(db))
	rg.GET("/scoreboard", ScoreboardHandler(db))
	rg.GET("/scoreboard/:userId", UserStatsHandler(db))
	rg.GET("/leaderboard", LeaderboardHandler(db))

	// Chatbot for games with bot opponents
	rg.POST("/games/:id/chatbot", ChatbotHandler(db))
}
