package models

import (
	"database/sql"
)

type GamePlayer struct {
	GameID        int64   `json:"game_id"`
	UserID        int64   `json:"user_id"`
	Position      int64   `json:"position"`
	Score         int64   `json:"score"`
	Hand          string  `json:"hand"` // JSON array string
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

func ListGamePlayersByGame(db *sql.DB, gameID int64) ([]GamePlayer, error) {
	rows, err := db.Query(
		`SELECT game_id, user_id, position, score, hand, crib_cards, is_bot, bot_difficulty FROM game_players WHERE game_id = ? ORDER BY position ASC`,
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
		var isBotInt int
		var botDiff sql.NullString
		if err := rows.Scan(&gp.GameID, &gp.UserID, &gp.Position, &gp.Score, &gp.Hand, &crib, &isBotInt, &botDiff); err != nil {
			return nil, err
		}
		if crib.Valid {
			v := crib.String
			gp.CribCards = &v
		}
		gp.IsBot = isBotInt != 0
		if botDiff.Valid {
			v := botDiff.String
			gp.BotDifficulty = &v
		}
		out = append(out, gp)
	}
	return out, rows.Err()
}

func UpdatePlayerHand(db *sql.DB, gameID, userID int64, handJSON string) error {
	_, err := db.Exec(`UPDATE game_players SET hand = ? WHERE game_id = ? AND user_id = ?`, handJSON, gameID, userID)
	return err
}

func UpdatePlayerScore(db *sql.DB, gameID, userID int64, score int64) error {
	_, err := db.Exec(`UPDATE game_players SET score = ? WHERE game_id = ? AND user_id = ?`, score, gameID, userID)
	return err
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}


