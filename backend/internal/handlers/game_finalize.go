package handlers

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
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

	type row = finalizeRow
	rows := make([]row, 0, len(players))
	for _, p := range players {
		pos := int(p.Position)
		var sc int64
		if pos >= 0 && pos < len(scores) {
			sc = int64(scores[pos])
		}
		rows = append(rows, row{userID: p.UserID, pos: p.Position, score: sc, username: p.Username, isBot: p.IsBot})
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

	if err := awardUsedCardsTx(ctx, tx, gameID, rows); err != nil {
		return fmt.Errorf("maybeFinalizeGame: awardUsedCardsTx (game_id=%d): %w", gameID, err)
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
	return nil
}

type finalizeRow struct {
	userID   int64
	pos      int64
	score    int64
	username string
	isBot    bool
}

// awardUsedCardsTx inserts one user_cards row per distinct card each player
// played (move_type = 'play_card') during this game. Uses the uncorrected move
// where available: if a move was corrected, the correction carries the
// authoritative card_played. INSERT OR IGNORE on the unique (user_id, game_id,
// card) index keeps this idempotent.
func awardUsedCardsTx(ctx context.Context, tx *sql.Tx, gameID int64, players []finalizeRow) error {
	rs, err := tx.QueryContext(
		ctx,
		`SELECT player_id, card_played
		 FROM game_moves
		 WHERE game_id = ?
		   AND move_type = 'play_card'
		   AND card_played IS NOT NULL
		   AND is_corrected = 0`,
		gameID,
	)
	if err != nil {
		return fmt.Errorf("query played cards: %w", err)
	}
	defer rs.Close()

	realPlayer := make(map[int64]struct{}, len(players))
	for _, p := range players {
		if p.isBot {
			continue
		}
		realPlayer[p.userID] = struct{}{}
	}

	cardsByUser := map[int64]map[string]struct{}{}
	for rs.Next() {
		var pid int64
		var card sql.NullString
		if err := rs.Scan(&pid, &card); err != nil {
			return fmt.Errorf("scan played card: %w", err)
		}
		if !card.Valid || card.String == "" {
			continue
		}
		if _, ok := realPlayer[pid]; !ok {
			continue
		}
		m, ok := cardsByUser[pid]
		if !ok {
			m = map[string]struct{}{}
			cardsByUser[pid] = m
		}
		m[card.String] = struct{}{}
	}
	if err := rs.Err(); err != nil {
		return fmt.Errorf("iterate played cards: %w", err)
	}

	for userID, set := range cardsByUser {
		list := make([]string, 0, len(set))
		for c := range set {
			list = append(list, c)
		}
		if err := models.AwardCardsForGameTx(ctx, tx, userID, gameID, list); err != nil {
			return fmt.Errorf("award cards (user_id=%d game_id=%d): %w", userID, gameID, err)
		}
	}
	return nil
}
