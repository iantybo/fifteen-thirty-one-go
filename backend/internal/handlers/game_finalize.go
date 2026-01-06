package handlers

import (
	"database/sql"
	"sort"

	"fifteen-thirty-one-go/backend/internal/models"
)

// maybeFinalizeGame persists immutable end-of-game results once the engine reaches stage "finished".
// It is safe to call multiple times (idempotent per game_id).
func maybeFinalizeGame(db *sql.DB, gameID int64) error {
	players, err := models.ListGamePlayersByGame(db, gameID)
	if err != nil {
		return err
	}
	if len(players) == 0 {
		return nil
	}

	st, unlock, err := ensureGameStateLocked(db, gameID, players)
	if err != nil {
		return err
	}
	if st == nil {
		return nil
	}
	if st.Stage != "finished" {
		unlock()
		return nil
	}

	// Copy what we need while holding the lock.
	scores := append([]int(nil), st.Scores...)
	unlock()

	type row struct {
		userID   int64
		pos      int64
		score    int64
		username string
	}
	rows := make([]row, 0, len(players))
	for _, p := range players {
		pos := int(p.Position)
		var sc int64
		if pos >= 0 && pos < len(scores) {
			sc = int64(scores[pos])
		}
		rows = append(rows, row{userID: p.UserID, pos: p.Position, score: sc, username: p.Username})
	}
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].score != rows[j].score {
			return rows[i].score > rows[j].score
		}
		return rows[i].pos < rows[j].pos
	})
	if len(rows) == 0 {
		return nil
	}
	winnerID := rows[0].userID

	g, err := models.GetGameByID(db, gameID)
	if err != nil {
		return err
	}

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	var existing int64
	if err := tx.QueryRow(`SELECT COUNT(*) FROM scoreboard WHERE game_id = ?`, gameID).Scan(&existing); err != nil {
		return err
	}
	if existing > 0 {
		// Already finalized.
		if err := tx.Commit(); err != nil {
			return err
		}
		committed = true
		return nil
	}

	for i, r := range rows {
		rank := int64(i + 1)
		if _, err := tx.Exec(
			`INSERT INTO scoreboard(user_id, game_id, final_score, position) VALUES (?, ?, ?, ?)`,
			r.userID, gameID, r.score, rank,
		); err != nil {
			return err
		}
		if _, err := tx.Exec(`UPDATE users SET games_played = games_played + 1 WHERE id = ?`, r.userID); err != nil {
			return err
		}
	}
	if _, err := tx.Exec(`UPDATE users SET games_won = games_won + 1 WHERE id = ?`, winnerID); err != nil {
		return err
	}
	if err := models.SetGameStatusTx(tx, gameID, "finished"); err != nil {
		return err
	}
	if err := models.SetLobbyStatusTx(tx, g.LobbyID, "finished"); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return err
	}
	committed = true
	return nil
}
