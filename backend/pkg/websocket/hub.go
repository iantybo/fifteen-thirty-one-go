package websocket

import (
	"encoding/json"
	"log"
	"sync"
	"time"
)

// Hub manages websocket clients and room-based broadcasts.
type Hub struct {
	register   chan *Client
	unregister chan *Client
	join       chan joinReq
	broadcast  chan Broadcast
	sendToUser chan sendToUserReq

	rooms map[string]map[*Client]bool

	stopOnce sync.Once
	stop     chan struct{}
}

// sendToUserReq sends a message to a single user in a room (e.g. for WebRTC signaling).
type sendToUserReq struct {
	Room     string
	ToUserID int64
	Type     string
	Payload  any
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
		broadcast:  make(chan Broadcast, 256),
		sendToUser: make(chan sendToUserReq, 64),
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
		case s := <-h.sendToUser:
			h.sendToUserInRoom(s.Room, s.ToUserID, s.Type, s.Payload)
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
	select {
	case <-h.stop:
		return
	case h.broadcast <- Broadcast{Room: room, Type: typ, Payload: payload}:
		return
	default:
		// Drop rather than block forever (e.g., if Run() has exited and the
		// channel buffer fills).
		return
	}
}

// SendToUser sends a message to a single user in the room (e.g. WebRTC offer/answer/ICE).
// If that user has no client in the room, the message is dropped.
func (h *Hub) SendToUser(room string, toUserID int64, typ string, payload any) {
	select {
	case <-h.stop:
		return
	case h.sendToUser <- sendToUserReq{Room: room, ToUserID: toUserID, Type: typ, Payload: payload}:
		return
	default:
		return
	}
}

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

func (h *Hub) sendToUserInRoom(room string, toUserID int64, typ string, payload any) {
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
		log.Printf("ws sendToUser marshal error: room=%s type=%s err=%v", room, typ, err)
		return
	}
	var deadClients []*Client
	for c := range clients {
		if c.UserID != toUserID {
			continue
		}
		select {
		case c.Send <- data:
		default:
			deadClients = append(deadClients, c)
		}
		break // at most one client per user per room
	}
	for _, c := range deadClients {
		h.removeClient(c)
	}
}
