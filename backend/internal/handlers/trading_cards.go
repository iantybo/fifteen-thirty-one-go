package handlers

import (
	"database/sql"
	"errors"
	"net/http"
	"strconv"

	"fifteen-thirty-one-go/backend/internal/models"
	"github.com/gin-gonic/gin"
)

// GetAllCardsHandler returns a handler that retrieves all trading cards.
// Returns a JSON array of all cards sorted by rarity and name.
func GetAllCardsHandler(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		cards, err := models.GetAllTradingCards(db)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch cards"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"cards": cards})
	}
}

// GetUserCardsHandler returns a handler that retrieves the authenticated user's trading card collection.
// Requires authentication. Returns a JSON array of cards the user owns with quantity and acquisition date.
func GetUserCardsHandler(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, exists := c.Get("user_id")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
			return
		}

		cards, err := models.GetUserTradingCards(db, userID.(int64))
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch user cards"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"cards": cards})
	}
}

// ClaimCardHandler returns a handler that allows users to claim trading cards they've earned.
// Validates that the user meets the reward requirements before granting the card.
// Requires authentication. Returns 403 if requirements are not met, 409 if card is already owned.
func ClaimCardHandler(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, exists := c.Get("user_id")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
			return
		}

		cardIDStr := c.Param("id")
		cardID, err := strconv.ParseInt(cardIDStr, 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid card ID"})
			return
		}

		card, err := models.GetTradingCardByID(db, cardID)
		if err != nil {
			if errors.Is(err, models.ErrNotFound) {
				c.JSON(http.StatusNotFound, gin.H{"error": "Card not found"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch card"})
			return
		}

		hasCard, err := models.UserHasCard(db, userID.(int64), cardID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check card ownership"})
			return
		}
		if hasCard {
			c.JSON(http.StatusConflict, gin.H{"error": "Card already claimed"})
			return
		}

		user, err := models.GetUserByID(db, userID.(int64))
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch user"})
			return
		}

		rewards, err := models.GetCardRewards(db, cardID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch card rewards"})
			return
		}

		if len(rewards) == 0 {
			c.JSON(http.StatusForbidden, gin.H{"error": "Card has no reward conditions"})
			return
		}

		eligible := false
		for _, reward := range rewards {
			switch reward.RewardType {
			case "game_win":
				if user.GamesWon >= int64(reward.RequirementValue) {
					eligible = true
				}
			case "games_played":
				if user.GamesPlayed >= int64(reward.RequirementValue) {
					eligible = true
				}
			case "win_streak", "high_score", "special_event":
				// These reward types require additional tracking not yet implemented.
				// For now, skip eligibility check for these cards.
				continue
			}
		}

		if !eligible {
			c.JSON(http.StatusForbidden, gin.H{"error": "Requirements not met for this card"})
			return
		}

		if err := models.AddCardToUser(db, userID.(int64), cardID); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to claim card"})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"message": "Card claimed successfully",
			"card":    card,
		})
	}
}

// GetCardProgressHandler returns a handler that shows the user's progress toward earning all trading cards.
// Requires authentication. Returns a JSON array showing each card, whether it's unlocked,
// and the user's progress toward the requirement.
func GetCardProgressHandler(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, exists := c.Get("user_id")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
			return
		}

		progress, err := models.GetUserCardProgress(db, userID.(int64))
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch card progress"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"progress": progress})
	}
}
