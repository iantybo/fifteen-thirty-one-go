package stats

import (
	"math"
	"sort"
	"time"
)

// RatingConfig is a tunable configuration for the Elo-like rating system.
type RatingConfig struct {
	KFactor       float64
	Floor         int
	Ceiling       int
	StartingValue int
	DrawScore     float64
}

// DefaultRatingConfig returns a sensible default.
func DefaultRatingConfig() RatingConfig {
	return RatingConfig{
		KFactor:       32.0,
		Floor:         100,
		Ceiling:       3500,
		StartingValue: 1200,
		DrawScore:     0.5,
	}
}

// Expected returns the expected score for player A vs player B.
func Expected(ratingA, ratingB int) float64 {
	diff := float64(ratingB - ratingA)
	return 1.0 / (1.0 + math.Pow(10.0, diff/400.0))
}

// Update returns the rating change for a player given an outcome.
func Update(cfg RatingConfig, ratingA, ratingB int, result GameResult) int {
	exp := Expected(ratingA, ratingB)
	var actual float64
	switch result {
	case ResultWin:
		actual = 1.0
	case ResultLoss:
		actual = 0.0
	case ResultDraw:
		actual = cfg.DrawScore
	default:
		return 0
	}
	delta := cfg.KFactor * (actual - exp)
	next := ratingA + int(math.Round(delta))
	if cfg.Floor > 0 && next < cfg.Floor {
		next = cfg.Floor
	}
	if cfg.Ceiling > 0 && next > cfg.Ceiling {
		next = cfg.Ceiling
	}
	return next - ratingA
}

// Engine maintains rolling rating state per player.
type Engine struct {
	cfg     RatingConfig
	ratings map[string]int
	history map[string][]RatingPoint
}

// NewEngine builds a new Engine.
func NewEngine(cfg RatingConfig) *Engine {
	return &Engine{
		cfg:     cfg,
		ratings: make(map[string]int),
		history: make(map[string][]RatingPoint),
	}
}

// Get returns the current rating for a player (starting value if unseen).
func (e *Engine) Get(playerID string) int {
	if v, ok := e.ratings[playerID]; ok {
		return v
	}
	return e.cfg.StartingValue
}

// Set sets the rating for a player (clamped to config).
func (e *Engine) Set(playerID string, r int) {
	if e.cfg.Floor > 0 && r < e.cfg.Floor {
		r = e.cfg.Floor
	}
	if e.cfg.Ceiling > 0 && r > e.cfg.Ceiling {
		r = e.cfg.Ceiling
	}
	e.ratings[playerID] = r
}

// Record applies a game to the engine, returning the post-game ratings.
func (e *Engine) Record(g Game) (newPlayer, newOpp int) {
	ra := e.Get(g.PlayerID)
	rb := e.Get(g.OpponentID)
	dA := Update(e.cfg, ra, rb, g.Result)
	var oppResult GameResult
	switch g.Result {
	case ResultWin:
		oppResult = ResultLoss
	case ResultLoss:
		oppResult = ResultWin
	case ResultDraw:
		oppResult = ResultDraw
	default:
		oppResult = ResultUnknown
	}
	dB := Update(e.cfg, rb, ra, oppResult)
	newPlayer = ra + dA
	newOpp = rb + dB
	e.Set(g.PlayerID, newPlayer)
	e.Set(g.OpponentID, newOpp)
	e.history[g.PlayerID] = append(e.history[g.PlayerID], RatingPoint{
		At: g.EndedAt, Rating: newPlayer, GameID: g.ID,
	})
	e.history[g.OpponentID] = append(e.history[g.OpponentID], RatingPoint{
		At: g.EndedAt, Rating: newOpp, GameID: g.ID,
	})
	return
}

// Series returns the rating history for a player.
func (e *Engine) Series(playerID string) RatingSeries {
	pts := e.history[playerID]
	cp := make([]RatingPoint, len(pts))
	copy(cp, pts)
	sort.Slice(cp, func(i, j int) bool { return cp[i].At.Before(cp[j].At) })
	return RatingSeries{PlayerID: playerID, Points: cp}
}

// Snapshot returns all current ratings as a map copy.
func (e *Engine) Snapshot() map[string]int {
	out := make(map[string]int, len(e.ratings))
	for k, v := range e.ratings {
		out[k] = v
	}
	return out
}

// Reset clears all ratings and history.
func (e *Engine) Reset() {
	e.ratings = make(map[string]int)
	e.history = make(map[string][]RatingPoint)
}

// PeakRating returns the maximum rating ever attained for a player.
func (e *Engine) PeakRating(playerID string) int {
	pts := e.history[playerID]
	peak := e.cfg.StartingValue
	for _, p := range pts {
		if p.Rating > peak {
			peak = p.Rating
		}
	}
	return peak
}

// LowRating returns the minimum rating ever attained for a player.
func (e *Engine) LowRating(playerID string) int {
	pts := e.history[playerID]
	if len(pts) == 0 {
		return e.cfg.StartingValue
	}
	low := pts[0].Rating
	for _, p := range pts[1:] {
		if p.Rating < low {
			low = p.Rating
		}
	}
	return low
}

// AverageRatingSince returns a weighted average rating since the given time.
func (e *Engine) AverageRatingSince(playerID string, since time.Time) float64 {
	pts := e.history[playerID]
	var sum float64
	var n int
	for _, p := range pts {
		if !p.At.Before(since) {
			sum += float64(p.Rating)
			n++
		}
	}
	if n == 0 {
		return 0
	}
	return sum / float64(n)
}

// Volatility returns the standard deviation of rating values in window.
func (e *Engine) Volatility(playerID string, window time.Duration, now time.Time) float64 {
	pts := e.history[playerID]
	cutoff := now.Add(-window)
	var values []float64
	for _, p := range pts {
		if !p.At.Before(cutoff) {
			values = append(values, float64(p.Rating))
		}
	}
	return stdDev(values)
}

func stdDev(xs []float64) float64 {
	if len(xs) < 2 {
		return 0
	}
	var sum float64
	for _, v := range xs {
		sum += v
	}
	mean := sum / float64(len(xs))
	var sqSum float64
	for _, v := range xs {
		d := v - mean
		sqSum += d * d
	}
	return math.Sqrt(sqSum / float64(len(xs)-1))
}

// Decay applies a per-day decay to ratings of players whose last activity is older
// than the cutoff. Each decayed player moves toward the starting value.
func (e *Engine) Decay(decayPerDay float64, cutoff time.Time, now time.Time) int {
	if decayPerDay <= 0 {
		return 0
	}
	var changed int
	for player, rating := range e.ratings {
		hist := e.history[player]
		if len(hist) == 0 {
			continue
		}
		last := hist[len(hist)-1].At
		if !last.Before(cutoff) {
			continue
		}
		days := now.Sub(last).Hours() / 24.0
		if days <= 0 {
			continue
		}
		delta := decayPerDay * days
		target := float64(e.cfg.StartingValue)
		newR := float64(rating)
		if newR > target {
			newR -= delta
			if newR < target {
				newR = target
			}
		} else if newR < target {
			newR += delta
			if newR > target {
				newR = target
			}
		}
		e.ratings[player] = int(math.Round(newR))
		changed++
	}
	return changed
}
