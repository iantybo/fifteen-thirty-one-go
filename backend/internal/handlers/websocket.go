package handlers

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"fifteen-thirty-one-go/backend/internal/auth"
	"fifteen-thirty-one-go/backend/internal/config"
	ws "fifteen-thirty-one-go/backend/pkg/websocket"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		origin := strings.TrimSpace(r.Header.Get("Origin"))
		if origin == "" {
			// Non-browser clients (no Origin) are allowed.
			return true
		}
		if cfgDevAllowAll() {
			return true
		}
		if cfgIsDev() {
			return isLocalhostOrigin(origin) || isAllowedOrigin(origin)
		}
		return isAllowedOrigin(origin)
	},
}

// set by config at startup
var originMu sync.RWMutex
var allowedOrigins = map[string]bool{}
var devMode = false
var devAllowAll = false

func SetWebSocketOriginPolicy(isDev bool, allowAllDev bool, origins []string) {
	originMu.Lock()
	defer originMu.Unlock()
	devMode = isDev
	devAllowAll = allowAllDev
	allowedOrigins = map[string]bool{}
	for _, o := range origins {
		o = strings.TrimSpace(o)
		if o != "" {
			allowedOrigins[o] = true
		}
	}
}

func cfgIsDev() bool {
	originMu.RLock()
	defer originMu.RUnlock()
	return devMode
}
func cfgDevAllowAll() bool {
	originMu.RLock()
	defer originMu.RUnlock()
	return devMode && devAllowAll
}
func isAllowedOrigin(origin string) bool {
	originMu.RLock()
	defer originMu.RUnlock()
	return allowedOrigins[origin]
}

func isLocalhostOrigin(origin string) bool {
	u, err := url.Parse(origin)
	if err != nil {
		return false
	}
	host := u.Hostname()
	return host == "localhost" || host == "127.0.0.1" || host == "::1"
}

// WebSocketHandler upgrades the connection and registers the client.
// Full message routing is implemented in Phase 4.
func WebSocketHandler(hubProvider func() (*ws.Hub, bool), db *sql.DB, cfg config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		token := tokenFromHeaderOrQuery(c, cfg)
		if token == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing token"})
			return
		}
		claims, err := auth.ParseAndValidateToken(token, cfg)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
			return
		}

		// Preconditions before attempting the upgrade so we can return HTTP errors normally.
		room := strings.TrimSpace(c.Query("room"))
		if room == "" {
			room = "lobby:global"
		}
		hub, ok := hubProvider()
		if !ok || hub == nil {
			// Should never happen; treat as an internal error (still HTTP at this point).
			log.Printf("WebSocketHandler hubProvider returned nil: user_id=%d room=%q", claims.UserID, room)
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
			return
		}

		conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			log.Printf("WebSocketHandler upgrade failed: method=%s path=%s remote=%s origin=%q err=%v",
				c.Request.Method, c.Request.URL.Path, c.ClientIP(), c.Request.Header.Get("Origin"), err,
			)
			return
		}

		// Defensive: hub should never be nil here, but if it is, close the WS and return.
		if hub == nil {
			log.Printf(
				"WebSocketHandler unexpected nil hub after upgrade (closing ws): path=%s remote=%s user_id=%d room=%q",
				c.Request.URL.Path, c.Request.RemoteAddr, claims.UserID, room,
			)
			// Best-effort: send a close control message so the peer sees a clean disconnect.
			if err := conn.WriteControl(
				websocket.CloseMessage,
				websocket.FormatCloseMessage(websocket.CloseInternalServerErr, "internal error"),
				time.Now().Add(1*time.Second),
			); err != nil {
				log.Printf(
					"WebSocketHandler close control write failed: path=%s remote=%s user_id=%d room=%q err=%v",
					c.Request.URL.Path, c.Request.RemoteAddr, claims.UserID, room, err,
				)
			}
			if err := conn.Close(); err != nil {
				log.Printf(
					"WebSocketHandler conn.Close failed: path=%s remote=%s user_id=%d room=%q err=%v",
					c.Request.URL.Path, c.Request.RemoteAddr, claims.UserID, room, err,
				)
			}
			return
		}
		client := ws.NewClient(conn, hub, room, claims.UserID)
		hub.Register(client)

		go client.WritePump()
		go client.ReadPump(func(msg []byte) {
			handleWSMessage(hub, client, db, msg)
		})

		// Send a direct "connected" ack.
		_ = sendDirect(client, "connected", map[string]any{
			"user_id": client.UserID,
			"room":    room,
		})
	}
}

type inboundMessage struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

func handleWSMessage(hub *ws.Hub, client *ws.Client, db *sql.DB, msg []byte) {
	var in inboundMessage
	if err := json.Unmarshal(msg, &in); err != nil {
		_ = sendDirect(client, "error", map[string]any{"error": "invalid json"})
		return
	}

	switch in.Type {
	case "join_room":
		var p struct {
			Room string `json:"room"`
		}
		if err := json.Unmarshal(in.Payload, &p); err != nil || strings.TrimSpace(p.Room) == "" {
			_ = sendDirect(client, "error", map[string]any{"error": "invalid room"})
			return
		}
		room := strings.TrimSpace(p.Room)
		hub.Join(client, room)
		_ = sendDirect(client, "joined_room", map[string]any{"room": room})
	case "move":
		var p struct {
			GameID int64      `json:"game_id"`
			Move   moveRequest `json:"move"`
		}
		if err := json.Unmarshal(in.Payload, &p); err != nil || p.GameID <= 0 {
			_ = sendDirect(client, "error", map[string]any{"error": "invalid move payload"})
			return
		}
		resp, err := ApplyMove(db, p.GameID, client.UserID, p.Move)
		if err != nil {
			// Avoid leaking internal details; ApplyMove errors are mapped in HTTP handlers only.
			_ = sendDirect(client, "error", map[string]any{"error": "invalid move"})
			return
		}
		_ = sendDirect(client, "move_ok", resp)

		// Broadcast updated snapshot to the game room.
		snap, err := BuildGameSnapshotPublic(db, p.GameID)
		if err == nil {
			hub.Broadcast("game:"+strconv.FormatInt(p.GameID, 10), "game_update", snap)
		} else {
			log.Printf("BuildGameSnapshotPublic failed: game_id=%d err=%v", p.GameID, err)
		}
	default:
		_ = sendDirect(client, "error", map[string]any{"error": "unknown message type"})
	}
}

func sendDirect(c *ws.Client, typ string, payload any) error {
	msg := map[string]any{
		"type":      typ,
		"payload":   payload,
		"timestamp": time.Now().UTC().Format(time.RFC3339Nano),
	}
	b, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	select {
	case c.Send <- b:
	default:
		log.Printf("ws send drop: user_id=%d room=%s type=%s", c.UserID, c.Room, typ)
	}
	return nil
}

func tokenFromHeaderOrQuery(c *gin.Context, cfg config.Config) string {
	authz := c.GetHeader("Authorization")
	if authz != "" {
		parts := strings.SplitN(authz, " ", 2)
		if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
			return strings.TrimSpace(parts[1])
		}
	}
	if cfg.WSAllowQueryTokens {
		return strings.TrimSpace(c.Query("token"))
	}
	return ""
}


