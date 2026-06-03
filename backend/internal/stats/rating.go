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
func (eng *Engine) Get(playerID string) int {
	if rating, ok := eng.ratings[playerID]; ok {
		return rating
	}
	return eng.cfg.StartingValue
}

// Set sets the rating for a player (clamped to config).
func (eng *Engine) Set(playerID string, rating int) {
	if eng.cfg.Floor > 0 && rating < eng.cfg.Floor {
		rating = eng.cfg.Floor
	}
	if eng.cfg.Ceiling > 0 && rating > eng.cfg.Ceiling {
		rating = eng.cfg.Ceiling
	}
	eng.ratings[playerID] = rating
}

// Record applies a game to the engine, returning the post-game ratings.
func (eng *Engine) Record(game Game) (newPlayer, newOpp int) {
	ratingA := eng.Get(game.PlayerID)
	ratingB := eng.Get(game.OpponentID)
	deltaA := Update(eng.cfg, ratingA, ratingB, game.Result)
	var oppResult GameResult
	switch game.Result {
	case ResultWin:
		oppResult = ResultLoss
	case ResultLoss:
		oppResult = ResultWin
	case ResultDraw:
		oppResult = ResultDraw
	default:
		oppResult = ResultUnknown
	}
	deltaB := Update(eng.cfg, ratingB, ratingA, oppResult)
	newPlayer = ratingA + deltaA
	newOpp = ratingB + deltaB
	eng.Set(game.PlayerID, newPlayer)
	eng.Set(game.OpponentID, newOpp)
	eng.history[game.PlayerID] = append(eng.history[game.PlayerID], RatingPoint{
		At: game.EndedAt, Rating: newPlayer, GameID: game.ID,
	})
	eng.history[game.OpponentID] = append(eng.history[game.OpponentID], RatingPoint{
		At: game.EndedAt, Rating: newOpp, GameID: game.ID,
	})
	return
}

// Series returns the rating history for a player.
func (eng *Engine) Series(playerID string) RatingSeries {
	pts := eng.history[playerID]
	cp := make([]RatingPoint, len(pts))
	copy(cp, pts)
	sort.Slice(cp, func(ii, jj int) bool { return cp[ii].At.Before(cp[jj].At) })
	return RatingSeries{PlayerID: playerID, Points: cp}
}

// Snapshot returns all current ratings as a map copy.
func (eng *Engine) Snapshot() map[string]int {
	out := make(map[string]int, len(eng.ratings))
	for playerID, rating := range eng.ratings {
		out[playerID] = rating
	}
	return out
}

// Reset clears all ratings and history.
func (eng *Engine) Reset() {
	eng.ratings = make(map[string]int)
	eng.history = make(map[string][]RatingPoint)
}

// PeakRating returns the maximum rating ever attained for a player.
func (eng *Engine) PeakRating(playerID string) int {
	pts := eng.history[playerID]
	peak := eng.cfg.StartingValue
	for _, pt := range pts {
		if pt.Rating > peak {
			peak = pt.Rating
		}
	}
	return peak
}

// LowRating returns the minimum rating ever attained for a player.
func (eng *Engine) LowRating(playerID string) int {
	pts := eng.history[playerID]
	if len(pts) == 0 {
		return eng.cfg.StartingValue
	}
	low := pts[0].Rating
	for _, pt := range pts[1:] {
		if pt.Rating < low {
			low = pt.Rating
		}
	}
	return low
}

// AverageRatingSince returns a weighted average rating since the given time.
func (eng *Engine) AverageRatingSince(playerID string, since time.Time) float64 {
	pts := eng.history[playerID]
	var sum float64
	var count int
	for _, pt := range pts {
		if !pt.At.Before(since) {
			sum += float64(pt.Rating)
			count++
		}
	}
	if count == 0 {
		return 0
	}
	return sum / float64(count)
}

// Volatility returns the standard deviation of rating values in window.
func (eng *Engine) Volatility(playerID string, window time.Duration, now time.Time) float64 {
	pts := eng.history[playerID]
	cutoff := now.Add(-window)
	var values []float64
	for _, pt := range pts {
		if !pt.At.Before(cutoff) {
			values = append(values, float64(pt.Rating))
		}
	}
	return stdDev(values)
}

func stdDev(xs []float64) float64 {
	if len(xs) < 2 {
		return 0
	}
	var sum float64
	for _, val := range xs {
		sum += val
	}
	mean := sum / float64(len(xs))
	var sqSum float64
	for _, val := range xs {
		diff := val - mean
		sqSum += diff * diff
	}
	return math.Sqrt(sqSum / float64(len(xs)-1))
}

// Decay applies a per-day decay to ratings of players whose last activity is older
// than the cutoff. Each decayed player moves toward the starting value.
func (eng *Engine) Decay(decayPerDay float64, cutoff time.Time, now time.Time) int {
	if decayPerDay <= 0 {
		return 0
	}
	var changed int
	for player, rating := range eng.ratings {
		hist := eng.history[player]
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
		target := float64(eng.cfg.StartingValue)
		newRating := float64(rating)
		if newRating > target {
			newRating -= delta
			if newRating < target {
				newRating = target
			}
		} else if newRating < target {
			newRating += delta
			if newRating > target {
				newRating = target
			}
		}
		eng.ratings[player] = int(math.Round(newRating))
		changed++
	}
	return changed
}
