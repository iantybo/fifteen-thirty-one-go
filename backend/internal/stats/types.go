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

func (gr GameResult) String() string {
	switch gr {
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

func (gm GameMode) String() string {
	switch gm {
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
func ParseMode(modeStr string) GameMode {
	switch modeStr {
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
func (game Game) Duration() time.Duration {
	if game.DurationSec > 0 {
		return time.Duration(game.DurationSec) * time.Second
	}
	if !game.StartedAt.IsZero() && !game.EndedAt.IsZero() {
		return game.EndedAt.Sub(game.StartedAt)
	}
	return 0
}

// ScoreDelta is the player's score minus the opponent's.
func (game Game) ScoreDelta() int {
	return game.PlayerScore - game.OppScore
}

// RatingDelta is the rating change for the player.
func (game Game) RatingDelta() int {
	return game.RatingAfter - game.RatingBefore
}

// IsRanked reports whether the game affects rating.
func (game Game) IsRanked() bool {
	return game.Mode == ModeRanked || game.Mode == ModeTournament
}

// PlayerSummary captures aggregate per-player statistics.
type PlayerSummary struct {
	PlayerID          string
	Games             int
	Wins              int
	Losses            int
	Draws             int
	WinRate           float64
	AvgScore          float64
	AvgOppScore       float64
	AvgScoreDelta     float64
	TotalPlayTime     time.Duration
	AvgGameDuration   time.Duration
	AvgMoves          float64
	CurrentRating     int
	PeakRating        int
	LowestRating      int
	RatingDelta30Day  int
	CurrentStreak     Streak
	LongestWinStreak  int
	LongestLossStreak int
	ModeBreakdown     map[GameMode]ModeStats
	LastPlayed        time.Time
	FirstPlayed       time.Time
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
	Rank     int
	PlayerID string
	Rating   int
	Games    int
	WinRate  float64
	LastSeen time.Time
	Trend    int
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

// Contains reports whether the given time falls within the range.
func (tr TimeRange) Contains(ts time.Time) bool {
	if !tr.From.IsZero() && ts.Before(tr.From) {
		return false
	}
	if !tr.To.IsZero() && !ts.Before(tr.To) {
		return false
	}
	return true
}

// Filter narrows the set of games considered when computing stats.
type Filter struct {
	PlayerID string
	Modes    []GameMode
	Range    TimeRange
	MinMoves int
	MaxMoves int
	Tags     []string
	OnlyWins bool
	OnlyLoss bool
}

// AppliesTo returns true if the game matches the filter.
func (fl Filter) AppliesTo(game Game) bool {
	if fl.PlayerID != "" && game.PlayerID != fl.PlayerID {
		return false
	}
	if !fl.Range.From.IsZero() || !fl.Range.To.IsZero() {
		if !fl.Range.Contains(game.EndedAt) {
			return false
		}
	}
	if len(fl.Modes) > 0 {
		found := false
		for _, mode := range fl.Modes {
			if game.Mode == mode {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	if fl.MinMoves > 0 && game.Moves < fl.MinMoves {
		return false
	}
	if fl.MaxMoves > 0 && game.Moves > fl.MaxMoves {
		return false
	}
	if len(fl.Tags) > 0 {
		if !hasAnyTag(game.Tags, fl.Tags) {
			return false
		}
	}
	if fl.OnlyWins && game.Result != ResultWin {
		return false
	}
	if fl.OnlyLoss && game.Result != ResultLoss {
		return false
	}
	return true
}

func hasAnyTag(gameTags []string, wantTags []string) bool {
	set := make(map[string]struct{}, len(gameTags))
	for _, tag := range gameTags {
		set[tag] = struct{}{}
	}
	for _, wantTag := range wantTags {
		if _, ok := set[wantTag]; ok {
			return true
		}
	}

}
