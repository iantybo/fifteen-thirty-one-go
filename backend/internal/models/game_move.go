package models

import (
	"database/sql"
	"errors"
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
	var isCorrInt int
	err := db.QueryRow(
		`SELECT id, game_id, player_id, move_type, card_played, score_claimed, score_verified, is_corrected, created_at FROM game_moves WHERE id = ?`,
		id,
	).Scan(&m.ID, &m.GameID, &m.PlayerID, &m.MoveType, &card, &sc, &sv, &isCorrInt, &m.CreatedAt)
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
	m.IsCorrected = isCorrInt != 0
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
		var isCorrInt int
		if err := rows.Scan(&m.ID, &m.GameID, &m.PlayerID, &m.MoveType, &card, &sc, &sv, &isCorrInt, &m.CreatedAt); err != nil {
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
		m.IsCorrected = isCorrInt != 0
		out = append(out, m)
	}
	return out, rows.Err()
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


