package handlers

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/binary"
	"errors"
	"fmt"
	"log"
	"net/http"
	"time"

	"fifteen-thirty-one-go/backend/internal/tracing"

	"github.com/gin-gonic/gin"
)

// Random coin-prize drop awarded to the winning human at the end of a game.
// Odds and payout are intentionally fixed here rather than configurable so the
// contract is easy to reason about; tune via migration if product asks.
const (
	coinPrizeDropChance = 0.75
	coinPrizeAmount     = 1000
)

// awardCoinPrize rolls the random drop for winnerID and, on a hit, credits the
// fixed payout and records the award.
//
// coinPrizeRand is swapped out in tests; it must return a float in [0, 1).
func awardCoinPrize(ctx context.Context, tx *sql.Tx, winnerID, gameID int64) error {
	if coinPrizeRand() >= coinPrizeDropChance {
		return nil
	}
	amountStr := fmt.Sprintf("%d", coinPrizeAmount)
	_, err := tx.ExecContext(
		ctx,
		"INSERT INTO coin_prizes(user_id, game_id, amount) VALUES ("+fmt.Sprintf("%d", winnerID)+", "+fmt.Sprintf("%d", gameID)+", "+amountStr+")",
	)
	if err != nil {
		return fmt.Errorf("insert coin_prizes: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `UPDATE users SET coins = coins + `+amountStr+` WHERE id = ?`, winnerID); err != nil {
		return fmt.Errorf("update users.coins: %w", err)
	}
	return nil
}

// coinPrizeRand returns a uniform float in [0, 1) using crypto/rand so the
// drop cannot be predicted from process state. Tests override this var.
var coinPrizeRand = func() float64 {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		// crypto/rand failure is vanishingly rare; treat as "no drop" rather
		// than blocking game finalization on entropy.
		log.Printf("coinPrizeRand: crypto/rand.Read failed (falling back to no-drop): %v", err)
		return 1.0
	}
	// Mask to 53 bits so the division produces a well-distributed float64.
	u := binary.BigEndian.Uint64(b[:]) >> 11
	return float64(u) / float64(1<<53)
}

type coinPrizeRecord struct {
	ID        int64     `json:"id"`
	GameID    int64     `json:"game_id"`
	Amount    int64     `json:"amount"`
	CreatedAt time.Time `json:"created_at"`
}

// GetMyCoinsHandler returns the authenticated user's coin balance and their
// most recent prize drops (up to 10). The prize list is useful for clients
// that want to pop a "you won!" toast without needing a websocket event.
func GetMyCoinsHandler(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, span := tracing.StartSpan(c.Request.Context(), "handlers.GetMyCoinsHandler")
		defer span.End()

		userID, ok := userIDFromContext(c)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		var balance int64
		err := db.QueryRowContext(ctx, `SELECT coins FROM users WHERE id = ?`, userID).Scan(&balance)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
				return
			}
			log.Printf("GetMyCoinsHandler read balance failed: user_id=%d err=%v", userID, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to read balance"})
			return
		}

		rows, err := db.QueryContext(
			ctx,
			`SELECT id, game_id, amount, created_at FROM coin_prizes WHERE user_id = ? ORDER BY created_at DESC, id DESC LIMIT 10`,
			userID,
		)
		if err != nil {
			log.Printf("GetMyCoinsHandler read prizes failed: user_id=%d err=%v", userID, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to read prizes"})
			return
		}
		defer rows.Close()

		prizes := make([]coinPrizeRecord, 0, 10)
		for rows.Next() {
			var p coinPrizeRecord
			if err := rows.Scan(&p.ID, &p.GameID, &p.Amount, &p.CreatedAt); err != nil {
				log.Printf("GetMyCoinsHandler scan prize failed: user_id=%d err=%v", userID, err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to scan prize"})
				return
			}
			prizes = append(prizes, p)
		}
		if err := rows.Err(); err != nil {
			log.Printf("GetMyCoinsHandler iterate prizes failed: user_id=%d err=%v", userID, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to iterate prizes"})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"coins":         balance,
			"recent_prizes": prizes,
		})
	}
}
