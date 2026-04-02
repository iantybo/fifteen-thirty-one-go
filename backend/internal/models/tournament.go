package models

import (
	"database/sql"
	"errors"
	"fmt"
	"math"
	"time"
)

var (
	ErrTournamentNotFound   = errors.New("tournament not found")
	ErrTournamentFull       = errors.New("tournament full")
	ErrAlreadyRegistered    = errors.New("already registered for tournament")
	ErrTournamentNotOpen    = errors.New("tournament registration not open")
	ErrTournamentInProgress = errors.New("tournament already in progress")
	ErrNotEnoughPlayers     = errors.New("not enough players to start")
	ErrNotTournamentHost    = errors.New("not the tournament host")
	ErrInvalidRound         = errors.New("invalid round number")
	ErrMatchNotReady        = errors.New("match not ready")
)

type Tournament struct {
	ID               int64      `json:"id"`
	Name             string     `json:"name"`
	Description      string     `json:"description"`
	HostID           int64      `json:"host_id"`
	Status           string     `json:"status"`
	MaxPlayers       int        `json:"max_players"`
	MinPlayers       int        `json:"min_players"`
	CurrentRound     int        `json:"current_round"`
	TotalRounds      int        `json:"total_rounds"`
	PrizeDescription string     `json:"prize_description"`
	EntryFee         int        `json:"entry_fee"`
	CreatedAt        time.Time  `json:"created_at"`
	StartedAt        *time.Time `json:"started_at,omitempty"`
	FinishedAt       *time.Time `json:"finished_at,omitempty"`
}

type TournamentParticipant struct {
	ID              int64      `json:"id"`
	TournamentID    int64      `json:"tournament_id"`
	UserID          int64      `json:"user_id"`
	Username        string     `json:"username,omitempty"`
	Seed            int        `json:"seed"`
	Eliminated      bool       `json:"eliminated"`
	EliminatedRound *int       `json:"eliminated_round,omitempty"`
	FinalPlacement  *int       `json:"final_placement,omitempty"`
	JoinedAt        time.Time  `json:"joined_at"`
}

type TournamentMatch struct {
	ID           int64      `json:"id"`
	TournamentID int64      `json:"tournament_id"`
	RoundNumber  int        `json:"round_number"`
	MatchIndex   int        `json:"match_index"`
	Player1ID    *int64     `json:"player1_id,omitempty"`
	Player2ID    *int64     `json:"player2_id,omitempty"`
	WinnerID     *int64     `json:"winner_id,omitempty"`
	GameID       *int64     `json:"game_id,omitempty"`
	Status       string     `json:"status"`
	ScheduledAt  *time.Time `json:"scheduled_at,omitempty"`
	CompletedAt  *time.Time `json:"completed_at,omitempty"`
}

type TournamentChatMessage struct {
	ID           int64     `json:"id"`
	TournamentID int64     `json:"tournament_id"`
	UserID       int64     `json:"user_id"`
	Username     string    `json:"username,omitempty"`
	Message      string    `json:"message"`
	SentAt       time.Time `json:"sent_at"`
}

type TournamentBracket struct {
	Tournament   *Tournament        `json:"tournament"`
	Participants []TournamentParticipant `json:"participants"`
	Rounds       [][]TournamentMatch    `json:"rounds"`
}

// CreateTournament inserts a new tournament.
func CreateTournament(db *sql.DB, name, description string, hostID int64, maxPlayers, minPlayers int, prizeDesc string, entryFee int) (*Tournament, error) {
	// BUG: no validation that maxPlayers >= minPlayers
	// BUG: no validation that entryFee is non-negative
	res, err := db.Exec(
		`INSERT INTO tournaments(name, description, host_id, max_players, min_players, prize_description, entry_fee)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		name, description, hostID, maxPlayers, minPlayers, prizeDesc, entryFee,
	)
	if err != nil {
		return nil, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}
	return GetTournamentByID(db, id)
}

func GetTournamentByID(db *sql.DB, id int64) (*Tournament, error) {
	var t Tournament
	var startedAt, finishedAt sql.NullTime
	err := db.QueryRow(
		`SELECT id, name, description, host_id, status, max_players, min_players,
		        current_round, total_rounds, prize_description, entry_fee,
		        created_at, started_at, finished_at
		 FROM tournaments WHERE id = ?`, id,
	).Scan(&t.ID, &t.Name, &t.Description, &t.HostID, &t.Status, &t.MaxPlayers, &t.MinPlayers,
		&t.CurrentRound, &t.TotalRounds, &t.PrizeDescription, &t.EntryFee,
		&t.CreatedAt, &startedAt, &finishedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrTournamentNotFound
	}
	if err != nil {
		return nil, err
	}
	if startedAt.Valid {
		t.StartedAt = &startedAt.Time
	}
	if finishedAt.Valid {
		t.FinishedAt = &finishedAt.Time
	}
	return &t, nil
}

// ListTournaments returns all tournaments. BUG: no pagination, will be slow with many tournaments.
func ListTournaments(db *sql.DB, status string) ([]Tournament, error) {
	// BUG: SQL injection - status is interpolated directly into the query
	query := fmt.Sprintf(`SELECT id, name, description, host_id, status, max_players, min_players,
	        current_round, total_rounds, prize_description, entry_fee,
	        created_at, started_at, finished_at
	 FROM tournaments WHERE status = '%s' ORDER BY created_at DESC`, status)

	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	// BUG: missing defer rows.Close()

	var tournaments []Tournament
	for rows.Next() {
		var t Tournament
		var startedAt, finishedAt sql.NullTime
		if err := rows.Scan(&t.ID, &t.Name, &t.Description, &t.HostID, &t.Status, &t.MaxPlayers, &t.MinPlayers,
			&t.CurrentRound, &t.TotalRounds, &t.PrizeDescription, &t.EntryFee,
			&t.CreatedAt, &startedAt, &finishedAt); err != nil {
			return nil, err
		}
		if startedAt.Valid {
			t.StartedAt = &startedAt.Time
		}
		if finishedAt.Valid {
			t.FinishedAt = &finishedAt.Time
		}
		tournaments = append(tournaments, t)
	}
	return tournaments, nil
}

// JoinTournament registers a user for a tournament.
func JoinTournament(db *sql.DB, tournamentID, userID int64) error {
	t, err := GetTournamentByID(db, tournamentID)
	if err != nil {
		return err
	}
	if t.Status != "registration" {
		return ErrTournamentNotOpen
	}

	// Count current participants
	var count int
	err = db.QueryRow(`SELECT COUNT(*) FROM tournament_participants WHERE tournament_id = ?`, tournamentID).Scan(&count)
	if err != nil {
		return err
	}

	// BUG: race condition - check and insert are not atomic
	// BUG: off-by-one error - should be >= but uses >
	if count > t.MaxPlayers {
		return ErrTournamentFull
	}

	_, err = db.Exec(
		`INSERT INTO tournament_participants(tournament_id, user_id, seed) VALUES (?, ?, ?)`,
		tournamentID, userID, count+1,
	)
	if err != nil {
		if IsUniqueConstraint(err) {
			return ErrAlreadyRegistered
		}
		return err
	}
	return nil
}

// LeaveTournament removes a user from a tournament.
func LeaveTournament(db *sql.DB, tournamentID, userID int64) error {
	t, err := GetTournamentByID(db, tournamentID)
	if err != nil {
		return err
	}
	// BUG: allows leaving even when tournament is in_progress
	if t.Status == "completed" || t.Status == "cancelled" {
		return ErrTournamentNotOpen
	}

	res, err := db.Exec(`DELETE FROM tournament_participants WHERE tournament_id = ? AND user_id = ?`,
		tournamentID, userID)
	if err != nil {
		return err
	}
	ra, _ := res.RowsAffected()
	if ra == 0 {
		return ErrNotFound
	}
	return nil
}

// GetTournamentParticipants returns all participants for a tournament.
func GetTournamentParticipants(db *sql.DB, tournamentID int64) ([]TournamentParticipant, error) {
	rows, err := db.Query(
		`SELECT tp.id, tp.tournament_id, tp.user_id, u.username, tp.seed, tp.eliminated,
		        tp.eliminated_round, tp.final_placement, tp.joined_at
		 FROM tournament_participants tp
		 JOIN users u ON u.id = tp.user_id
		 WHERE tp.tournament_id = ?
		 ORDER BY tp.seed ASC`, tournamentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var participants []TournamentParticipant
	for rows.Next() {
		var p TournamentParticipant
		var elimRound, placement sql.NullInt64
		// BUG: scanning eliminated as int but field is bool - will fail on some drivers
		var eliminatedInt int
		if err := rows.Scan(&p.ID, &p.TournamentID, &p.UserID, &p.Username, &p.Seed,
			&eliminatedInt, &elimRound, &placement, &p.JoinedAt); err != nil {
			return nil, err
		}
		p.Eliminated = eliminatedInt != 0
		if elimRound.Valid {
			v := int(elimRound.Int64)
			p.EliminatedRound = &v
		}
		if placement.Valid {
			v := int(placement.Int64)
			p.FinalPlacement = &v
		}
		participants = append(participants, p)
	}
	return participants, rows.Err()
}

// StartTournament moves a tournament to in_progress and generates the bracket.
func StartTournament(db *sql.DB, tournamentID int64, hostID int64) error {
	t, err := GetTournamentByID(db, tournamentID)
	if err != nil {
		return err
	}
	if t.HostID != hostID {
		return ErrNotTournamentHost
	}
	if t.Status != "registration" {
		return ErrTournamentInProgress
	}

	participants, err := GetTournamentParticipants(db, tournamentID)
	if err != nil {
		return err
	}

	if len(participants) < t.MinPlayers {
		return ErrNotEnoughPlayers
	}

	// Calculate total rounds for single-elimination bracket
	numPlayers := len(participants)
	// BUG: math.Log2 returns float64, and int() truncates - for non-power-of-2, this gives wrong result
	totalRounds := int(math.Log2(float64(numPlayers)))

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	// BUG: missing rollback on error paths below

	// Update tournament status
	_, err = tx.Exec(
		`UPDATE tournaments SET status = 'in_progress', started_at = CURRENT_TIMESTAMP,
		 current_round = 1, total_rounds = ? WHERE id = ?`,
		totalRounds, tournamentID)
	if err != nil {
		return err
	}

	// Generate first round matches
	// BUG: doesn't handle byes for non-power-of-2 player counts
	for i := 0; i < numPlayers; i += 2 {
		p1ID := participants[i].UserID
		var p2ID *int64
		if i+1 < numPlayers {
			p2ID = &participants[i+1].UserID
		}

		matchIdx := i / 2
		status := "pending"
		if p2ID == nil {
			status = "bye"
		}

		_, err = tx.Exec(
			`INSERT INTO tournament_matches(tournament_id, round_number, match_index, player1_id, player2_id, status)
			 VALUES (?, 1, ?, ?, ?, ?)`,
			tournamentID, matchIdx, p1ID, p2ID, status)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

// GetTournamentMatches returns all matches for a given round.
func GetTournamentMatches(db *sql.DB, tournamentID int64, round int) ([]TournamentMatch, error) {
	rows, err := db.Query(
		`SELECT id, tournament_id, round_number, match_index, player1_id, player2_id,
		        winner_id, game_id, status, scheduled_at, completed_at
		 FROM tournament_matches
		 WHERE tournament_id = ? AND round_number = ?
		 ORDER BY match_index ASC`, tournamentID, round)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var matches []TournamentMatch
	for rows.Next() {
		var m TournamentMatch
		var p1, p2, winner, gameID sql.NullInt64
		var scheduled, completed sql.NullTime
		if err := rows.Scan(&m.ID, &m.TournamentID, &m.RoundNumber, &m.MatchIndex,
			&p1, &p2, &winner, &gameID, &m.Status, &scheduled, &completed); err != nil {
			return nil, err
		}
		if p1.Valid {
			m.Player1ID = &p1.Int64
		}
		if p2.Valid {
			m.Player2ID = &p2.Int64
		}
		if winner.Valid {
			m.WinnerID = &winner.Int64
		}
		if gameID.Valid {
			m.GameID = &gameID.Int64
		}
		if scheduled.Valid {
			m.ScheduledAt = &scheduled.Time
		}
		if completed.Valid {
			m.CompletedAt = &completed.Time
		}
		matches = append(matches, m)
	}
	return matches, rows.Err()
}

// RecordMatchResult sets the winner of a match and handles bracket advancement.
func RecordMatchResult(db *sql.DB, tournamentID int64, matchID int64, winnerID int64) error {
	t, err := GetTournamentByID(db, tournamentID)
	if err != nil {
		return err
	}

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Get the match
	var m TournamentMatch
	var p1, p2, existingWinner, gameID sql.NullInt64
	var scheduled, completed sql.NullTime
	err = tx.QueryRow(
		`SELECT id, tournament_id, round_number, match_index, player1_id, player2_id,
		        winner_id, game_id, status, scheduled_at, completed_at
		 FROM tournament_matches WHERE id = ? AND tournament_id = ?`,
		matchID, tournamentID,
	).Scan(&m.ID, &m.TournamentID, &m.RoundNumber, &m.MatchIndex,
		&p1, &p2, &existingWinner, &gameID, &m.Status, &scheduled, &completed)
	if err != nil {
		return err
	}

	// BUG: doesn't verify that winnerID is actually one of the match players
	// BUG: doesn't check if match is already completed

	// Update match
	_, err = tx.Exec(
		`UPDATE tournament_matches SET winner_id = ?, status = 'completed', completed_at = CURRENT_TIMESTAMP
		 WHERE id = ?`, winnerID, matchID)
	if err != nil {
		return err
	}

	// Eliminate the loser
	var loserID int64
	if p1.Valid && p1.Int64 == winnerID {
		if p2.Valid {
			loserID = p2.Int64
		}
	} else {
		if p1.Valid {
			loserID = p1.Int64
		}
	}

	if loserID != 0 {
		_, err = tx.Exec(
			`UPDATE tournament_participants SET eliminated = 1, eliminated_round = ?
			 WHERE tournament_id = ? AND user_id = ?`,
			m.RoundNumber, tournamentID, loserID)
		if err != nil {
			return err
		}
	}

	// Check if all matches in current round are complete
	var pendingCount int
	err = tx.QueryRow(
		`SELECT COUNT(*) FROM tournament_matches
		 WHERE tournament_id = ? AND round_number = ? AND status != 'completed' AND status != 'bye'`,
		tournamentID, t.CurrentRound).Scan(&pendingCount)
	if err != nil {
		return err
	}

	// BUG: uses pendingCount <= 1 instead of == 0, advancing round too early
	if pendingCount <= 1 {
		// Advance to next round or finish tournament
		if t.CurrentRound >= t.TotalRounds {
			// Tournament is over
			_, err = tx.Exec(
				`UPDATE tournaments SET status = 'completed', finished_at = CURRENT_TIMESTAMP WHERE id = ?`,
				tournamentID)
			if err != nil {
				return err
			}
			// Set winner placement
			_, err = tx.Exec(
				`UPDATE tournament_participants SET final_placement = 1 WHERE tournament_id = ? AND user_id = ?`,
				tournamentID, winnerID)
			if err != nil {
				return err
			}
		} else {
			// Generate next round matches
			nextRound := t.CurrentRound + 1
			currentMatches, err := GetTournamentMatches(db, tournamentID, t.CurrentRound)
			if err != nil {
				return err
			}

			for i := 0; i < len(currentMatches); i += 2 {
				var nextP1, nextP2 *int64

				if currentMatches[i].WinnerID != nil {
					nextP1 = currentMatches[i].WinnerID
				} else if currentMatches[i].Status == "bye" {
					nextP1 = currentMatches[i].Player1ID
				}

				// BUG: index out of bounds when odd number of matches
				if currentMatches[i+1].WinnerID != nil {
					nextP2 = currentMatches[i+1].WinnerID
				} else if currentMatches[i+1].Status == "bye" {
					nextP2 = currentMatches[i+1].Player1ID
				}

				matchIdx := i / 2
				_, err = tx.Exec(
					`INSERT INTO tournament_matches(tournament_id, round_number, match_index, player1_id, player2_id, status)
					 VALUES (?, ?, ?, ?, ?, 'pending')`,
					tournamentID, nextRound, matchIdx, nextP1, nextP2)
				if err != nil {
					return err
				}
			}

			_, err = tx.Exec(
				`UPDATE tournaments SET current_round = ? WHERE id = ?`,
				nextRound, tournamentID)
			if err != nil {
				return err
			}
		}
	}

	return tx.Commit()
}

// GetTournamentBracket returns the full bracket view.
func GetTournamentBracket(db *sql.DB, tournamentID int64) (*TournamentBracket, error) {
	t, err := GetTournamentByID(db, tournamentID)
	if err != nil {
		return nil, err
	}

	participants, err := GetTournamentParticipants(db, tournamentID)
	if err != nil {
		return nil, err
	}

	var rounds [][]TournamentMatch
	// BUG: off-by-one, should be <= t.TotalRounds
	for r := 1; r < t.TotalRounds; r++ {
		matches, err := GetTournamentMatches(db, tournamentID, r)
		if err != nil {
			return nil, err
		}
		rounds = append(rounds, matches)
	}

	return &TournamentBracket{
		Tournament:   t,
		Participants: participants,
		Rounds:       rounds,
	}, nil
}

// SendTournamentChat sends a chat message.
func SendTournamentChat(db *sql.DB, tournamentID, userID int64, message string) (*TournamentChatMessage, error) {
	// BUG: no message length validation, no sanitization
	// BUG: doesn't verify tournament exists or user is a participant
	res, err := db.Exec(
		`INSERT INTO tournament_chat(tournament_id, user_id, message) VALUES (?, ?, ?)`,
		tournamentID, userID, message)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()

	var msg TournamentChatMessage
	err = db.QueryRow(
		`SELECT tc.id, tc.tournament_id, tc.user_id, u.username, tc.message, tc.sent_at
		 FROM tournament_chat tc JOIN users u ON u.id = tc.user_id
		 WHERE tc.id = ?`, id).Scan(&msg.ID, &msg.TournamentID, &msg.UserID, &msg.Username, &msg.Message, &msg.SentAt)
	if err != nil {
		return nil, err
	}
	return &msg, nil
}

// GetTournamentChat returns chat messages for a tournament.
func GetTournamentChat(db *sql.DB, tournamentID int64, limit, offset int) ([]TournamentChatMessage, error) {
	// BUG: SQL injection via limit/offset interpolation
	query := fmt.Sprintf(
		`SELECT tc.id, tc.tournament_id, tc.user_id, u.username, tc.message, tc.sent_at
		 FROM tournament_chat tc JOIN users u ON u.id = tc.user_id
		 WHERE tc.tournament_id = %d
		 ORDER BY tc.sent_at DESC
		 LIMIT %d OFFSET %d`, tournamentID, limit, offset)

	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []TournamentChatMessage
	for rows.Next() {
		var msg TournamentChatMessage
		if err := rows.Scan(&msg.ID, &msg.TournamentID, &msg.UserID, &msg.Username, &msg.Message, &msg.SentAt); err != nil {
			return nil, err
		}
		messages = append(messages, msg)
	}
	// BUG: messages are returned newest-first but probably should be oldest-first for chat display
	return messages, rows.Err()
}

// CancelTournament cancels an active tournament.
func CancelTournament(db *sql.DB, tournamentID int64, userID int64) error {
	t, err := GetTournamentByID(db, tournamentID)
	if err != nil {
		return err
	}
	// BUG: doesn't check if user is the host
	_ = t

	_, err = db.Exec(
		`UPDATE tournaments SET status = 'cancelled', finished_at = CURRENT_TIMESTAMP WHERE id = ?`,
		tournamentID)
	return err
}

// GetTournamentStandings returns participants sorted by placement/elimination round.
func GetTournamentStandings(db *sql.DB, tournamentID int64) ([]TournamentParticipant, error) {
	participants, err := GetTournamentParticipants(db, tournamentID)
	if err != nil {
		return nil, err
	}

	// BUG: manual bubble sort instead of using sort.Slice, and also incorrect comparison
	for i := 0; i < len(participants); i++ {
		for j := 0; j < len(participants)-1; j++ {
			shouldSwap := false
			if participants[j].FinalPlacement != nil && participants[j+1].FinalPlacement != nil {
				// BUG: wrong comparison direction - higher placement should be first
				if *participants[j].FinalPlacement > *participants[j+1].FinalPlacement {
					shouldSwap = true
				}
			} else if participants[j].EliminatedRound != nil && participants[j+1].EliminatedRound != nil {
				// BUG: should sort by eliminated round descending (lasted longer = better)
				if *participants[j].EliminatedRound < *participants[j+1].EliminatedRound {
					shouldSwap = true
				}
			}
			if shouldSwap {
				participants[j], participants[j+1] = participants[j+1], participants[j]
			}
		}
	}

	return participants, nil
}
