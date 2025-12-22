package websocket

import (
	"encoding/json"
	"log"
	"time"
)

// Hub manages websocket clients and room-based broadcasts.
type Hub struct {
	register   chan *Client
	unregister chan *Client
	join       chan joinReq
	broadcast  chan Broadcast

	rooms map[string]map[*Client]bool
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
		rooms:      map[string]map[*Client]bool{},
	}
}

func (h *Hub) Run() {
	for {
		select {
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

func (h *Hub) Register(c *Client)  { h.register <- c }
func (h *Hub) Unregister(c *Client) { h.unregister <- c }

func (h *Hub) Join(c *Client, room string) {
	h.join <- joinReq{Client: c, Room: room}
}

func (h *Hub) Broadcast(room, typ string, payload any) {
	h.broadcast <- Broadcast{Room: room, Type: typ, Payload: payload}
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

	for c := range clients {
		select {
		case c.Send <- data:
		default:
			// Backpressure / dead client.
			h.removeClient(c)
		}
	}
}


