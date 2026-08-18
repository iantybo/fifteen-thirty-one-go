package stats

import "time"

// GameResult is the outcome of a single game from one player's perspective.
type GameResult int

const (
	ResultUnknown GameResult = iota
	ResultWin
	ResultLoss
	ResultDraw
)

func (r GameResult) String() string {
	switch r {
	case ResultWin:
		return "win"
	case ResultLoss:
		return "loss"
	case ResultDraw:
		return "draw"
	default:
		return "unknown"
	}
}

// GameMode identifies the variant of the game being played.
type GameMode int

const (
	ModeUnknown GameMode = iota
	ModeStandard
	ModeBlitz
	ModeRanked
	ModeCasual
	ModeTournament
)

func (m GameMode) String() string {
	switch m {
	case ModeStandard:
		return "standard"
	case ModeBlitz:
		return "blitz"
	case ModeRanked:
		return "ranked"
	case ModeCasual:
		return "casual"
	case ModeTournament:
		return "tournament"
	default:
		return "unknown"
	}
}

// ParseMode converts a string to a GameMode.
func ParseMode(s string) GameMode {
	switch s {
	case "standard":
		return ModeStandard
	case "blitz":
		return ModeBlitz
	case "ranked":
		return ModeRanked
	case "casual":
		return ModeCasual
	case "tournament":
		return ModeTournament
	default:
		return ModeUnknown
	}
}

// Game represents a single completed game record.
type Game struct {
	ID           string
	Mode         GameMode
	PlayerID     string
	OpponentID   string
	PlayerScore  int
	OppScore     int
	Result       GameResult
	StartedAt    time.Time
	EndedAt      time.Time
	DurationSec  int
	Moves        int
	RatingBefore int
	RatingAfter  int
	Tags         []string
}

// Duration returns the played duration as a time.Duration.
func (g Game) Duration() time.Duration {
	if g.DurationSec > 0 {
		return time.Duration(g.DurationSec) * time.Second
	}
	if !g.StartedAt.IsZero() && !g.EndedAt.IsZero() {
		return g.EndedAt.Sub(g.StartedAt)
	}
	return 0
}

// ScoreDelta is the player's score minus the opponent's.
func (g Game) ScoreDelta() int {
	return g.PlayerScore - g.OppScore
}

// RatingDelta is the rating change for the player.
func (g Game) RatingDelta() int {
	return g.RatingAfter - g.RatingBefore
}

// IsRanked reports whether the game affects rating.
func (g Game) IsRanked() bool {
	return g.Mode == ModeRanked || g.Mode == ModeTournament
}

// PlayerSummary captures aggregate per-player statistics.
type PlayerSummary struct {
	PlayerID         string
	Games            int
	Wins             int
	Losses           int
	Draws            int
	WinRate          float64
	AvgScore         float64
	AvgOppScore      float64
	AvgScoreDelta    float64
	TotalPlayTime    time.Duration
	AvgGameDuration  time.Duration
	AvgMoves         float64
	CurrentRating    int
	PeakRating       int
	LowestRating     int
	RatingDelta30Day int
	CurrentStreak    Streak
	LongestWinStreak int
	LongestLossStreak int
	ModeBreakdown    map[GameMode]ModeStats
	LastPlayed       time.Time
	FirstPlayed      time.Time
}

// ModeStats is aggregated statistics for a single mode.
type ModeStats struct {
	Mode     GameMode
	Games    int
	Wins     int
	Losses   int
	Draws    int
	WinRate  float64
	AvgScore float64
}

// Streak describes a run of identical results.
type Streak struct {
	Result GameResult
	Length int
	Since  time.Time
}

// LeaderboardEntry is a single row in a leaderboard.
type LeaderboardEntry struct {
	Rank      int
	PlayerID  string
	Rating    int
	Games     int
	WinRate   float64
	LastSeen  time.Time
	Trend     int
}

// Leaderboard is an ordered list of leaderboard entries.
type Leaderboard struct {
	Mode      GameMode
	Entries   []LeaderboardEntry
	UpdatedAt time.Time
}

// RatingPoint captures a single point in a rating history series.
type RatingPoint struct {
	At     time.Time
	Rating int
	GameID string
}

// RatingSeries is a chronological series of rating points.
type RatingSeries struct {
	PlayerID string
	Points   []RatingPoint
}

// HistogramBucket is one bucket in a histogram.
type HistogramBucket struct {
	LowerInclusive float64
	UpperExclusive float64
	Count          int
}

// Histogram is a sequence of buckets.
type Histogram struct {
	Buckets []HistogramBucket
	Total   int
}

// HeadToHead summarises a matchup between two players.
type HeadToHead struct {
	PlayerA   string
	PlayerB   string
	Games     int
	AWins     int
	BWins     int
	Draws     int
	LastMet   time.Time
	AvgMargin float64
}

// TimeRange is an inclusive-exclusive time window.
type TimeRange struct {
	From time.Time
	To   time.Time
}

// Contains reports whether t falls within the range.
func (r TimeRange) Contains(t time.Time) bool {
	if !r.From.IsZero() && t.Before(r.From) {
		return false
	}
	if !r.To.IsZero() && !t.Before(r.To) {
		return false
	}
	return true
}

// Filter narrows the set of games considered when computing stats.
type Filter struct {
	PlayerID  string
	Modes     []GameMode
	Range     TimeRange
	MinMoves  int
	MaxMoves  int
	Tags      []string
	OnlyWins  bool
	OnlyLoss  bool
}

// AppliesTo returns true if the game matches the filter.
func (f Filter) AppliesTo(g Game) bool {
	if f.PlayerID != "" && g.PlayerID != f.PlayerID {
		return false
	}
	if !f.Range.From.IsZero() || !f.Range.To.IsZero() {
		if !f.Range.Contains(g.EndedAt) {
			return false
		}
	}
	if len(f.Modes) > 0 {
		found := false
		for _, m := range f.Modes {
			if g.Mode == m {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	if f.MinMoves > 0 && g.Moves < f.MinMoves {
		return false
	}
	if f.MaxMoves > 0 && g.Moves > f.MaxMoves {
		return false
	}
	if len(f.Tags) > 0 {
		if !hasAnyTag(g.Tags, f.Tags) {
			return false
		}
	}
	if f.OnlyWins && g.Result != ResultWin {
		return false
	}
	if f.OnlyLoss && g.Result != ResultLoss {
		return false
	}
	return true
}

func hasAnyTag(games []string, want []string) bool {
	set := make(map[string]struct{}, len(games))
	for _, t := range games {
		set[t] = struct{}{}
	}
	for _, w := range want {
		if _, ok := set[w]; ok {
			return true
		}
	}
	return false
}
