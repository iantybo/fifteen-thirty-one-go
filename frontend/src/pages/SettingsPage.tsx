import { useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { api } from '../api/client'
import type { CardDeckColors, UserPreferences } from '../api/types'
import { useAuth } from '../auth/auth'

const DEFAULT_COLORS: CardDeckColors = {
  H: '#dc2626', // Hearts - red
  D: '#dc2626', // Diamonds - red
  C: '#0f172a', // Clubs - black
  S: '#0f172a', // Spades - black
}

export function SettingsPage() {
  const { user } = useAuth()
  const navigate = useNavigate()
  const [prefs, setPrefs] = useState<UserPreferences | null>(null)
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [cardColors, setCardColors] = useState<CardDeckColors>(DEFAULT_COLORS)

  useEffect(() => {
    if (!user) {
      navigate('/login')
      return
    }
    loadPreferences()
  }, [user, navigate])

  async function loadPreferences() {
    try {
      setLoading(true)
      setError(null)
      const data = await api.getPreferences()
      setPrefs(data)
      if (data.card_deck_colors) {
        setCardColors(data.card_deck_colors)
      } else {
        setCardColors(DEFAULT_COLORS)
      }
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'Failed to load preferences')
    } finally {
      setLoading(false)
    }
  }

  async function savePreferences() {
    if (!user) return
    try {
      setSaving(true)
      setError(null)
      const updated = await api.putPreferences({
        auto_count_mode: prefs?.auto_count_mode,
        card_deck_colors: cardColors,
      })
      setPrefs(updated)
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'Failed to save preferences')
    } finally {
      setSaving(false)
    }
  }

  function resetColors() {
    setCardColors(DEFAULT_COLORS)
  }

  if (loading) {
    return (
      <div style={{ maxWidth: 800, margin: '40px auto', padding: '0 16px' }}>
        <div>Loading preferences...</div>
      </div>
    )
  }

  return (
    <div style={{ maxWidth: 800, margin: '40px auto', padding: '0 16px' }}>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'baseline', marginBottom: 24 }}>
        <h1>Settings</h1>
        <div style={{ display: 'flex', gap: 12 }}>
          <button type="button" onClick={() => navigate('/premium')}>
            Premium Portal
          </button>
          <button type="button" onClick={() => navigate('/lobbies')}>
            ← Back to lobbies
          </button>
        </div>
      </div>

      {error && (
        <div style={{ padding: 12, background: '#fee2e2', color: '#991b1b', borderRadius: 8, marginBottom: 24 }}>
          {error}
        </div>
      )}

      <div style={{ display: 'grid', gap: 24 }}>
        {/* Card Deck Colors Section */}
        <section style={{ padding: 20, border: '1px solid #e2e8f0', borderRadius: 12 }}>
          <h2 style={{ fontSize: 20, fontWeight: 700, marginBottom: 16 }}>Card Deck Colors</h2>
          <p style={{ color: '#64748b', marginBottom: 20, fontSize: 14 }}>
            Customize the colors for each suit. Changes apply to all cards in the game.
          </p>

          <div style={{ display: 'grid', gap: 16 }}>
            {(['H', 'D', 'C', 'S'] as const).map((suit) => {
              const suitName = suit === 'H' ? 'Hearts' : suit === 'D' ? 'Diamonds' : suit === 'C' ? 'Clubs' : 'Spades'
              const suitSymbol = suit === 'H' ? '♥' : suit === 'D' ? '♦' : suit === 'C' ? '♣' : '♠'
              return (
                <div key={suit} style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
                  <label style={{ minWidth: 100, fontWeight: 600, display: 'flex', alignItems: 'center', gap: 8 }}>
                    <span style={{ fontSize: 24 }}>{suitSymbol}</span>
                    {suitName}
                  </label>
                  <input
                    type="color"
                    value={cardColors[suit]}
                    onChange={(e) => setCardColors((prev) => ({ ...prev, [suit]: e.target.value }))}
                    style={{ width: 80, height: 40, border: '1px solid #cbd5e1', borderRadius: 6, cursor: 'pointer' }}
                  />
                  <input
                    type="text"
                    value={cardColors[suit]}
                    onChange={(e) => {
                      const value = e.target.value.trim()
                      if (/^#[0-9A-Fa-f]{6}$/.test(value)) {
                        setCardColors((prev) => ({ ...prev, [suit]: value }))
                      }
                    }}
                    placeholder="#000000"
                    style={{
                      width: 100,
                      padding: '8px 12px',
                      border: '1px solid #cbd5e1',
                      borderRadius: 6,
                      fontFamily: 'monospace',
                      fontSize: 14,
                    }}
                  />
                  <div
                    style={{
                      width: 40,
                      height: 40,
                      border: '1px solid #cbd5e1',
                      borderRadius: 6,
                      background: cardColors[suit],
                      display: 'flex',
                      alignItems: 'center',
                      justifyContent: 'center',
                      fontSize: 24,
                      color: cardColors[suit] === '#0f172a' || cardColors[suit] === '#000000' ? '#ffffff' : '#000000',
                    }}
                  >
                    {suitSymbol}
                  </div>
                </div>
              )
            })}
          </div>

          <div style={{ display: 'flex', gap: 12, marginTop: 20 }}>
            <button type="button" onClick={savePreferences} disabled={saving} style={{ padding: '10px 20px' }}>
              {saving ? 'Saving...' : 'Save Colors'}
            </button>
            <button type="button" onClick={resetColors} disabled={saving} style={{ padding: '10px 20px' }}>
              Reset to Default
            </button>
          </div>
        </section>

        {/* Auto Count Mode Section (if needed in future) */}
        {prefs && (
          <section style={{ padding: 20, border: '1px solid #e2e8f0', borderRadius: 12 }}>
            <h2 style={{ fontSize: 20, fontWeight: 700, marginBottom: 16 }}>Game Preferences</h2>
            <div style={{ color: '#64748b', fontSize: 14 }}>
              <div>
                <strong>Auto Count Mode:</strong> {prefs.auto_count_mode}
              </div>
            </div>
          </section>
        )}
      </div>
    </div>
  )
}
