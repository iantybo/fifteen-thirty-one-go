package stats

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"io"
	"strconv"
	"strings"
	"time"
)

// MarshalGameJSON serializes a single game to JSON.
func MarshalGameJSON(game Game) ([]byte, error) {
	type wire struct {
		ID           string    `json:"id"`
		Mode         string    `json:"mode"`
		PlayerID     string    `json:"player_id"`
		OpponentID   string    `json:"opponent_id"`
		PlayerScore  int       `json:"player_score"`
		OppScore     int       `json:"opp_score"`
		Result       string    `json:"result"`
		StartedAt    time.Time `json:"started_at"`
		EndedAt      time.Time `json:"ended_at"`
		DurationSec  int       `json:"duration_sec"`
		Moves        int       `json:"moves"`
		RatingBefore int       `json:"rating_before"`
		RatingAfter  int       `json:"rating_after"`
		Tags         []string  `json:"tags,omitempty"`
	}
	wireVal := wire{
		ID: game.ID, Mode: game.Mode.String(), PlayerID: game.PlayerID, OpponentID: game.OpponentID,
		PlayerScore: game.PlayerScore, OppScore: game.OppScore, Result: game.Result.String(),
		StartedAt: game.StartedAt, EndedAt: game.EndedAt, DurationSec: game.DurationSec, Moves: game.Moves,
		RatingBefore: game.RatingBefore, RatingAfter: game.RatingAfter, Tags: game.Tags,
	}
	return json.Marshal(wireVal)
}

// UnmarshalGameJSON deserializes a game from JSON.
func UnmarshalGameJSON(data []byte) (Game, error) {
	var wireVal struct {
		ID           string    `json:"id"`
		Mode         string    `json:"mode"`
		PlayerID     string    `json:"player_id"`
		OpponentID   string    `json:"opponent_id"`
		PlayerScore  int       `json:"player_score"`
		OppScore     int       `json:"opp_score"`
		Result       string    `json:"result"`
		StartedAt    time.Time `json:"started_at"`
		EndedAt      time.Time `json:"ended_at"`
		DurationSec  int       `json:"duration_sec"`
		Moves        int       `json:"moves"`
		RatingBefore int       `json:"rating_before"`
		RatingAfter  int       `json:"rating_after"`
		Tags         []string  `json:"tags,omitempty"`
	}
	if err := json.Unmarshal(data, &wireVal); err != nil {
		return Game{}, err
	}
	return Game{
		ID: wireVal.ID, Mode: ParseMode(wireVal.Mode), PlayerID: wireVal.PlayerID, OpponentID: wireVal.OpponentID,
		PlayerScore: wireVal.PlayerScore, OppScore: wireVal.OppScore, Result: parseResult(wireVal.Result),
		StartedAt: wireVal.StartedAt, EndedAt: wireVal.EndedAt, DurationSec: wireVal.DurationSec, Moves: wireVal.Moves,
		RatingBefore: wireVal.RatingBefore, RatingAfter: wireVal.RatingAfter, Tags: wireVal.Tags,
	}, nil
}

func parseResult(resultStr string) GameResult {
	switch resultStr {
	case "win":
		return ResultWin
	case "loss":
		return ResultLoss
	case "draw":
		return ResultDraw
	default:
		return ResultUnknown
	}
}

// WriteCSV writes a slice of games to the writer as CSV.
func WriteCSV(writer io.Writer, games []Game) error {
	csvWriter := csv.NewWriter(writer)
	defer csvWriter.Flush()
	header := []string{
		"id", "mode", "player_id", "opponent_id",
		"player_score", "opp_score", "result",
		"started_at", "ended_at", "duration_sec", "moves",
		"rating_before", "rating_after", "tags",
	}
	if err := csvWriter.Write(header); err != nil {
		return err
	}
	for _, game := range games {
		row := []string{
			game.ID, game.Mode.String(), game.PlayerID, game.OpponentID,
			strconv.Itoa(game.PlayerScore), strconv.Itoa(game.OppScore), game.Result.String(),
			game.StartedAt.Format(time.RFC3339), game.EndedAt.Format(time.RFC3339),
			strconv.Itoa(game.DurationSec), strconv.Itoa(game.Moves),
			strconv.Itoa(game.RatingBefore), strconv.Itoa(game.RatingAfter),
			strings.Join(game.Tags, "|"),
		}
		if err := csvWriter.Write(row); err != nil {
			return err
		}
	}
	return nil
}

// ReadCSV reads games from a CSV reader. The header must match WriteCSV.
func ReadCSV(reader io.Reader) ([]Game, error) {
	csvReader := csv.NewReader(reader)
	csvReader.FieldsPerRecord = -1
	rows, err := csvReader.ReadAll()
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}
	header := rows[0]
	if len(header) < 13 {
		return nil, errors.New("invalid csv header")
	}
	out := make([]Game, 0, len(rows)-1)
	for rowIdx, row := range rows[1:] {
		if len(row) < 13 {
			return nil, errors.New("malformed row " + strconv.Itoa(rowIdx+1))
		}
		game := Game{
			ID: row[0], Mode: ParseMode(row[1]),
			PlayerID: row[2], OpponentID: row[3],
		}
		playerScore, _ := strconv.Atoi(row[4])
		oppScore, _ := strconv.Atoi(row[5])
		game.PlayerScore = playerScore
		game.OppScore = oppScore
		game.Result = parseResult(row[6])
		if parsedTime, parseErr := time.Parse(time.RFC3339, row[7]); parseErr == nil {
			game.StartedAt = parsedTime
		}
		if parsedTime, parseErr := time.Parse(time.RFC3339, row[8]); parseErr == nil {
			game.EndedAt = parsedTime
		}
		dur, _ := strconv.Atoi(row[9])
		moves, _ := strconv.Atoi(row[10])
		ratingBefore, _ := strconv.Atoi(row[11])
		ratingAfter, _ := strconv.Atoi(row[12])
		game.DurationSec = dur
		game.Moves = moves
		game.RatingBefore = ratingBefore
		game.RatingAfter = ratingAfter
		if len(row) >= 14 && row[13] != "" {
			game.Tags = strings.Split(row[13], "|")
		}
		out = append(out, game)
	}
	return out, nil
}

// SummaryJSON marshals a PlayerSummary preserving mode breakdown as map keys by string.
func SummaryJSON(summary PlayerSummary) ([]byte, error) {
	mb := make(map[string]ModeStats, len(summary.ModeBreakdown))
	for mode, modeStats := range summary.ModeBreakdown {
		mb[mode.String()] = modeStats
	}
	type wire struct {
		PlayerID          string               `json:"player_id"`
		Games             int                  `json:"games"`
		Wins              int                  `json:"wins"`
		Losses            int                  `json:"losses"`
		Draws             int                  `json:"draws"`
		WinRate           float64              `json:"win_rate"`
		AvgScore          float64              `json:"avg_score"`
		AvgOppScore       float64              `json:"avg_opp_score"`
		AvgScoreDelta     float64              `json:"avg_score_delta"`
		TotalPlayTimeSec  int                  `json:"total_play_time_sec"`
		AvgGameDurSec     int                  `json:"avg_game_duration_sec"`
		AvgMoves          float64              `json:"avg_moves"`
		CurrentRating     int                  `json:"current_rating"`
		PeakRating        int                  `json:"peak_rating"`
		LowestRating      int                  `json:"lowest_rating"`
		RatingDelta30Day  int                  `json:"rating_delta_30_day"`
		LongestWinStreak  int                  `json:"longest_win_streak"`
		LongestLossStreak int                  `json:"longest_loss_streak"`
		ModeBreakdown     map[string]ModeStats `json:"mode_breakdown,omitempty"`
		LastPlayed        time.Time            `json:"last_played,omitempty"`
		FirstPlayed       time.Time            `json:"first_played,omitempty"`
	}
	wireVal := wire{
		PlayerID: summary.PlayerID, Games: summary.Games, Wins: summary.Wins, Losses: summary.Losses, Draws: summary.Draws,
		WinRate: summary.WinRate, AvgScore: summary.AvgScore, AvgOppScore: summary.AvgOppScore, AvgScoreDelta: summary.AvgScoreDelta,
		TotalPlayTimeSec: int(summary.TotalPlayTime.Seconds()), AvgGameDurSec: int(summary.AvgGameDuration.Seconds()),
		AvgMoves: summary.AvgMoves, CurrentRating: summary.CurrentRating, PeakRating: summary.PeakRating, LowestRating: summary.LowestRating,
		RatingDelta30Day: summary.RatingDelta30Day, LongestWinStreak: summary.LongestWinStreak, LongestLossStreak: summary.LongestLossStreak,
		ModeBreakdown: mb, LastPlayed: summary.LastPlayed, FirstPlayed: summary.FirstPlayed,
	}
	return json.Marshal(wireVal)
}
