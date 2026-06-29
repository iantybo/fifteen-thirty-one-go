package models

import (
	"database/sql"
	"testing"

	"fifteen-thirty-one-go/backend/internal/database"
)

func TestComputeStreaks(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		wins        []bool
		wantCurrent int64
		wantLongest int64
	}{
		{name: "empty", wins: nil, wantCurrent: 0, wantLongest: 0},
		{name: "all_losses", wins: []bool{false, false}, wantCurrent: 0, wantLongest: 0},
		{name: "all_wins", wins: []bool{true, true, true}, wantCurrent: 3, wantLongest: 3},
		{name: "trailing_loss", wins: []bool{true, true, false}, wantCurrent: 0, wantLongest: 2},
		{name: "trailing_win", wins: []bool{false, true, true}, wantCurrent: 2, wantLongest: 2},
		{name: "longest_in_middle", wins: []bool{true, true, true, false, true}, wantCurrent: 1, wantLongest: 3},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			current, longest := computeStreaks(tc.wins)
			if current != tc.wantCurrent {
				t.Errorf("current streak = %d, want %d", current, tc.wantCurrent)
			}
			if longest != tc.wantLongest {
				t.Errorf("longest streak = %d, want %d", longest, tc.wantLongest)
			}
		})
	}
}

func newTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := database.OpenAndMigrate(":memory:")
	if err != nil {
		t.Fatalf("OpenAndMigrate: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func seedUser(t *testing.T, db *sql.DB, username string, gamesPlayed, gamesWon int64) int64 {
	t.Helper()
	res, err := db.Exec(
		`INSERT INTO users(username, password_hash, games_played, games_won) VALUES (?, 'x', ?, ?)`,
		username, gamesPlayed, gamesWon,
	)
	if err != nil {
		t.Fatalf("insert user: %v", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		t.Fatalf("LastInsertId: %v", err)
	}
	return id
}

// seedGame creates a lobby and a game owned/hosted by userID so scoreboard rows
// can satisfy their foreign-key constraints. It returns the new game's ID.
func seedGame(t *testing.T, db *sql.DB, userID int64) int64 {
	t.Helper()
	lobbyRes, err := db.Exec(
		`INSERT INTO lobbies(name, host_id, max_players) VALUES ('test', ?, 2)`,
		userID,
	)
	if err != nil {
		t.Fatalf("insert lobby: %v", err)
	}
	lobbyID, err := lobbyRes.LastInsertId()
	if err != nil {
		t.Fatalf("LastInsertId lobby: %v", err)
	}
	gameRes, err := db.Exec(
		`INSERT INTO games(lobby_id, status) VALUES (?, 'finished')`,
		lobbyID,
	)
	if err != nil {
		t.Fatalf("insert game: %v", err)
	}
	gameID, err := gameRes.LastInsertId()
	if err != nil {
		t.Fatalf("LastInsertId game: %v", err)
	}
	return gameID
}

func TestGetUserStatsNotFound(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)
	if _, err := GetUserStats(db, 999); err != ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestGetUserStatsAggregates(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)

	// Player has 4 finished games: win, win, loss, win (chronological).
	userID := seedUser(t, db, "alice", 4, 3)
	games := []struct {
		finalScore, position int64
	}{
		{121, 1},
		{110, 1},
		{95, 2},
		{121, 1},
	}
	for _, g := range games {
		gameID := seedGame(t, db, userID)
		if _, err := db.Exec(
			`INSERT INTO scoreboard(user_id, game_id, final_score, position) VALUES (?, ?, ?, ?)`,
			userID, gameID, g.finalScore, g.position,
		); err != nil {
			t.Fatalf("insert scoreboard: %v", err)
		}
	}

	stats, err := GetUserStats(db, userID)
	if err != nil {
		t.Fatalf("GetUserStats: %v", err)
	}
	if stats.GamesPlayed != 4 || stats.GamesWon != 3 {
		t.Errorf("games played/won = %d/%d, want 4/3", stats.GamesPlayed, stats.GamesWon)
	}
	if stats.WinRate != 0.75 {
		t.Errorf("win rate = %v, want 0.75", stats.WinRate)
	}
	if stats.CurrentWinStreak != 1 {
		t.Errorf("current win streak = %d, want 1", stats.CurrentWinStreak)
	}
	if stats.LongestWinStreak != 2 {
		t.Errorf("longest win streak = %d, want 2", stats.LongestWinStreak)
	}
	if stats.BestScore != 121 {
		t.Errorf("best score = %d, want 121", stats.BestScore)
	}
	// (121 + 110 + 95 + 121) / 4 = 111.75 -> 111.8 rounded to one decimal.
	if stats.AverageScore != 111.8 {
		t.Errorf("average score = %v, want 111.8", stats.AverageScore)
	}
}

func TestGetUserStatsNoGames(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)

	userID := seedUser(t, db, "newbie", 0, 0)
	stats, err := GetUserStats(db, userID)
	if err != nil {
		t.Fatalf("GetUserStats: %v", err)
	}
	if stats.WinRate != 0 || stats.CurrentWinStreak != 0 || stats.LongestWinStreak != 0 ||
		stats.BestScore != 0 || stats.AverageScore != 0 {
		t.Errorf("expected zeroed stats for player with no games, got %+v", stats)
	}
}
