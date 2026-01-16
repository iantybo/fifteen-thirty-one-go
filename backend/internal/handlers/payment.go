package handlers

import (
	"database/sql"
	"net/http"

	"github.com/gin-gonic/gin"

	"fifteen-thirty-one/internal/models"
	"fifteen-thirty-one/internal/services"
)

type PaymentHandler struct {
	paymentService *services.PaymentService
}

func NewPaymentHandler(paymentService *services.PaymentService) *PaymentHandler {
	return &PaymentHandler{
		paymentService: paymentService,
	}
}

// GetPlans returns all available subscription plans
// GET /api/payments/plans
func (h *PaymentHandler) GetPlans(c *gin.Context) {
	plans, err := h.paymentService.GetAllPlans()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve plans"})
		return
	}

	c.JSON(http.StatusOK, plans)
}

// GetSubscription returns the user's current subscription
// GET /api/payments/subscription
func (h *PaymentHandler) GetSubscription(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	subscription, err := h.paymentService.GetUserSubscription(userID.(int))
	if err == services.ErrSubscriptionNotFound {
		c.JSON(http.StatusNotFound, gin.H{"error": "No active subscription found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve subscription"})
		return
	}

	c.JSON(http.StatusOK, subscription)
}

// CreateSetupIntent creates a Stripe Setup Intent for collecting payment method
// POST /api/payments/setup-intent
func (h *PaymentHandler) CreateSetupIntent(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	// Get user details
	username, _ := c.Get("username")
	email, emailExists := c.Get("email")

	emailStr := ""
	if emailExists {
		emailStr = email.(string)
	}

	// Get or create Stripe customer
	customerID, err := h.paymentService.GetOrCreateStripeCustomer(
		userID.(int),
		emailStr,
		username.(string),
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create customer"})
		return
	}

	// Create setup intent
	setupIntent, err := h.paymentService.CreateSetupIntent(customerID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create setup intent"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"client_secret": setupIntent.ClientSecret,
		"customer_id":   customerID,
	})
}

// CreateSubscription creates a new subscription for the user
// POST /api/payments/subscription
func (h *PaymentHandler) CreateSubscription(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	var req models.CreateSubscriptionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	// Get user details for customer creation
	username, _ := c.Get("username")
	email, emailExists := c.Get("email")

	emailStr := ""
	if emailExists {
		emailStr = email.(string)
	}

	// Get or create Stripe customer
	customerID, err := h.paymentService.GetOrCreateStripeCustomer(
		userID.(int),
		emailStr,
		username.(string),
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create customer"})
		return
	}

	// Attach payment method to customer
	_, err = h.paymentService.AttachPaymentMethod(
		userID.(int),
		customerID,
		req.PaymentMethodID,
	)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to attach payment method"})
		return
	}

	// Create subscription
	subscription, err := h.paymentService.CreateSubscription(
		userID.(int),
		req.PlanID,
		req.PaymentMethodID,
		customerID,
	)
	if err != nil {
		if err == services.ErrInvalidPlan {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid plan"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create subscription"})
		return
	}

	c.JSON(http.StatusCreated, subscription)
}

// CancelSubscription cancels the user's subscription
// DELETE /api/payments/subscription
func (h *PaymentHandler) CancelSubscription(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	var req models.CancelSubscriptionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		// Default to canceling at period end if no body provided
		req.CancelAtPeriodEnd = true
	}

	err := h.paymentService.CancelSubscription(userID.(int), req.CancelAtPeriodEnd)
	if err == services.ErrSubscriptionNotFound {
		c.JSON(http.StatusNotFound, gin.H{"error": "No active subscription found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to cancel subscription"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Subscription canceled successfully"})
}

// GetPaymentMethods returns all payment methods for the user
// GET /api/payments/methods
func (h *PaymentHandler) GetPaymentMethods(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	methods, err := h.paymentService.GetPaymentMethods(userID.(int))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve payment methods"})
		return
	}

	c.JSON(http.StatusOK, methods)
}

// UpdatePaymentMethod updates the default payment method for the user's subscription
// PUT /api/payments/methods
func (h *PaymentHandler) UpdatePaymentMethod(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	var req models.UpdatePaymentMethodRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	// Get user subscription to get customer ID
	subscription, err := h.paymentService.GetUserSubscription(userID.(int))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No active subscription found"})
		return
	}

	if subscription.StripeCustomerID == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No customer ID found"})
		return
	}

	// Update payment method
	err = h.paymentService.UpdateSubscriptionPaymentMethod(
		*subscription.StripeSubscriptionID,
		req.PaymentMethodID,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update payment method"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Payment method updated successfully"})
}

// HandleWebhook handles Stripe webhook events
// POST /api/payments/webhook
func (h *PaymentHandler) HandleWebhook(c *gin.Context) {
	payload, err := c.GetRawData()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to read request body"})
		return
	}

	signature := c.GetHeader("Stripe-Signature")

	err = h.paymentService.HandleStripeWebhook(payload, signature)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"received": true})
}
