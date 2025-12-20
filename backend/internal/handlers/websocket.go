package handlers

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	"fifteen-thirty-one-go/backend/internal/auth"
	ws "fifteen-thirty-one-go/backend/pkg/websocket"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		origin := strings.TrimSpace(r.Header.Get("Origin"))
		// Allow non-browser clients (no Origin) only in dev.
		if cfgIsDev() {
			return true
		}
		if origin == "" {
			return false
		}
		return isAllowedOrigin(origin)
	},
}

// set by config at startup
var allowedOrigins = map[string]bool{}
var devMode = false

func SetWebSocketOriginPolicy(isDev bool, origins []string) {
	devMode = isDev
	allowedOrigins = map[string]bool{}
	for _, o := range origins {
		o = strings.TrimSpace(o)
		if o != "" {
			allowedOrigins[o] = true
		}
	}
}

func cfgIsDev() bool { return devMode }
func isAllowedOrigin(origin string) bool { return allowedOrigins[origin] }

// WebSocketHandler upgrades the connection and registers the client.
// Full message routing is implemented in Phase 4.
func WebSocketHandler(hub *ws.Hub, db *sql.DB, cfg Config) gin.HandlerFunc {
	_ = db
	return func(c *gin.Context) {
		token := tokenFromHeaderOrQuery(c)
		if token == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing token"})
			return
		}
		claims, err := auth.ParseAndValidateToken(token, cfg)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
			return
		}

		conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			return
		}

		room := strings.TrimSpace(c.Query("room"))
		if room == "" {
			room = "lobby:global"
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
		hub.Join(client, strings.TrimSpace(p.Room))
		_ = sendDirect(client, "joined_room", map[string]any{"room": p.Room})
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
			_ = sendDirect(client, "error", map[string]any{"error": err.Error()})
			return
		}
		_ = sendDirect(client, "move_ok", resp)

		// Broadcast updated snapshot to the game room.
		snap, err := BuildGameSnapshotPublic(db, p.GameID)
		if err == nil {
			hub.Broadcast("game:"+intToString(p.GameID), "game_update", snap)
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

func intToString(v int64) string {
	// avoid fmt for hot path
	if v == 0 {
		return "0"
	}
	neg := v < 0
	if neg {
		v = -v
	}
	buf := make([]byte, 0, 20)
	for v > 0 {
		d := byte(v % 10)
		buf = append(buf, '0'+d)
		v /= 10
	}
	if neg {
		buf = append(buf, '-')
	}
	// reverse
	for i, j := 0, len(buf)-1; i < j; i, j = i+1, j-1 {
		buf[i], buf[j] = buf[j], buf[i]
	}
	return string(buf)
}

func tokenFromHeaderOrQuery(c *gin.Context) string {
	authz := c.GetHeader("Authorization")
	if authz != "" {
		parts := strings.SplitN(authz, " ", 2)
		if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
			return strings.TrimSpace(parts[1])
		}
	}
	return strings.TrimSpace(c.Query("token"))
}


