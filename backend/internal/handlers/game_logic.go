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

	st, ok := defaultGameManager.Get(gameID)
	if !ok {
		st = cribbage.NewState(len(players))
		_ = st.Deal()
		defaultGameManager.Set(gameID, st)
	}

	view := *st
	for i := range view.Hands {
		view.Hands[i] = []common.Card{}
	}
	for _, gp := range players {
		if gp.UserID == userID {
			var yourHand []common.Card
			_ = json.Unmarshal([]byte(gp.Hand), &yourHand)
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
	st, ok := defaultGameManager.Get(gameID)
	if !ok {
		st = cribbage.NewState(len(players))
		_ = st.Deal()
		defaultGameManager.Set(gameID, st)
	}
	view := *st
	for i := range view.Hands {
		view.Hands[i] = []common.Card{}
	}
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

	st, ok := defaultGameManager.Get(gameID)
	if !ok {
		st = cribbage.NewState(len(players))
		_ = st.Deal()
		defaultGameManager.Set(gameID, st)
	}

	// Load player's current hand from DB for action validation and to keep UI consistent.
	var hand []common.Card
	for _, gp := range players {
		if gp.UserID == userID {
			_ = json.Unmarshal([]byte(gp.Hand), &hand)
			break
		}
	}
	if int(pos) < len(st.Hands) {
		st.Hands[pos] = hand
	}

	switch req.Type {
	case "discard":
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
		if b, err := json.Marshal(st.Hands[pos]); err == nil {
			_ = models.UpdatePlayerHand(db, gameID, userID, string(b))
		}
		_, _ = models.InsertMove(db, models.GameMove{GameID: gameID, PlayerID: userID, MoveType: "discard"})
		return map[string]any{"ok": true}, nil
	case "play_card":
		card, err := common.ParseCard(req.Card)
		if err != nil {
			return nil, errors.New("invalid card")
		}
		points, reasons, err := st.PlayPeggingCard(int(pos), card)
		if err != nil {
			return nil, err
		}
		if b, err := json.Marshal(st.Hands[pos]); err == nil {
			_ = models.UpdatePlayerHand(db, gameID, userID, string(b))
		}
		cardStr := card.String()
		verified := int64(points)
		_, _ = models.InsertMove(db, models.GameMove{GameID: gameID, PlayerID: userID, MoveType: "play_card", CardPlayed: &cardStr, ScoreVerified: &verified})
		return map[string]any{"points": points, "reasons": reasons, "total": st.PeggingTotal}, nil
	case "go":
		awarded, err := st.Go(int(pos))
		if err != nil {
			return nil, err
		}
		verified := int64(awarded)
		_, _ = models.InsertMove(db, models.GameMove{GameID: gameID, PlayerID: userID, MoveType: "go", ScoreVerified: &verified})
		return map[string]any{"awarded": awarded}, nil
	default:
		return nil, errors.New("unknown move type")
	}
}


