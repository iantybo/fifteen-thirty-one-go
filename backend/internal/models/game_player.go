package models

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

type GamePlayer struct {
	GameID        int64   `json:"game_id"`
	UserID        int64   `json:"user_id"`
	Username      string  `json:"username"`
	Position      int64   `json:"position"`
	Score         int64   `json:"score"`
	Hand          string  `json:"hand"`                 // JSON array string
	HandCount     *int64  `json:"hand_count,omitempty"` // exposed count only; used to avoid leaking opponent hand contents
	CribCards     *string `json:"crib_cards,omitempty"`
	IsBot         bool    `json:"is_bot"`
	BotDifficulty *string `json:"bot_difficulty,omitempty"`
}

func AddGamePlayer(db *sql.DB, gameID, userID int64, position int64, isBot bool, botDifficulty *string) error {
	_, err := db.Exec(
		`INSERT INTO game_players(game_id, user_id, position, is_bot, bot_difficulty) VALUES (?, ?, ?, ?, ?)`,
		gameID, userID, position, boolToInt(isBot), botDifficulty,
	)
	return err
}

func AddGamePlayerAutoPosition(db *sql.DB, gameID, userID, maxPlayers int64, isBot bool, botDifficulty *string) (int64, error) {
	// Retry on unique position collision (due to concurrent joins).
	//
	// Important: retries must start a new transaction. In SQLite, constraint
	// violations can abort the current transaction, so we never retry within a
	// single tx.
	for attempt := 0; attempt < 3; attempt++ {
		tx, err := db.Begin()
		if err != nil {
			return 0, err
		}

		pos, err := AddGamePlayerAutoPositionTx(tx, gameID, userID, maxPlayers, isBot, botDifficulty)
		if err != nil {
			if IsUniqueConstraint(err) && attempt < 2 {
				_ = tx.Rollback()
				continue
			}
			_ = tx.Rollback()
			return 0, err
		}
		if err := tx.Commit(); err != nil {
			_ = tx.Rollback()
			return 0, err
		}
		return pos, nil
	}
	return 0, errors.New("could not allocate position")
}

func AddGamePlayerAutoPositionTx(tx *sql.Tx, gameID, userID, maxPlayers int64, isBot bool, botDifficulty *string) (int64, error) {
	if maxPlayers <= 0 {
		return 0, errors.New("invalid max_players")
	}
	maxPos := maxPlayers - 1

	// Do a single insert attempt.
	//
	// Important: do NOT retry inside this transaction on unique-constraint errors.
	// In SQLite, a constraint violation can abort the transaction; callers that
	// want retries must do so by starting a new transaction.
	res, err := tx.Exec(
		`INSERT INTO game_players(game_id, user_id, position, is_bot, bot_difficulty)
		 SELECT ?, ?, COALESCE(MAX(position), -1) + 1, ?, ?
		 FROM game_players WHERE game_id = ?
		 HAVING COALESCE(MAX(position), -1) + 1 <= ?`,
		gameID, userID, boolToInt(isBot), botDifficulty, gameID, maxPos,
	)
	if err != nil {
		return 0, err
	}
	ra, err := res.RowsAffected()
	if err != nil {
		return 0, err
	}
	if ra == 0 {
		return 0, errors.New("could not allocate position")
	}

	var pos int64
	if err := tx.QueryRow(`SELECT position FROM game_players WHERE game_id = ? AND user_id = ?`, gameID, userID).Scan(&pos); err != nil {
		return 0, err
	}
	if pos < 0 {
		return 0, errors.New("invalid assigned position")
	}
	return pos, nil
}

// ListGamePlayersByGame returns all players for the given game ID ordered by position.
// It joins the users table to populate usernames and returns an error on query or scan failure.
func ListGamePlayersByGame(db *sql.DB, gameID int64) ([]GamePlayer, error) {
	return ListGamePlayersByGameContext(context.Background(), db, gameID)
}

// ListGamePlayersByGameContext returns all players for the given game ID ordered by position.
// It joins the users table to populate usernames and returns an error on query or scan failure.
func ListGamePlayersByGameContext(ctx context.Context, db *sql.DB, gameID int64) ([]GamePlayer, error) {
	rows, err := db.QueryContext(
		ctx,
		`SELECT gp.game_id, gp.user_id, COALESCE(u.username, '') AS username, gp.position, gp.score, gp.hand, gp.crib_cards, gp.is_bot, gp.bot_difficulty
		 FROM game_players gp
		 LEFT JOIN users u ON u.id = gp.user_id
		 WHERE gp.game_id = ? ORDER BY gp.position ASC`,
		gameID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []GamePlayer
	for rows.Next() {
		var gp GamePlayer
		var crib sql.NullString
		var isBotVal any
		var botDiff sql.NullString
		if err := rows.Scan(&gp.GameID, &gp.UserID, &gp.Username, &gp.Position, &gp.Score, &gp.Hand, &crib, &isBotVal, &botDiff); err != nil {
			return nil, fmt.Errorf("ListGamePlayersByGameContext: scan game player (game_id=%d): %w", gameID, err)
		}
		if crib.Valid {
			v := crib.String
			gp.CribCards = &v
		}
		gp.IsBot = parseSQLiteBool(isBotVal)
		if botDiff.Valid {
			v := botDiff.String
			gp.BotDifficulty = &v
		}
		out = append(out, gp)
	}
	return out, rows.Err()
}

func UpdatePlayerHand(db *sql.DB, gameID, userID int64, handJSON string) error {
	res, err := db.Exec(`UPDATE game_players SET hand = ? WHERE game_id = ? AND user_id = ?`, handJSON, gameID, userID)
	if err != nil {
		return fmt.Errorf("update player hand: game_id=%d user_id=%d: %w", gameID, userID, err)
	}
	ra, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("update player hand rows affected: game_id=%d user_id=%d: %w", gameID, userID, err)
	}
	if ra == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func UpdatePlayerHandTx(tx *sql.Tx, gameID, userID int64, handJSON string) error {
	res, err := tx.Exec(`UPDATE game_players SET hand = ? WHERE game_id = ? AND user_id = ?`, handJSON, gameID, userID)
	if err != nil {
		return fmt.Errorf("update player hand (tx): game_id=%d user_id=%d: %w", gameID, userID, err)
	}
	ra, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("update player hand rows affected (tx): game_id=%d user_id=%d: %w", gameID, userID, err)
	}
	if ra == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// UpdatePlayerHandIfEmpty sets the hand only when it is still the default '[]'.
// This makes initial dealing persistence idempotent.
func UpdatePlayerHandIfEmpty(db *sql.DB, gameID, userID int64, handJSON string) (bool, error) {
	res, err := db.Exec(`UPDATE game_players SET hand = ? WHERE game_id = ? AND user_id = ? AND hand = '[]'`, handJSON, gameID, userID)
	if err != nil {
		return false, err
	}
	ra, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	return ra > 0, nil
}

func UpdatePlayerHandIfEmptyTx(tx *sql.Tx, gameID, userID int64, handJSON string) (bool, error) {
	res, err := tx.Exec(`UPDATE game_players SET hand = ? WHERE game_id = ? AND user_id = ? AND hand = '[]'`, handJSON, gameID, userID)
	if err != nil {
		return false, err
	}
	ra, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	return ra > 0, nil
}

func UpdatePlayerScore(db *sql.DB, gameID, userID int64, score int64) error {
	res, err := db.Exec(`UPDATE game_players SET score = ? WHERE game_id = ? AND user_id = ?`, score, gameID, userID)
	if err != nil {
		return fmt.Errorf("update player score: game_id=%d user_id=%d: %w", gameID, userID, err)
	}
	ra, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("update player score rows affected: game_id=%d user_id=%d: %w", gameID, userID, err)
	}
	if ra == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
