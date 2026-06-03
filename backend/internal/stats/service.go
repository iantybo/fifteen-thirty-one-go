package stats

import (
	"errors"
	"sync"
	"time"
)

// Service is a high-level facade that combines storage, aggregation, and rating.
type Service struct {
	mu     sync.RWMutex
	store  *Store
	engine *Engine
	now    func() time.Time
	cache  *cacheState
}

type cacheState struct {
	summaries    map[string]PlayerSummary
	leaderboards map[GameMode]Leaderboard
	lastBuiltAt  time.Time
	dirty        bool
}

// NewService constructs a Service with default rating configuration.
func NewService() *Service {
	return &Service{
		store:  NewStore(),
		engine: NewEngine(DefaultRatingConfig()),
		now:    time.Now,
		cache:  &cacheState{dirty: true},
	}
}

// NewServiceWithEngine builds a service backed by a custom rating engine.
func NewServiceWithEngine(eng *Engine) *Service {
	if eng == nil {
		eng = NewEngine(DefaultRatingConfig())
	}
	return &Service{
		store:  NewStore(),
		engine: eng,
		now:    time.Now,
		cache:  &cacheState{dirty: true},
	}
}

// SetClock allows overriding the clock for tests.
func (svc *Service) SetClock(now func() time.Time) {
	svc.mu.Lock()
	defer svc.mu.Unlock()
	if now != nil {
		svc.now = now
	}
}

// Ingest stores the game and applies it to the rating engine.
func (svc *Service) Ingest(game Game) error {
	if game.ID == "" {
		return errors.New("missing game id")
	}
	svc.mu.Lock()
	defer svc.mu.Unlock()
	if game.IsRanked() {
		newP, newO := svc.engine.Record(game)
		game.RatingBefore = newP - (newP - game.RatingBefore)
		game.RatingAfter = newP
		_ = newO
	}
	if err := svc.store.Insert(game); err != nil {
		return err
	}
	svc.cache.dirty = true
	return nil
}

// IngestBatch ingests a batch of games. Stops on first error.
func (svc *Service) IngestBatch(games []Game) (int, error) {
	for idx, game := range games {
		if err := svc.Ingest(game); err != nil {
			return idx, err
		}
	}
	return len(games), nil
}

// PlayerSummary returns the summary for a single player.
func (svc *Service) PlayerSummary(playerID string) PlayerSummary {
	svc.mu.RLock()
	defer svc.mu.RUnlock()
	games := svc.store.ForPlayer(playerID)
	return summarizeGames(playerID, games, svc.now())
}

// FilteredSummary returns a summary subject to a filter.
func (svc *Service) FilteredSummary(fl Filter) PlayerSummary {
	svc.mu.RLock()
	defer svc.mu.RUnlock()
	games := svc.store.Query(fl)
	return summarizeGames(fl.PlayerID, games, svc.now())
}

// Leaderboard returns the leaderboard for a given mode.
func (svc *Service) Leaderboard(mode GameMode, opts LeaderboardOptions) (Leaderboard, error) {
	svc.mu.Lock()
	defer svc.mu.Unlock()
	svc.ensureCacheLocked()
	if lb, ok := svc.cache.leaderboards[mode]; ok && opts.Limit <= 0 && opts.MinGames <= 0 {
		return lb, nil
	}
	opts.Mode = mode
	return BuildLeaderboard(svc.cache.summaries, svc.engine, opts, svc.now())
}

func (svc *Service) ensureCacheLocked() {
	if !svc.cache.dirty {
		return
	}
	agg := NewAggregatorWithClock(svc.now)
	agg.AddMany(svc.store.All())
	summaries := agg.AllSummaries()
	svc.cache.summaries = summaries
	svc.cache.leaderboards = make(map[GameMode]Leaderboard)
	modes := []GameMode{ModeRanked, ModeStandard, ModeBlitz, ModeCasual, ModeTournament}
	for _, mode := range modes {
		opts := DefaultLeaderboardOptions()
		opts.Mode = mode
		lb, _ := BuildLeaderboard(summaries, svc.engine, opts, svc.now())
		svc.cache.leaderboards[mode] = lb
	}
	svc.cache.lastBuiltAt = svc.now()
	svc.cache.dirty = false
}

// Rating returns the current rating for a player.
func (svc *Service) Rating(playerID string) int {
	svc.mu.RLock()
	defer svc.mu.RUnlock()
	return svc.engine.Get(playerID)
}

// RatingSeries returns the player's rating progression.
func (svc *Service) RatingSeries(playerID string) RatingSeries {
	svc.mu.RLock()
	defer svc.mu.RUnlock()
	return svc.engine.Series(playerID)
}

// HeadToHead returns the H2H statistics for two players.
func (svc *Service) HeadToHead(playerA, playerB string) HeadToHead {
	svc.mu.RLock()
	defer svc.mu.RUnlock()
	games := svc.store.All()
	return ComputeHeadToHead(games, playerA, playerB)
}

// Snapshot returns a snapshot of stored data.
type Snapshot struct {
	GeneratedAt time.Time
	Games       int
	Players     int
	Ratings     map[string]int
}

// Snapshot returns a quick descriptive snapshot.
func (svc *Service) Snapshot() Snapshot {
	svc.mu.RLock()
	defer svc.mu.RUnlock()
	return Snapshot{
		GeneratedAt: svc.now(),
		Games:       svc.store.Count(),
		Players:     len(svc.store.Players()),
		Ratings:     svc.engine.Snapshot(),
	}
}

// GamesPerDay returns a time series of games per day across all players.
func (svc *Service) GamesPerDay() Series {
	svc.mu.RLock()
	defer svc.mu.RUnlock()
	return GamesPerPeriod(svc.store.All(), GranularityDay)
}

// WinRateSeries returns a per-day win rate series for a player.
func (svc *Service) WinRateSeries(playerID string) Series {
	svc.mu.RLock()
	defer svc.mu.RUnlock()
	return WinRatePerPeriod(svc.store.All(), playerID, GranularityDay)
}

// ScoreDescriptive returns descriptive statistics over all stored games.
func (svc *Service) ScoreDescriptive() Summary {
	svc.mu.RLock()
	defer svc.mu.RUnlock()
	games := svc.store.All()
	values := make([]float64, len(games))
	for idx, game := range games {
		values[idx] = float64(game.PlayerScore)
	}
	return Describe(values)
}

// Invalidate marks the cache dirty.
func (svc *Service) Invalidate() {
	svc.mu.Lock()
	defer svc.mu.Unlock()
	svc.cache.dirty = true
}
