package handlers

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"fifteen-thirty-one-go/backend/internal/config"
	"fifteen-thirty-one-go/backend/internal/models"
	"fifteen-thirty-one-go/backend/internal/tracing"

	"github.com/gin-gonic/gin"
	"github.com/stripe/stripe-go/v78"
	"github.com/stripe/stripe-go/v78/checkout/session"
	"github.com/stripe/stripe-go/v78/customer"
	"github.com/stripe/stripe-go/v78/subscription"
	"github.com/stripe/stripe-go/v78/webhook"
)

func CreateCheckoutSessionHandler(db *sql.DB, cfg config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		startTime := time.Now()
		_, span := tracing.StartSpan(c.Request.Context(), "handlers.CreateCheckoutSessionHandler")
		defer span.End()

		log.Printf("CreateCheckoutSessionHandler: request started")

		userID, ok := userIDFromContext(c)
		if !ok {
			log.Printf("CreateCheckoutSessionHandler: unauthorized request - no user_id in context")
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}
		log.Printf("CreateCheckoutSessionHandler: user_id=%d", userID)

		if cfg.StripeSecretKey == "" || cfg.StripePriceID == "" {
			log.Printf("CreateCheckoutSessionHandler: premium features not configured - StripeSecretKey empty=%v StripePriceID empty=%v",
				cfg.StripeSecretKey == "", cfg.StripePriceID == "")
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "premium features not configured"})
			return
		}

		stripe.Key = cfg.StripeSecretKey
		log.Printf("CreateCheckoutSessionHandler: Stripe configured with price_id=%s frontend_url=%s", cfg.StripePriceID, cfg.FrontendURL)

		user, err := models.GetUserByID(db, userID)
		if err != nil {
			log.Printf("CreateCheckoutSessionHandler: GetUserByID failed - user_id=%d err=%v", userID, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
			return
		}
		log.Printf("CreateCheckoutSessionHandler: user found - user_id=%d username=%s", user.ID, user.Username)

		// Check if user already has an active subscription
		existingSub, err := models.GetPremiumSubscriptionByUserID(db, userID)
		if err != nil && !errors.Is(err, models.ErrNotFound) {
			log.Printf("CreateCheckoutSessionHandler: GetPremiumSubscriptionByUserID failed - user_id=%d err=%v", userID, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
			return
		}

		if existingSub != nil {
			isActive := existingSub.Status == "active" && existingSub.CurrentPeriodEnd.After(time.Now())
			log.Printf("CreateCheckoutSessionHandler: existing subscription found - user_id=%d subscription_id=%d status=%s period_end=%v is_active=%v",
				userID, existingSub.ID, existingSub.Status, existingSub.CurrentPeriodEnd, isActive)
			if isActive {
				log.Printf("CreateCheckoutSessionHandler: user already has active subscription - user_id=%d subscription_id=%d", userID, existingSub.ID)
				c.JSON(http.StatusBadRequest, gin.H{"error": "user already has active subscription"})
				return
			}
		} else {
			log.Printf("CreateCheckoutSessionHandler: no existing subscription found - user_id=%d", userID)
		}

		// Create or get Stripe customer
		var stripeCustomerID string
		if existingSub != nil && existingSub.StripeCustomerID != "" {
			stripeCustomerID = existingSub.StripeCustomerID
			log.Printf("CreateCheckoutSessionHandler: using existing Stripe customer - user_id=%d stripe_customer_id=%s",
				userID, stripeCustomerID)
		} else {
			log.Printf("CreateCheckoutSessionHandler: creating new Stripe customer - user_id=%d username=%s", userID, user.Username)
			params := &stripe.CustomerParams{
				Email: stripe.String(user.Username + "@example.com"), // Placeholder - adjust as needed
				Metadata: map[string]string{
					"user_id":  fmt.Sprintf("%d", userID),
					"username": user.Username,
				},
			}
			cust, err := customer.New(params)
			if err != nil {
				log.Printf("CreateCheckoutSessionHandler: Stripe customer.New failed - user_id=%d username=%s err=%v stripe_error_type=%s",
					userID, user.Username, err, getStripeErrorType(err))
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create customer"})
				return
			}
			stripeCustomerID = cust.ID
			log.Printf("CreateCheckoutSessionHandler: Stripe customer created - user_id=%d stripe_customer_id=%s email=%s",
				userID, stripeCustomerID, cust.Email)
		}

		// Create checkout session
		log.Printf("CreateCheckoutSessionHandler: creating Stripe checkout session - user_id=%d stripe_customer_id=%s price_id=%s",
			userID, stripeCustomerID, cfg.StripePriceID)
		params := &stripe.CheckoutSessionParams{
			Customer:  stripe.String(stripeCustomerID),
			Mode:      stripe.String(string(stripe.CheckoutSessionModeSubscription)),
			LineItems: []*stripe.CheckoutSessionLineItemParams{
				{
					Price:    stripe.String(cfg.StripePriceID),
					Quantity: stripe.Int64(1),
				},
			},
			SuccessURL: stripe.String(cfg.FrontendURL + "/premium?success=true"),
			CancelURL:  stripe.String(cfg.FrontendURL + "/premium?canceled=true"),
			Metadata: map[string]string{
				"user_id": fmt.Sprintf("%d", userID),
			},
		}
		sess, err := session.New(params)
		if err != nil {
			log.Printf("CreateCheckoutSessionHandler: Stripe session.New failed - user_id=%d stripe_customer_id=%s price_id=%s err=%v stripe_error_type=%s",
				userID, stripeCustomerID, cfg.StripePriceID, err, getStripeErrorType(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create checkout session"})
			return
		}
		duration := time.Since(startTime)
		log.Printf("CreateCheckoutSessionHandler: checkout session created successfully - user_id=%d session_id=%s url=%s duration_ms=%d",
			userID, sess.ID, sess.URL, duration.Milliseconds())

		c.JSON(http.StatusOK, gin.H{"session_id": sess.ID, "url": sess.URL})
	}
}

func StripeWebhookHandler(db *sql.DB, cfg config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		startTime := time.Now()
		_, span := tracing.StartSpan(c.Request.Context(), "handlers.StripeWebhookHandler")
		defer span.End()

		log.Printf("StripeWebhookHandler: webhook received - remote_addr=%s user_agent=%s",
			c.ClientIP(), c.GetHeader("User-Agent"))

		if cfg.StripeSecretKey == "" {
			log.Printf("StripeWebhookHandler: premium features not configured - rejecting webhook")
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "premium features not configured"})
			return
		}

		const MaxBodyBytes = int64(65536)
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, MaxBodyBytes)
		payload, err := c.GetRawData()
		if err != nil {
			log.Printf("StripeWebhookHandler: error reading request body - err=%v body_size_limit=%d", err, MaxBodyBytes)
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "error reading request body"})
			return
		}
		log.Printf("StripeWebhookHandler: payload read - size_bytes=%d", len(payload))

		// Get the Stripe signature from the request header
		sigHeader := c.GetHeader("Stripe-Signature")
		if sigHeader == "" {
			log.Printf("StripeWebhookHandler: missing Stripe-Signature header - rejecting webhook")
			c.JSON(http.StatusBadRequest, gin.H{"error": "missing stripe signature"})
			return
		}
		log.Printf("StripeWebhookHandler: signature header present - length=%d", len(sigHeader))

		// Get webhook secret from environment
		webhookSecret := cfg.StripeSecretKey // In production, use a separate webhook secret
		event, err := webhook.ConstructEvent(payload, sigHeader, webhookSecret)
		if err != nil {
			log.Printf("StripeWebhookHandler: webhook signature verification failed - err=%v payload_size=%d signature_length=%d",
				err, len(payload), len(sigHeader))
			c.JSON(http.StatusBadRequest, gin.H{"error": "webhook signature verification failed"})
			return
		}
		log.Printf("StripeWebhookHandler: webhook verified - event_id=%s event_type=%s livemode=%v",
			event.ID, event.Type, event.Livemode)

		// Handle the event
		switch event.Type {
		case "checkout.session.completed":
			log.Printf("StripeWebhookHandler: processing checkout.session.completed - event_id=%s", event.ID)
			var sess stripe.CheckoutSession
			err := json.Unmarshal(event.Data.Raw, &sess)
			if err != nil {
				log.Printf("StripeWebhookHandler: error parsing checkout.session.completed - event_id=%s err=%v payload_size=%d",
					event.ID, err, len(event.Data.Raw))
				c.JSON(http.StatusBadRequest, gin.H{"error": "error parsing webhook"})
				return
			}
			log.Printf("StripeWebhookHandler: checkout session parsed - event_id=%s session_id=%s customer_id=%s subscription_id=%s",
				event.ID, sess.ID, sess.Customer.ID, sess.Subscription.ID)
			handleCheckoutSessionCompleted(db, &sess, event.ID)

		case "customer.subscription.updated":
			log.Printf("StripeWebhookHandler: processing customer.subscription.updated - event_id=%s", event.ID)
			var sub stripe.Subscription
			err := json.Unmarshal(event.Data.Raw, &sub)
			if err != nil {
				log.Printf("StripeWebhookHandler: error parsing customer.subscription.updated - event_id=%s err=%v payload_size=%d",
					event.ID, err, len(event.Data.Raw))
				c.JSON(http.StatusBadRequest, gin.H{"error": "error parsing webhook"})
				return
			}
			log.Printf("StripeWebhookHandler: subscription updated parsed - event_id=%s subscription_id=%s customer_id=%s status=%s",
				event.ID, sub.ID, sub.Customer.ID, sub.Status)
			handleSubscriptionUpdate(db, &sub, event.ID, "updated")

		case "customer.subscription.deleted":
			log.Printf("StripeWebhookHandler: processing customer.subscription.deleted - event_id=%s", event.ID)
			var sub stripe.Subscription
			err := json.Unmarshal(event.Data.Raw, &sub)
			if err != nil {
				log.Printf("StripeWebhookHandler: error parsing customer.subscription.deleted - event_id=%s err=%v payload_size=%d",
					event.ID, err, len(event.Data.Raw))
				c.JSON(http.StatusBadRequest, gin.H{"error": "error parsing webhook"})
				return
			}
			log.Printf("StripeWebhookHandler: subscription deleted parsed - event_id=%s subscription_id=%s customer_id=%s",
				event.ID, sub.ID, sub.Customer.ID)
			handleSubscriptionUpdate(db, &sub, event.ID, "deleted")

		default:
			log.Printf("StripeWebhookHandler: unhandled event type - event_id=%s event_type=%s", event.ID, event.Type)
		}

		duration := time.Since(startTime)
		log.Printf("StripeWebhookHandler: webhook processed successfully - event_id=%s event_type=%s duration_ms=%d",
			event.ID, event.Type, duration.Milliseconds())
		c.JSON(http.StatusOK, gin.H{"received": true})
	}
}

func handleCheckoutSessionCompleted(db *sql.DB, sess *stripe.CheckoutSession, eventID string) {
	startTime := time.Now()
	log.Printf("handleCheckoutSessionCompleted: processing - event_id=%s session_id=%s customer_id=%s",
		eventID, sess.ID, sess.Customer.ID)

	userIDStr := sess.Metadata["user_id"]
	if userIDStr == "" {
		log.Printf("handleCheckoutSessionCompleted: CRITICAL - missing user_id in metadata - event_id=%s session_id=%s metadata=%+v",
			eventID, sess.ID, sess.Metadata)
		return
	}

	var userID int64
	_, err := fmt.Sscanf(userIDStr, "%d", &userID)
	if err != nil {
		log.Printf("handleCheckoutSessionCompleted: CRITICAL - invalid user_id format - event_id=%s session_id=%s user_id_str=%s err=%v",
			eventID, sess.ID, userIDStr, err)
		return
	}
	log.Printf("handleCheckoutSessionCompleted: user_id extracted - event_id=%s session_id=%s user_id=%d",
		eventID, sess.ID, userID)

	// Get subscription details
	subID := sess.Subscription.ID
	log.Printf("handleCheckoutSessionCompleted: fetching subscription from Stripe - event_id=%s session_id=%s subscription_id=%s",
		eventID, sess.ID, subID)
	sub, err := subscription.Get(subID, nil)
	if err != nil {
		log.Printf("handleCheckoutSessionCompleted: CRITICAL - failed to get subscription from Stripe - event_id=%s session_id=%s subscription_id=%s err=%v stripe_error_type=%s",
			eventID, sess.ID, subID, err, getStripeErrorType(err))
		return
	}
	log.Printf("handleCheckoutSessionCompleted: subscription fetched - event_id=%s subscription_id=%s status=%s customer_id=%s period_start=%d period_end=%d",
		eventID, sub.ID, sub.Status, sub.Customer.ID, sub.CurrentPeriodStart, sub.CurrentPeriodEnd)

	// Create or update premium subscription
	periodStart := time.Unix(sub.CurrentPeriodStart, 0)
	periodEnd := time.Unix(sub.CurrentPeriodEnd, 0)
	status := string(sub.Status)
	log.Printf("handleCheckoutSessionCompleted: subscription details - event_id=%s user_id=%d status=%s period_start=%v period_end=%v",
		eventID, userID, status, periodStart, periodEnd)

	existingSub, err := models.GetPremiumSubscriptionByUserID(db, userID)
	if err != nil && !errors.Is(err, models.ErrNotFound) {
		log.Printf("handleCheckoutSessionCompleted: CRITICAL - GetPremiumSubscriptionByUserID failed - event_id=%s user_id=%d err=%v",
			eventID, userID, err)
		return
	}

	if errors.Is(err, models.ErrNotFound) || existingSub == nil {
		// Create new subscription
		log.Printf("handleCheckoutSessionCompleted: creating new premium subscription - event_id=%s user_id=%d stripe_customer_id=%s stripe_subscription_id=%s",
			eventID, userID, sess.Customer.ID, sub.ID)
		newSub, err := models.CreatePremiumSubscription(
			db, userID, sess.Customer.ID, &sub.ID, periodStart, periodEnd,
		)
		if err != nil {
			log.Printf("handleCheckoutSessionCompleted: CRITICAL - failed to create premium subscription - event_id=%s user_id=%d stripe_customer_id=%s stripe_subscription_id=%s err=%v",
				eventID, userID, sess.Customer.ID, sub.ID, err)
			return
		}
		duration := time.Since(startTime)
		log.Printf("handleCheckoutSessionCompleted: premium subscription created successfully - event_id=%s user_id=%d subscription_id=%d stripe_customer_id=%s stripe_subscription_id=%s duration_ms=%d",
			eventID, userID, newSub.ID, sess.Customer.ID, sub.ID, duration.Milliseconds())
	} else {
		// Update existing subscription
		log.Printf("handleCheckoutSessionCompleted: updating existing premium subscription - event_id=%s user_id=%d existing_subscription_id=%d existing_status=%s new_status=%s",
			eventID, userID, existingSub.ID, existingSub.Status, status)
		err = models.UpdatePremiumSubscriptionStatus(db, userID, status, periodStart, periodEnd)
		if err != nil {
			log.Printf("handleCheckoutSessionCompleted: CRITICAL - failed to update premium subscription - event_id=%s user_id=%d subscription_id=%d err=%v",
				eventID, userID, existingSub.ID, err)
			return
		}
		duration := time.Since(startTime)
		log.Printf("handleCheckoutSessionCompleted: premium subscription updated successfully - event_id=%s user_id=%d subscription_id=%d new_status=%s duration_ms=%d",
			eventID, userID, existingSub.ID, status, duration.Milliseconds())
	}
}

func handleSubscriptionUpdate(db *sql.DB, sub *stripe.Subscription, eventID, eventAction string) {
	startTime := time.Now()
	log.Printf("handleSubscriptionUpdate: processing - event_id=%s event_action=%s subscription_id=%s customer_id=%s status=%s",
		eventID, eventAction, sub.ID, sub.Customer.ID, sub.Status)

	userIDStr := sub.Metadata["user_id"]
	if userIDStr == "" {
		log.Printf("handleSubscriptionUpdate: CRITICAL - missing user_id in metadata - event_id=%s subscription_id=%s metadata=%+v",
			eventID, sub.ID, sub.Metadata)
		// Try to find subscription by Stripe subscription ID as fallback
		log.Printf("handleSubscriptionUpdate: attempting fallback lookup by stripe_subscription_id - event_id=%s subscription_id=%s",
			eventID, sub.ID)
		// Note: We would need a GetPremiumSubscriptionByStripeSubscriptionID function
		// For now, log the issue and return
		return
	}

	var userID int64
	_, err := fmt.Sscanf(userIDStr, "%d", &userID)
	if err != nil {
		log.Printf("handleSubscriptionUpdate: CRITICAL - invalid user_id format - event_id=%s subscription_id=%s user_id_str=%s err=%v",
			eventID, sub.ID, userIDStr, err)
		return
	}
	log.Printf("handleSubscriptionUpdate: user_id extracted - event_id=%s subscription_id=%s user_id=%d",
		eventID, sub.ID, userID)

	periodStart := time.Unix(sub.CurrentPeriodStart, 0)
	periodEnd := time.Unix(sub.CurrentPeriodEnd, 0)
	status := string(sub.Status)
	log.Printf("handleSubscriptionUpdate: subscription details - event_id=%s user_id=%d subscription_id=%s status=%s period_start=%v period_end=%v cancel_at_period_end=%v",
		eventID, userID, sub.ID, status, periodStart, periodEnd, sub.CancelAtPeriodEnd)

	// Verify subscription exists before updating
	existingSub, err := models.GetPremiumSubscriptionByUserID(db, userID)
	if err != nil {
		if errors.Is(err, models.ErrNotFound) {
			log.Printf("handleSubscriptionUpdate: WARNING - subscription not found in database - event_id=%s user_id=%d stripe_subscription_id=%s (may be expected for new subscriptions)",
				eventID, userID, sub.ID)
		} else {
			log.Printf("handleSubscriptionUpdate: CRITICAL - GetPremiumSubscriptionByUserID failed - event_id=%s user_id=%d err=%v",
				eventID, userID, err)
		}
		return
	}

	log.Printf("handleSubscriptionUpdate: existing subscription found - event_id=%s user_id=%d subscription_id=%d existing_status=%s existing_stripe_subscription_id=%s new_status=%s",
		eventID, userID, existingSub.ID, existingSub.Status, getStringValue(existingSub.StripeSubscriptionID), status)

	err = models.UpdatePremiumSubscriptionStatus(db, userID, status, periodStart, periodEnd)
	if err != nil {
		log.Printf("handleSubscriptionUpdate: CRITICAL - failed to update premium subscription - event_id=%s user_id=%d subscription_id=%d new_status=%s err=%v",
			eventID, userID, existingSub.ID, status, err)
		return
	}

	duration := time.Since(startTime)
	log.Printf("handleSubscriptionUpdate: premium subscription updated successfully - event_id=%s user_id=%d subscription_id=%d old_status=%s new_status=%s duration_ms=%d",
		eventID, userID, existingSub.ID, existingSub.Status, status, duration.Milliseconds())
}

func GetPremiumStatusHandler(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		startTime := time.Now()
		_, span := tracing.StartSpan(c.Request.Context(), "handlers.GetPremiumStatusHandler")
		defer span.End()

		log.Printf("GetPremiumStatusHandler: request started")

		userID, ok := userIDFromContext(c)
		if !ok {
			log.Printf("GetPremiumStatusHandler: unauthorized request - no user_id in context")
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}
		log.Printf("GetPremiumStatusHandler: user_id=%d", userID)

		sub, err := models.GetPremiumSubscriptionByUserID(db, userID)
		if err != nil {
			if errors.Is(err, models.ErrNotFound) {
				log.Printf("GetPremiumStatusHandler: no subscription found - user_id=%d", userID)
				c.JSON(http.StatusOK, gin.H{"is_premium": false, "subscription": nil})
				return
			}
			log.Printf("GetPremiumStatusHandler: GetPremiumSubscriptionByUserID failed - user_id=%d err=%v", userID, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
			return
		}

		isActive := sub.Status == "active" && sub.CurrentPeriodEnd.After(time.Now())
		log.Printf("GetPremiumStatusHandler: subscription found - user_id=%d subscription_id=%d status=%s period_end=%v is_active=%v",
			userID, sub.ID, sub.Status, sub.CurrentPeriodEnd, isActive)

		duration := time.Since(startTime)
		log.Printf("GetPremiumStatusHandler: request completed - user_id=%d is_premium=%v duration_ms=%d",
			userID, isActive, duration.Milliseconds())

		c.JSON(http.StatusOK, gin.H{
			"is_premium": isActive,
			"subscription": sub,
		})
	}
}

func UpdateAvatarHandler(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		startTime := time.Now()
		_, span := tracing.StartSpan(c.Request.Context(), "handlers.UpdateAvatarHandler")
		defer span.End()

		log.Printf("UpdateAvatarHandler: request started")

		userID, ok := userIDFromContext(c)
		if !ok {
			log.Printf("UpdateAvatarHandler: unauthorized request - no user_id in context")
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}
		log.Printf("UpdateAvatarHandler: user_id=%d", userID)

		// Check if user has premium
		isPremium, err := models.HasActivePremiumSubscription(db, userID)
		if err != nil {
			log.Printf("UpdateAvatarHandler: HasActivePremiumSubscription failed - user_id=%d err=%v", userID, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
			return
		}
		log.Printf("UpdateAvatarHandler: premium check - user_id=%d is_premium=%v", userID, isPremium)
		if !isPremium {
			log.Printf("UpdateAvatarHandler: premium subscription required - user_id=%d", userID)
			c.JSON(http.StatusForbidden, gin.H{"error": "premium subscription required"})
			return
		}

		var req struct {
			AvatarURL *string `json:"avatar_url"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			log.Printf("UpdateAvatarHandler: invalid JSON - user_id=%d err=%v", userID, err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid json"})
			return
		}

		avatarURLStr := "nil"
		if req.AvatarURL != nil {
			avatarURLStr = *req.AvatarURL
		}
		log.Printf("UpdateAvatarHandler: request parsed - user_id=%d avatar_url=%s", userID, avatarURLStr)

		// Validate URL format (basic validation)
		if req.AvatarURL != nil && *req.AvatarURL != "" {
			if !strings.HasPrefix(*req.AvatarURL, "http://") && !strings.HasPrefix(*req.AvatarURL, "https://") {
				log.Printf("UpdateAvatarHandler: invalid avatar URL format - user_id=%d avatar_url=%s", userID, *req.AvatarURL)
				c.JSON(http.StatusBadRequest, gin.H{"error": "invalid avatar URL format"})
				return
			}
			// Log URL length for monitoring
			if len(*req.AvatarURL) > 2048 {
				log.Printf("UpdateAvatarHandler: WARNING - avatar URL is very long - user_id=%d url_length=%d", userID, len(*req.AvatarURL))
			}
		}

		log.Printf("UpdateAvatarHandler: updating avatar URL - user_id=%d avatar_url=%s", userID, avatarURLStr)
		err = models.UpdateUserAvatarURL(db, userID, req.AvatarURL)
		if err != nil {
			log.Printf("UpdateAvatarHandler: CRITICAL - UpdateUserAvatarURL failed - user_id=%d avatar_url=%s err=%v",
				userID, avatarURLStr, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
			return
		}

		user, err := models.GetUserByID(db, userID)
		if err != nil {
			log.Printf("UpdateAvatarHandler: CRITICAL - GetUserByID failed after update - user_id=%d err=%v", userID, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
			return
		}

		duration := time.Since(startTime)
		log.Printf("UpdateAvatarHandler: avatar updated successfully - user_id=%d avatar_url=%s duration_ms=%d",
			userID, avatarURLStr, duration.Milliseconds())

		c.JSON(http.StatusOK, gin.H{"user": user})
	}
}

// Helper functions for logging

func getStripeErrorType(err error) string {
	if err == nil {
		return "none"
	}
	// Stripe errors typically have a type field, but we'll use the error string
	return fmt.Sprintf("%T", err)
}

func getStringValue(s *string) string {
	if s == nil {
		return "nil"
	}
	return *s
}

