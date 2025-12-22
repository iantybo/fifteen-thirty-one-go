package handlers

import (
	"sync"

	"fifteen-thirty-one-go/backend/internal/game/cribbage"
)

type gameEntry struct {
	mu    sync.Mutex
	state *cribbage.State
}

type GameManager struct {
	mu    sync.RWMutex
	games map[int64]*gameEntry
}

func NewGameManager() *GameManager {
	return &GameManager{games: map[int64]*gameEntry{}}
}

func (m *GameManager) GetLocked(gameID int64) (*cribbage.State, func(), bool) {
	m.mu.RLock()
	e, ok := m.games[gameID]
	m.mu.RUnlock()
	if !ok || e == nil || e.state == nil {
		return nil, nil, false
	}
	e.mu.Lock()
	return e.state, func() { e.mu.Unlock() }, true
}

func (m *GameManager) Set(gameID int64, st *cribbage.State) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.games[gameID] = &gameEntry{state: st}
}

func (m *GameManager) Delete(gameID int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.games, gameID)
}

var defaultGameManager = NewGameManager()


