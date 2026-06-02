package stats

import (
	"sort"
	"time"
)

// Aggregator builds player summaries from a stream of game records.
type Aggregator struct {
	now     func() time.Time
	games   []Game
	indexed bool
	byPlay  map[string][]int
}

// NewAggregator constructs a new aggregator with the system clock.
func NewAggregator() *Aggregator {
	return &Aggregator{now: time.Now}
}

// NewAggregatorWithClock constructs an aggregator with a custom clock.
// Useful for tests.
func NewAggregatorWithClock(now func() time.Time) *Aggregator {
	if now == nil {
		now = time.Now
	}
	return &Aggregator{now: now}
}

// Add appends a single game.
func (a *Aggregator) Add(g Game) {
	a.games = append(a.games, g)
	a.indexed = false
}

// AddMany appends a batch of games.
func (a *Aggregator) AddMany(gs []Game) {
	a.games = append(a.games, gs...)
	a.indexed = false
}

// Len reports the number of games tracked.
func (a *Aggregator) Len() int {
	return len(a.games)
}

// Reset removes all tracked games.
func (a *Aggregator) Reset() {
	a.games = a.games[:0]
	a.byPlay = nil
	a.indexed = false
}

func (a *Aggregator) ensureIndex() {
	if a.indexed {
		return
	}
	a.byPlay = make(map[string][]int, len(a.games)/2+1)
	for i, g := range a.games {
		a.byPlay[g.PlayerID] = append(a.byPlay[g.PlayerID], i)
	}
	a.indexed = true
}

// Players returns the list of unique player IDs.
func (a *Aggregator) Players() []string {
	a.ensureIndex()
	out := make([]string, 0, len(a.byPlay))
	for k := range a.byPlay {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// GamesFor returns the games associated with a player.
func (a *Aggregator) GamesFor(playerID string) []Game {
	a.ensureIndex()
	idx := a.byPlay[playerID]
	out := make([]Game, len(idx))
	for i, j := range idx {
		out[i] = a.games[j]
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].EndedAt.Before(out[j].EndedAt)
	})
	return out
}

// Summarize produces the aggregate statistics for one player.
func (a *Aggregator) Summarize(playerID string) PlayerSummary {
	games := a.GamesFor(playerID)
	return summarizeGames(playerID, games, a.now())
}

// SummarizeFiltered produces a summary applying a filter.
func (a *Aggregator) SummarizeFiltered(f Filter) PlayerSummary {
	a.ensureIndex()
	var games []Game
	if f.PlayerID != "" {
		for _, i := range a.byPlay[f.PlayerID] {
			g := a.games[i]
			if f.AppliesTo(g) {
				games = append(games, g)
			}
		}
	} else {
		for _, g := range a.games {
			if f.AppliesTo(g) {
				games = append(games, g)
			}
		}
	}
	sort.Slice(games, func(i, j int) bool {
		return games[i].EndedAt.Before(games[j].EndedAt)
	})
	return summarizeGames(f.PlayerID, games, a.now())
}

func summarizeGames(playerID string, games []Game, now time.Time) PlayerSummary {
	s := PlayerSummary{
		PlayerID:      playerID,
		ModeBreakdown: make(map[GameMode]ModeStats),
	}
	if len(games) == 0 {
		return s
	}

	var totalScore, totalOpp, totalMoves int
	var totalDur time.Duration
	s.PeakRating = games[0].RatingAfter
	s.LowestRating = games[0].RatingAfter
	s.FirstPlayed = games[0].EndedAt
	s.LastPlayed = games[len(games)-1].EndedAt
	s.CurrentRating = games[len(games)-1].RatingAfter

	curStreakResult := games[0].Result
	curStreakLen := 0
	curStreakStart := games[0].EndedAt
	longestWin := 0
	longestLoss := 0
	tmpWin := 0
	tmpLoss := 0

	thirtyDaysAgo := now.Add(-30 * 24 * time.Hour)
	var rating30Start, rating30End int
	rating30Init := false

	for _, g := range games {
		s.Games++
		switch g.Result {
		case ResultWin:
			s.Wins++
			tmpWin++
			tmpLoss = 0
		case ResultLoss:
			s.Losses++
			tmpLoss++
			tmpWin = 0
		case ResultDraw:
			s.Draws++
			tmpWin = 0
			tmpLoss = 0
		}
		if tmpWin > longestWin {
			longestWin = tmpWin
		}
		if tmpLoss > longestLoss {
			longestLoss = tmpLoss
		}

		if g.Result == curStreakResult {
			curStreakLen++
		} else {
			curStreakResult = g.Result
			curStreakLen = 1
			curStreakStart = g.EndedAt
		}

		totalScore += g.PlayerScore
		totalOpp += g.OppScore
		totalMoves += g.Moves
		totalDur += g.Duration()

		if g.RatingAfter > s.PeakRating {
			s.PeakRating = g.RatingAfter
		}
		if g.RatingAfter < s.LowestRating || s.LowestRating == 0 {
			s.LowestRating = g.RatingAfter
		}

		ms := s.ModeBreakdown[g.Mode]
		ms.Mode = g.Mode
		ms.Games++
		switch g.Result {
		case ResultWin:
			ms.Wins++
		case ResultLoss:
			ms.Losses++
		case ResultDraw:
			ms.Draws++
		}
		ms.AvgScore += float64(g.PlayerScore)
		s.ModeBreakdown[g.Mode] = ms

		if !g.EndedAt.Before(thirtyDaysAgo) {
			if !rating30Init {
				rating30Start = g.RatingBefore
				rating30Init = true
			}
			rating30End = g.RatingAfter
		}
	}

	s.WinRate = ratio(s.Wins, s.Games)
	s.AvgScore = float64(totalScore) / float64(s.Games)
	s.AvgOppScore = float64(totalOpp) / float64(s.Games)
	s.AvgScoreDelta = s.AvgScore - s.AvgOppScore
	s.AvgMoves = float64(totalMoves) / float64(s.Games)
	s.TotalPlayTime = totalDur
	s.AvgGameDuration = time.Duration(int64(totalDur) / int64(s.Games))
	s.LongestWinStreak = longestWin
	s.LongestLossStreak = longestLoss
	s.CurrentStreak = Streak{
		Result: curStreakResult,
		Length: curStreakLen,
		Since:  curStreakStart,
	}
	if rating30Init {
		s.RatingDelta30Day = rating30End - rating30Start
	}

	for k, v := range s.ModeBreakdown {
		v.WinRate = ratio(v.Wins, v.Games)
		if v.Games > 0 {
			v.AvgScore = v.AvgScore / float64(v.Games)
		}
		s.ModeBreakdown[k] = v
	}

	return s
}

func ratio(num, denom int) float64 {
	if denom <= 0 {
		return 0
	}
	return float64(num) / float64(denom)
}

// AllSummaries returns a summary for every known player.
func (a *Aggregator) AllSummaries() map[string]PlayerSummary {
	a.ensureIndex()
	out := make(map[string]PlayerSummary, len(a.byPlay))
	now := a.now()
	for p := range a.byPlay {
		games := a.GamesFor(p)
		out[p] = summarizeGames(p, games, now)
	}
	return out
}

// PlayerCount returns the count of unique players.
func (a *Aggregator) PlayerCount() int {
	a.ensureIndex()
	return len(a.byPlay)
}

// GamesInRange returns games whose end time falls within range.
func (a *Aggregator) GamesInRange(r TimeRange) []Game {
	var out []Game
	for _, g := range a.games {
		if r.Contains(g.EndedAt) {
			out = append(out, g)
		}
	}
	return out
}

// GamesByMode returns all games for a given mode.
func (a *Aggregator) GamesByMode(m GameMode) []Game {
	var out []Game
	for _, g := range a.games {
		if g.Mode == m {
			out = append(out, g)
		}
	}
	return out
}
