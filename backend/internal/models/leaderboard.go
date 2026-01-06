package models

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"time"
)

// LeaderboardDayPoint represents a single day's statistics for a player within the leaderboard
// time window. GamesPlayed and GamesWon are per-day counts; WinRate is cumulative within the window.
type LeaderboardDayPoint struct {
	Date        string  `json:"date"`         // YYYY-MM-DD
	GamesPlayed int64   `json:"games_played"` // games played on this day
	GamesWon    int64   `json:"games_won"`    // games won on this day
	WinRate     float64 `json:"win_rate"`     // cumulative within the window [0..1]
}

// LeaderboardPlayer represents a player's all-time statistics and daily performance series
// within the leaderboard time window.
type LeaderboardPlayer struct {
	UserID      int64                 `json:"user_id"`
	Username    string                `json:"username"`
	GamesPlayed int64                 `json:"games_played"` // all-time (from scoreboard)
	GamesWon    int64                 `json:"games_won"`    // all-time (from scoreboard)
	WinRate     float64               `json:"win_rate"`     // all-time [0..1]
	Series      []LeaderboardDayPoint `json:"series"`
}

// LeaderboardResponse contains leaderboard data for a specified time window.
type LeaderboardResponse struct {
	Days  int64               `json:"days"`
	Items []LeaderboardPlayer `json:"items"`
}

// BuildLeaderboard constructs a leaderboard response containing player statistics for the specified
// time window. The days parameter is normalized to [1, 365]. Returns an error if database queries fail.
func BuildLeaderboard(ctx context.Context, db *sql.DB, days int64) (*LeaderboardResponse, error) {
	if days <= 0 {
		days = 30
	}
	if days > 365 {
		days = 365
	}

	type u struct {
		id       int64
		username string
	}
	users := make([]u, 0)
	{
		rows, err := db.QueryContext(ctx, `SELECT id, username FROM users ORDER BY username COLLATE NOCASE ASC`)
		if err != nil {
			return nil, fmt.Errorf("BuildLeaderboard: querying users: %w", err)
		}
		defer rows.Close()
		for rows.Next() {
			var id int64
			var username string
			if err := rows.Scan(&id, &username); err != nil {
				return nil, fmt.Errorf("BuildLeaderboard: scanning user row: %w", err)
			}
			users = append(users, u{id: id, username: username})
		}
		if err := rows.Err(); err != nil {
			return nil, fmt.Errorf("BuildLeaderboard: iterating user rows: %w", err)
		}
	}

	type totals struct {
		played int64
		won    int64
	}
	byUserTotals := map[int64]totals{}
	{
		rows, err := db.QueryContext(
			ctx,
			`SELECT user_id,
			        COUNT(*) AS games_played,
			        SUM(CASE WHEN position = 1 THEN 1 ELSE 0 END) AS games_won
			 FROM scoreboard
			 GROUP BY user_id`,
		)
		if err != nil {
			return nil, fmt.Errorf("BuildLeaderboard: querying totals from scoreboard: %w", err)
		}
		defer rows.Close()
		for rows.Next() {
			var userID, played, won int64
			if err := rows.Scan(&userID, &played, &won); err != nil {
				return nil, fmt.Errorf("BuildLeaderboard: scanning totals row: %w", err)
			}
			byUserTotals[userID] = totals{played: played, won: won}
		}
		if err := rows.Err(); err != nil {
			return nil, fmt.Errorf("BuildLeaderboard: iterating totals rows: %w", err)
		}
	}

	type dayAgg struct {
		played int64
		won    int64
	}
	byUserDay := map[int64]map[string]dayAgg{}
	{
		since := fmt.Sprintf("-%d days", days-1)
		rows, err := db.QueryContext(
			ctx,
			`SELECT user_id,
			        DATE(created_at) AS day,
			        COUNT(*) AS games_played,
			        SUM(CASE WHEN position = 1 THEN 1 ELSE 0 END) AS games_won
			 FROM scoreboard
			 WHERE created_at >= DATE('now', ?)
			 GROUP BY user_id, DATE(created_at)
			 ORDER BY day ASC`,
			since,
		)
		if err != nil {
			return nil, fmt.Errorf("BuildLeaderboard: querying daily aggregates from scoreboard: %w", err)
		}
		defer rows.Close()
		for rows.Next() {
			var userID, played, won int64
			var day string
			if err := rows.Scan(&userID, &day, &played, &won); err != nil {
				return nil, fmt.Errorf("BuildLeaderboard: scanning daily aggregate row: %w", err)
			}
			m := byUserDay[userID]
			if m == nil {
				m = map[string]dayAgg{}
				byUserDay[userID] = m
			}
			m[day] = dayAgg{played: played, won: won}
		}
		if err := rows.Err(); err != nil {
			return nil, fmt.Errorf("BuildLeaderboard: iterating daily aggregate rows: %w", err)
		}
	}

	// Respect cancellations before expensive in-memory processing.
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("BuildLeaderboard: context cancelled: %w", err)
	}

	// Build the date list (oldest -> newest) as YYYY-MM-DD in UTC to match SQLite DATE('now', ...).
	start := time.Now().UTC().AddDate(0, 0, -int(days)+1)
	dates := make([]string, 0, days)
	for i := int64(0); i < days; i++ {
		d := start.AddDate(0, 0, int(i))
		dates = append(dates, d.Format("2006-01-02"))
	}

	out := make([]LeaderboardPlayer, 0, len(users))
	for _, usr := range users {
		if err := ctx.Err(); err != nil {
			return nil, fmt.Errorf("BuildLeaderboard: context cancelled: %w", err)
		}
		t := byUserTotals[usr.id]
		var allTimeRate float64
		if t.played > 0 {
			allTimeRate = float64(t.won) / float64(t.played)
		}

		series := make([]LeaderboardDayPoint, 0, len(dates))
		cumPlayed := int64(0)
		cumWon := int64(0)
		dayMap := byUserDay[usr.id]
		for _, day := range dates {
			if err := ctx.Err(); err != nil {
				return nil, fmt.Errorf("BuildLeaderboard: context cancelled: %w", err)
			}
			da := dayAgg{}
			if dayMap != nil {
				da = dayMap[day]
			}
			cumPlayed += da.played
			cumWon += da.won
			var wr float64
			if cumPlayed > 0 {
				wr = float64(cumWon) / float64(cumPlayed)
			}
			series = append(series, LeaderboardDayPoint{
				Date:        day,
				GamesPlayed: da.played,
				GamesWon:    da.won,
				WinRate:     wr,
			})
		}

		out = append(out, LeaderboardPlayer{
			UserID:      usr.id,
			Username:    usr.username,
			GamesPlayed: t.played,
			GamesWon:    t.won,
			WinRate:     allTimeRate,
			Series:      series,
		})
	}

	sort.SliceStable(out, func(i, j int) bool {
		// Players with games come first.
		if (out[i].GamesPlayed == 0) != (out[j].GamesPlayed == 0) {
			return out[i].GamesPlayed > 0
		}
		if out[i].WinRate != out[j].WinRate {
			return out[i].WinRate > out[j].WinRate
		}
		if out[i].GamesPlayed != out[j].GamesPlayed {
			return out[i].GamesPlayed > out[j].GamesPlayed
		}
		return out[i].Username < out[j].Username
	})

	return &LeaderboardResponse{Days: days, Items: out}, nil
}
