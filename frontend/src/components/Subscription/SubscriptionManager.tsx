import { useState, useEffect } from 'react'
import { paymentsApi } from '../../api/payments'
import type { UserSubscriptionWithPlan } from '../../api/types'
import { SubscriptionPlans } from './SubscriptionPlans'
import { PaymentForm } from './PaymentForm'
import type { SubscriptionPlan } from '../../api/types'

export function SubscriptionManager() {
  const [subscription, setSubscription] = useState<UserSubscriptionWithPlan | null>(null)
  const [selectedPlan, setSelectedPlan] = useState<SubscriptionPlan | null>(null)
  const [showPlans, setShowPlans] = useState(false)
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    fetchSubscription()
  }, [])

  const fetchSubscription = async () => {
    try {
      const data = await paymentsApi.getSubscription()
      setSubscription(data)
    } catch (err) {
      // No active subscription, which is fine
      console.log('No active subscription')
    } finally {
      setLoading(false)
    }
  }

  const handleCancelSubscription = async () => {
    if (!subscription) return

    const confirmed = window.confirm(
      'Are you sure you want to cancel your subscription? It will remain active until the end of your billing period.'
    )

    if (!confirmed) return

    try {
      await paymentsApi.cancelSubscription({ cancel_at_period_end: true })
      await fetchSubscription()
    } catch (err) {
      alert('Failed to cancel subscription: ' + (err instanceof Error ? err.message : 'Unknown error'))
    }
  }

  const handleSubscriptionSuccess = () => {
    setSelectedPlan(null)
    setShowPlans(false)
    fetchSubscription()
  }

  if (loading) {
    return <div className="loading">Loading subscription...</div>
  }

  // Show payment form if a plan is selected
  if (selectedPlan) {
    return (
      <div className="subscription-container">
        <PaymentForm
          plan={selectedPlan}
          onSuccess={handleSubscriptionSuccess}
          onCancel={() => setSelectedPlan(null)}
        />
      </div>
    )
  }

  // Show plan selection
  if (showPlans) {
    return (
      <div className="subscription-container">
        <SubscriptionPlans onSelectPlan={(plan) => setSelectedPlan(plan)} />
        <button onClick={() => setShowPlans(false)} className="back-btn">
          Back
        </button>
      </div>
    )
  }

  // Show current subscription status
  return (
    <div className="subscription-container">
      <h2>Subscription</h2>

      {subscription && subscription.plan ? (
        <div className="current-subscription">
          <h3>Current Plan: {subscription.plan.display_name}</h3>
          <p>Status: {subscription.status}</p>
          <p>
            Current period: {new Date(subscription.current_period_start).toLocaleDateString()} -{' '}
            {new Date(subscription.current_period_end).toLocaleDateString()}
          </p>

          {subscription.cancel_at_period_end && (
            <p className="warning">
              Your subscription will be canceled at the end of the current period.
            </p>
          )}

          <div className="subscription-features">
            <h4>Features:</h4>
            <ul>
              {subscription.plan.features.map((feature, idx) => (
                <li key={idx}>{feature}</li>
              ))}
            </ul>
          </div>

          <div className="subscription-actions">
            {!subscription.cancel_at_period_end && (
              <button onClick={handleCancelSubscription} className="cancel-btn">
                Cancel Subscription
              </button>
            )}
            <button onClick={() => setShowPlans(true)} className="upgrade-btn">
              Change Plan
            </button>
          </div>
        </div>
      ) : (
        <div className="no-subscription">
          <p>You don't have an active subscription.</p>
          <button onClick={() => setShowPlans(true)} className="subscribe-btn">
            View Plans
          </button>
        </div>
      )}
    </div>
  )
}
