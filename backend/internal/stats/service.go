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
	summaries     map[string]PlayerSummary
	leaderboards  map[GameMode]Leaderboard
	lastBuiltAt   time.Time
	dirty         bool
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
func (s *Service) SetClock(now func() time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if now != nil {
		s.now = now
	}
}

// Ingest stores the game and applies it to the rating engine.
func (s *Service) Ingest(g Game) error {
	if g.ID == "" {
		return errors.New("missing game id")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if g.IsRanked() {
		newP, newO := s.engine.Record(g)
		g.RatingBefore = newP - (newP - g.RatingBefore)
		g.RatingAfter = newP
		_ = newO
	}
	if err := s.store.Insert(g); err != nil {
		return err
	}
	s.cache.dirty = true
	return nil
}

// IngestBatch ingests a batch of games. Stops on first error.
func (s *Service) IngestBatch(games []Game) (int, error) {
	for i, g := range games {
		if err := s.Ingest(g); err != nil {
			return i, err
		}
	}
	return len(games), nil
}

// PlayerSummary returns the summary for a single player.
func (s *Service) PlayerSummary(playerID string) PlayerSummary {
	s.mu.RLock()
	defer s.mu.RUnlock()
	games := s.store.ForPlayer(playerID)
	return summarizeGames(playerID, games, s.now())
}

// FilteredSummary returns a summary subject to a filter.
func (s *Service) FilteredSummary(f Filter) PlayerSummary {
	s.mu.RLock()
	defer s.mu.RUnlock()
	games := s.store.Query(f)
	return summarizeGames(f.PlayerID, games, s.now())
}

// Leaderboard returns the leaderboard for a given mode.
func (s *Service) Leaderboard(mode GameMode, opts LeaderboardOptions) (Leaderboard, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ensureCacheLocked()
	if lb, ok := s.cache.leaderboards[mode]; ok && opts.Limit <= 0 && opts.MinGames <= 0 {
		return lb, nil
	}
	opts.Mode = mode
	return BuildLeaderboard(s.cache.summaries, s.engine, opts, s.now())
}

func (s *Service) ensureCacheLocked() {
	if !s.cache.dirty {
		return
	}
	agg := NewAggregatorWithClock(s.now)
	agg.AddMany(s.store.All())
	summaries := agg.AllSummaries()
	s.cache.summaries = summaries
	s.cache.leaderboards = make(map[GameMode]Leaderboard)
	modes := []GameMode{ModeRanked, ModeStandard, ModeBlitz, ModeCasual, ModeTournament}
	for _, m := range modes {
		opts := DefaultLeaderboardOptions()
		opts.Mode = m
		lb, _ := BuildLeaderboard(summaries, s.engine, opts, s.now())
		s.cache.leaderboards[m] = lb
	}
	s.cache.lastBuiltAt = s.now()
	s.cache.dirty = false
}

// Rating returns the current rating for a player.
func (s *Service) Rating(playerID string) int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.engine.Get(playerID)
}

// RatingSeries returns the player's rating progression.
func (s *Service) RatingSeries(playerID string) RatingSeries {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.engine.Series(playerID)
}

// HeadToHead returns the H2H statistics for two players.
func (s *Service) HeadToHead(a, b string) HeadToHead {
	s.mu.RLock()
	defer s.mu.RUnlock()
	games := s.store.All()
	return ComputeHeadToHead(games, a, b)
}

// Snapshot returns a snapshot of stored data.
type Snapshot struct {
	GeneratedAt time.Time
	Games       int
	Players     int
	Ratings     map[string]int
}

// Snapshot returns a quick descriptive snapshot.
func (s *Service) Snapshot() Snapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return Snapshot{
		GeneratedAt: s.now(),
		Games:       s.store.Count(),
		Players:     len(s.store.Players()),
		Ratings:     s.engine.Snapshot(),
	}
}

// GamesPerDay returns a time series of games per day across all players.
func (s *Service) GamesPerDay() Series {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return GamesPerPeriod(s.store.All(), GranularityDay)
}

// WinRateSeries returns a per-day win rate series for a player.
func (s *Service) WinRateSeries(playerID string) Series {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return WinRatePerPeriod(s.store.All(), playerID, GranularityDay)
}

// ScoreDescriptive returns descriptive statistics over all stored games.
func (s *Service) ScoreDescriptive() Summary {
	s.mu.RLock()
	defer s.mu.RUnlock()
	games := s.store.All()
	values := make([]float64, len(games))
	for i, g := range games {
		values[i] = float64(g.PlayerScore)
	}
	return Describe(values)
}

// Invalidate marks the cache dirty.
func (s *Service) Invalidate() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cache.dirty = true
}
