package models

import "errors"

var (
	ErrInvalidJSON             = errors.New("invalid json")
	ErrInvalidCard             = errors.New("invalid card")
	ErrNotAPlayer              = errors.New("not a player")
	ErrNotYourTurn             = errors.New("not your turn")
	ErrNotInPeggingStage       = errors.New("not in pegging stage")
	ErrWouldExceed31           = errors.New("would exceed 31")
	ErrCardNotInHand           = errors.New("card not in hand")
	ErrNotInDiscardStage       = errors.New("not in discard stage")
	ErrDiscardCardNotInHand    = errors.New("discard card not in hand")
	ErrDiscardAlreadyCompleted = errors.New("discard already completed")
	ErrInvalidPlayerPosition   = errors.New("invalid player position")
	ErrUnknownMoveType         = errors.New("unknown move type")
	ErrHasLegalPlay            = errors.New("you have a legal play")
	ErrInvalidDiscardCount     = errors.New("invalid discard count")
	ErrInvalidPlayer           = errors.New("invalid player")
	ErrLobbyFull               = errors.New("lobby full")
	ErrLobbyNotJoinable        = errors.New("lobby not joinable")
	ErrGameStateMissing        = errors.New("persisted game state missing")
	ErrGameStateConflict       = errors.New("game state conflict")
	ErrPlayerNotInGame         = errors.New("player not in game")
	ErrGameNotFound            = errors.New("game not found")
)
