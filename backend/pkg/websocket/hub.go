package websocket

import (
	"encoding/json"
	"log"
	"sync"
	"sync/atomic"
	"time"
)

const broadcastQueueSize = 1024

// Hub manages websocket clients and room-based broadcasts.
type Hub struct {
	register   chan *Client
	unregister chan *Client
	join       chan joinReq
	broadcast  chan Broadcast

	rooms map[string]map[*Client]bool

	stopOnce sync.Once
	stop     chan struct{}

	droppedBroadcasts atomic.Uint64
}

type joinReq struct {
	Client *Client
	Room   string
}

type Broadcast struct {
	Room    string
	Type    string
	Payload any
}

func NewHub() *Hub {
	return &Hub{
		register:   make(chan *Client),
		unregister: make(chan *Client),
		join:       make(chan joinReq),
		broadcast:  make(chan Broadcast, broadcastQueueSize),
		rooms:      map[string]map[*Client]bool{},
		stop:       make(chan struct{}),
	}
}

func (h *Hub) Run() {
	for {
		select {
		case <-h.stop:
			return
		case c := <-h.register:
			if c.Room == "" {
				c.Room = "lobby:global"
			}
			if h.rooms[c.Room] == nil {
				h.rooms[c.Room] = map[*Client]bool{}
			}
			h.rooms[c.Room][c] = true
		case c := <-h.unregister:
			h.removeClient(c)
		case jr := <-h.join:
			h.moveClientToRoom(jr.Client, jr.Room)
		case b := <-h.broadcast:
			h.broadcastToRoom(b.Room, b.Type, b.Payload)
		}
	}
}

// Register enqueues a client registration.
//
// Behavior when the hub is stopped: this is a no-op and returns immediately.
// This avoids blocking forever after Stop() closes h.stop (since Run() will no
// longer be receiving from h.register).
func (h *Hub) Register(c *Client) {
	select {
	case <-h.stop:
		return
	case h.register <- c:
	}
}

// Unregister enqueues a client unregister.
//
// Behavior when the hub is stopped: this is a no-op and returns immediately.
// This avoids blocking forever after Stop() closes h.stop (since Run() will no
// longer be receiving from h.unregister).
func (h *Hub) Unregister(c *Client) {
	select {
	case <-h.stop:
		return
	case h.unregister <- c:
	}
}

func (h *Hub) Stop() { h.stopOnce.Do(func() { close(h.stop) }) }

// Join enqueues a room move for a client.
//
// Behavior when the hub is stopped: this is a no-op and returns immediately.
// This avoids blocking forever after Stop() closes h.stop (since Run() will no
// longer be receiving from h.join).
func (h *Hub) Join(c *Client, room string) {
	select {
	case <-h.stop:
		return
	case h.join <- joinReq{Client: c, Room: room}:
	}
}

func (h *Hub) Broadcast(room, typ string, payload any) {
	msg := Broadcast{Room: room, Type: typ, Payload: payload}
	select {
	case <-h.stop:
		return
	case h.broadcast <- msg:
		return
	default:
	}
	// Buffer full. Block briefly so we don't silently drop under load, but
	// stay responsive to Stop() and bail if the queue is genuinely saturated.
	select {
	case <-h.stop:
		return
	case h.broadcast <- msg:
	case <-time.After(50 * time.Millisecond):
		n := h.droppedBroadcasts.Add(1)
		// Log only occasionally to avoid flooding.
		if n == 1 || n%100 == 0 {
			log.Printf("ws broadcast dropped: room=%s type=%s total_dropped=%d", room, typ, n)
		}
	}
}

// DroppedBroadcasts returns the cumulative count of broadcasts dropped due to
// a saturated queue. Useful for metrics and tests.
func (h *Hub) DroppedBroadcasts() uint64 { return h.droppedBroadcasts.Load() }

func (h *Hub) removeClient(c *Client) {
	if c == nil {
		return
	}
	if c.Room != "" && h.rooms[c.Room] != nil {
		delete(h.rooms[c.Room], c)
		if len(h.rooms[c.Room]) == 0 {
			delete(h.rooms, c.Room)
		}
	}
	c.SendCloseOnce.Do(func() { close(c.Send) })
}

func (h *Hub) moveClientToRoom(c *Client, room string) {
	if c == nil {
		return
	}
	if room == "" {
		room = "lobby:global"
	}
	// Remove from previous room.
	if c.Room != "" && h.rooms[c.Room] != nil {
		delete(h.rooms[c.Room], c)
		if len(h.rooms[c.Room]) == 0 {
			delete(h.rooms, c.Room)
		}
	}
	c.Room = room
	if h.rooms[room] == nil {
		h.rooms[room] = map[*Client]bool{}
	}
	h.rooms[room][c] = true
}

func (h *Hub) broadcastToRoom(room, typ string, payload any) {
	clients := h.rooms[room]
	if len(clients) == 0 {
		return
	}

	msg := map[string]any{
		"type":      typ,
		"payload":   payload,
		"timestamp": time.Now().UTC().Format(time.RFC3339Nano),
	}
	data, err := json.Marshal(msg)
	if err != nil {
		log.Printf("ws broadcast marshal error: room=%s type=%s err=%v", room, typ, err)
		return
	}

	var deadClients []*Client
	for c := range clients {
		select {
		case c.Send <- data:
		default:
			// Backpressure / dead client.
			deadClients = append(deadClients, c)
		}
	}
	for _, c := range deadClients {
		h.removeClient(c)
	}
}
