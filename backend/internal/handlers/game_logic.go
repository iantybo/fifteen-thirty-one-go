package handlers

import (
	"database/sql"
	"encoding/json"
	"errors"
	"strings"

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
		return nil, models.ErrNotAPlayer
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
	working := cloneStateDeep(st)
	if int(pos) < len(working.Hands) {
		working.Hands[pos] = hand
	}

	// Turn validation (pegging). Discard stage currently doesn't track per-player discard completion.
	if req.Type == "play_card" || req.Type == "go" {
		if working.Stage != "pegging" {
			return nil, models.ErrNotInPeggingStage
		}
		if working.CurrentIndex != int(pos) {
			return nil, models.ErrNotYourTurn
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
				return nil, models.ErrInvalidCard
			}
			discards = append(discards, card)
		}
		if err := (&working).Discard(int(pos), discards); err != nil {
			return nil, err
		}
		b, err := json.Marshal(working.Hands[pos])
		if err != nil {
			return nil, err
		}
		if err := models.UpdatePlayerHandTx(tx, gameID, userID, string(b)); err != nil {
			return nil, err
		}
		if err := models.InsertMoveTx(tx, models.GameMove{GameID: gameID, PlayerID: userID, MoveType: "discard"}); err != nil {
			return nil, err
		}
		sb, err := json.Marshal(working)
		if err != nil {
			return nil, err
		}
		if err := models.UpdateGameStateTx(tx, gameID, string(sb)); err != nil {
			return nil, err
		}
		if err := tx.Commit(); err != nil {
			return nil, err
		}
		*st = working
		return map[string]any{"ok": true}, nil
	case "play_card":
		tx, err := db.Begin()
		if err != nil {
			return nil, err
		}
		defer tx.Rollback()

		card, err := common.ParseCard(req.Card)
		if err != nil {
			return nil, models.ErrInvalidCard
		}
		points, reasons, err := (&working).PlayPeggingCard(int(pos), card)
		if err != nil {
			return nil, err
		}
		b, err := json.Marshal(working.Hands[pos])
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
		sb, err := json.Marshal(working)
		if err != nil {
			return nil, err
		}
		if err := models.UpdateGameStateTx(tx, gameID, string(sb)); err != nil {
			return nil, err
		}
		if err := tx.Commit(); err != nil {
			return nil, err
		}
		*st = working
		return map[string]any{"points": points, "reasons": reasons, "total": working.PeggingTotal}, nil
	case "go":
		tx, err := db.Begin()
		if err != nil {
			return nil, err
		}
		defer tx.Rollback()

		awarded, err := (&working).Go(int(pos))
		if err != nil {
			return nil, err
		}
		verified := int64(awarded)
		if err := models.InsertMoveTx(tx, models.GameMove{GameID: gameID, PlayerID: userID, MoveType: "go", ScoreVerified: &verified}); err != nil {
			return nil, err
		}
		sb, err := json.Marshal(working)
		if err != nil {
			return nil, err
		}
		if err := models.UpdateGameStateTx(tx, gameID, string(sb)); err != nil {
			return nil, err
		}
		if err := tx.Commit(); err != nil {
			return nil, err
		}
		*st = working
		return map[string]any{"awarded": awarded}, nil
	default:
		return nil, models.ErrUnknownMoveType
	}
}

func ensureGameStateLocked(db *sql.DB, gameID int64, players []models.GamePlayer) (*cribbage.State, func(), error) {
	playerCount := len(players)
	return defaultGameManager.GetOrCreateLocked(gameID, func() (*cribbage.State, error) {
		// Prefer restoring the full persisted engine state when available.
		if raw, ok, err := models.GetGameStateJSON(db, gameID); err != nil {
			return nil, err
		} else if ok {
			var restored cribbage.State
			if err := json.Unmarshal([]byte(raw), &restored); err != nil {
				// If we have persisted state but can't decode it, we cannot safely resume.
				return nil, err
			}
			// Sanity: if this doesn't match the current lobby size, we cannot safely resume.
			if restored.Rules.MaxPlayers != playerCount ||
				len(restored.Hands) != playerCount ||
				len(restored.KeptHands) != playerCount ||
				len(restored.Scores) != playerCount {
				return nil, models.ErrInvalidJSON
			}
			return &restored, nil
		}

		tmp := cribbage.NewState(playerCount)
		// If hands already exist in DB (e.g., after restart) but full state is missing,
		// do NOT attempt to resume: hands alone are insufficient to reconstruct the game.
		hasHands := false
		for _, p := range players {
			if strings.TrimSpace(p.Hand) != "" && strings.TrimSpace(p.Hand) != "[]" {
				hasHands = true
				break
			}
		}
		if hasHands {
			// Without a full persisted engine state, we cannot safely resume an in-progress game.
			// (Hands alone are insufficient to reconstruct deck/cut/crib/scores/pegging history/etc.)
			return nil, models.ErrGameStateMissing
		}

		if err := tmp.Deal(); err != nil {
			return nil, err
		}

		// Persist initial dealt hands immediately so a restart doesn't lose the deal.
		// This is idempotent: it only updates rows whose hand is still the default '[]'.
		tx, err := db.Begin()
		if err != nil {
			return nil, err
		}
		defer func() {
			// rollback is safe even after commit
			_ = tx.Rollback()
		}()
		for _, p := range players {
			pos := int(p.Position)
			if pos < 0 || pos >= len(tmp.Hands) {
				return nil, models.ErrInvalidPlayerPosition
			}
			b, err := json.Marshal(tmp.Hands[pos])
			if err != nil {
				return nil, err
			}
			if _, err := models.UpdatePlayerHandIfEmptyTx(tx, gameID, p.UserID, string(b)); err != nil {
				return nil, err
			}
		}
		// Persist the full engine state (including deck/cut/crib/scores) for restart recovery.
		sb, err := json.Marshal(tmp)
		if err != nil {
			return nil, err
		}
		if err := models.UpdateGameStateTx(tx, gameID, string(sb)); err != nil {
			return nil, err
		}
		if err := tx.Commit(); err != nil {
			return nil, err
		}

		return tmp, nil
	})
}

func cloneStateDeep(st *cribbage.State) cribbage.State {
	if st == nil {
		return cribbage.State{}
	}
	var out cribbage.State
	out.Rules = st.Rules
	out.DealerIndex = st.DealerIndex
	out.CurrentIndex = st.CurrentIndex
	out.LastPlayIndex = st.LastPlayIndex
	out.PeggingTotal = st.PeggingTotal
	out.Stage = st.Stage
	if st.Cut != nil {
		c := *st.Cut
		out.Cut = &c
	}
	if st.Deck != nil {
		out.Deck = append([]common.Card(nil), st.Deck...)
	}
	if st.Crib != nil {
		out.Crib = append([]common.Card(nil), st.Crib...)
	}
	if st.PeggingSeq != nil {
		out.PeggingSeq = append([]common.Card(nil), st.PeggingSeq...)
	}
	if st.PeggingPassed != nil {
		out.PeggingPassed = append([]bool(nil), st.PeggingPassed...)
	}
	if st.DiscardCompleted != nil {
		out.DiscardCompleted = append([]bool(nil), st.DiscardCompleted...)
	}
	if st.Scores != nil {
		out.Scores = append([]int(nil), st.Scores...)
	}
	out.Hands = make([][]common.Card, len(st.Hands))
	for i := range st.Hands {
		out.Hands[i] = append([]common.Card(nil), st.Hands[i]...)
	}
	out.KeptHands = make([][]common.Card, len(st.KeptHands))
	for i := range st.KeptHands {
		out.KeptHands[i] = append([]common.Card(nil), st.KeptHands[i]...)
	}
	return out
}


