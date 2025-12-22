package game

import "sync"

// Registry allows registering game engine factories by type.
type Registry struct {
	mu        sync.RWMutex
	factories map[string]func() Game
}

func NewRegistry() *Registry {
	return &Registry{factories: map[string]func() Game{}}
}

func (r *Registry) Register(gameType string, factory func() Game) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.factories[gameType] = factory
}

func (r *Registry) New(gameType string) (Game, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	f, ok := r.factories[gameType]
	if !ok {
		return nil, false
	}
	return f(), true
}


