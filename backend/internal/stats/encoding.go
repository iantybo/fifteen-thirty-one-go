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
func MarshalGameJSON(g Game) ([]byte, error) {
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
	w := wire{
		ID: g.ID, Mode: g.Mode.String(), PlayerID: g.PlayerID, OpponentID: g.OpponentID,
		PlayerScore: g.PlayerScore, OppScore: g.OppScore, Result: g.Result.String(),
		StartedAt: g.StartedAt, EndedAt: g.EndedAt, DurationSec: g.DurationSec, Moves: g.Moves,
		RatingBefore: g.RatingBefore, RatingAfter: g.RatingAfter, Tags: g.Tags,
	}
	return json.Marshal(w)
}

// UnmarshalGameJSON deserializes a game from JSON.
func UnmarshalGameJSON(data []byte) (Game, error) {
	var w struct {
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
	if err := json.Unmarshal(data, &w); err != nil {
		return Game{}, err
	}
	return Game{
		ID: w.ID, Mode: ParseMode(w.Mode), PlayerID: w.PlayerID, OpponentID: w.OpponentID,
		PlayerScore: w.PlayerScore, OppScore: w.OppScore, Result: parseResult(w.Result),
		StartedAt: w.StartedAt, EndedAt: w.EndedAt, DurationSec: w.DurationSec, Moves: w.Moves,
		RatingBefore: w.RatingBefore, RatingAfter: w.RatingAfter, Tags: w.Tags,
	}, nil
}

func parseResult(s string) GameResult {
	switch s {
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
func WriteCSV(w io.Writer, games []Game) error {
	cw := csv.NewWriter(w)
	defer cw.Flush()
	header := []string{
		"id", "mode", "player_id", "opponent_id",
		"player_score", "opp_score", "result",
		"started_at", "ended_at", "duration_sec", "moves",
		"rating_before", "rating_after", "tags",
	}
	if err := cw.Write(header); err != nil {
		return err
	}
	for _, g := range games {
		row := []string{
			g.ID, g.Mode.String(), g.PlayerID, g.OpponentID,
			strconv.Itoa(g.PlayerScore), strconv.Itoa(g.OppScore), g.Result.String(),
			g.StartedAt.Format(time.RFC3339), g.EndedAt.Format(time.RFC3339),
			strconv.Itoa(g.DurationSec), strconv.Itoa(g.Moves),
			strconv.Itoa(g.RatingBefore), strconv.Itoa(g.RatingAfter),
			strings.Join(g.Tags, "|"),
		}
		if err := cw.Write(row); err != nil {
			return err
		}
	}
	return nil
}

// ReadCSV reads games from a CSV reader. The header must match WriteCSV.
func ReadCSV(r io.Reader) ([]Game, error) {
	cr := csv.NewReader(r)
	cr.FieldsPerRecord = -1
	rows, err := cr.ReadAll()
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
	for i, row := range rows[1:] {
		if len(row) < 13 {
			return nil, errors.New("malformed row " + strconv.Itoa(i+1))
		}
		g := Game{
			ID: row[0], Mode: ParseMode(row[1]),
			PlayerID: row[2], OpponentID: row[3],
		}
		ps, _ := strconv.Atoi(row[4])
		os, _ := strconv.Atoi(row[5])
		g.PlayerScore = ps
		g.OppScore = os
		g.Result = parseResult(row[6])
		if t, err := time.Parse(time.RFC3339, row[7]); err == nil {
			g.StartedAt = t
		}
		if t, err := time.Parse(time.RFC3339, row[8]); err == nil {
			g.EndedAt = t
		}
		dur, _ := strconv.Atoi(row[9])
		mv, _ := strconv.Atoi(row[10])
		rb, _ := strconv.Atoi(row[11])
		ra, _ := strconv.Atoi(row[12])
		g.DurationSec = dur
		g.Moves = mv
		g.RatingBefore = rb
		g.RatingAfter = ra
		if len(row) >= 14 && row[13] != "" {
			g.Tags = strings.Split(row[13], "|")
		}
		out = append(out, g)
	}
	return out, nil
}

// SummaryJSON marshals a PlayerSummary preserving mode breakdown as map keys by string.
func SummaryJSON(s PlayerSummary) ([]byte, error) {
	mb := make(map[string]ModeStats, len(s.ModeBreakdown))
	for k, v := range s.ModeBreakdown {
		mb[k.String()] = v
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
	w := wire{
		PlayerID: s.PlayerID, Games: s.Games, Wins: s.Wins, Losses: s.Losses, Draws: s.Draws,
		WinRate: s.WinRate, AvgScore: s.AvgScore, AvgOppScore: s.AvgOppScore, AvgScoreDelta: s.AvgScoreDelta,
		TotalPlayTimeSec: int(s.TotalPlayTime.Seconds()), AvgGameDurSec: int(s.AvgGameDuration.Seconds()),
		AvgMoves: s.AvgMoves, CurrentRating: s.CurrentRating, PeakRating: s.PeakRating, LowestRating: s.LowestRating,
		RatingDelta30Day: s.RatingDelta30Day, LongestWinStreak: s.LongestWinStreak, LongestLossStreak: s.LongestLossStreak,
		ModeBreakdown: mb, LastPlayed: s.LastPlayed, FirstPlayed: s.FirstPlayed,
	}
	return json.Marshal(w)
}
