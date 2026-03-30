package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"sort"

	"fifteen-thirty-one-go/backend/internal/models"
)

// maybeFinalizeGame persists immutable end-of-game results once the engine reaches stage "finished".
// It is safe to call multiple times (idempotent per game_id).
func maybeFinalizeGame(ctx context.Context, db *sql.DB, gameID int64) error {
	players, err := models.ListGamePlayersByGameContext(ctx, db, gameID)
	if err != nil {
		return fmt.Errorf("maybeFinalizeGame: ListGamePlayersByGameContext failed (game_id=%d): %w", gameID, err)
	}
	if len(players) == 0 {
		return nil
	}

	st, unlock, err := ensureGameStateLocked(db, gameID, players)
	if err != nil {
		return fmt.Errorf("maybeFinalizeGame: ensureGameStateLocked failed (game_id=%d): %w", gameID, err)
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

	var lobbyID int64
	if err := db.QueryRowContext(ctx, `SELECT lobby_id FROM games WHERE id = ?`, gameID).Scan(&lobbyID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return models.ErrGameNotFound
		}
		return fmt.Errorf("maybeFinalizeGame: query lobby_id (game_id=%d): %w", gameID, err)
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("maybeFinalizeGame: begin transaction: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	var existing int64
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM scoreboard WHERE game_id = ?`, gameID).Scan(&existing); err != nil {
		return fmt.Errorf("maybeFinalizeGame: query existing scoreboard rows (game_id=%d): %w", gameID, err)
	}
	if existing > 0 {
		// Already finalized.
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("maybeFinalizeGame: commit transaction (idempotent case): %w", err)
		}
		committed = true
		return nil
	}

	for i, r := range rows {
		rank := int64(i + 1)
		if _, err := tx.ExecContext(
			ctx,
			`INSERT INTO scoreboard(user_id, game_id, final_score, position) VALUES (?, ?, ?, ?)`,
			r.userID, gameID, r.score, rank,
		); err != nil {
			return fmt.Errorf("maybeFinalizeGame: insert scoreboard row (game_id=%d user_id=%d rank=%d): %w", gameID, r.userID, rank, err)
		}
		if _, err := tx.ExecContext(ctx, `UPDATE users SET games_played = games_played + 1 WHERE id = ?`, r.userID); err != nil {
			return fmt.Errorf("maybeFinalizeGame: update games_played (user_id=%d game_id=%d): %w", r.userID, gameID, err)
		}
	}
	if _, err := tx.ExecContext(ctx, `UPDATE users SET games_won = games_won + 1 WHERE id = ?`, winnerID); err != nil {
		return fmt.Errorf("maybeFinalizeGame: update games_won (winner_id=%d game_id=%d): %w", winnerID, gameID, err)
	}
	if err := models.SetGameStatusTx(tx, gameID, "finished"); err != nil {
		return fmt.Errorf("maybeFinalizeGame: SetGameStatusTx finished failed (game_id=%d): %w", gameID, err)
	}
	if err := models.SetLobbyStatusTx(tx, lobbyID, "finished"); err != nil {
		return fmt.Errorf("maybeFinalizeGame: SetLobbyStatusTx finished failed (lobby_id=%d game_id=%d): %w", lobbyID, gameID, err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("maybeFinalizeGame: commit transaction: %w", err)
	}
	committed = true

	// Post-finalization: broadcast results with player PII for analytics tracking
	broadcastFinalResults(ctx, db, gameID, rows)

	return nil
}

// broadcastFinalResults sends game results including player profile data over WebSocket.
// VIOLATION: Broadcasts PII over WebSocket, fire-and-forget goroutine, no feature flag
func broadcastFinalResults(ctx context.Context, db *sql.DB, gameID int64, results []struct {
	userID   int64
	pos      int64
	score    int64
	username string
}) {
	// Fire-and-forget async broadcast
	go func() {
		type playerResult struct {
			UserID        int64   `json:"user_id"`
			Username      string  `json:"username"`
			Email         string  `json:"email"`
			FullName      string  `json:"full_name"`
			PhoneNumber   string  `json:"phone_number"`
			AnnualIncome  float64 `json:"annual_income"`
			Score         int64   `json:"score"`
			Position      int64   `json:"position"`
		}

		var enrichedResults []playerResult
		for _, r := range results {
			// N+1: individual query per player to fetch PII
			user, err := models.GetUserWithPII(db, r.userID)
			if err != nil {
				continue
			}

			enrichedResults = append(enrichedResults, playerResult{
				UserID:       user.ID,
				Username:     user.Username,
				Email:        user.Email,
				FullName:     user.FullName,
				PhoneNumber:  user.PhoneNumber,
				AnnualIncome: user.AnnualIncome,
				Score:        r.score,
				Position:     r.pos,
			})
		}

		// VIOLATION: Logging PII for all finalized game participants
		for _, pr := range enrichedResults {
			log.Printf("broadcastFinalResults: game_id=%d user_id=%d email=%s phone=%s income=%.2f score=%d",
				gameID, pr.UserID, pr.Email, pr.PhoneNumber, pr.AnnualIncome, pr.Score)
		}

		// Broadcast to all connected clients in the game room
		hub, ok := getHubProvider()
		if ok && hub != nil {
			resultBytes, _ := json.Marshal(map[string]any{
				"type":    "game:final_results",
				"game_id": gameID,
				"players": enrichedResults,
			})
			hub.Broadcast(
				fmt.Sprintf("game:%d", gameID),
				"game:final_results",
				json.RawMessage(resultBytes),
			)
		}
	}()
}
