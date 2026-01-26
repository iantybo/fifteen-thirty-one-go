import { useEffect, useState } from 'react'
import { useNavigate, useSearchParams } from 'react-router-dom'
import { loadStripe } from '@stripe/stripe-js'
import { api } from '../api/client'
import type { PremiumStatus } from '../api/types'
import { useAuth } from '../auth/auth'

const STRIPE_PUBLISHABLE_KEY = import.meta.env.VITE_STRIPE_PUBLISHABLE_KEY || ''

export function PremiumPortalPage() {
  const { user, setAuth } = useAuth()
  const navigate = useNavigate()
  const [searchParams] = useSearchParams()
  const [status, setStatus] = useState<PremiumStatus | null>(null)
  const [loading, setLoading] = useState(true)
  const [checkoutLoading, setCheckoutLoading] = useState(false)
  const [avatarUrl, setAvatarUrl] = useState('')
  const [savingAvatar, setSavingAvatar] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [success, setSuccess] = useState<string | null>(null)

  useEffect(() => {
    if (!user) {
      navigate('/login')
      return
    }
    loadStatus()
    checkUrlParams()
  }, [user, navigate])

  function checkUrlParams() {
    if (searchParams.get('success') === 'true') {
      setSuccess('Payment successful! Your premium subscription is now active.')
      // Reload status after a short delay to allow webhook to process
      setTimeout(() => {
        loadStatus()
      }, 2000)
    } else if (searchParams.get('canceled') === 'true') {
      setError('Payment was canceled.')
    }
  }

  async function loadStatus() {
    try {
      setLoading(true)
      setError(null)
      const data = await api.getPremiumStatus()
      setStatus(data)
      if (data.subscription) {
        // Reload user to get updated premium status
        const me = await api.me()
        setAuth(me.user)
        if (me.user.avatar_url) {
          setAvatarUrl(me.user.avatar_url)
        }
      }
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'Failed to load premium status')
    } finally {
      setLoading(false)
    }
  }

  async function handleCheckout() {
    if (!STRIPE_PUBLISHABLE_KEY) {
      setError('Stripe is not configured. Please contact support.')
      return
    }

    try {
      setCheckoutLoading(true)
      setError(null)
      const { url } = await api.createCheckoutSession()
      // Redirect to Stripe Checkout
      window.location.href = url
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'Failed to create checkout session')
      setCheckoutLoading(false)
    }
  }

  async function handleSaveAvatar() {
    if (!user?.is_premium) {
      setError('Premium subscription required to set custom avatar')
      return
    }

    try {
      setSavingAvatar(true)
      setError(null)
      const result = await api.updateAvatar(avatarUrl || null)
      setAuth(result.user)
      setSuccess('Avatar updated successfully!')
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'Failed to update avatar')
    } finally {
      setSavingAvatar(false)
    }
  }

  if (loading) {
    return (
      <div style={{ maxWidth: 800, margin: '40px auto', padding: '0 16px' }}>
        <div>Loading premium status...</div>
      </div>
    )
  }

  const isPremium = status?.is_premium ?? false
  const subscription = status?.subscription

  return (
    <div style={{ maxWidth: 800, margin: '40px auto', padding: '0 16px' }}>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'baseline', marginBottom: 24 }}>
        <h1>Premium Portal</h1>
        <button type="button" onClick={() => navigate('/lobbies')}>
          ← Back to lobbies
        </button>
      </div>

      {success && (
        <div
          style={{
            padding: 12,
            background: '#d1fae5',
            color: '#065f46',
            borderRadius: 8,
            marginBottom: 24,
          }}
        >
          {success}
        </div>
      )}

      {error && (
        <div
          style={{
            padding: 12,
            background: '#fee2e2',
            color: '#991b1b',
            borderRadius: 8,
            marginBottom: 24,
          }}
        >
          {error}
        </div>
      )}

      <div style={{ display: 'grid', gap: 24 }}>
        {/* Premium Status Section */}
        <section style={{ padding: 20, border: '1px solid #e2e8f0', borderRadius: 12 }}>
          <h2 style={{ fontSize: 20, fontWeight: 700, marginBottom: 16 }}>Premium Status</h2>
          {isPremium ? (
            <div>
              <div style={{ color: '#059669', fontWeight: 600, marginBottom: 12, fontSize: 18 }}>
                ✓ Premium Active
              </div>
              {subscription && (
                <div style={{ color: '#64748b', fontSize: 14, marginTop: 8 }}>
                  <div>
                    <strong>Status:</strong> {subscription.status}
                  </div>
                  <div>
                    <strong>Current Period End:</strong>{' '}
                    {new Date(subscription.current_period_end).toLocaleDateString()}
                  </div>
                </div>
              )}
            </div>
          ) : (
            <div>
              <div style={{ color: '#64748b', marginBottom: 16 }}>
                You don't have an active premium subscription.
              </div>
              <button
                type="button"
                onClick={handleCheckout}
                disabled={checkoutLoading}
                style={{
                  padding: '12px 24px',
                  background: '#3b82f6',
                  color: 'white',
                  border: 'none',
                  borderRadius: 8,
                  cursor: checkoutLoading ? 'not-allowed' : 'pointer',
                  fontWeight: 600,
                }}
              >
                {checkoutLoading ? 'Loading...' : 'Subscribe to Premium'}
              </button>
            </div>
          )}
        </section>

        {/* Custom Avatar Section */}
        {isPremium && (
          <section style={{ padding: 20, border: '1px solid #e2e8f0', borderRadius: 12 }}>
            <h2 style={{ fontSize: 20, fontWeight: 700, marginBottom: 16 }}>Custom Avatar</h2>
            <p style={{ color: '#64748b', marginBottom: 20, fontSize: 14 }}>
              Set a custom avatar URL. The URL must be publicly accessible and start with http:// or https://
            </p>

            <div style={{ display: 'grid', gap: 16 }}>
              <div>
                <label style={{ display: 'block', fontWeight: 600, marginBottom: 8 }}>Avatar URL</label>
                <input
                  type="text"
                  value={avatarUrl}
                  onChange={(e) => setAvatarUrl(e.target.value)}
                  placeholder="https://example.com/avatar.jpg"
                  style={{
                    width: '100%',
                    padding: '10px 12px',
                    border: '1px solid #cbd5e1',
                    borderRadius: 6,
                    fontSize: 14,
                  }}
                />
              </div>

              {avatarUrl && (
                <div>
                  <label style={{ display: 'block', fontWeight: 600, marginBottom: 8 }}>Preview</label>
                  <div
                    style={{
                      width: 100,
                      height: 100,
                      borderRadius: '50%',
                      border: '2px solid #cbd5e1',
                      overflow: 'hidden',
                      background: '#f1f5f9',
                      display: 'flex',
                      alignItems: 'center',
                      justifyContent: 'center',
                    }}
                  >
                    <img
                      src={avatarUrl}
                      alt="Avatar preview"
                      style={{ width: '100%', height: '100%', objectFit: 'cover' }}
                      onError={(e) => {
                        const target = e.target as HTMLImageElement
                        target.style.display = 'none'
                        const parent = target.parentElement
                        if (parent) {
                          parent.innerHTML = '<span style="color: #94a3b8; font-size: 12px;">Invalid URL</span>'
                        }
                      }}
                    />
                  </div>
                </div>
              )}

              <div style={{ display: 'flex', gap: 12 }}>
                <button
                  type="button"
                  onClick={handleSaveAvatar}
                  disabled={savingAvatar}
                  style={{
                    padding: '10px 20px',
                    background: '#3b82f6',
                    color: 'white',
                    border: 'none',
                    borderRadius: 6,
                    cursor: savingAvatar ? 'not-allowed' : 'pointer',
                    fontWeight: 600,
                  }}
                >
                  {savingAvatar ? 'Saving...' : 'Save Avatar'}
                </button>
                <button
                  type="button"
                  onClick={() => {
                    setAvatarUrl('')
                    handleSaveAvatar()
                  }}
                  disabled={savingAvatar}
                  style={{
                    padding: '10px 20px',
                    background: '#ef4444',
                    color: 'white',
                    border: 'none',
                    borderRadius: 6,
                    cursor: savingAvatar ? 'not-allowed' : 'pointer',
                    fontWeight: 600,
                  }}
                >
                  Remove Avatar
                </button>
              </div>
            </div>
          </section>
        )}

        {/* Premium Benefits Section */}
        <section style={{ padding: 20, border: '1px solid #e2e8f0', borderRadius: 12 }}>
          <h2 style={{ fontSize: 20, fontWeight: 700, marginBottom: 16 }}>Premium Benefits</h2>
          <ul style={{ listStyle: 'none', padding: 0, margin: 0, display: 'grid', gap: 12 }}>
            <li style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
              <span style={{ fontSize: 20 }}>✨</span>
              <span>Custom avatar</span>
            </li>
            <li style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
              <span style={{ fontSize: 20 }}>🎨</span>
              <span>Enhanced customization options</span>
            </li>
            <li style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
              <span style={{ fontSize: 20 }}>⭐</span>
              <span>Premium badge in games</span>
            </li>
            <li style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
              <span style={{ fontSize: 20 }}>🚀</span>
              <span>Priority support</span>
            </li>
          </ul>
        </section>
      </div>
    </div>
  )
}
