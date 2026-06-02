package stats

import (
	"errors"
	"sort"
	"sync"
	"time"
)

// Store provides thread-safe storage and querying for game records.
type Store struct {
	mu      sync.RWMutex
	games   []Game
	byID    map[string]int
	byPlay  map[string][]int
	byMode  map[GameMode][]int
}

// NewStore returns an empty store.
func NewStore() *Store {
	return &Store{
		byID:   make(map[string]int),
		byPlay: make(map[string][]int),
		byMode: make(map[GameMode][]int),
	}
}

// Insert adds a game to the store. The game ID must be non-empty.
func (s *Store) Insert(g Game) error {
	if g.ID == "" {
		return errors.New("missing game id")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.byID[g.ID]; exists {
		return errors.New("game already exists")
	}
	idx := len(s.games)
	s.games = append(s.games, g)
	s.byID[g.ID] = idx
	s.byPlay[g.PlayerID] = append(s.byPlay[g.PlayerID], idx)
	if g.OpponentID != "" && g.OpponentID != g.PlayerID {
		s.byPlay[g.OpponentID] = append(s.byPlay[g.OpponentID], idx)
	}
	s.byMode[g.Mode] = append(s.byMode[g.Mode], idx)
	return nil
}

// InsertBatch inserts a slice of games and stops on the first error.
func (s *Store) InsertBatch(games []Game) (int, error) {
	for i, g := range games {
		if err := s.Insert(g); err != nil {
			return i, err
		}
	}
	return len(games), nil
}

// Get retrieves a game by ID.
func (s *Store) Get(id string) (Game, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	i, ok := s.byID[id]
	if !ok {
		return Game{}, false
	}
	return s.games[i], true
}

// Delete removes a game by ID and rebuilds indexes.
func (s *Store) Delete(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	idx, ok := s.byID[id]
	if !ok {
		return false
	}
	last := len(s.games) - 1
	if idx != last {
		s.games[idx] = s.games[last]
	}
	s.games = s.games[:last]
	s.rebuildIndexesLocked()
	return true
}

func (s *Store) rebuildIndexesLocked() {
	s.byID = make(map[string]int, len(s.games))
	s.byPlay = make(map[string][]int)
	s.byMode = make(map[GameMode][]int)
	for i, g := range s.games {
		s.byID[g.ID] = i
		s.byPlay[g.PlayerID] = append(s.byPlay[g.PlayerID], i)
		if g.OpponentID != "" && g.OpponentID != g.PlayerID {
			s.byPlay[g.OpponentID] = append(s.byPlay[g.OpponentID], i)
		}
		s.byMode[g.Mode] = append(s.byMode[g.Mode], i)
	}
}

// All returns a copy of all games in insertion order.
func (s *Store) All() []Game {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Game, len(s.games))
	copy(out, s.games)
	return out
}

// Count returns the number of games stored.
func (s *Store) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.games)
}

// ForPlayer returns the games involving a specific player.
func (s *Store) ForPlayer(playerID string) []Game {
	s.mu.RLock()
	defer s.mu.RUnlock()
	idxs := s.byPlay[playerID]
	out := make([]Game, len(idxs))
	for i, j := range idxs {
		out[i] = s.games[j]
	}
	sort.Slice(out, func(i, j int) bool { return out[i].EndedAt.Before(out[j].EndedAt) })
	return out
}

// ForMode returns games in a specific mode.
func (s *Store) ForMode(m GameMode) []Game {
	s.mu.RLock()
	defer s.mu.RUnlock()
	idxs := s.byMode[m]
	out := make([]Game, len(idxs))
	for i, j := range idxs {
		out[i] = s.games[j]
	}
	return out
}

// Query returns games that match the supplied filter.
func (s *Store) Query(f Filter) []Game {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []Game
	if f.PlayerID != "" {
		for _, i := range s.byPlay[f.PlayerID] {
			g := s.games[i]
			if f.AppliesTo(g) {
				out = append(out, g)
			}
		}
	} else {
		for _, g := range s.games {
			if f.AppliesTo(g) {
				out = append(out, g)
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].EndedAt.Before(out[j].EndedAt) })
	return out
}

// Players returns all distinct player IDs known to the store.
func (s *Store) Players() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]string, 0, len(s.byPlay))
	for k := range s.byPlay {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// LastN returns the last n games (most recent first).
func (s *Store) LastN(n int) []Game {
	s.mu.RLock()
	defer s.mu.RUnlock()
	cp := make([]Game, len(s.games))
	copy(cp, s.games)
	sort.Slice(cp, func(i, j int) bool { return cp[i].EndedAt.After(cp[j].EndedAt) })
	if n > 0 && n < len(cp) {
		cp = cp[:n]
	}
	return cp
}

// Range returns the inclusive [from,to) range of stored games' EndedAt.
func (s *Store) Range() (time.Time, time.Time) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if len(s.games) == 0 {
		return time.Time{}, time.Time{}
	}
	mn := s.games[0].EndedAt
	mx := s.games[0].EndedAt
	for _, g := range s.games[1:] {
		if g.EndedAt.Before(mn) {
			mn = g.EndedAt
		}
		if g.EndedAt.After(mx) {
			mx = g.EndedAt
		}
	}
	return mn, mx
}

// Clear removes all games from the store.
func (s *Store) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.games = nil
	s.byID = make(map[string]int)
	s.byPlay = make(map[string][]int)
	s.byMode = make(map[GameMode][]int)
}
