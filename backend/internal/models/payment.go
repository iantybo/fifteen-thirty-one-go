package models

import (
	"time"
)

// SubscriptionPlan represents a pricing tier
type SubscriptionPlan struct {
	ID            string    `json:"id" db:"id"`
	Name          string    `json:"name" db:"name"`
	DisplayName   string    `json:"display_name" db:"display_name"`
	Description   string    `json:"description" db:"description"`
	PriceCents    int       `json:"price_cents" db:"price_cents"`
	Currency      string    `json:"currency" db:"currency"`
	BillingPeriod string    `json:"billing_period" db:"billing_period"` // 'month', 'year'
	StripePriceID *string   `json:"stripe_price_id,omitempty" db:"stripe_price_id"`
	FeaturesJSON  string    `json:"-" db:"features_json"`
	Features      []string  `json:"features"`
	IsActive      bool      `json:"is_active" db:"is_active"`
	CreatedAt     time.Time `json:"created_at" db:"created_at"`
	UpdatedAt     time.Time `json:"updated_at" db:"updated_at"`
}

// UserSubscription represents a user's active subscription
type UserSubscription struct {
	ID                   string     `json:"id" db:"id"`
	UserID               int        `json:"user_id" db:"user_id"`
	PlanID               string     `json:"plan_id" db:"plan_id"`
	StripeSubscriptionID *string    `json:"stripe_subscription_id,omitempty" db:"stripe_subscription_id"`
	StripeCustomerID     *string    `json:"stripe_customer_id,omitempty" db:"stripe_customer_id"`
	Status               string     `json:"status" db:"status"` // 'active', 'canceled', 'past_due', 'trialing', 'incomplete'
	CurrentPeriodStart   time.Time  `json:"current_period_start" db:"current_period_start"`
	CurrentPeriodEnd     time.Time  `json:"current_period_end" db:"current_period_end"`
	CancelAtPeriodEnd    bool       `json:"cancel_at_period_end" db:"cancel_at_period_end"`
	CanceledAt           *time.Time `json:"canceled_at,omitempty" db:"canceled_at"`
	TrialEnd             *time.Time `json:"trial_end,omitempty" db:"trial_end"`
	CreatedAt            time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt            time.Time  `json:"updated_at" db:"updated_at"`
}

// UserSubscriptionWithPlan includes plan details
type UserSubscriptionWithPlan struct {
	UserSubscription
	Plan *SubscriptionPlan `json:"plan,omitempty"`
}

// PaymentMethod represents a tokenized credit card or payment method
type PaymentMethod struct {
	ID                    string    `json:"id" db:"id"`
	UserID                int       `json:"user_id" db:"user_id"`
	StripePaymentMethodID string    `json:"stripe_payment_method_id" db:"stripe_payment_method_id"`
	StripeCustomerID      string    `json:"stripe_customer_id" db:"stripe_customer_id"`
	Type                  string    `json:"type" db:"type"` // 'card', 'bank_account'
	CardBrand             *string   `json:"card_brand,omitempty" db:"card_brand"`
	CardLast4             *string   `json:"card_last4,omitempty" db:"card_last4"`
	CardExpMonth          *int      `json:"card_exp_month,omitempty" db:"card_exp_month"`
	CardExpYear           *int      `json:"card_exp_year,omitempty" db:"card_exp_year"`
	IsDefault             bool      `json:"is_default" db:"is_default"`
	CreatedAt             time.Time `json:"created_at" db:"created_at"`
	UpdatedAt             time.Time `json:"updated_at" db:"updated_at"`
}

// PaymentTransaction represents a payment or invoice
type PaymentTransaction struct {
	ID                     string     `json:"id" db:"id"`
	UserID                 int        `json:"user_id" db:"user_id"`
	SubscriptionID         *string    `json:"subscription_id,omitempty" db:"subscription_id"`
	StripePaymentIntentID  *string    `json:"stripe_payment_intent_id,omitempty" db:"stripe_payment_intent_id"`
	StripeInvoiceID        *string    `json:"stripe_invoice_id,omitempty" db:"stripe_invoice_id"`
	AmountCents            int        `json:"amount_cents" db:"amount_cents"`
	Currency               string     `json:"currency" db:"currency"`
	Status                 string     `json:"status" db:"status"` // 'succeeded', 'pending', 'failed', 'refunded'
	Description            *string    `json:"description,omitempty" db:"description"`
	FailureCode            *string    `json:"failure_code,omitempty" db:"failure_code"`
	FailureMessage         *string    `json:"failure_message,omitempty" db:"failure_message"`
	ReceiptURL             *string    `json:"receipt_url,omitempty" db:"receipt_url"`
	CreatedAt              time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt              time.Time  `json:"updated_at" db:"updated_at"`
}

// StripeWebhookEvent logs Stripe webhook events
type StripeWebhookEvent struct {
	ID            string     `json:"id" db:"id"`
	StripeEventID string     `json:"stripe_event_id" db:"stripe_event_id"`
	EventType     string     `json:"event_type" db:"event_type"`
	PayloadJSON   string     `json:"-" db:"payload_json"`
	Processed     bool       `json:"processed" db:"processed"`
	ErrorMessage  *string    `json:"error_message,omitempty" db:"error_message"`
	CreatedAt     time.Time  `json:"created_at" db:"created_at"`
	ProcessedAt   *time.Time `json:"processed_at,omitempty" db:"processed_at"`
}

// CreateSubscriptionRequest is the request body for creating a subscription
type CreateSubscriptionRequest struct {
	PlanID          string `json:"plan_id" binding:"required"`
	PaymentMethodID string `json:"payment_method_id" binding:"required"`
}

// UpdatePaymentMethodRequest is the request body for updating payment method
type UpdatePaymentMethodRequest struct {
	PaymentMethodID string `json:"payment_method_id" binding:"required"`
}

// CancelSubscriptionRequest is the request body for canceling a subscription
type CancelSubscriptionRequest struct {
	CancelAtPeriodEnd bool `json:"cancel_at_period_end"`
}
