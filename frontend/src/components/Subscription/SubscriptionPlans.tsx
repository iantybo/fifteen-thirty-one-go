import { useState, useEffect } from 'react'
import { paymentsApi } from '../../api/payments'
import type { SubscriptionPlan } from '../../api/types'

interface SubscriptionPlansProps {
  onSelectPlan: (plan: SubscriptionPlan) => void
}

export function SubscriptionPlans({ onSelectPlan }: SubscriptionPlansProps) {
  const [plans, setPlans] = useState<SubscriptionPlan[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    const fetchPlans = async () => {
      try {
        const data = await paymentsApi.getPlans()
        setPlans(data)
      } catch (err) {
        setError(err instanceof Error ? err.message : 'Failed to load plans')
      } finally {
        setLoading(false)
      }
    }

    fetchPlans()
  }, [])

  if (loading) {
    return <div className="loading">Loading subscription plans...</div>
  }

  if (error) {
    return <div className="error">Error: {error}</div>
  }

  const formatPrice = (cents: number, currency: string) => {
    const amount = cents / 100
    return new Intl.NumberFormat('en-US', {
      style: 'currency',
      currency: currency.toUpperCase(),
    }).format(amount)
  }

  return (
    <div className="subscription-plans">
      <h2>Choose Your Plan</h2>
      <div className="plans-grid">
        {plans.map((plan) => (
          <div key={plan.id} className="plan-card">
            <h3>{plan.display_name}</h3>
            <div className="price">
              {plan.price_cents === 0 ? (
                <span className="free">Free</span>
              ) : (
                <>
                  <span className="amount">
                    {formatPrice(plan.price_cents, plan.currency)}
                  </span>
                  <span className="period">/{plan.billing_period}</span>
                </>
              )}
            </div>
            <p className="description">{plan.description}</p>
            <ul className="features">
              {plan.features.map((feature, idx) => (
                <li key={idx}>{feature}</li>
              ))}
            </ul>
            <button
              className="select-plan-btn"
              onClick={() => onSelectPlan(plan)}
              disabled={plan.id === 'free'}
            >
              {plan.id === 'free' ? 'Current Plan' : 'Select Plan'}
            </button>
          </div>
        ))}
      </div>
    </div>
  )
}
