import { useState, useEffect } from 'react'
import type { FormEvent } from 'react'
import { useNavigate } from 'react-router-dom'
import { api } from '../api/client'
import { useAuth } from '../auth/auth'

export function ProfileSettingsPage() {
  const { user: authUser, setAuth } = useAuth()
  const nav = useNavigate()
  const [email, setEmail] = useState('')
  const [displayName, setDisplayName] = useState('')
  const [err, setErr] = useState<string | null>(null)
  const [successMsg, setSuccessMsg] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    async function loadProfile() {
      try {
        const profile = await api.getProfile()
        setEmail(profile.email || '')
        setDisplayName(profile.display_name || '')
      } catch (e: unknown) {
        setErr(e instanceof Error ? e.message : 'Failed to load profile')
      } finally {
        setLoading(false)
      }
    }
    loadProfile()
  }, [])

  async function onSubmit(e: FormEvent) {
    e.preventDefault()
    setErr(null)
    setSuccessMsg(null)
    setBusy(true)

    try {
      const updatedUser = await api.updateProfile({
        email: email.trim() || undefined,
        display_name: displayName.trim() || undefined,
      })

      // Update auth context with new user data
      setAuth(updatedUser)
      setSuccessMsg('Profile updated successfully!')
    } catch (e: unknown) {
      setErr(e instanceof Error ? e.message : 'Failed to update profile')
    } finally {
      setBusy(false)
    }
  }

  if (loading) {
    return (
      <div style={{ maxWidth: 420, margin: '40px auto' }}>
        <p>Loading profile...</p>
      </div>
    )
  }

  return (
    <div style={{ maxWidth: 420, margin: '40px auto' }}>
      <h1>Profile Settings</h1>
      <div style={{ marginBottom: 16 }}>
        <strong>Username:</strong> {authUser?.username}
      </div>
      <form onSubmit={onSubmit}>
        <label>
          Display Name
          <input
            value={displayName}
            onChange={(e) => setDisplayName(e.target.value)}
            placeholder="Enter your display name"
            disabled={busy}
            maxLength={100}
          />
        </label>
        <label>
          Email Address
          <input
            type="email"
            value={email}
            onChange={(e) => setEmail(e.target.value)}
            placeholder="your.email@example.com"
            disabled={busy}
          />
        </label>
        {err && <div style={{ color: 'crimson', marginTop: 8 }}>{err}</div>}
        {successMsg && <div style={{ color: 'green', marginTop: 8 }}>{successMsg}</div>}
        <div style={{ marginTop: 12, display: 'flex', gap: 12 }}>
          <button type="submit" disabled={busy}>
            {busy ? 'Saving…' : 'Save Changes'}
          </button>
          <button type="button" onClick={() => nav('/lobbies')} disabled={busy}>
            Cancel
          </button>
        </div>
      </form>
    </div>
  )
}
