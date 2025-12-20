package handlers

import (
	"sync"

	"fifteen-thirty-one-go/backend/internal/game/cribbage"
)

type GameManager struct {
	mu    sync.RWMutex
	games map[int64]*cribbage.State
}

func NewGameManager() *GameManager {
	return &GameManager{games: map[int64]*cribbage.State{}}
}

func (m *GameManager) Get(gameID int64) (*cribbage.State, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	g, ok := m.games[gameID]
	return g, ok
}

func (m *GameManager) Set(gameID int64, st *cribbage.State) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.games[gameID] = st
}

var defaultGameManager = NewGameManager()


