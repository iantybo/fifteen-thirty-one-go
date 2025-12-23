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
	const maxAttempts = 3

	for attempt := 0; attempt < maxAttempts; attempt++ {
		players, err := models.ListGamePlayersByGame(db, gameID)
		if err != nil {
			return nil, err
		}
		pos := int64(-1)
		var hand []common.Card
		for _, p := range players {
			if p.UserID == userID {
				pos = p.Position
				if err := json.Unmarshal([]byte(p.Hand), &hand); err != nil {
					return nil, err
				}
				break
			}
		}
		if pos < 0 {
			return nil, models.ErrNotAPlayer
		}

		// 1) Lock just long enough to validate + compute the move against a consistent runtime snapshot.
		st, unlock, err := ensureGameStateLocked(db, gameID, players)
		if err != nil {
			return nil, err
		}
		baseVersion := st.Version

		working := cloneStateDeep(st)
		working.Version = baseVersion
		if int(pos) < len(working.Hands) {
			working.Hands[pos] = hand
		}

		// Turn validation (pegging).
		if req.Type == "play_card" || req.Type == "go" {
			if working.Stage != "pegging" {
				unlock()
				return nil, models.ErrNotInPeggingStage
			}
			if working.CurrentIndex != int(pos) {
				unlock()
				return nil, models.ErrNotYourTurn
			}
		}

		// Compute the move under the lock.
		var (
			resp    any
			move    models.GameMove
			handOut *string
		)

		switch req.Type {
		case "discard":
			var discards []common.Card
			for _, s := range req.Cards {
				card, err := common.ParseCard(s)
				if err != nil {
					unlock()
					return nil, models.ErrInvalidCard
				}
				discards = append(discards, card)
			}
			if err := (&working).Discard(int(pos), discards); err != nil {
				unlock()
				return nil, err
			}
			b, err := json.Marshal(working.Hands[pos])
			if err != nil {
				unlock()
				return nil, err
			}
			s := string(b)
			handOut = &s
			move = models.GameMove{GameID: gameID, PlayerID: userID, MoveType: "discard"}
			resp = map[string]any{"ok": true}

		case "play_card":
			card, err := common.ParseCard(req.Card)
			if err != nil {
				unlock()
				return nil, models.ErrInvalidCard
			}
			points, reasons, err := (&working).PlayPeggingCard(int(pos), card)
			if err != nil {
				unlock()
				return nil, err
			}
			b, err := json.Marshal(working.Hands[pos])
			if err != nil {
				unlock()
				return nil, err
			}
			s := string(b)
			handOut = &s
			cardStr := card.String()
			verified := int64(points)
			move = models.GameMove{GameID: gameID, PlayerID: userID, MoveType: "play_card", CardPlayed: &cardStr, ScoreVerified: &verified}
			resp = map[string]any{"points": points, "reasons": reasons, "total": working.PeggingTotal}

		case "go":
			awarded, err := (&working).Go(int(pos))
			if err != nil {
				unlock()
				return nil, err
			}
			verified := int64(awarded)
			move = models.GameMove{GameID: gameID, PlayerID: userID, MoveType: "go", ScoreVerified: &verified}
			resp = map[string]any{"awarded": awarded}

		default:
			unlock()
			return nil, models.ErrUnknownMoveType
		}

		// Copy the computed state and release the per-game lock before DB I/O.
		unlock()

		// 2) Persist the computed changes in a transaction, using optimistic (version) checks.
		tx, err := db.Begin()
		if err != nil {
			return nil, err
		}
		committed := false
		defer func() {
			if !committed {
				_ = tx.Rollback()
			}
		}()

		if handOut != nil {
			if err := models.UpdatePlayerHandTx(tx, gameID, userID, *handOut); err != nil {
				return nil, err
			}
		}
		if err := models.InsertMoveTx(tx, move); err != nil {
			return nil, err
		}
		sb, err := json.Marshal(working)
		if err != nil {
			return nil, err
		}
		if err := models.UpdateGameStateTxCAS(tx, gameID, baseVersion, string(sb)); err != nil {
			// Another move committed first; retry from latest state.
			if errors.Is(err, models.ErrGameStateConflict) && attempt < maxAttempts-1 {
				_ = tx.Rollback()
				continue
			}
			return nil, err
		}
		if err := tx.Commit(); err != nil {
			return nil, err
		}
		committed = true

		// 3) Re-acquire the game lock and apply the committed state to runtime memory.
		newVersion := baseVersion + 1
		working.Version = newVersion

		st2, unlock2, err := ensureGameStateLocked(db, gameID, players)
		if err == nil {
			if st2.Version == baseVersion {
				*st2 = working
			} else if st2.Version != newVersion {
				// Unexpected: don't overwrite newer runtime state. Best-effort: reload runtime from DB.
				if raw, v, ok, rerr := models.GetGameStateJSON(db, gameID); rerr == nil && ok {
					var restored cribbage.State
					if uerr := json.Unmarshal([]byte(raw), &restored); uerr == nil {
						restored.Version = v
						*st2 = restored
					}
				}
			}
			unlock2()
		}

		return resp, nil
	}

	return nil, models.ErrGameStateConflict
}

func ensureGameStateLocked(db *sql.DB, gameID int64, players []models.GamePlayer) (*cribbage.State, func(), error) {
	playerCount := len(players)
	return defaultGameManager.GetOrCreateLocked(gameID, func() (*cribbage.State, error) {
		// Prefer restoring the full persisted engine state when available.
		if raw, ver, ok, err := models.GetGameStateJSON(db, gameID); err != nil {
			return nil, err
		} else if ok {
			var restored cribbage.State
			if err := json.Unmarshal([]byte(raw), &restored); err != nil {
				// If we have persisted state but can't decode it, we cannot safely resume.
				return nil, err
			}
			restored.Version = ver
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
		// Refresh the optimistic version from DB (UpdateGameStateTx increments it).
		var ver int64
		if err := db.QueryRow(`SELECT state_version FROM games WHERE id = ?`, gameID).Scan(&ver); err == nil {
			tmp.Version = ver
		} else {
			// Best-effort fallback for new games.
			tmp.Version = 1
		}
		return tmp, nil
	})
}

func cloneStateDeep(st *cribbage.State) cribbage.State {
	if st == nil {
		return cribbage.State{}
	}
	var out cribbage.State
	out.Version = st.Version
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


