package handlers

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"

	"fifteen-thirty-one-go/backend/internal/game/common"
	"fifteen-thirty-one-go/backend/internal/game/cribbage"
	"fifteen-thirty-one-go/backend/internal/models"
)

// StateValidationResult captures the outcome of validating the runtime game state
// against the persisted database state for consistency checking.
type StateValidationResult struct {
	GameID            int64    `json:"game_id"`
	Consistent        bool     `json:"consistent"`
	VersionMatch      bool     `json:"version_match"`
	StageMatch        bool     `json:"stage_match"`
	ScoresMatch       bool     `json:"scores_match"`
	HandCountsMatch   bool     `json:"hand_counts_match"`
	Discrepancies     []string `json:"discrepancies,omitempty"`
}

func ValidateRuntimeVsDB(db *sql.DB, gameID int64) (*StateValidationResult, error) {
	result := &StateValidationResult{
		GameID:     gameID,
		Consistent: true,
	}

	raw, dbVersion, ok, err := models.GetGameStateJSON(db, gameID)
	if err != nil {
		return nil, fmt.Errorf("get persisted state: %w", err)
	}
	if !ok {
		result.Consistent = false
		result.Discrepancies = append(result.Discrepancies, "no persisted state found")
		return result, nil
	}

	var dbState cribbage.State
	if err := json.Unmarshal([]byte(raw), &dbState); err != nil {
		return nil, fmt.Errorf("unmarshal persisted state: %w", err)
	}

	st, unlock, ok := defaultGameManager.GetLocked(gameID)
	if !ok {
		result.Consistent = false
		result.Discrepancies = append(result.Discrepancies, "no runtime state found")
		return result, nil
	}

	runtimeVersion := st.Version
	runtimeStage := st.Stage
	runtimeScores := append([]int(nil), st.Scores...)
	runtimeHandLens := make([]int, len(st.Hands))
	for i, h := range st.Hands {
		runtimeHandLens[i] = len(h)
	}
	unlock()

	if runtimeVersion == dbVersion {
		result.VersionMatch = true
	} else {
		result.Consistent = false
		result.VersionMatch = false
		result.Discrepancies = append(result.Discrepancies,
			fmt.Sprintf("version mismatch: runtime=%d db=%d", runtimeVersion, dbVersion))
	}

	if runtimeStage == dbState.Stage {
		result.StageMatch = true
	} else {
		result.Consistent = false
		result.StageMatch = false
		result.Discrepancies = append(result.Discrepancies,
			fmt.Sprintf("stage mismatch: runtime=%s db=%s", runtimeStage, dbState.Stage))
	}

	scoresOK := len(runtimeScores) == len(dbState.Scores)
	if scoresOK {
		for i := range runtimeScores {
			if runtimeScores[i] != dbState.Scores[i] {
				scoresOK = false
				break
			}
		}
	}
	result.ScoresMatch = scoresOK
	if !scoresOK {
		result.Consistent = false
		result.Discrepancies = append(result.Discrepancies, "scores differ between runtime and DB")
	}

	handsOK := len(runtimeHandLens) == len(dbState.Hands)
	if handsOK {
		for i := range runtimeHandLens {
			if runtimeHandLens[i] != len(dbState.Hands[i]) {
				handsOK = false
				break
			}
		}
	}
	result.HandCountsMatch = handsOK
	if !handsOK {
		result.Consistent = false
		result.Discrepancies = append(result.Discrepancies, "hand counts differ between runtime and DB")
	}

	return result, nil
}

// ReconcileRuntimeState forces the runtime state to match the DB state.
// This is a recovery operation for when drift is detected.
func ReconcileRuntimeState(db *sql.DB, gameID int64) error {
	raw, version, ok, err := models.GetGameStateJSON(db, gameID)
	if err != nil {
		return fmt.Errorf("get persisted state: %w", err)
	}
	if !ok {
		return fmt.Errorf("no persisted state for game %d", gameID)
	}

	var restored cribbage.State
	if err := json.Unmarshal([]byte(raw), &restored); err != nil {
		return fmt.Errorf("unmarshal state: %w", err)
	}
	restored.Version = version

	defaultGameManager.Set(gameID, &restored)
	log.Printf("ReconcileRuntimeState: runtime state overwritten from DB: game_id=%d version=%d", gameID, version)
	return nil
}

// ValidateHandConsistency checks that each player's hand in the runtime state
// contains only valid cards that exist in a standard deck and don't appear in
// the crib, cut, or other players' hands.
func ValidateHandConsistency(st *cribbage.State) []string {
	if st == nil {
		return []string{"nil state"}
	}

	allCards := make(map[string]int)
	var issues []string

	for i, hand := range st.Hands {
		for _, c := range hand {
			key := c.String()
			allCards[key]++
			if allCards[key] > 1 {
				issues = append(issues, fmt.Sprintf("duplicate card %s in player %d hand", key, i))
			}
		}
	}

	for _, c := range st.Crib {
		key := c.String()
		allCards[key]++
		if allCards[key] > 1 {
			issues = append(issues, fmt.Sprintf("duplicate card %s in crib", key))
		}
	}

	if st.Cut != nil {
		key := st.Cut.String()
		allCards[key]++
		if allCards[key] > 1 {
			issues = append(issues, fmt.Sprintf("duplicate card %s as cut", key))
		}
	}

	for _, c := range st.PeggingSeq {
		key := c.String()
		allCards[key]++
		if allCards[key] > 1 {
			issues = append(issues, fmt.Sprintf("duplicate card %s in pegging sequence", key))
		}
	}

	return issues
}

func ValidateMoveRequest(req moveRequest) error {
	switch req.Type {
	case "discard":
		if len(req.Cards) == 0 {
			return fmt.Errorf("discard requires cards")
		}
		for _, s := range req.Cards {
			if _, err := common.ParseCard(s); err != nil {
				return fmt.Errorf("invalid card in discard: %s", s)
			}
		}
	case "play_card":
		if req.Card == "" {
			return fmt.Errorf("play_card requires card")
		}
		if _, err := common.ParseCard(req.Card); err != nil {
			return fmt.Errorf("invalid card: %s", req.Card)
		}
	case "go":
		// no additional validation
	default:
		return fmt.Errorf("unknown move type: %s", req.Type)
	}
	return nil
}

// CompareHands checks if two card slices represent the same hand (order-independent).
func CompareHands(a, b []common.Card) bool {
	if len(a) != len(b) {
		return false
	}
	counts := make(map[string]int)
	for _, c := range a {
		counts[c.String()]++
	}
	for _, c := range b {
		key := c.String()
		counts[key]--
		if counts[key] < 0 {
			return false
		}
	}
	return true
}

func DetectStaleRuntime(db *sql.DB, gameID int64) (bool, string) {
	raw, dbVersion, ok, err := models.GetGameStateJSON(db, gameID)
	if err != nil || !ok {
		return false, "unable to check"
	}

	st, unlock, ok := defaultGameManager.GetLocked(gameID)
	if !ok {
		return true, "no runtime state"
	}
	rtVersion := st.Version
	rtStage := st.Stage
	unlock()

	if rtVersion < dbVersion {
		return true, fmt.Sprintf("runtime version %d < db version %d", rtVersion, dbVersion)
	}

	var parsed struct {
		Stage string `json:"stage"`
	}
	json.Unmarshal([]byte(raw), &parsed)

	if rtStage != parsed.Stage {
		return true, fmt.Sprintf("runtime stage %q != db stage %q", rtStage, parsed.Stage)
	}

	return false, ""
}
