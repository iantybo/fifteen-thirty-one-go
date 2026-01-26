package timer

import (
	"database/sql"
	"log"
	"sync"
	"time"

	"fifteen-thirty-one-go/backend/internal/models"
)

// Service manages game timers and sends notifications
type Service struct {
	db         *sql.DB
	timers     map[int64]*timerState // gameID -> timer state
	mu         sync.RWMutex
	stopChan   chan struct{}
	stopOnce   sync.Once
	onExpired  func(gameID int64, userID int64)
	onWarning  func(gameID int64, userID int64, secondsRemaining int)
	warnAt     int // seconds remaining when to send warning
}

type timerState struct {
	gameID          int64
	currentPlayerID int64
	startTime       time.Time
	timeLimit       int
	warned          bool
	cancel          chan struct{}
	cancelOnce      sync.Once
}

// Close safely closes the cancel channel exactly once
func (ts *timerState) Close() {
	ts.cancelOnce.Do(func() {
		close(ts.cancel)
	})
}

// NewService creates a new timer service
func NewService(db *sql.DB) *Service {
	return &Service{
		db:       db,
		timers:   make(map[int64]*timerState),
		stopChan: make(chan struct{}),
		warnAt:   10, // default: warn at 10 seconds remaining
	}
}

// SetWarningThreshold sets when to send low-time warnings
func (s *Service) SetWarningThreshold(seconds int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.warnAt = seconds
}

// SetExpirationHandler sets callback for when timer expires
func (s *Service) SetExpirationHandler(fn func(gameID int64, userID int64)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onExpired = fn
}

// SetWarningHandler sets callback for low-time warnings
func (s *Service) SetWarningHandler(fn func(gameID int64, userID int64, secondsRemaining int)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onWarning = fn
}

// StartTimer starts a timer for a player's turn
func (s *Service) StartTimer(gameID int64, currentPlayerID int64, timeLimitSeconds int) error {
	if timeLimitSeconds <= 0 {
		return nil // no timer configured
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Stop existing timer if any
	if existing, ok := s.timers[gameID]; ok {
		existing.Close()
	}

	// Create new timer state
	state := &timerState{
		gameID:          gameID,
		currentPlayerID: currentPlayerID,
		startTime:       time.Now(),
		timeLimit:       timeLimitSeconds,
		warned:          false,
		cancel:          make(chan struct{}),
	}
	s.timers[gameID] = state

	// Start timer goroutine
	go s.runTimer(state)

	// Log timer start event
	if err := models.LogTimerEvent(s.db, gameID, currentPlayerID, "turn_start", &timeLimitSeconds); err != nil {
		log.Printf("Error logging timer event: %v", err)
	}

	return nil
}

// StopTimer stops the timer for a game
func (s *Service) StopTimer(gameID int64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if state, ok := s.timers[gameID]; ok {
		state.Close()
		delete(s.timers, gameID)
	}
}

// GetTimeRemaining returns seconds remaining for current turn
func (s *Service) GetTimeRemaining(gameID int64) int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	state, ok := s.timers[gameID]
	if !ok {
		return 0
	}

	elapsed := int(time.Since(state.startTime).Seconds())
	remaining := state.timeLimit - elapsed
	if remaining < 0 {
		return 0
	}
	return remaining
}

// runTimer manages a single timer
func (s *Service) runTimer(state *timerState) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-state.cancel:
			return
		case <-s.stopChan:
			return
		case <-ticker.C:
			elapsed := int(time.Since(state.startTime).Seconds())
			remaining := state.timeLimit - elapsed

			// Read warnAt with lock
			s.mu.RLock()
			warnAt := s.warnAt
			s.mu.RUnlock()

			// Send warning at threshold
			if !state.warned && remaining <= warnAt && remaining > 0 {
				state.warned = true
				if err := models.SetTimerWarned(s.db, state.gameID); err != nil {
					log.Printf("Error setting timer warned: %v", err)
				}
				if err := models.LogTimerEvent(s.db, state.gameID, state.currentPlayerID, "warning", &remaining); err != nil {
					log.Printf("Error logging warning event: %v", err)
				}
				s.mu.RLock()
				onWarning := s.onWarning
				s.mu.RUnlock()
				if onWarning != nil {
					onWarning(state.gameID, state.currentPlayerID, remaining)
				}
			}

			// Timer expired
			if remaining <= 0 {
				zero := 0
				if err := models.LogTimerEvent(s.db, state.gameID, state.currentPlayerID, "time_expired", &zero); err != nil {
					log.Printf("Error logging expiration event: %v", err)
				}
				s.mu.RLock()
				onExpired := s.onExpired
				s.mu.RUnlock()
				if onExpired != nil {
					onExpired(state.gameID, state.currentPlayerID)
				}
				s.StopTimer(state.gameID)
				return
			}
		}
	}
}

// Shutdown stops all timers
func (s *Service) Shutdown() {
	s.stopOnce.Do(func() {
		close(s.stopChan)
	})
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, state := range s.timers {
		state.Close()
	}
	s.timers = make(map[int64]*timerState)
}

// RestoreTimers restores active timers from database (on server restart)
func (s *Service) RestoreTimers() error {
	rows, err := s.db.Query(`
		SELECT gt.game_id, gt.current_player_id, gt.turn_started_at, gt.turn_time_limit, gt.warned
		FROM game_timers gt
		INNER JOIN games g ON gt.game_id = g.id
		WHERE g.status = 'in_progress'
	`)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var gameID int64
		var currentPlayerID sql.NullInt64
		var turnStartedAt time.Time
		var turnTimeLimit int
		var warned bool

		if err := rows.Scan(&gameID, &currentPlayerID, &turnStartedAt, &turnTimeLimit, &warned); err != nil {
			log.Printf("Error scanning timer row: %v", err)
			continue
		}

		if !currentPlayerID.Valid {
			log.Printf("Skipping timer for game %d: no current player", gameID)
			continue
		}

		elapsed := int(time.Since(turnStartedAt).Seconds())
		remaining := turnTimeLimit - elapsed

		if remaining <= 0 {
			// Timer already expired, handle it
			s.mu.RLock()
			onExpired := s.onExpired
			s.mu.RUnlock()
			if onExpired != nil {
				onExpired(gameID, currentPlayerID.Int64)
			}
			continue
		}

		// Restore timer with adjusted time limit
		s.mu.Lock()
		state := &timerState{
			gameID:          gameID,
			currentPlayerID: currentPlayerID.Int64,
			startTime:       time.Now(),
			timeLimit:       remaining,
			warned:          warned,
			cancel:          make(chan struct{}),
		}
		s.timers[gameID] = state
		s.mu.Unlock()

		go s.runTimer(state)
	}

	return rows.Err()
}
