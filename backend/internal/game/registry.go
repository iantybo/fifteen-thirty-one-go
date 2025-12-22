package game

import (
	"fmt"
	"sync"
)

// Registry allows registering game engine factories by type.
type Registry struct {
	mu        sync.RWMutex
	factories map[string]func() Game
}

func NewRegistry() *Registry {
	return &Registry{factories: map[string]func() Game{}}
}

func (r *Registry) Register(gameType string, factory func() Game) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if factory == nil {
		return fmt.Errorf("nil factory for gameType %q", gameType)
	}
	if _, exists := r.factories[gameType]; exists {
		return fmt.Errorf("duplicate registration for gameType %q", gameType)
	}
	r.factories[gameType] = factory
	return nil
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


