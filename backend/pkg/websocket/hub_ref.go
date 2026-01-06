package websocket

import "sync/atomic"

// HubRef provides an atomic indirection to the currently-active Hub.
// This allows the server to swap in a fresh hub instance after a panic without
// restarting the HTTP server (handlers call Get() for each new connection).
type HubRef struct {
	v atomic.Value // stores *Hub
}

func NewHubRef(initial *Hub) *HubRef {
	r := &HubRef{}
	r.v.Store(initial)
	return r
}

func (r *HubRef) Get() (*Hub, bool) {
	h, ok := r.v.Load().(*Hub)
	return h, ok && h != nil
}

func (r *HubRef) Set(h *Hub) {
	r.v.Store(h)
}
