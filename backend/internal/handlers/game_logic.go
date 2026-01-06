package handlers

import (
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
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
	view := CloneStateForView(st)

	// Best-effort fallback: if the DB hand JSON is missing/empty for the requesting player,
	// we can recover from the server-authoritative engine state and re-persist it.
	userPos := int64(-1)
	for _, gp := range players {
		if gp.UserID == userID {
			userPos = gp.Position
			break
		}
	}
	var fallbackHand []common.Card
	if userPos >= 0 && int(userPos) < len(st.Hands) {
		fallbackHand = append([]common.Card(nil), st.Hands[userPos]...)
	}
	unlock()

	for _, gp := range players {
		if gp.UserID == userID {
			var yourHand []common.Card
			if err := json.Unmarshal([]byte(gp.Hand), &yourHand); err != nil {
				return nil, err
			}
			if len(yourHand) == 0 && len(fallbackHand) > 0 {
				// Repair: show the fallback hand and re-persist it to keep DB and runtime aligned.
				yourHand = fallbackHand
				if b, err := json.Marshal(fallbackHand); err == nil {
					if err := models.UpdatePlayerHand(db, gameID, userID, string(b)); err != nil {
						log.Printf("BuildGameSnapshotForUser: best-effort hand repair persist failed: game_id=%d user_id=%d err=%v", gameID, userID, err)
					}
				}
			}
			if int(gp.Position) < len(view.Hands) {
				view.Hands[gp.Position] = yourHand
			}
		}
	}

	// Do NOT leak opponent hand contents to the client. Provide hand_count for all players,
	// and blank out Hand for non-requesting players.
	//
	// We use the server-authoritative runtime hand lengths as the source of truth.
	st2, unlock2, err := ensureGameStateLocked(db, gameID, players)
	if err == nil {
		for i := range players {
			pos := int(players[i].Position)
			if pos >= 0 && pos < len(st2.Hands) {
				n := int64(len(st2.Hands[pos]))
				players[i].HandCount = &n
			}
			if players[i].UserID != userID {
				players[i].Hand = "[]"
			}
		}
		unlock2()
	} else {
		// If runtime state isn't available, still blank opponent hand contents (best-effort).
		for i := range players {
			if players[i].UserID != userID {
				players[i].Hand = "[]"
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
	view := CloneStateForView(st)
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
		prevStage := st.Stage
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

		// If the engine dealt a new round (pegging -> discard), we must persist the new dealt
		// hands for all players; otherwise clients (and bots) will keep seeing stale/empty hands.
		dealtNewRound := prevStage == "pegging" && working.Stage == "discard"

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
		if dealtNewRound {
			for _, p := range players {
				posIdx := int(p.Position)
				if posIdx < 0 || posIdx >= len(working.Hands) {
					return nil, models.ErrInvalidPlayerPosition
				}
				b, err := json.Marshal(working.Hands[posIdx])
				if err != nil {
					return nil, err
				}
				if err := models.UpdatePlayerHandTx(tx, gameID, p.UserID, string(b)); err != nil {
					return nil, err
				}
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
		if err != nil {
			// This should be rare; failing to re-acquire can leave runtime state stale even though DB committed.
			// Make it observable and attempt a best-effort runtime reload from the DB snapshot.
			log.Printf("ApplyMove: failed to re-acquire game state lock after commit; game_id=%d players=%+v err=%v", gameID, players, err)
			if raw, v, ok, rerr := models.GetGameStateJSON(db, gameID); rerr != nil {
				log.Printf("ApplyMove: best-effort runtime reload failed (GetGameStateJSON); game_id=%d err=%v", gameID, rerr)
			} else if ok {
				var restored cribbage.State
				if uerr := json.Unmarshal([]byte(raw), &restored); uerr != nil {
					log.Printf("ApplyMove: best-effort runtime reload failed (unmarshal state_json); game_id=%d err=%v", gameID, uerr)
				} else {
					restored.Version = v
					defaultGameManager.Set(gameID, &restored)
				}
			}
			return resp, nil
		}

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

		return resp, nil
	}

	return nil, models.ErrGameStateConflict
}

func maybeRunBotTurns(db *sql.DB, gameID int64) error {
	// Safety: prevent infinite bot loops.
	const maxSteps = 64

	for step := 0; step < maxSteps; step++ {
		players, err := models.ListGamePlayersByGame(db, gameID)
		if err != nil {
			return err
		}
		if len(players) == 0 {
			return nil
		}

		st, unlock, err := ensureGameStateLocked(db, gameID, players)
		if err != nil {
			// If the game isn't ready (e.g., lobby not full), just stop.
			return nil
		}

		stage := st.Stage
		currentIdx := st.CurrentIndex
		peggingTotal := st.PeggingTotal
		peggingSeq := append([]common.Card(nil), st.PeggingSeq...)
		discardCompleted := append([]bool(nil), st.DiscardCompleted...)
		maxPlayers := st.Rules.MaxPlayers
		unlock()

		// Map position -> player row.
		byPos := make(map[int64]models.GamePlayer, len(players))
		for _, p := range players {
			byPos[p.Position] = p
		}

		switch stage {
		case "discard":
			// Let any bots who haven't discarded yet discard their required cards.
			discardCount := 1
			if maxPlayers == 2 {
				discardCount = 2
			}

			didOne := false
			for _, p := range players {
				if !p.IsBot {
					continue
				}
				pos := int(p.Position)
				if pos < 0 || pos >= len(discardCompleted) {
					continue
				}
				if discardCompleted[pos] {
					continue
				}
				var hand []common.Card
				if err := json.Unmarshal([]byte(p.Hand), &hand); err != nil {
					return err
				}
				diff := cribbage.BotEasy
				if p.BotDifficulty != nil {
					diff = cribbage.BotDifficulty(*p.BotDifficulty)
				}
				discards, err := cribbage.ChooseDiscardN(hand, discardCount, diff)
				if err != nil {
					return err
				}
				var out []string
				for _, c := range discards {
					out = append(out, c.String())
				}
				_, err = ApplyMove(db, gameID, p.UserID, moveRequest{Type: "discard", Cards: out})
				if err != nil {
					return err
				}
				didOne = true
				break
			}
			if didOne {
				continue
			}
			return nil

		case "pegging":
			gp, ok := byPos[int64(currentIdx)]
			if !ok || !gp.IsBot {
				return nil
			}
			var hand []common.Card
			if err := json.Unmarshal([]byte(gp.Hand), &hand); err != nil {
				return err
			}
			diff := cribbage.BotEasy
			if gp.BotDifficulty != nil {
				diff = cribbage.BotDifficulty(*gp.BotDifficulty)
			}
			card, goPlay := cribbage.ChoosePeggingPlay(hand, peggingTotal, peggingSeq, diff)
			var mr moveRequest
			if goPlay {
				mr = moveRequest{Type: "go"}
			} else {
				mr = moveRequest{Type: "play_card", Card: card.String()}
			}
			if _, err := ApplyMove(db, gameID, gp.UserID, mr); err != nil {
				return err
			}
			continue

		default:
			return nil
		}
	}
	return fmt.Errorf("bot loop exceeded max steps (game_id=%d)", gameID)
}

func randSuffix(n int) (string, error) {
	if n <= 0 {
		return "", nil
	}
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	const alphabet = "abcdefghijklmnopqrstuvwxyz0123456789"
	for i := range b {
		b[i] = alphabet[int(b[i])%len(alphabet)]
	}
	return string(b), nil
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
