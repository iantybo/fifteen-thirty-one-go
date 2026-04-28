package models

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// SellPricePerCard is the flat number of coins a user earns for selling a single
// used card. Kept as a single constant here so the pricing policy has exactly
// one source of truth. Rank-based pricing is an intentional future extension.
const SellPricePerCard int64 = 5

type UserCard struct {
	ID         int64      `json:"id"`
	UserID     int64      `json:"user_id"`
	GameID     int64      `json:"game_id"`
	Card       string     `json:"card"`
	AcquiredAt time.Time  `json:"acquired_at"`
	SoldAt     *time.Time `json:"sold_at,omitempty"`
	SoldPrice  *int64     `json:"sold_price,omitempty"`
}

// AwardCardsForGameTx inserts one row per distinct card the user played in the
// given game. INSERT OR IGNORE against the (user_id, game_id, card) unique
// index keeps this idempotent across retries/re-finalizations.
func AwardCardsForGameTx(ctx context.Context, tx *sql.Tx, userID, gameID int64, cards []string) error {
	seen := make(map[string]struct{}, len(cards))
	for _, card := range cards {
		if card == "" {
			continue
		}
		if _, dup := seen[card]; dup {
			continue
		}
		seen[card] = struct{}{}
		if _, err := tx.ExecContext(
			ctx,
			`INSERT OR IGNORE INTO user_cards(user_id, game_id, card) VALUES (?, ?, ?)`,
			userID, gameID, card,
		); err != nil {
			return fmt.Errorf("insert user_card (user_id=%d game_id=%d card=%s): %w", userID, gameID, card, err)
		}
	}
	return nil
}

// MaxUnsoldCardsReturned caps the page size of ListUnsoldCardsByUser to keep
// the response bounded. Pagination beyond this cap is an intentional future
// extension; today the UI pages aren't wired for it.
const MaxUnsoldCardsReturned = 500

// ListUnsoldCardsByUser returns the caller's still-owned cards, newest first,
// capped at MaxUnsoldCardsReturned rows.
func ListUnsoldCardsByUser(ctx context.Context, db *sql.DB, userID int64) ([]UserCard, error) {
	rows, err := db.QueryContext(
		ctx,
		`SELECT id, user_id, game_id, card, acquired_at, sold_at, sold_price
		 FROM user_cards WHERE user_id = ? AND sold_at IS NULL
		 ORDER BY acquired_at DESC, id DESC
		 LIMIT ?`,
		userID, MaxUnsoldCardsReturned,
	)
	if err != nil {
		return nil, fmt.Errorf("query unsold cards: %w", err)
	}
	defer rows.Close()

	var out []UserCard
	for rows.Next() {
		var uc UserCard
		var soldAt sql.NullTime
		var soldPrice sql.NullInt64
		if err := rows.Scan(&uc.ID, &uc.UserID, &uc.GameID, &uc.Card, &uc.AcquiredAt, &soldAt, &soldPrice); err != nil {
			return nil, fmt.Errorf("scan user_card: %w", err)
		}
		if soldAt.Valid {
			t := soldAt.Time
			uc.SoldAt = &t
		}
		if soldPrice.Valid {
			v := soldPrice.Int64
			uc.SoldPrice = &v
		}
		out = append(out, uc)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate unsold cards: %w", err)
	}
	return out, nil
}

// SellUserCard marks a card sold and credits the owner's coin balance in a
// single transaction. Returns ErrNotFound when the card doesn't exist, isn't
// owned by the caller, or has already been sold.
func SellUserCard(ctx context.Context, db *sql.DB, userID, cardID int64) (price int64, newBalance int64, err error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return 0, 0, fmt.Errorf("begin tx: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	price = SellPricePerCard
	res, err := tx.ExecContext(
		ctx,
		`UPDATE user_cards SET sold_at = CURRENT_TIMESTAMP, sold_price = ?
		 WHERE id = ? AND user_id = ? AND sold_at IS NULL`,
		price, cardID, userID,
	)
	if err != nil {
		return 0, 0, fmt.Errorf("mark card sold: %w", err)
	}
	ra, err := res.RowsAffected()
	if err != nil {
		return 0, 0, fmt.Errorf("rows affected: %w", err)
	}
	if ra == 0 {
		return 0, 0, ErrNotFound
	}

	if _, err := tx.ExecContext(ctx, `UPDATE users SET coins = coins + ? WHERE id = ?`, price, userID); err != nil {
		return 0, 0, fmt.Errorf("credit coins: %w", err)
	}

	if err := tx.QueryRowContext(ctx, `SELECT coins FROM users WHERE id = ?`, userID).Scan(&newBalance); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, 0, ErrNotFound
		}
		return 0, 0, fmt.Errorf("read new balance: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return 0, 0, fmt.Errorf("commit: %w", err)
	}
	committed = true
	return price, newBalance, nil
}

// GetCoins returns the caller's current coin balance.
func GetCoins(ctx context.Context, db *sql.DB, userID int64) (int64, error) {
	var coins int64
	err := db.QueryRowContext(ctx, `SELECT coins FROM users WHERE id = ?`, userID).Scan(&coins)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, ErrNotFound
	}
	if err != nil {
		return 0, fmt.Errorf("query coins: %w", err)
	}
	return coins, nil
}
