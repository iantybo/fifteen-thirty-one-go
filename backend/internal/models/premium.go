package models

import (
	"database/sql"
	"errors"
	"time"
)

type PremiumSubscription struct {
	ID                    int64     `json:"id"`
	UserID                int64     `json:"user_id"`
	StripeCustomerID      string    `json:"stripe_customer_id"`
	StripeSubscriptionID  *string   `json:"stripe_subscription_id,omitempty"`
	Status                string    `json:"status"`
	CurrentPeriodStart    time.Time `json:"current_period_start"`
	CurrentPeriodEnd      time.Time `json:"current_period_end"`
	CreatedAt             time.Time `json:"created_at"`
	UpdatedAt             time.Time `json:"updated_at"`
}

func HasActivePremiumSubscription(db *sql.DB, userID int64) (bool, error) {
	var count int
	err := db.QueryRow(
		`SELECT COUNT(*) FROM premium_subscriptions 
		 WHERE user_id = ? AND status = 'active' AND current_period_end > ?`,
		userID, time.Now(),
	).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func GetPremiumSubscriptionByUserID(db *sql.DB, userID int64) (*PremiumSubscription, error) {
	var sub PremiumSubscription
	var stripeSubscriptionID sql.NullString
	err := db.QueryRow(
		`SELECT id, user_id, stripe_customer_id, stripe_subscription_id, status, 
		 current_period_start, current_period_end, created_at, updated_at
		 FROM premium_subscriptions WHERE user_id = ?`,
		userID,
	).Scan(
		&sub.ID, &sub.UserID, &sub.StripeCustomerID, &stripeSubscriptionID,
		&sub.Status, &sub.CurrentPeriodStart, &sub.CurrentPeriodEnd,
		&sub.CreatedAt, &sub.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if stripeSubscriptionID.Valid {
		sub.StripeSubscriptionID = &stripeSubscriptionID.String
	}
	return &sub, nil
}

func CreatePremiumSubscription(db *sql.DB, userID int64, stripeCustomerID string, stripeSubscriptionID *string, periodStart, periodEnd time.Time) (*PremiumSubscription, error) {
	res, err := db.Exec(
		`INSERT INTO premium_subscriptions 
		 (user_id, stripe_customer_id, stripe_subscription_id, status, current_period_start, current_period_end)
		 VALUES (?, ?, ?, 'active', ?, ?)`,
		userID, stripeCustomerID, stripeSubscriptionID, periodStart, periodEnd,
	)
	if err != nil {
		return nil, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}
	return GetPremiumSubscriptionByID(db, id)
}

func GetPremiumSubscriptionByID(db *sql.DB, id int64) (*PremiumSubscription, error) {
	var sub PremiumSubscription
	var stripeSubscriptionID sql.NullString
	err := db.QueryRow(
		`SELECT id, user_id, stripe_customer_id, stripe_subscription_id, status,
		 current_period_start, current_period_end, created_at, updated_at
		 FROM premium_subscriptions WHERE id = ?`,
		id,
	).Scan(
		&sub.ID, &sub.UserID, &sub.StripeCustomerID, &stripeSubscriptionID,
		&sub.Status, &sub.CurrentPeriodStart, &sub.CurrentPeriodEnd,
		&sub.CreatedAt, &sub.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if stripeSubscriptionID.Valid {
		sub.StripeSubscriptionID = &stripeSubscriptionID.String
	}
	return &sub, nil
}

func UpdatePremiumSubscriptionStatus(db *sql.DB, userID int64, status string, periodStart, periodEnd time.Time) error {
	_, err := db.Exec(
		`UPDATE premium_subscriptions 
		 SET status = ?, current_period_start = ?, current_period_end = ?, updated_at = ?
		 WHERE user_id = ?`,
		status, periodStart, periodEnd, time.Now(), userID,
	)
	return err
}

func UpdateUserAvatarURL(db *sql.DB, userID int64, avatarURL *string) error {
	_, err := db.Exec(
		`UPDATE users SET avatar_url = ? WHERE id = ?`,
		avatarURL, userID,
	)
	return err
}
