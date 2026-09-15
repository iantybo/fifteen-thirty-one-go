package handlers

import (
	"database/sql"
	"errors"
	"log"
	"net/http"
	"strconv"

	"fifteen-thirty-one-go/backend/internal/models"
	"fifteen-thirty-one-go/backend/internal/tracing"

	"github.com/gin-gonic/gin"
)

type deckRequest struct {
	Name              string `json:"name"`
	BackImageURL      string `json:"back_image_url"`
	FaceImageTemplate string `json:"face_image_template"`
	RedSuitColor      string `json:"red_suit_color"`
	BlackSuitColor    string `json:"black_suit_color"`
	BorderColor       string `json:"border_color"`
}

func (r deckRequest) toInput() models.DeckInput {
	return models.DeckInput{
		Name:              r.Name,
		BackImageURL:      r.BackImageURL,
		FaceImageTemplate: r.FaceImageTemplate,
		RedSuitColor:      r.RedSuitColor,
		BlackSuitColor:    r.BlackSuitColor,
		BorderColor:       r.BorderColor,
	}
}

type setActiveDeckRequest struct {
	ActiveDeck string `json:"active_deck"`
}

// deckErrorStatus maps deck model errors to an HTTP status and client message.
// It returns ok=false for errors that are not client faults.
func deckErrorStatus(err error) (status int, msg string, ok bool) {
	switch {
	case errors.Is(err, models.ErrInvalidDeckName):
		return http.StatusBadRequest, "invalid deck name", true
	case errors.Is(err, models.ErrInvalidDeckImageURL):
		return http.StatusBadRequest, "image urls must be absolute https urls", true
	case errors.Is(err, models.ErrInvalidDeckTemplate):
		return http.StatusBadRequest, "face image template must be an https url containing {rank}, {suit} or {code}", true
	case errors.Is(err, models.ErrInvalidDeckColor):
		return http.StatusBadRequest, "colors must be hex values like #rrggbb", true
	case errors.Is(err, models.ErrDeckNameTaken):
		return http.StatusConflict, "a deck with that name already exists", true
	case errors.Is(err, models.ErrDeckNotFound):
		return http.StatusNotFound, "deck not found", true
	case errors.Is(err, models.ErrInvalidDeckRef):
		return http.StatusBadRequest, "invalid deck selection", true
	}
	return 0, "", false
}

// respondDeckError writes the mapped client error, or a 500 for server faults.
func respondDeckError(c *gin.Context, op string, userID int64, err error) {
	if status, msg, ok := deckErrorStatus(err); ok {
		c.JSON(status, gin.H{"error": msg})
		return
	}
	log.Printf("%s failed: user_id=%d err=%v", op, userID, err)
	c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
}

// deckIDParam parses the :deckId path parameter.
func deckIDParam(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("deckId"), 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid deck id"})
		return 0, false
	}
	return id, true
}

// ListDecksHandler returns the built-in decks, the caller's custom decks, and
// the caller's current selection.
func ListDecksHandler(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		_, span := tracing.StartSpan(c.Request.Context(), "handlers.ListDecksHandler")
		defer span.End()

		userID, ok := userIDFromContext(c)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		decks, err := models.ListCardDecks(db, userID)
		if err != nil {
			respondDeckError(c, "ListCardDecks", userID, err)
			return
		}
		prefs, err := models.GetUserPreferences(db, userID)
		if err != nil {
			respondDeckError(c, "GetUserPreferences", userID, err)
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"builtin":     models.BuiltinDecks(),
			"custom":      decks,
			"active_deck": prefs.ActiveDeck,
		})
	}
}

// CreateDeckHandler creates a custom deck for the caller.
func CreateDeckHandler(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		_, span := tracing.StartSpan(c.Request.Context(), "handlers.CreateDeckHandler")
		defer span.End()

		userID, ok := userIDFromContext(c)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		var req deckRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid json"})
			return
		}

		deck, err := models.CreateCardDeck(db, userID, req.toInput())
		if err != nil {
			respondDeckError(c, "CreateCardDeck", userID, err)
			return
		}
		c.JSON(http.StatusCreated, gin.H{"deck": deck})
	}
}

// UpdateDeckHandler replaces a custom deck owned by the caller.
func UpdateDeckHandler(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		_, span := tracing.StartSpan(c.Request.Context(), "handlers.UpdateDeckHandler")
		defer span.End()

		userID, ok := userIDFromContext(c)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}
		deckID, ok := deckIDParam(c)
		if !ok {
			return
		}

		var req deckRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid json"})
			return
		}

		deck, err := models.UpdateCardDeck(db, userID, deckID, req.toInput())
		if err != nil {
			respondDeckError(c, "UpdateCardDeck", userID, err)
			return
		}
		c.JSON(http.StatusOK, gin.H{"deck": deck})
	}
}

// DeleteDeckHandler removes a custom deck owned by the caller.
func DeleteDeckHandler(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		_, span := tracing.StartSpan(c.Request.Context(), "handlers.DeleteDeckHandler")
		defer span.End()

		userID, ok := userIDFromContext(c)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}
		deckID, ok := deckIDParam(c)
		if !ok {
			return
		}

		if err := models.DeleteCardDeck(db, userID, deckID); err != nil {
			respondDeckError(c, "DeleteCardDeck", userID, err)
			return
		}
		c.Status(http.StatusNoContent)
	}
}

// SetActiveDeckHandler records the caller's deck selection.
func SetActiveDeckHandler(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		_, span := tracing.StartSpan(c.Request.Context(), "handlers.SetActiveDeckHandler")
		defer span.End()

		userID, ok := userIDFromContext(c)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		var req setActiveDeckRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid json"})
			return
		}

		prefs, err := models.SetActiveDeck(db, userID, req.ActiveDeck)
		if err != nil {
			respondDeckError(c, "SetActiveDeck", userID, err)
			return
		}
		c.JSON(http.StatusOK, prefs)
	}
}
