package handlers

import (
	"database/sql"
	"encoding/json"
	"errors"

	"fifteen-thirty-one-go/backend/internal/game/common"
	"fifteen-thirty-one-go/backend/internal/game/cribbage"
	"fifteen-thirty-one-go/backend/internal/models"
)

type GameSnapshot struct {
	Game    *models.Game        `json:"game"`
	Players []models.GamePlayer `json:"players"`
	State   cribbage.State      `json:"state"`
}

func BuildGameSnapshotForUser(db *sql.DB, gameID int64, userID int64) (*GameSnapshot, error) {
	g, err := models.GetGameByID(db, gameID)
	if err != nil {
		return nil, err
	}
	players, err := models.ListGamePlayersByGame(db, gameID)
	if err != nil {
		return nil, err
	}
	if len(players) == 0 {
		return nil, errors.New("no players")
	}

	st, unlock, err := ensureGameStateLocked(db, gameID, players)
	if err != nil {
		return nil, err
	}
	view := cloneStateForView(st)
	unlock()

	for _, gp := range players {
		if gp.UserID == userID {
			var yourHand []common.Card
			if err := json.Unmarshal([]byte(gp.Hand), &yourHand); err != nil {
				return nil, err
			}
			if int(gp.Position) < len(view.Hands) {
				view.Hands[gp.Position] = yourHand
			}
		}
	}

	return &GameSnapshot{
		Game:    g,
		Players: players,
		State:   view,
	}, nil
}

func BuildGameSnapshotPublic(db *sql.DB, gameID int64) (*GameSnapshot, error) {
	g, err := models.GetGameByID(db, gameID)
	if err != nil {
		return nil, err
	}
	players, err := models.ListGamePlayersByGame(db, gameID)
	if err != nil {
		return nil, err
	}
	if len(players) == 0 {
		return nil, errors.New("no players")
	}
	st, unlock, err := ensureGameStateLocked(db, gameID, players)
	if err != nil {
		return nil, err
	}
	view := cloneStateForView(st)
	unlock()
	return &GameSnapshot{Game: g, Players: players, State: view}, nil
}

func ApplyMove(db *sql.DB, gameID int64, userID int64, req moveRequest) (any, error) {
	players, err := models.ListGamePlayersByGame(db, gameID)
	if err != nil {
		return nil, err
	}
	pos := int64(-1)
	for _, p := range players {
		if p.UserID == userID {
			pos = p.Position
			break
		}
	}
	if pos < 0 {
		return nil, errors.New("not a player in this game")
	}

	st, unlock, err := ensureGameStateLocked(db, gameID, players)
	if err != nil {
		return nil, err
	}
	defer unlock()

	// Load player's current hand from DB for action validation and to keep UI consistent.
	var hand []common.Card
	for _, gp := range players {
		if gp.UserID == userID {
			if err := json.Unmarshal([]byte(gp.Hand), &hand); err != nil {
				return nil, err
			}
			break
		}
	}
	if int(pos) < len(st.Hands) {
		st.Hands[pos] = hand
	}

	// Turn validation (pegging). Discard stage currently doesn't track per-player discard completion.
	if req.Type == "play_card" || req.Type == "go" {
		if st.Stage != "pegging" {
			return nil, errors.New("not in pegging stage")
		}
		if st.CurrentIndex != int(pos) {
			return nil, errors.New("not your turn")
		}
	}

	switch req.Type {
	case "discard":
		tx, err := db.Begin()
		if err != nil {
			return nil, err
		}
		defer tx.Rollback()

		var discards []common.Card
		for _, s := range req.Cards {
			card, err := common.ParseCard(s)
			if err != nil {
				return nil, errors.New("invalid card")
			}
			discards = append(discards, card)
		}
		if err := st.Discard(int(pos), discards); err != nil {
			return nil, err
		}
		b, err := json.Marshal(st.Hands[pos])
		if err != nil {
			return nil, err
		}
		if err := models.UpdatePlayerHandTx(tx, gameID, userID, string(b)); err != nil {
			return nil, err
		}
		if err := models.InsertMoveTx(tx, models.GameMove{GameID: gameID, PlayerID: userID, MoveType: "discard"}); err != nil {
			return nil, err
		}
		if err := tx.Commit(); err != nil {
			return nil, err
		}
		return map[string]any{"ok": true}, nil
	case "play_card":
		tx, err := db.Begin()
		if err != nil {
			return nil, err
		}
		defer tx.Rollback()

		card, err := common.ParseCard(req.Card)
		if err != nil {
			return nil, errors.New("invalid card")
		}
		points, reasons, err := st.PlayPeggingCard(int(pos), card)
		if err != nil {
			return nil, err
		}
		b, err := json.Marshal(st.Hands[pos])
		if err != nil {
			return nil, err
		}
		if err := models.UpdatePlayerHandTx(tx, gameID, userID, string(b)); err != nil {
			return nil, err
		}
		cardStr := card.String()
		verified := int64(points)
		if err := models.InsertMoveTx(tx, models.GameMove{GameID: gameID, PlayerID: userID, MoveType: "play_card", CardPlayed: &cardStr, ScoreVerified: &verified}); err != nil {
			return nil, err
		}
		if err := tx.Commit(); err != nil {
			return nil, err
		}
		return map[string]any{"points": points, "reasons": reasons, "total": st.PeggingTotal}, nil
	case "go":
		tx, err := db.Begin()
		if err != nil {
			return nil, err
		}
		defer tx.Rollback()

		awarded, err := st.Go(int(pos))
		if err != nil {
			return nil, err
		}
		verified := int64(awarded)
		if err := models.InsertMoveTx(tx, models.GameMove{GameID: gameID, PlayerID: userID, MoveType: "go", ScoreVerified: &verified}); err != nil {
			return nil, err
		}
		if err := tx.Commit(); err != nil {
			return nil, err
		}
		return map[string]any{"awarded": awarded}, nil
	default:
		return nil, errors.New("unknown move type")
	}
}

func ensureGameStateLocked(db *sql.DB, gameID int64, players []models.GamePlayer) (*cribbage.State, func(), error) {
	playerCount := len(players)
	return defaultGameManager.GetOrCreateLocked(gameID, func() (*cribbage.State, error) {
		tmp := cribbage.NewState(playerCount)
		if err := tmp.Deal(); err != nil {
			return nil, err
		}

		// Persist initial dealt hands immediately so a restart doesn't lose the deal.
		// This is idempotent: it only updates rows whose hand is still the default '[]'.
		for _, p := range players {
			pos := int(p.Position)
			if pos < 0 || pos >= len(tmp.Hands) {
				return nil, errors.New("invalid player position")
			}
			b, err := json.Marshal(tmp.Hands[pos])
			if err != nil {
				return nil, err
			}
			if _, err := models.UpdatePlayerHandIfEmpty(db, gameID, p.UserID, string(b)); err != nil {
				return nil, err
			}
		}

		return tmp, nil
	})
}


