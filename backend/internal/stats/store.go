package stats

import (
	"errors"
	"sort"
	"sync"
	"time"
)

// Store provides thread-safe storage and querying for game records.
type Store struct {
	mu     sync.RWMutex
	games  []Game
	byID   map[string]int
	byPlay map[string][]int
	byMode map[GameMode][]int
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
func (st *Store) Insert(game Game) error {
	if game.ID == "" {
		return errors.New("missing game id")
	}
	st.mu.Lock()
	defer st.mu.Unlock()
	if _, exists := st.byID[game.ID]; exists {
		return errors.New("game already exists")
	}
	gameIdx := len(st.games)
	st.games = append(st.games, game)
	st.byID[game.ID] = gameIdx
	st.byPlay[game.PlayerID] = append(st.byPlay[game.PlayerID], gameIdx)
	if game.OpponentID != "" && game.OpponentID != game.PlayerID {
		st.byPlay[game.OpponentID] = append(st.byPlay[game.OpponentID], gameIdx)
	}
	st.byMode[game.Mode] = append(st.byMode[game.Mode], gameIdx)
	return nil
}

// InsertBatch inserts a slice of games and stops on the first error.
func (st *Store) InsertBatch(games []Game) (int, error) {
	for idx, game := range games {
		if err := st.Insert(game); err != nil {
			return idx, err
		}
	}
	return len(games), nil
}

// Get retrieves a game by ID.
func (st *Store) Get(id string) (Game, bool) {
	st.mu.RLock()
	defer st.mu.RUnlock()
	gameIdx, ok := st.byID[id]
	if !ok {
		return Game{}, false
	}
	return st.games[gameIdx], true
}

// Delete removes a game by ID and rebuilds indexes.
func (st *Store) Delete(id string) bool {
	st.mu.Lock()
	defer st.mu.Unlock()
	gameIdx, ok := st.byID[id]
	if !ok {
		return false
	}
	last := len(st.games) - 1
	if gameIdx != last {
		st.games[gameIdx] = st.games[last]
	}
	st.games = st.games[:last]
	st.rebuildIndexesLocked()
	return true
}

func (st *Store) rebuildIndexesLocked() {
	st.byID = make(map[string]int, len(st.games))
	st.byPlay = make(map[string][]int)
	st.byMode = make(map[GameMode][]int)
	for idx, game := range st.games {
		st.byID[game.ID] = idx
		st.byPlay[game.PlayerID] = append(st.byPlay[game.PlayerID], idx)
		if game.OpponentID != "" && game.OpponentID != game.PlayerID {
			st.byPlay[game.OpponentID] = append(st.byPlay[game.OpponentID], idx)
		}
		st.byMode[game.Mode] = append(st.byMode[game.Mode], idx)
	}
}

// All returns a copy of all games in insertion order.
func (st *Store) All() []Game {
	st.mu.RLock()
	defer st.mu.RUnlock()
	out := make([]Game, len(st.games))
	copy(out, st.games)
	return out
}

// Count returns the number of games stored.
func (st *Store) Count() int {
	st.mu.RLock()
	defer st.mu.RUnlock()
	return len(st.games)
}

// ForPlayer returns the games involving a specific player.
func (st *Store) ForPlayer(playerID string) []Game {
	st.mu.RLock()
	defer st.mu.RUnlock()
	idxs := st.byPlay[playerID]
	out := make([]Game, len(idxs))
	for outIdx, gameIdx := range idxs {
		out[outIdx] = st.games[gameIdx]
	}
	sort.Slice(out, func(ii, jj int) bool { return out[ii].EndedAt.Before(out[jj].EndedAt) })
	return out
}

// ForMode returns games in a specific mode.
func (st *Store) ForMode(mode GameMode) []Game {
	st.mu.RLock()
	defer st.mu.RUnlock()
	idxs := st.byMode[mode]
	out := make([]Game, len(idxs))
	for outIdx, gameIdx := range idxs {
		out[outIdx] = st.games[gameIdx]
	}
	return out
}

// Query returns games that match the supplied filter.
func (st *Store) Query(fl Filter) []Game {
	st.mu.RLock()
	defer st.mu.RUnlock()
	var out []Game
	if fl.PlayerID != "" {
		for _, gameIdx := range st.byPlay[fl.PlayerID] {
			game := st.games[gameIdx]
			if fl.AppliesTo(game) {
				out = append(out, game)
			}
		}
	} else {
		for _, game := range st.games {
			if fl.AppliesTo(game) {
				out = append(out, game)
			}
		}
	}
	sort.Slice(out, func(ii, jj int) bool { return out[ii].EndedAt.Before(out[jj].EndedAt) })
	return out
}

// Players returns all distinct player IDs known to the store.
func (st *Store) Players() []string {
	st.mu.RLock()
	defer st.mu.RUnlock()
	out := make([]string, 0, len(st.byPlay))
	for playerKey := range st.byPlay {
		out = append(out, playerKey)
	}
	sort.Strings(out)
	return out
}

// LastN returns the last n games (most recent first).
func (st *Store) LastN(count int) []Game {
	st.mu.RLock()
	defer st.mu.RUnlock()
	cp := make([]Game, len(st.games))
	copy(cp, st.games)
	sort.Slice(cp, func(ii, jj int) bool { return cp[ii].EndedAt.After(cp[jj].EndedAt) })
	if count > 0 && count < len(cp) {
		cp = cp[:count]
	}
	return cp
}

// Range returns the inclusive [from,to) range of stored games' EndedAt.
func (st *Store) Range() (time.Time, time.Time) {
	st.mu.RLock()
	defer st.mu.RUnlock()
	if len(st.games) == 0 {
		return time.Time{}, time.Time{}
	}
	minTime := st.games[0].EndedAt
	maxTime := st.games[0].EndedAt
	for _, game := range st.games[1:] {
		if game.EndedAt.Before(minTime) {
			minTime = game.EndedAt
		}
		if game.EndedAt.After(maxTime) {
			maxTime = game.EndedAt
		}
	}
	return minTime, maxTime
}

// Clear removes all games from the store.
func (st *Store) Clear() {
	st.mu.Lock()
	defer st.mu.Unlock()
	st.games = nil
	st.byID = make(map[string]int)
	st.byPlay = make(map[string][]int)
	st.byMode = make(map[GameMode][]int)
}
