package websocket

import (
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = (pongWait * 9) / 10
	maxMessageSize = 64 * 1024
)

// Client is a single websocket connection registered to a room.
type Client struct {
	Conn *websocket.Conn
	Hub  *Hub

	Room   string
	UserID int64

	CloseOnce     sync.Once
	SendCloseOnce sync.Once
	Send          chan []byte
}

// NewClient creates a new websocket Client for the given connection, hub, room, and user.
// Returns an error if required parameters are invalid.
func NewClient(conn *websocket.Conn, hub *Hub, room string, userID int64) (*Client, error) {
	if conn == nil {
		return nil, fmt.Errorf("NewClient: conn cannot be nil")
	}
	if hub == nil {
		return nil, fmt.Errorf("NewClient: hub cannot be nil")
	}
	if room == "" {
		return nil, fmt.Errorf("NewClient: room cannot be empty")
	}
	if userID <= 0 {
		return nil, fmt.Errorf("NewClient: userID must be positive")
	}
	return &Client{
		Conn:   conn,
		Hub:    hub,
		Room:   room,
		UserID: userID,
		Send:   make(chan []byte, 256),
	}, nil
}

func (c *Client) Close() {
	c.CloseOnce.Do(func() {
		// Best-effort: Unregister triggers hub-side removal/cleanup (including closing Send via SendCloseOnce).
		// IMPORTANT: do not close Send here while the hub is active; the hub closes it on its own goroutine to
		// avoid send-on-closed panics.
		if c.Hub != nil {
			c.Hub.Unregister(c)
		} else {
			// Fallback: when Hub is nil we must close Send ourselves, otherwise WritePump can leak forever.
			if c.Send != nil {
				c.SendCloseOnce.Do(func() { close(c.Send) })
			}
		}
		if c.Conn != nil {
			_ = c.Conn.Close()
		}
	})
}

func (c *Client) ReadPump(onMessage func([]byte)) {
	defer func() {
		c.Close()
	}()

	c.Conn.SetReadLimit(maxMessageSize)
	_ = c.Conn.SetReadDeadline(time.Now().Add(pongWait))
	c.Conn.SetPongHandler(func(string) error {
		_ = c.Conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		_, message, err := c.Conn.ReadMessage()
		if err != nil {
			// Expected close errors are common.
			return
		}
		if onMessage != nil {
			onMessage(message)
		}
	}
}

func (c *Client) WritePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.Close()
	}()

	for {
		select {
		case msg, ok := <-c.Send:
			_ = c.Conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				_ = c.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.Conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				return
			}
		case <-ticker.C:
			_ = c.Conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				log.Printf("ws ping error: %v", err)
				return
			}
		}
	}
}
