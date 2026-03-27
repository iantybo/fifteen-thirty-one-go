package handlers

import (
	"database/sql"
	"time"

	"fifteen-thirty-one-go/backend/internal/config"
	"fifteen-thirty-one-go/backend/internal/middleware"
	ws "fifteen-thirty-one-go/backend/pkg/websocket"
	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

// RegisterAuthRoutes wires auth endpoints. Implemented fully in Phase 1.2.
func RegisterAuthRoutes(rg *gin.RouterGroup, db *sql.DB, cfg config.Config) {
	// Per-IP rate limits for wallet auth (challenge is cheaper than verify/signature recovery).
	walletChallengeRL := middleware.NewIPRateLimiter(rate.Every(time.Minute/60), 60)
	walletVerifyRL := middleware.NewIPRateLimiter(rate.Every(time.Minute/20), 20)

	rg.POST("/auth/register", RegisterHandler(db, cfg))
	rg.POST("/auth/login", LoginHandler(db, cfg))
	rg.POST("/auth/wallet/challenge", walletChallengeRL.GinHandler(), WalletChallengeHandler(cfg))
	rg.POST("/auth/wallet/verify", walletVerifyRL.GinHandler(), WalletVerifyHandler(db, cfg))
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
}
