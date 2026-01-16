import { apiFetch } from '../lib/http'
import type {
  SubscriptionPlan,
  UserSubscriptionWithPlan,
  PaymentMethod,
  CreateSubscriptionRequest,
  UpdatePaymentMethodRequest,
  CancelSubscriptionRequest,
  SetupIntentResponse,
} from './types'

export const paymentsApi = {
  // Get all available subscription plans
  getPlans: async (): Promise<SubscriptionPlan[]> => {
    return apiFetch<SubscriptionPlan[]>('/api/payments/plans')
  },

  // Get current user's subscription
  getSubscription: async (): Promise<UserSubscriptionWithPlan> => {
    return apiFetch<UserSubscriptionWithPlan>('/api/payments/subscription')
  },

  // Create a setup intent for collecting payment method
  createSetupIntent: async (): Promise<SetupIntentResponse> => {
    return apiFetch<SetupIntentResponse>('/api/payments/setup-intent', {
      method: 'POST',
    })
  },

  // Create a new subscription
  createSubscription: async (
    data: CreateSubscriptionRequest
  ): Promise<UserSubscriptionWithPlan> => {
    return apiFetch<UserSubscriptionWithPlan>('/api/payments/subscription', {
      method: 'POST',
      body: JSON.stringify(data),
    })
  },

  // Cancel subscription
  cancelSubscription: async (
    data: CancelSubscriptionRequest
  ): Promise<{ message: string }> => {
    return apiFetch<{ message: string }>('/api/payments/subscription', {
      method: 'DELETE',
      body: JSON.stringify(data),
    })
  },

  // Get payment methods
  getPaymentMethods: async (): Promise<PaymentMethod[]> => {
    return apiFetch<PaymentMethod[]>('/api/payments/methods')
  },

  // Update payment method
  updatePaymentMethod: async (
    data: UpdatePaymentMethodRequest
  ): Promise<{ message: string }> => {
    return apiFetch<{ message: string }>('/api/payments/methods', {
      method: 'PUT',
      body: JSON.stringify(data),
    })
  },
}
