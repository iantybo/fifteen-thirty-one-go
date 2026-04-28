package handlers

import (
	"database/sql"
	"errors"
	"log"
	"net/http"
	"strconv"

	"fifteen-thirty-one-go/backend/internal/models"

	"github.com/gin-gonic/gin"
)

// ListMyCardsHandler returns the caller's unsold cards and current coin balance.
// GET /api/cards
func ListMyCardsHandler(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, ok := userIDFromContext(c)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		cards, err := models.ListUnsoldCardsByUser(c.Request.Context(), db, userID)
		if err != nil {
			log.Printf("ListMyCardsHandler: list cards user_id=%d: %v", userID, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
			return
		}
		coins, err := models.GetCoins(c.Request.Context(), db, userID)
		if err != nil {
			if errors.Is(err, models.ErrNotFound) {
				c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
				return
			}
			log.Printf("ListMyCardsHandler: get coins user_id=%d: %v", userID, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
			return
		}
		if cards == nil {
			cards = []models.UserCard{}
		}
		c.JSON(http.StatusOK, gin.H{
			"cards":            cards,
			"coins":            coins,
			"sell_price":       models.SellPricePerCard,
		})
	}
}

// SellCardHandler marks the specified card sold and credits coins.
// POST /api/cards/:id/sell
func SellCardHandler(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, ok := userIDFromContext(c)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		cardID, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil || cardID <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid card id"})
			return
		}

		price, newBalance, err := models.SellUserCard(c.Request.Context(), db, userID, cardID)
		if err != nil {
			if errors.Is(err, models.ErrNotFound) {
				c.JSON(http.StatusNotFound, gin.H{"error": "card not found or already sold"})
				return
			}
			log.Printf("SellCardHandler: sell user_id=%d card_id=%d: %v", userID, cardID, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"sold_card_id": cardID,
			"price":        price,
			"coins":        newBalance,
		})
	}
}
