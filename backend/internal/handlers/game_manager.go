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
	if !ok || e == nil {
		return nil, nil, false
	}
	e.mu.Lock()
	if e.state == nil {
		e.mu.Unlock()
		return nil, nil, false
	}
	return e.state, func() { e.mu.Unlock() }, true
}

func (m *GameManager) Set(gameID int64, st *cribbage.State) {
	m.mu.Lock()
	e, ok := m.games[gameID]
	if !ok || e == nil {
		e = &gameEntry{}
		m.games[gameID] = e
	}
	e.mu.Lock()
	m.mu.Unlock()
	e.state = st
	e.mu.Unlock()
}

func (m *GameManager) Delete(gameID int64) {
	m.mu.Lock()
	e, ok := m.games[gameID]
	if !ok || e == nil {
		m.mu.Unlock()
		return
	}
	e.mu.Lock()
	// While holding both locks, make the entry inert and remove it from the map.
	e.state = nil
	delete(m.games, gameID)
	e.mu.Unlock()
	m.mu.Unlock()
}

func (m *GameManager) GetOrCreateLocked(gameID int64, createFn func() (*cribbage.State, error)) (*cribbage.State, func(), error) {
	m.mu.Lock()
	e, ok := m.games[gameID]
	if !ok || e == nil {
		e = &gameEntry{}
		m.games[gameID] = e
	}
	e.mu.Lock()
	m.mu.Unlock()

	defer func() {
		if r := recover(); r != nil {
			// Clean up on panic so future calls don't deadlock.
			m.mu.Lock()
			if m.games[gameID] == e {
				delete(m.games, gameID)
			}
			m.mu.Unlock()
			e.mu.Unlock()
			panic(r)
		}
	}()

	if e.state == nil {
		st, err := createFn()
		if err != nil {
			// Remove placeholder entry on failure so future attempts can retry.
			m.mu.Lock()
			if m.games[gameID] == e {
				delete(m.games, gameID)
			}
			m.mu.Unlock()
			e.mu.Unlock()
			return nil, nil, err
		}
		e.state = st
	}
	return e.state, func() { e.mu.Unlock() }, nil
}

var defaultGameManager = NewGameManager()


