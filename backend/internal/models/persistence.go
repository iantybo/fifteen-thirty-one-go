package models

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"
)

// PersistenceConfig holds tuning parameters for game state persistence operations.
type PersistenceConfig struct {
	MaxRetries     int
	RetryBaseDelay time.Duration
	BatchSize      int
}

var DefaultPersistenceConfig = PersistenceConfig{
	MaxRetries:     3,
	RetryBaseDelay: 50 * time.Millisecond,
	BatchSize:      25,
}

// GameStatePersister handles atomic persistence of game engine state alongside
// player hand updates. It encapsulates the transaction management, CAS checks,
// and retry logic that was previously duplicated across handler functions.
type GameStatePersister struct {
	db  *sql.DB
	cfg PersistenceConfig
}

func NewGameStatePersister(db *sql.DB, cfg PersistenceConfig) *GameStatePersister {
	return &GameStatePersister{db: db, cfg: cfg}
}

// PersistStateChange atomically persists a game state change: updates the engine
// state JSON via CAS, records the move, and optionally updates player hands.
// Returns the new state version on success.
func (p *GameStatePersister) PersistStateChange(
	ctx context.Context,
	gameID int64,
	baseVersion int64,
	stateJSON string,
	move GameMove,
	handUpdates map[int64]string,
) (int64, error) {
	for attempt := 0; attempt < p.cfg.MaxRetries; attempt++ {
		tx, err := p.db.BeginTx(ctx, nil)
		if err != nil {
			return 0, fmt.Errorf("begin transaction: %w", err)
		}
		committed := false
		defer func() {
			if !committed {
				tx.Rollback()
			}
		}()

		for userID, handJSON := range handUpdates {
			if err := UpdatePlayerHandTx(tx, gameID, userID, handJSON); err != nil {
				return 0, fmt.Errorf("update player hand (user_id=%d): %w", userID, err)
			}
		}

		if err := InsertMoveTx(tx, move); err != nil {
			return 0, fmt.Errorf("insert move: %w", err)
		}

		if err := UpdateGameStateTxCAS(tx, gameID, baseVersion, stateJSON); err != nil {
			if err == ErrGameStateConflict && attempt < p.cfg.MaxRetries-1 {
				tx.Rollback()
				time.Sleep(p.cfg.RetryBaseDelay * time.Duration(attempt+1))
				continue
			}
			return 0, fmt.Errorf("update game state CAS: %w", err)
		}

		if err := tx.Commit(); err != nil {
			return 0, fmt.Errorf("commit transaction: %w", err)
		}
		committed = true

		return baseVersion + 1, nil
	}
	return 0, ErrGameStateConflict
}

// PersistNewRoundHands persists all player hands after a new round is dealt.
// This is a batch operation within a single transaction.
func (p *GameStatePersister) PersistNewRoundHands(
	ctx context.Context,
	gameID int64,
	playerHands map[int64]string,
) error {
	tx, err := p.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	for userID, handJSON := range playerHands {
		if err := UpdatePlayerHandTx(tx, gameID, userID, handJSON); err != nil {
			return fmt.Errorf("update player hand (user_id=%d): %w", userID, err)
		}
	}

	return tx.Commit()
}

// BatchUpdatePlayerScores updates scores for all players in a game from the
// engine state. This replaces individual score update calls with a single
// transaction.
func (p *GameStatePersister) BatchUpdatePlayerScores(
	ctx context.Context,
	gameID int64,
	scoresByUserID map[int64]int64,
) error {
	tx, err := p.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	for userID, score := range scoresByUserID {
		res, err := tx.ExecContext(ctx,
			`UPDATE game_players SET score = ? WHERE game_id = ? AND user_id = ?`,
			score, gameID, userID,
		)
		if err != nil {
			return fmt.Errorf("update score (user_id=%d): %w", userID, err)
		}
		ra, _ := res.RowsAffected()
		if ra == 0 {
			log.Printf("BatchUpdatePlayerScores: no rows affected for user_id=%d game_id=%d (player may have left)", userID, gameID)
		}
	}

	return tx.Commit()
}

// GameEventType represents the type of game lifecycle event for audit logging.
type GameEventType string

const (
	EventGameCreated  GameEventType = "game_created"
	EventGameStarted  GameEventType = "game_started"
	EventRoundDealt   GameEventType = "round_dealt"
	EventGameFinished GameEventType = "game_finished"
	EventPlayerJoined GameEventType = "player_joined"
	EventPlayerLeft   GameEventType = "player_left"
	EventBotAdded     GameEventType = "bot_added"
)

// GameEvent represents a lifecycle event for audit logging and observability.
type GameEvent struct {
	ID        int64         `json:"id"`
	GameID    int64         `json:"game_id"`
	EventType GameEventType `json:"event_type"`
	PlayerID  *int64        `json:"player_id,omitempty"`
	Metadata  string        `json:"metadata,omitempty"`
	CreatedAt time.Time     `json:"created_at"`
}

func RecordGameEvent(db *sql.DB, gameID int64, eventType GameEventType, playerID *int64, metadata map[string]any) {
	metaJSON := "{}"
	if metadata != nil {
		if b, err := json.Marshal(metadata); err == nil {
			metaJSON = string(b)
		}
	}
	db.Exec(
		`INSERT INTO game_events(game_id, event_type, player_id, metadata) VALUES (?, ?, ?, ?)`,
		gameID, string(eventType), playerID, metaJSON,
	)
}

func RecordGameEventTx(tx *sql.Tx, gameID int64, eventType GameEventType, playerID *int64, metadata map[string]any) {
	metaJSON := "{}"
	if metadata != nil {
		if b, err := json.Marshal(metadata); err == nil {
			metaJSON = string(b)
		}
	}
	tx.Exec(
		`INSERT INTO game_events(game_id, event_type, player_id, metadata) VALUES (?, ?, ?, ?)`,
		gameID, string(eventType), playerID, metaJSON,
	)
}

// ListGameEvents returns events for a game, ordered newest first.
func ListGameEvents(db *sql.DB, gameID int64, limit int64) ([]GameEvent, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := db.Query(
		`SELECT id, game_id, event_type, player_id, metadata, created_at
		 FROM game_events WHERE game_id = ? ORDER BY created_at DESC LIMIT ?`,
		gameID, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []GameEvent
	for rows.Next() {
		var e GameEvent
		var pid sql.NullInt64
		var meta sql.NullString
		if err := rows.Scan(&e.ID, &e.GameID, &e.EventType, &pid, &meta, &e.CreatedAt); err != nil {
			return nil, err
		}
		if pid.Valid {
			v := pid.Int64
			e.PlayerID = &v
		}
		if meta.Valid {
			e.Metadata = meta.String
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// ValidateStateTransition checks that a stage transition is valid according to
// the cribbage game flow. Returns an error if the transition is not allowed.
func ValidateStateTransition(fromStage, toStage string) error {
	valid := map[string][]string{
		"dealing":  {"discard"},
		"discard":  {"pegging"},
		"pegging":  {"counting", "discard"},
		"counting": {"dealing", "finished"},
		"finished": {},
	}

	allowed, ok := valid[fromStage]
	if !ok {
		return fmt.Errorf("unknown stage: %s", fromStage)
	}
	for _, s := range allowed {
		if s == toStage {
			return nil
		}
	}
	return fmt.Errorf("invalid transition from %s to %s", fromStage, toStage)
}

// PlayerHandSummary holds a compact representation of a player's hand state
// for use in event metadata and debugging.
type PlayerHandSummary struct {
	UserID   int64    `json:"user_id"`
	Position int64    `json:"position"`
	Cards    []string `json:"cards"`
}

func BuildPlayerHandSummaries(db *sql.DB, gameID int64) ([]PlayerHandSummary, error) {
	players, err := ListGamePlayersByGame(db, gameID)
	if err != nil {
		return nil, err
	}

	summaries := make([]PlayerHandSummary, 0, len(players))
	for _, p := range players {
		var cards []string
		if p.Hand != "" && p.Hand != "[]" {
			json.Unmarshal([]byte(p.Hand), &cards)
		}
		summaries = append(summaries, PlayerHandSummary{
			UserID:   p.UserID,
			Position: p.Position,
			Cards:    cards,
		})
	}
	return summaries, nil
}

// GameStateSnapshot captures a point-in-time view of a game for diagnostics.
type GameStateSnapshot struct {
	GameID       int64               `json:"game_id"`
	Stage        string              `json:"stage"`
	Version      int64               `json:"version"`
	Scores       []int               `json:"scores"`
	DealerIndex  int                 `json:"dealer_index"`
	CurrentIndex int                 `json:"current_index"`
	Players      []PlayerHandSummary `json:"players"`
	CapturedAt   time.Time           `json:"captured_at"`
}

func CaptureGameSnapshot(db *sql.DB, gameID int64) (*GameStateSnapshot, error) {
	raw, version, ok, err := GetGameStateJSON(db, gameID)
	if err != nil {
		return nil, fmt.Errorf("get game state: %w", err)
	}
	if !ok {
		return nil, fmt.Errorf("no state for game %d", gameID)
	}

	var parsed struct {
		Stage        string `json:"stage"`
		Scores       []int  `json:"scores"`
		DealerIndex  int    `json:"dealer_index"`
		CurrentIndex int    `json:"current_index"`
	}
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		return nil, fmt.Errorf("unmarshal state: %w", err)
	}

	summaries, _ := BuildPlayerHandSummaries(db, gameID)

	return &GameStateSnapshot{
		GameID:       gameID,
		Stage:        parsed.Stage,
		Version:      version,
		Scores:       parsed.Scores,
		DealerIndex:  parsed.DealerIndex,
		CurrentIndex: parsed.CurrentIndex,
		Players:      summaries,
		CapturedAt:   time.Now().UTC(),
	}, nil
}

// CleanupFinishedGames removes runtime artifacts for games that have been finished
// for longer than the specified duration. This is intended to be called periodically
// to prevent unbounded memory growth in long-running servers.
func CleanupFinishedGames(ctx context.Context, db *sql.DB, olderThan time.Duration) (int64, error) {
	cutoff := time.Now().Add(-olderThan)
	res, err := db.ExecContext(ctx,
		`DELETE FROM game_events WHERE game_id IN (
			SELECT id FROM games WHERE status = 'finished' AND finished_at < ?
		)`,
		cutoff,
	)
	if err != nil {
		return 0, fmt.Errorf("cleanup finished game events: %w", err)
	}
	ra, _ := res.RowsAffected()
	return ra, nil
}

// LobbyPlayerInfo holds denormalized player information for lobby display.
type LobbyPlayerInfo struct {
	UserID      int64  `json:"user_id"`
	Username    string `json:"username"`
	Position    int64  `json:"position"`
	IsBot       bool   `json:"is_bot"`
	IsReady     bool   `json:"is_ready"`
	GamesPlayed int64  `json:"games_played"`
	GamesWon    int64  `json:"games_won"`
}

func GetLobbyPlayersWithStats(db *sql.DB, lobbyID int64) ([]LobbyPlayerInfo, error) {
	var gameID int64
	if err := db.QueryRow(`SELECT id FROM games WHERE lobby_id = ? ORDER BY id DESC LIMIT 1`, lobbyID).Scan(&gameID); err != nil {
		return nil, fmt.Errorf("find game for lobby: %w", err)
	}

	players, err := ListGamePlayersByGame(db, gameID)
	if err != nil {
		return nil, fmt.Errorf("list players: %w", err)
	}

	result := make([]LobbyPlayerInfo, 0, len(players))
	for _, p := range players {
		info := LobbyPlayerInfo{
			UserID:   p.UserID,
			Username: p.Username,
			Position: p.Position,
			IsBot:    p.IsBot,
		}

		var stats struct {
			played int64
			won    int64
		}
		db.QueryRow(
			`SELECT games_played, games_won FROM users WHERE id = ?`,
			p.UserID,
		).Scan(&stats.played, &stats.won)

		info.GamesPlayed = stats.played
		info.GamesWon = stats.won
		result = append(result, info)
	}
	return result, nil
}

func ResetGameForRematch(ctx context.Context, db *sql.DB, gameID int64) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	var status string
	if err := tx.QueryRowContext(ctx, `SELECT status FROM games WHERE id = ?`, gameID).Scan(&status); err != nil {
		return fmt.Errorf("query game status: %w", err)
	}
	if status != "finished" {
		return fmt.Errorf("game must be finished to rematch (current: %s)", status)
	}

	if _, err := tx.ExecContext(ctx,
		`UPDATE games SET status = 'waiting', state_json = NULL, state_version = 0, finished_at = NULL WHERE id = ?`,
		gameID,
	); err != nil {
		return fmt.Errorf("reset game row: %w", err)
	}

	if _, err := tx.ExecContext(ctx,
		`UPDATE game_players SET hand = '[]', score = 0, crib_cards = NULL WHERE game_id = ?`,
		gameID,
	); err != nil {
		return fmt.Errorf("reset player hands: %w", err)
	}

	var lobbyID int64
	if err := tx.QueryRowContext(ctx, `SELECT lobby_id FROM games WHERE id = ?`, gameID).Scan(&lobbyID); err != nil {
		return fmt.Errorf("query lobby id: %w", err)
	}

	if _, err := tx.ExecContext(ctx,
		`UPDATE lobbies SET status = 'waiting' WHERE id = ?`,
		lobbyID,
	); err != nil {
		return fmt.Errorf("reset lobby status: %w", err)
	}

	RecordGameEventTx(tx, gameID, EventGameCreated, nil, map[string]any{"reason": "rematch"})

	return tx.Commit()
}

// GameMoveStats computes aggregate statistics for a game's moves.
type GameMoveStats struct {
	TotalMoves  int64            `json:"total_moves"`
	ByType      map[string]int64 `json:"by_type"`
	ByPlayer    map[int64]int64  `json:"by_player"`
	TotalPoints int64            `json:"total_points"`
}

func ComputeGameMoveStats(db *sql.DB, gameID int64) (*GameMoveStats, error) {
	moves, err := ListMovesByGame(db, gameID, 500)
	if err != nil {
		return nil, err
	}

	stats := &GameMoveStats{
		ByType:   make(map[string]int64),
		ByPlayer: make(map[int64]int64),
	}

	for _, m := range moves {
		stats.TotalMoves++
		stats.ByType[m.MoveType]++
		stats.ByPlayer[m.PlayerID]++
		if m.ScoreVerified != nil {
			stats.TotalPoints += *m.ScoreVerified
		}
	}

	return stats, nil
}

func SanitizeHandJSON(handJSON string) string {
	handJSON = strings.TrimSpace(handJSON)
	if handJSON == "" {
		return "[]"
	}
	var parsed []json.RawMessage
	if err := json.Unmarshal([]byte(handJSON), &parsed); err != nil {
		return "[]"
	}
	return handJSON
}
