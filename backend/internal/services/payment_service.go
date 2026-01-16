package services

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/stripe/stripe-go/v81"
	"github.com/stripe/stripe-go/v81/customer"
	"github.com/stripe/stripe-go/v81/paymentmethod"
	"github.com/stripe/stripe-go/v81/setupintent"
	"github.com/stripe/stripe-go/v81/subscription"
	"github.com/stripe/stripe-go/v81/webhook"

	"fifteen-thirty-one/internal/models"
)

var (
	ErrInvalidPlan          = errors.New("invalid subscription plan")
	ErrInvalidPaymentMethod = errors.New("invalid payment method")
	ErrSubscriptionNotFound = errors.New("subscription not found")
	ErrCustomerNotFound     = errors.New("customer not found")
)

type PaymentService struct {
	db                 *sql.DB
	webhookSecret      string
}

func NewPaymentService(db *sql.DB, stripeSecretKey, webhookSecret string) *PaymentService {
	stripe.Key = stripeSecretKey
	return &PaymentService{
		db:                 db,
		webhookSecret:      webhookSecret,
	}
}

// GetAllPlans returns all active subscription plans
func (s *PaymentService) GetAllPlans() ([]*models.SubscriptionPlan, error) {
	query := `
		SELECT id, name, display_name, description, price_cents, currency,
		       billing_period, stripe_price_id, features_json, is_active,
		       created_at, updated_at
		FROM subscription_plans
		WHERE is_active = 1
		ORDER BY price_cents ASC
	`

	rows, err := s.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query plans: %w", err)
	}
	defer rows.Close()

	var plans []*models.SubscriptionPlan
	for rows.Next() {
		var plan models.SubscriptionPlan
		err := rows.Scan(
			&plan.ID, &plan.Name, &plan.DisplayName, &plan.Description,
			&plan.PriceCents, &plan.Currency, &plan.BillingPeriod,
			&plan.StripePriceID, &plan.FeaturesJSON, &plan.IsActive,
			&plan.CreatedAt, &plan.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan plan: %w", err)
		}

		// Parse features JSON
		if err := json.Unmarshal([]byte(plan.FeaturesJSON), &plan.Features); err != nil {
			plan.Features = []string{}
		}

		plans = append(plans, &plan)
	}

	return plans, nil
}

// GetPlanByID retrieves a subscription plan by ID
func (s *PaymentService) GetPlanByID(planID string) (*models.SubscriptionPlan, error) {
	query := `
		SELECT id, name, display_name, description, price_cents, currency,
		       billing_period, stripe_price_id, features_json, is_active,
		       created_at, updated_at
		FROM subscription_plans
		WHERE id = ? AND is_active = 1
	`

	var plan models.SubscriptionPlan
	err := s.db.QueryRow(query, planID).Scan(
		&plan.ID, &plan.Name, &plan.DisplayName, &plan.Description,
		&plan.PriceCents, &plan.Currency, &plan.BillingPeriod,
		&plan.StripePriceID, &plan.FeaturesJSON, &plan.IsActive,
		&plan.CreatedAt, &plan.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, ErrInvalidPlan
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get plan: %w", err)
	}

	// Parse features JSON
	if err := json.Unmarshal([]byte(plan.FeaturesJSON), &plan.Features); err != nil {
		plan.Features = []string{}
	}

	return &plan, nil
}

// GetOrCreateStripeCustomer gets or creates a Stripe customer for a user
func (s *PaymentService) GetOrCreateStripeCustomer(userID int, email, username string) (string, error) {
	// Check if user already has a subscription with customer ID
	var existingCustomerID *string
	query := `SELECT stripe_customer_id FROM user_subscriptions WHERE user_id = ? AND stripe_customer_id IS NOT NULL LIMIT 1`
	err := s.db.QueryRow(query, userID).Scan(&existingCustomerID)
	if err == nil && existingCustomerID != nil {
		return *existingCustomerID, nil
	}

	// Create new Stripe customer
	params := &stripe.CustomerParams{
		Email: stripe.String(email),
		Metadata: map[string]string{
			"user_id":  fmt.Sprintf("%d", userID),
			"username": username,
		},
	}

	cust, err := customer.New(params)
	if err != nil {
		return "", fmt.Errorf("failed to create Stripe customer: %w", err)
	}

	return cust.ID, nil
}

// AttachPaymentMethod attaches a payment method to a customer
func (s *PaymentService) AttachPaymentMethod(userID int, stripeCustomerID, paymentMethodID string) (*models.PaymentMethod, error) {
	// Attach payment method to customer in Stripe
	params := &stripe.PaymentMethodAttachParams{
		Customer: stripe.String(stripeCustomerID),
	}
	pm, err := paymentmethod.Attach(paymentMethodID, params)
	if err != nil {
		return "", fmt.Errorf("failed to attach payment method: %w", err)
	}

	// Set as default payment method for customer
	customerParams := &stripe.CustomerParams{
		InvoiceSettings: &stripe.CustomerInvoiceSettingsParams{
			DefaultPaymentMethod: stripe.String(paymentMethodID),
		},
	}
	_, err = customer.Update(stripeCustomerID, customerParams)
	if err != nil {
		return nil, fmt.Errorf("failed to set default payment method: %w", err)
	}

	// Unset any existing default payment methods
	_, err = s.db.Exec(`UPDATE payment_methods SET is_default = 0 WHERE user_id = ?`, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to unset default payment methods: %w", err)
	}

	// Save payment method to database
	paymentMethodRecord := &models.PaymentMethod{
		ID:                    uuid.New().String(),
		UserID:                userID,
		StripePaymentMethodID: pm.ID,
		StripeCustomerID:      stripeCustomerID,
		Type:                  string(pm.Type),
		IsDefault:             true,
		CreatedAt:             time.Now(),
		UpdatedAt:             time.Now(),
	}

	// Extract card details if it's a card payment method
	if pm.Card != nil {
		paymentMethodRecord.CardBrand = &pm.Card.Brand
		paymentMethodRecord.CardLast4 = &pm.Card.Last4
		expMonth := int(pm.Card.ExpMonth)
		expYear := int(pm.Card.ExpYear)
		paymentMethodRecord.CardExpMonth = &expMonth
		paymentMethodRecord.CardExpYear = &expYear
	}

	query := `
		INSERT INTO payment_methods (
			id, user_id, stripe_payment_method_id, stripe_customer_id,
			type, card_brand, card_last4, card_exp_month, card_exp_year,
			is_default, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err = s.db.Exec(query,
		paymentMethodRecord.ID, paymentMethodRecord.UserID,
		paymentMethodRecord.StripePaymentMethodID, paymentMethodRecord.StripeCustomerID,
		paymentMethodRecord.Type, paymentMethodRecord.CardBrand, paymentMethodRecord.CardLast4,
		paymentMethodRecord.CardExpMonth, paymentMethodRecord.CardExpYear,
		paymentMethodRecord.IsDefault, paymentMethodRecord.CreatedAt, paymentMethodRecord.UpdatedAt,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to save payment method: %w", err)
	}

	return paymentMethodRecord, nil
}

// CreateSubscription creates a new subscription for a user
func (s *PaymentService) CreateSubscription(userID int, planID, paymentMethodID, stripeCustomerID string) (*models.UserSubscription, error) {
	// Get the plan
	plan, err := s.GetPlanByID(planID)
	if err != nil {
		return nil, err
	}

	// Check if plan requires Stripe (free plans don't)
	if plan.PriceCents == 0 {
		// Create free subscription without Stripe
		return s.createFreeSubscription(userID, planID)
	}

	// For paid plans, create Stripe subscription
	if plan.StripePriceID == nil {
		return nil, fmt.Errorf("plan missing Stripe price ID")
	}

	params := &stripe.SubscriptionParams{
		Customer: stripe.String(stripeCustomerID),
		Items: []*stripe.SubscriptionItemsParams{
			{
				Price: stripe.String(*plan.StripePriceID),
			},
		},
		DefaultPaymentMethod: stripe.String(paymentMethodID),
		Metadata: map[string]string{
			"user_id": fmt.Sprintf("%d", userID),
			"plan_id": planID,
		},
	}

	sub, err := subscription.New(params)
	if err != nil {
		return nil, fmt.Errorf("failed to create Stripe subscription: %w", err)
	}

	// Save subscription to database
	userSub := &models.UserSubscription{
		ID:                   uuid.New().String(),
		UserID:               userID,
		PlanID:               planID,
		StripeSubscriptionID: &sub.ID,
		StripeCustomerID:     &stripeCustomerID,
		Status:               string(sub.Status),
		CurrentPeriodStart:   time.Unix(sub.CurrentPeriodStart, 0),
		CurrentPeriodEnd:     time.Unix(sub.CurrentPeriodEnd, 0),
		CancelAtPeriodEnd:    sub.CancelAtPeriodEnd,
		CreatedAt:            time.Now(),
		UpdatedAt:            time.Now(),
	}

	if sub.TrialEnd > 0 {
		trialEnd := time.Unix(sub.TrialEnd, 0)
		userSub.TrialEnd = &trialEnd
	}

	query := `
		INSERT INTO user_subscriptions (
			id, user_id, plan_id, stripe_subscription_id, stripe_customer_id,
			status, current_period_start, current_period_end,
			cancel_at_period_end, trial_end, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err = s.db.Exec(query,
		userSub.ID, userSub.UserID, userSub.PlanID,
		userSub.StripeSubscriptionID, userSub.StripeCustomerID,
		userSub.Status, userSub.CurrentPeriodStart, userSub.CurrentPeriodEnd,
		userSub.CancelAtPeriodEnd, userSub.TrialEnd,
		userSub.CreatedAt, userSub.UpdatedAt,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to save subscription: %w", err)
	}

	return userSub, nil
}

// createFreeSubscription creates a free subscription without Stripe
func (s *PaymentService) createFreeSubscription(userID int, planID string) (*models.UserSubscription, error) {
	now := time.Now()
	endDate := now.AddDate(100, 0, 0) // Free subscriptions never expire

	userSub := &models.UserSubscription{
		ID:                 uuid.New().String(),
		UserID:             userID,
		PlanID:             planID,
		Status:             "active",
		CurrentPeriodStart: now,
		CurrentPeriodEnd:   endDate,
		CancelAtPeriodEnd:  false,
		CreatedAt:          now,
		UpdatedAt:          now,
	}

	query := `
		INSERT INTO user_subscriptions (
			id, user_id, plan_id, status, current_period_start,
			current_period_end, cancel_at_period_end, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err := s.db.Exec(query,
		userSub.ID, userSub.UserID, userSub.PlanID, userSub.Status,
		userSub.CurrentPeriodStart, userSub.CurrentPeriodEnd,
		userSub.CancelAtPeriodEnd, userSub.CreatedAt, userSub.UpdatedAt,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to create free subscription: %w", err)
	}

	return userSub, nil
}

// GetUserSubscription retrieves a user's active subscription
func (s *PaymentService) GetUserSubscription(userID int) (*models.UserSubscriptionWithPlan, error) {
	query := `
		SELECT s.id, s.user_id, s.plan_id, s.stripe_subscription_id, s.stripe_customer_id,
		       s.status, s.current_period_start, s.current_period_end,
		       s.cancel_at_period_end, s.canceled_at, s.trial_end,
		       s.created_at, s.updated_at,
		       p.id, p.name, p.display_name, p.description, p.price_cents,
		       p.currency, p.billing_period, p.stripe_price_id, p.features_json,
		       p.is_active, p.created_at, p.updated_at
		FROM user_subscriptions s
		JOIN subscription_plans p ON s.plan_id = p.id
		WHERE s.user_id = ? AND s.status IN ('active', 'trialing')
		ORDER BY s.created_at DESC
		LIMIT 1
	`

	var result models.UserSubscriptionWithPlan
	var plan models.SubscriptionPlan

	err := s.db.QueryRow(query, userID).Scan(
		&result.ID, &result.UserID, &result.PlanID,
		&result.StripeSubscriptionID, &result.StripeCustomerID,
		&result.Status, &result.CurrentPeriodStart, &result.CurrentPeriodEnd,
		&result.CancelAtPeriodEnd, &result.CanceledAt, &result.TrialEnd,
		&result.CreatedAt, &result.UpdatedAt,
		&plan.ID, &plan.Name, &plan.DisplayName, &plan.Description,
		&plan.PriceCents, &plan.Currency, &plan.BillingPeriod,
		&plan.StripePriceID, &plan.FeaturesJSON, &plan.IsActive,
		&plan.CreatedAt, &plan.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, ErrSubscriptionNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get subscription: %w", err)
	}

	// Parse features JSON
	if err := json.Unmarshal([]byte(plan.FeaturesJSON), &plan.Features); err != nil {
		plan.Features = []string{}
	}

	result.Plan = &plan
	return &result, nil
}

// CancelSubscription cancels a user's subscription
func (s *PaymentService) CancelSubscription(userID int, cancelAtPeriodEnd bool) error {
	// Get current subscription
	userSub, err := s.GetUserSubscription(userID)
	if err != nil {
		return err
	}

	// If it's a Stripe subscription, cancel via Stripe
	if userSub.StripeSubscriptionID != nil {
		params := &stripe.SubscriptionParams{
			CancelAtPeriodEnd: stripe.Bool(cancelAtPeriodEnd),
		}

		if !cancelAtPeriodEnd {
			params.CancelAtPeriodEnd = stripe.Bool(false)
			// Immediately cancel
			_, err = subscription.Cancel(*userSub.StripeSubscriptionID, nil)
		} else {
			_, err = subscription.Update(*userSub.StripeSubscriptionID, params)
		}

		if err != nil {
			return fmt.Errorf("failed to cancel Stripe subscription: %w", err)
		}
	}

	// Update database
	now := time.Now()
	query := `
		UPDATE user_subscriptions
		SET cancel_at_period_end = ?, canceled_at = ?, status = ?, updated_at = ?
		WHERE id = ?
	`

	status := userSub.Status
	if !cancelAtPeriodEnd {
		status = "canceled"
	}

	_, err = s.db.Exec(query, cancelAtPeriodEnd, now, status, now, userSub.ID)
	if err != nil {
		return fmt.Errorf("failed to update subscription: %w", err)
	}

	return nil
}

// GetPaymentMethods retrieves all payment methods for a user
func (s *PaymentService) GetPaymentMethods(userID int) ([]*models.PaymentMethod, error) {
	query := `
		SELECT id, user_id, stripe_payment_method_id, stripe_customer_id,
		       type, card_brand, card_last4, card_exp_month, card_exp_year,
		       is_default, created_at, updated_at
		FROM payment_methods
		WHERE user_id = ?
		ORDER BY is_default DESC, created_at DESC
	`

	rows, err := s.db.Query(query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to query payment methods: %w", err)
	}
	defer rows.Close()

	var methods []*models.PaymentMethod
	for rows.Next() {
		var method models.PaymentMethod
		err := rows.Scan(
			&method.ID, &method.UserID, &method.StripePaymentMethodID,
			&method.StripeCustomerID, &method.Type, &method.CardBrand,
			&method.CardLast4, &method.CardExpMonth, &method.CardExpYear,
			&method.IsDefault, &method.CreatedAt, &method.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan payment method: %w", err)
		}
		methods = append(methods, &method)
	}

	return methods, nil
}

// CreateSetupIntent creates a Stripe Setup Intent for collecting payment method
func (s *PaymentService) CreateSetupIntent(customerID string) (*stripe.SetupIntent, error) {
	params := &stripe.SetupIntentParams{
		Customer: stripe.String(customerID),
		PaymentMethodTypes: stripe.StringSlice([]string{
			"card",
		}),
	}

	si, err := setupintent.New(params)
	if err != nil {
		return nil, fmt.Errorf("failed to create setup intent: %w", err)
	}

	return si, nil
}

// UpdateSubscriptionPaymentMethod updates the payment method for a subscription
func (s *PaymentService) UpdateSubscriptionPaymentMethod(subscriptionID, paymentMethodID string) error {
	params := &stripe.SubscriptionParams{
		DefaultPaymentMethod: stripe.String(paymentMethodID),
	}

	_, err := subscription.Update(subscriptionID, params)
	if err != nil {
		return fmt.Errorf("failed to update subscription payment method: %w", err)
	}

	return nil
}

// HandleStripeWebhook handles incoming Stripe webhook events
func (s *PaymentService) HandleStripeWebhook(payload []byte, signature string) error {
	event, err := webhook.ConstructEvent(payload, signature, s.webhookSecret)
	if err != nil {
		return fmt.Errorf("failed to verify webhook signature: %w", err)
	}

	// Log the webhook event
	eventID := uuid.New().String()
	query := `
		INSERT INTO stripe_webhook_events (id, stripe_event_id, event_type, payload_json, processed, created_at)
		VALUES (?, ?, ?, ?, 0, ?)
	`
	_, err = s.db.Exec(query, eventID, event.ID, event.Type, string(payload), time.Now())
	if err != nil {
		return fmt.Errorf("failed to log webhook event: %w", err)
	}

	// Handle specific event types
	switch event.Type {
	case "customer.subscription.updated":
		return s.handleSubscriptionUpdated(event, eventID)
	case "customer.subscription.deleted":
		return s.handleSubscriptionDeleted(event, eventID)
	case "invoice.payment_succeeded":
		return s.handleInvoicePaymentSucceeded(event, eventID)
	case "invoice.payment_failed":
		return s.handleInvoicePaymentFailed(event, eventID)
	}

	// Mark as processed for events we don't handle
	_, err = s.db.Exec(`UPDATE stripe_webhook_events SET processed = 1, processed_at = ? WHERE id = ?`, time.Now(), eventID)
	return err
}

func (s *PaymentService) handleSubscriptionUpdated(event stripe.Event, eventID string) error {
	var sub stripe.Subscription
	if err := json.Unmarshal(event.Data.Raw, &sub); err != nil {
		return fmt.Errorf("failed to parse subscription: %w", err)
	}

	// Update subscription in database
	query := `
		UPDATE user_subscriptions
		SET status = ?, current_period_start = ?, current_period_end = ?,
		    cancel_at_period_end = ?, updated_at = ?
		WHERE stripe_subscription_id = ?
	`

	_, err := s.db.Exec(query,
		string(sub.Status),
		time.Unix(sub.CurrentPeriodStart, 0),
		time.Unix(sub.CurrentPeriodEnd, 0),
		sub.CancelAtPeriodEnd,
		time.Now(),
		sub.ID,
	)

	if err != nil {
		return fmt.Errorf("failed to update subscription: %w", err)
	}

	// Mark webhook as processed
	_, err = s.db.Exec(`UPDATE stripe_webhook_events SET processed = 1, processed_at = ? WHERE id = ?`, time.Now(), eventID)
	return err
}

func (s *PaymentService) handleSubscriptionDeleted(event stripe.Event, eventID string) error {
	var sub stripe.Subscription
	if err := json.Unmarshal(event.Data.Raw, &sub); err != nil {
		return fmt.Errorf("failed to parse subscription: %w", err)
	}

	// Update subscription status to canceled
	query := `
		UPDATE user_subscriptions
		SET status = 'canceled', canceled_at = ?, updated_at = ?
		WHERE stripe_subscription_id = ?
	`

	_, err := s.db.Exec(query, time.Now(), time.Now(), sub.ID)
	if err != nil {
		return fmt.Errorf("failed to cancel subscription: %w", err)
	}

	// Mark webhook as processed
	_, err = s.db.Exec(`UPDATE stripe_webhook_events SET processed = 1, processed_at = ? WHERE id = ?`, time.Now(), eventID)
	return err
}

func (s *PaymentService) handleInvoicePaymentSucceeded(event stripe.Event, eventID string) error {
	var invoice stripe.Invoice
	if err := json.Unmarshal(event.Data.Raw, &invoice); err != nil {
		return fmt.Errorf("failed to parse invoice: %w", err)
	}

	// Get user ID from subscription metadata
	if invoice.Subscription == nil {
		return nil // Not a subscription payment
	}

	var userID int
	query := `SELECT user_id FROM user_subscriptions WHERE stripe_subscription_id = ?`
	err := s.db.QueryRow(query, invoice.Subscription.ID).Scan(&userID)
	if err != nil {
		return fmt.Errorf("failed to get user ID: %w", err)
	}

	// Check if subscription exists in database
	var subscriptionID *string
	subQuery := `SELECT id FROM user_subscriptions WHERE stripe_subscription_id = ?`
	err = s.db.QueryRow(subQuery, invoice.Subscription.ID).Scan(&subscriptionID)
	if err != nil && err != sql.ErrNoRows {
		return fmt.Errorf("failed to check subscription: %w", err)
	}

	// Record payment transaction
	transactionID := uuid.New().String()
	insertQuery := `
		INSERT INTO payment_transactions (
			id, user_id, subscription_id, stripe_payment_intent_id, stripe_invoice_id,
			amount_cents, currency, status, description, receipt_url, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, 'succeeded', ?, ?, ?, ?)
	`

	description := fmt.Sprintf("Payment for invoice %s", invoice.Number)
	_, err = s.db.Exec(insertQuery,
		transactionID, userID, subscriptionID, invoice.PaymentIntent.ID, invoice.ID,
		invoice.AmountPaid, string(invoice.Currency), description,
		invoice.HostedInvoiceURL, time.Now(), time.Now(),
	)

	if err != nil {
		return fmt.Errorf("failed to record transaction: %w", err)
	}

	// Mark webhook as processed
	_, err = s.db.Exec(`UPDATE stripe_webhook_events SET processed = 1, processed_at = ? WHERE id = ?`, time.Now(), eventID)
	return err
}

func (s *PaymentService) handleInvoicePaymentFailed(event stripe.Event, eventID string) error {
	var invoice stripe.Invoice
	if err := json.Unmarshal(event.Data.Raw, &invoice); err != nil {
		return fmt.Errorf("failed to parse invoice: %w", err)
	}

	// Get user ID from subscription
	if invoice.Subscription == nil {
		return nil
	}

	var userID int
	query := `SELECT user_id FROM user_subscriptions WHERE stripe_subscription_id = ?`
	err := s.db.QueryRow(query, invoice.Subscription.ID).Scan(&userID)
	if err != nil {
		return fmt.Errorf("failed to get user ID: %w", err)
	}

	// Check if subscription exists
	var subscriptionID *string
	subQuery := `SELECT id FROM user_subscriptions WHERE stripe_subscription_id = ?`
	err = s.db.QueryRow(subQuery, invoice.Subscription.ID).Scan(&subscriptionID)
	if err != nil && err != sql.ErrNoRows {
		return fmt.Errorf("failed to check subscription: %w", err)
	}

	// Record failed payment transaction
	transactionID := uuid.New().String()
	insertQuery := `
		INSERT INTO payment_transactions (
			id, user_id, subscription_id, stripe_payment_intent_id, stripe_invoice_id,
			amount_cents, currency, status, description, failure_code, failure_message,
			created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, 'failed', ?, ?, ?, ?, ?)
	`

	description := fmt.Sprintf("Failed payment for invoice %s", invoice.Number)
	var failureCode, failureMessage *string
	if invoice.PaymentIntent != nil && invoice.PaymentIntent.LastPaymentError != nil {
		failureCode = &invoice.PaymentIntent.LastPaymentError.Code
		failureMessage = &invoice.PaymentIntent.LastPaymentError.Message
	}

	_, err = s.db.Exec(insertQuery,
		transactionID, userID, subscriptionID,
		invoice.PaymentIntent.ID, invoice.ID,
		invoice.AmountDue, string(invoice.Currency),
		description, failureCode, failureMessage,
		time.Now(), time.Now(),
	)

	if err != nil {
		return fmt.Errorf("failed to record failed transaction: %w", err)
	}

	// Update subscription status to past_due
	updateQuery := `
		UPDATE user_subscriptions
		SET status = 'past_due', updated_at = ?
		WHERE stripe_subscription_id = ?
	`
	_, err = s.db.Exec(updateQuery, time.Now(), invoice.Subscription.ID)
	if err != nil {
		return fmt.Errorf("failed to update subscription status: %w", err)
	}

	// Mark webhook as processed
	_, err = s.db.Exec(`UPDATE stripe_webhook_events SET processed = 1, processed_at = ? WHERE id = ?`, time.Now(), eventID)
	return err
}
