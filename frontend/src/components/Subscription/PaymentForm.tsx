import { useState } from 'react'
import { Elements, PaymentElement, useStripe, useElements } from '@stripe/react-stripe-js'
import { loadStripe, StripeElementsOptions } from '@stripe/stripe-js'
import { paymentsApi } from '../../api/payments'
import type { SubscriptionPlan } from '../../api/types'

// Initialize Stripe with your publishable key
// This should be set via environment variable
const stripePromise = loadStripe(import.meta.env.VITE_STRIPE_PUBLISHABLE_KEY || '')

interface PaymentFormProps {
  plan: SubscriptionPlan
  onSuccess: () => void
  onCancel: () => void
}

function CheckoutForm({ plan, onSuccess, onCancel }: PaymentFormProps) {
  const stripe = useStripe()
  const elements = useElements()
  const [error, setError] = useState<string | null>(null)
  const [processing, setProcessing] = useState(false)

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()

    if (!stripe || !elements) {
      return
    }

    setProcessing(true)
    setError(null)

    try {
      // Confirm the SetupIntent with the payment details
      const { error: stripeError, setupIntent } = await stripe.confirmSetup({
        elements,
        redirect: 'if_required',
      })

      if (stripeError) {
        setError(stripeError.message || 'Payment failed')
        setProcessing(false)
        return
      }

      if (!setupIntent?.payment_method) {
        setError('No payment method provided')
        setProcessing(false)
        return
      }

      // Create subscription with the payment method
      await paymentsApi.createSubscription({
        plan_id: plan.id,
        payment_method_id: setupIntent.payment_method as string,
      })

      onSuccess()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to create subscription')
      setProcessing(false)
    }
  }

  return (
    <form onSubmit={handleSubmit} className="payment-form">
      <h3>Subscribe to {plan.display_name}</h3>

      <div className="plan-summary">
        <p className="price">
          ${(plan.price_cents / 100).toFixed(2)} / {plan.billing_period}
        </p>
      </div>

      <PaymentElement />

      {error && <div className="error-message">{error}</div>}

      <div className="form-actions">
        <button
          type="button"
          onClick={onCancel}
          disabled={processing}
          className="cancel-btn"
        >
          Cancel
        </button>
        <button
          type="submit"
          disabled={!stripe || processing}
          className="submit-btn"
        >
          {processing ? 'Processing...' : 'Subscribe'}
        </button>
      </div>
    </form>
  )
}

export function PaymentForm(props: PaymentFormProps) {
  const [clientSecret, setClientSecret] = useState<string | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  useState(() => {
    const initializePayment = async () => {
      try {
        const response = await paymentsApi.createSetupIntent()
        setClientSecret(response.client_secret)
      } catch (err) {
        setError(err instanceof Error ? err.message : 'Failed to initialize payment')
      } finally {
        setLoading(false)
      }
    }

    initializePayment()
  })

  if (loading) {
    return <div className="loading">Loading payment form...</div>
  }

  if (error || !clientSecret) {
    return (
      <div className="error">
        <p>Error: {error || 'Failed to load payment form'}</p>
        <button onClick={props.onCancel}>Go Back</button>
      </div>
    )
  }

  const options: StripeElementsOptions = {
    clientSecret,
    appearance: {
      theme: 'stripe',
    },
  }

  return (
    <Elements stripe={stripePromise} options={options}>
      <CheckoutForm {...props} />
    </Elements>
  )
}
