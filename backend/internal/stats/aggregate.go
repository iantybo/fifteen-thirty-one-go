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
func (agg *Aggregator) Add(game Game) {
	agg.games = append(agg.games, game)
	agg.indexed = false
}

// AddMany appends a batch of games.
func (agg *Aggregator) AddMany(games []Game) {
	agg.games = append(agg.games, games...)
	agg.indexed = false
}

// Len reports the number of games tracked.
func (agg *Aggregator) Len() int {
	return len(agg.games)
}

// Reset removes all tracked games.
func (agg *Aggregator) Reset() {
	agg.games = agg.games[:0]
	agg.byPlay = nil
	agg.indexed = false
}

func (agg *Aggregator) ensureIndex() {
	if agg.indexed {
		return
	}
	agg.byPlay = make(map[string][]int, len(agg.games)/2+1)
	for idx, game := range agg.games {
		agg.byPlay[game.PlayerID] = append(agg.byPlay[game.PlayerID], idx)
	}
	agg.indexed = true
}

// Players returns the list of unique player IDs.
func (agg *Aggregator) Players() []string {
	agg.ensureIndex()
	out := make([]string, 0, len(agg.byPlay))
	for playerKey := range agg.byPlay {
		out = append(out, playerKey)
	}
	sort.Strings(out)
	return out
}

// GamesFor returns the games associated with a player.
func (agg *Aggregator) GamesFor(playerID string) []Game {
	agg.ensureIndex()
	gameIndices := agg.byPlay[playerID]
	out := make([]Game, len(gameIndices))
	for outIdx, gameIdx := range gameIndices {
		out[outIdx] = agg.games[gameIdx]
	}
	sort.Slice(out, func(ii, jj int) bool {
		return out[ii].EndedAt.Before(out[jj].EndedAt)
	})
	return out
}

// Summarize produces the aggregate statistics for one player.
func (agg *Aggregator) Summarize(playerID string) PlayerSummary {
	games := agg.GamesFor(playerID)
	return summarizeGames(playerID, games, agg.now())
}

// SummarizeFiltered produces a summary applying a filter.
func (agg *Aggregator) SummarizeFiltered(fl Filter) PlayerSummary {
	agg.ensureIndex()
	var games []Game
	if fl.PlayerID != "" {
		for _, gameIdx := range agg.byPlay[fl.PlayerID] {
			game := agg.games[gameIdx]
			if fl.AppliesTo(game) {
				games = append(games, game)
			}
		}
	} else {
		for _, game := range agg.games {
			if fl.AppliesTo(game) {
				games = append(games, game)
			}
		}
	}
	sort.Slice(games, func(ii, jj int) bool {
		return games[ii].EndedAt.Before(games[jj].EndedAt)
	})
	return summarizeGames(fl.PlayerID, games, agg.now())
}

func summarizeGames(playerID string, games []Game, now time.Time) PlayerSummary {
	summary := PlayerSummary{
		PlayerID:      playerID,
		ModeBreakdown: make(map[GameMode]ModeStats),
	}
	if len(games) == 0 {
		return summary
	}

	var totalScore, totalOpp, totalMoves int
	var totalDur time.Duration
	summary.PeakRating = games[0].RatingAfter
	summary.LowestRating = games[0].RatingAfter
	summary.FirstPlayed = games[0].EndedAt
	summary.LastPlayed = games[len(games)-1].EndedAt
	summary.CurrentRating = games[len(games)-1].RatingAfter

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

	for _, game := range games {
		summary.Games++
		switch game.Result {
		case ResultWin:
			summary.Wins++
			tmpWin++
			tmpLoss = 0
		case ResultLoss:
			summary.Losses++
			tmpLoss++
			tmpWin = 0
		case ResultDraw:
			summary.Draws++
			tmpWin = 0
			tmpLoss = 0
		}
		if tmpWin > longestWin {
			longestWin = tmpWin
		}
		if tmpLoss > longestLoss {
			longestLoss = tmpLoss
		}

		if game.Result == curStreakResult {
			curStreakLen++
		} else {
			curStreakResult = game.Result
			curStreakLen = 1
			curStreakStart = game.EndedAt
		}

		totalScore += game.PlayerScore
		totalOpp += game.OppScore
		totalMoves += game.Moves
		totalDur += game.Duration()

		if game.RatingAfter > summary.PeakRating {
			summary.PeakRating = game.RatingAfter
		}
		if game.RatingAfter < summary.LowestRating || summary.LowestRating == 0 {
			summary.LowestRating = game.RatingAfter
		}

		modeStats := summary.ModeBreakdown[game.Mode]
		modeStats.Mode = game.Mode
		modeStats.Games++
		switch game.Result {
		case ResultWin:
			modeStats.Wins++
		case ResultLoss:
			modeStats.Losses++
		case ResultDraw:
			modeStats.Draws++
		}
		modeStats.AvgScore += float64(game.PlayerScore)
		summary.ModeBreakdown[game.Mode] = modeStats

		if !game.EndedAt.Before(thirtyDaysAgo) {
			if !rating30Init {
				rating30Start = game.RatingBefore
				rating30Init = true
			}
			rating30End = game.RatingAfter
		}
	}

	summary.WinRate = ratio(summary.Wins, summary.Games)
	summary.AvgScore = float64(totalScore) / float64(summary.Games)
	summary.AvgOppScore = float64(totalOpp) / float64(summary.Games)
	summary.AvgScoreDelta = summary.AvgScore - summary.AvgOppScore
	summary.AvgMoves = float64(totalMoves) / float64(summary.Games)
	summary.TotalPlayTime = totalDur
	summary.AvgGameDuration = time.Duration(int64(totalDur) / int64(summary.Games))
	summary.LongestWinStreak = longestWin
	summary.LongestLossStreak = longestLoss
	summary.CurrentStreak = Streak{
		Result: curStreakResult,
		Length: curStreakLen,
		Since:  curStreakStart,
	}
	if rating30Init {
		summary.RatingDelta30Day = rating30End - rating30Start
	}

	for mode, modeStats := range summary.ModeBreakdown {
		modeStats.WinRate = ratio(modeStats.Wins, modeStats.Games)
		if modeStats.Games > 0 {
			modeStats.AvgScore = modeStats.AvgScore / float64(modeStats.Games)
		}
		summary.ModeBreakdown[mode] = modeStats
	}

	return summary
}

func ratio(num, denom int) float64 {
	if denom <= 0 {
		return 0
	}
	return float64(num) / float64(denom)
}

// AllSummaries returns a summary for every known player.
func (agg *Aggregator) AllSummaries() map[string]PlayerSummary {
	agg.ensureIndex()
	out := make(map[string]PlayerSummary, len(agg.byPlay))
	now := agg.now()
	for playerKey := range agg.byPlay {
		games := agg.GamesFor(playerKey)
		out[playerKey] = summarizeGames(playerKey, games, now)
	}
	return out
}

// PlayerCount returns the count of unique players.
func (agg *Aggregator) PlayerCount() int {
	agg.ensureIndex()
	return len(agg.byPlay)
}

// GamesInRange returns games whose end time falls within range.
func (agg *Aggregator) GamesInRange(tr TimeRange) []Game {
	var out []Game
	for _, game := range agg.games {
		if tr.Contains(game.EndedAt) {
			out = append(out, game)
		}
	}
	return out
}

// GamesByMode returns all games for a given mode.
func (agg *Aggregator) GamesByMode(mode GameMode) []Game {
	var out []Game
	for _, game := range agg.games {
		if game.Mode == mode {
			out = append(out, game)
		}
	}
	return out
}
