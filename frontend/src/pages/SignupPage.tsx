import { useState } from 'react'
import type { FormEvent } from 'react'
import { Link } from 'react-router-dom'
import { api } from '../api/client'

const MAX_NAME_LEN = 80
const MAX_NOTE_LEN = 500

type FieldErrors = {
  email?: string
  name?: string
  note?: string
}

/**
 * Client-side mirror of the server's validation. The server is authoritative;
 * this only avoids a round trip for obvious mistakes.
 */
function validate(email: string, name: string, note: string): FieldErrors {
  const errs: FieldErrors = {}

  const trimmedEmail = email.trim()
  if (!trimmedEmail) {
    errs.email = 'Email is required.'
  } else if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(trimmedEmail)) {
    errs.email = 'Enter a valid email address.'
  }

  const trimmedName = name.trim()
  if (!trimmedName) {
    errs.name = 'Name is required.'
  } else if (trimmedName.length > MAX_NAME_LEN) {
    errs.name = `Name must be ${MAX_NAME_LEN} characters or fewer.`
  }

  if (note.trim().length > MAX_NOTE_LEN) {
    errs.note = `Note must be ${MAX_NOTE_LEN} characters or fewer.`
  }

  return errs
}

function labelStyle(): React.CSSProperties {
  return { display: 'grid', gap: 6, fontSize: 14, fontWeight: 600, marginBottom: 14 }
}

function inputStyle(invalid: boolean): React.CSSProperties {
  return {
    padding: '9px 11px',
    borderRadius: 8,
    border: `1px solid ${invalid ? '#dc2626' : '#cbd5e1'}`,
    fontSize: 14,
    fontWeight: 400,
    font: 'inherit',
  }
}

function fieldErrorStyle(): React.CSSProperties {
  return { color: '#b91c1c', fontSize: 12, fontWeight: 400 }
}

export function SignupPage() {
  const [email, setEmail] = useState('')
  const [name, setName] = useState('')
  const [note, setNote] = useState('')
  const [errors, setErrors] = useState<FieldErrors>({})
  const [formError, setFormError] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)
  const [done, setDone] = useState(false)

  async function onSubmit(e: FormEvent) {
    e.preventDefault()

    const found = validate(email, name, note)
    setErrors(found)
    if (Object.keys(found).length > 0) {
      setFormError(null)
      return
    }

    setFormError(null)
    setBusy(true)
    try {
      await api.createSignup({
        email: email.trim(),
        name: name.trim(),
        note: note.trim() || undefined,
      })
      setDone(true)
    } catch (err: unknown) {
      setFormError(err instanceof Error ? err.message : 'Signup failed. Please try again.')
    } finally {
      setBusy(false)
    }
  }

  if (done) {
    return (
      <div style={{ maxWidth: 460, margin: '48px auto', padding: '0 16px' }}>
        <h1 style={{ marginTop: 0 }}>You're on the list</h1>
        <p style={{ opacity: 0.8, lineHeight: 1.5 }}>
          Thanks for signing up. We'll email <strong>{email.trim()}</strong> when there's a spot for you.
        </p>
        <p style={{ marginTop: 20 }}>
          <Link to="/login">Back to sign in</Link>
        </p>
      </div>
    )
  }

  return (
    <div style={{ maxWidth: 460, margin: '48px auto', padding: '0 16px' }}>
      <h1 style={{ marginTop: 0 }}>Request an invite</h1>
      <p style={{ opacity: 0.8, lineHeight: 1.5, marginBottom: 24 }}>
        Leave your details and we'll be in touch when a spot opens up.
      </p>

      <form onSubmit={(e) => void onSubmit(e)} noValidate>
        <label style={labelStyle()}>
          Email
          <input
            type="email"
            value={email}
            onChange={(e) => setEmail(e.target.value)}
            autoComplete="email"
            disabled={busy}
            aria-invalid={errors.email ? true : undefined}
            aria-describedby={errors.email ? 'signup-email-error' : undefined}
            style={inputStyle(!!errors.email)}
          />
          {errors.email ? (
            <span id="signup-email-error" style={fieldErrorStyle()}>
              {errors.email}
            </span>
          ) : null}
        </label>

        <label style={labelStyle()}>
          Name
          <input
            value={name}
            onChange={(e) => setName(e.target.value)}
            autoComplete="name"
            maxLength={MAX_NAME_LEN}
            disabled={busy}
            aria-invalid={errors.name ? true : undefined}
            aria-describedby={errors.name ? 'signup-name-error' : undefined}
            style={inputStyle(!!errors.name)}
          />
          {errors.name ? (
            <span id="signup-name-error" style={fieldErrorStyle()}>
              {errors.name}
            </span>
          ) : null}
        </label>

        <label style={labelStyle()}>
          Anything we should know? <span style={{ fontWeight: 400, opacity: 0.6 }}>(optional)</span>
          <textarea
            value={note}
            onChange={(e) => setNote(e.target.value)}
            maxLength={MAX_NOTE_LEN}
            rows={3}
            disabled={busy}
            aria-invalid={errors.note ? true : undefined}
            aria-describedby={errors.note ? 'signup-note-error' : undefined}
            style={{ ...inputStyle(!!errors.note), resize: 'vertical' }}
          />
          <span style={{ fontWeight: 400, fontSize: 12, opacity: 0.6 }}>
            {note.trim().length}/{MAX_NOTE_LEN}
          </span>
          {errors.note ? (
            <span id="signup-note-error" style={fieldErrorStyle()}>
              {errors.note}
            </span>
          ) : null}
        </label>

        {formError ? (
          <div role="alert" style={{ ...fieldErrorStyle(), marginBottom: 12 }}>
            {formError}
          </div>
        ) : null}

        <button type="submit" disabled={busy} style={{ fontWeight: 700, padding: '9px 16px' }}>
          {busy ? 'Sending…' : 'Request invite'}
        </button>
      </form>

      <p style={{ marginTop: 20, fontSize: 14 }}>
        Already have an account? <Link to="/login">Sign in</Link>
      </p>
    </div>
  )
}
