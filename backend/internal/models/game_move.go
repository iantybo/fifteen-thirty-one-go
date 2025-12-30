package models

import (
	"database/sql"
	"errors"
	"strings"
	"time"
)

type GameMove struct {
	ID            int64      `json:"id"`
	GameID        int64      `json:"game_id"`
	PlayerID      int64      `json:"player_id"`
	MoveType      string     `json:"move_type"`
	CardPlayed    *string    `json:"card_played,omitempty"`
	ScoreClaimed  *int64     `json:"score_claimed,omitempty"`
	ScoreVerified *int64     `json:"score_verified,omitempty"`
	IsCorrected   bool       `json:"is_corrected"`
	CreatedAt     time.Time  `json:"created_at"`
}

func InsertMove(db *sql.DB, m GameMove) (*GameMove, error) {
	res, err := db.Exec(
		`INSERT INTO game_moves(game_id, player_id, move_type, card_played, score_claimed, score_verified, is_corrected) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		m.GameID, m.PlayerID, m.MoveType, m.CardPlayed, m.ScoreClaimed, m.ScoreVerified, boolToInt(m.IsCorrected),
	)
	if err != nil {
		return nil, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}
	return GetMoveByID(db, id)
}

func InsertMoveTx(tx *sql.Tx, m GameMove) error {
	_, err := tx.Exec(
		`INSERT INTO game_moves(game_id, player_id, move_type, card_played, score_claimed, score_verified, is_corrected) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		m.GameID, m.PlayerID, m.MoveType, m.CardPlayed, m.ScoreClaimed, m.ScoreVerified, boolToInt(m.IsCorrected),
	)
	return err
}

func GetMoveByID(db *sql.DB, id int64) (*GameMove, error) {
	var m GameMove
	var card sql.NullString
	var sc sql.NullInt64
	var sv sql.NullInt64
	var isCorrVal any
	err := db.QueryRow(
		`SELECT id, game_id, player_id, move_type, card_played, score_claimed, score_verified, is_corrected, created_at FROM game_moves WHERE id = ?`,
		id,
	).Scan(&m.ID, &m.GameID, &m.PlayerID, &m.MoveType, &card, &sc, &sv, &isCorrVal, &m.CreatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if card.Valid {
		v := card.String
		m.CardPlayed = &v
	}
	if sc.Valid {
		v := sc.Int64
		m.ScoreClaimed = &v
	}
	if sv.Valid {
		v := sv.Int64
		m.ScoreVerified = &v
	}
	m.IsCorrected = parseSQLiteBool(isCorrVal)
	return &m, nil
}

func ListMovesByGame(db *sql.DB, gameID int64, limit int64) ([]GameMove, error) {
	if limit <= 0 || limit > 500 {
		limit = 200
	}
	rows, err := db.Query(
		`SELECT id, game_id, player_id, move_type, card_played, score_claimed, score_verified, is_corrected, created_at
		 FROM game_moves WHERE game_id = ? ORDER BY created_at DESC LIMIT ?`,
		gameID, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []GameMove
	for rows.Next() {
		var m GameMove
		var card sql.NullString
		var sc sql.NullInt64
		var sv sql.NullInt64
		var isCorrVal any
		if err := rows.Scan(&m.ID, &m.GameID, &m.PlayerID, &m.MoveType, &card, &sc, &sv, &isCorrVal, &m.CreatedAt); err != nil {
			return nil, err
		}
		if card.Valid {
			v := card.String
			m.CardPlayed = &v
		}
		if sc.Valid {
			v := sc.Int64
			m.ScoreClaimed = &v
		}
		if sv.Valid {
			v := sv.Int64
			m.ScoreVerified = &v
		}
		m.IsCorrected = parseSQLiteBool(isCorrVal)
		out = append(out, m)
	}
	return out, rows.Err()
}

func parseSQLiteBool(v any) bool {
	// SQLite boolean handling is driver-dependent: we may see int64(0/1), bool, or string/[]byte.
	switch x := v.(type) {
	case int64:
		return x != 0
	case int:
		return x != 0
	case bool:
		return x
	case []byte:
		s := strings.TrimSpace(strings.ToLower(string(x)))
		return s == "1" || s == "true" || s == "t"
	case string:
		s := strings.TrimSpace(strings.ToLower(x))
		return s == "1" || s == "true" || s == "t"
	default:
		return false
	}
}

// HasUncorrectedMoveType returns true if there exists a move for the given game/player/type
// that has not been marked corrected.
func HasUncorrectedMoveType(db *sql.DB, gameID, playerID int64, moveType string) (bool, error) {
	var one int
	err := db.QueryRow(
		`SELECT 1
		 FROM game_moves
		 WHERE game_id = ? AND player_id = ? AND move_type = ? AND is_corrected = 0
		 LIMIT 1`,
		gameID, playerID, moveType,
	).Scan(&one)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func MarkMoveAsCorrected(db *sql.DB, moveID int64) error {
	res, err := db.Exec(`UPDATE game_moves SET is_corrected = 1 WHERE id = ?`, moveID)
	if err != nil {
		return err
	}
	ra, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if ra == 0 {
		return ErrNotFound
	}
	return nil
}

func MarkMoveAsCorrectedTx(tx *sql.Tx, moveID int64) error {
	res, err := tx.Exec(`UPDATE game_moves SET is_corrected = 1 WHERE id = ?`, moveID)
	if err != nil {
		return err
	}
	ra, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if ra == 0 {
		return ErrNotFound
	}
	return nil
}


