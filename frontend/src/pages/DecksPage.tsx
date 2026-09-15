import { useMemo, useState } from 'react'
import { Link } from 'react-router-dom'
import type { DeckRequest } from '../api/client'
import type { Card, CardDeck } from '../api/types'
import { CardBack, CardIcon } from '../decks/CardView'
import { useDeck } from '../decks/DeckProvider'
import { BUILTIN_DECK_PREFIX, type DeckTheme } from '../decks/deckTheme'

// A representative spread for deck previews: one red, one black, a face card
// and an ace, so both suit colors and rank rendering are visible.
const PREVIEW_CARDS: Card[] = [
  { rank: 1, suit: 'S' },
  { rank: 12, suit: 'H' },
  { rank: 7, suit: 'D' },
  { rank: 13, suit: 'C' },
]

type FormState = {
  name: string
  back_image_url: string
  face_image_template: string
  red_suit_color: string
  black_suit_color: string
  border_color: string
}

const EMPTY_FORM: FormState = {
  name: '',
  back_image_url: '',
  face_image_template: '',
  red_suit_color: '#dc2626',
  black_suit_color: '#0f172a',
  border_color: '#cbd5e1',
}

function formFromDeck(d: CardDeck): FormState {
  return {
    name: d.name,
    back_image_url: d.back_image_url ?? '',
    face_image_template: d.face_image_template ?? '',
    red_suit_color: d.red_suit_color,
    black_suit_color: d.black_suit_color,
    border_color: d.border_color,
  }
}

function formToRequest(f: FormState): DeckRequest {
  return {
    name: f.name.trim(),
    back_image_url: f.back_image_url.trim(),
    face_image_template: f.face_image_template.trim(),
    red_suit_color: f.red_suit_color,
    black_suit_color: f.black_suit_color,
    border_color: f.border_color,
  }
}

/** The live preview theme for the deck currently being edited. */
function formToTheme(f: FormState): DeckTheme {
  return {
    ref: '__preview__',
    name: f.name || 'Untitled deck',
    backImageUrl: f.back_image_url.trim() || undefined,
    faceImageTemplate: f.face_image_template.trim() || undefined,
    redSuitColor: f.red_suit_color,
    blackSuitColor: f.black_suit_color,
    borderColor: f.border_color,
  }
}

function labelStyle(): React.CSSProperties {
  return { display: 'grid', gap: 4, fontSize: 13, fontWeight: 600 }
}

function inputStyle(): React.CSSProperties {
  return {
    padding: '8px 10px',
    borderRadius: 8,
    border: '1px solid #cbd5e1',
    fontSize: 14,
    fontWeight: 400,
  }
}

function cardStyle(): React.CSSProperties {
  return {
    border: '1px solid rgba(15, 23, 42, 0.12)',
    borderRadius: 14,
    padding: 16,
    background: '#ffffff',
  }
}

export function DecksPage() {
  const { theme, available, decks, loading, error, selectDeck, createDeck, updateDeck, deleteDeck } = useDeck()

  const [form, setForm] = useState<FormState>(EMPTY_FORM)
  const [editingId, setEditingId] = useState<number | null>(null)
  const [busy, setBusy] = useState(false)
  const [formError, setFormError] = useState<string | null>(null)
  const [notice, setNotice] = useState<string | null>(null)

  const previewTheme = useMemo(() => formToTheme(form), [form])
  const customDecks = decks?.custom ?? []

  function set<K extends keyof FormState>(key: K, value: FormState[K]) {
    setForm((f) => ({ ...f, [key]: value }))
  }

  function resetForm() {
    setForm(EMPTY_FORM)
    setEditingId(null)
    setFormError(null)
  }

  function startEdit(d: CardDeck) {
    setForm(formFromDeck(d))
    setEditingId(d.id)
    setFormError(null)
    setNotice(null)
  }

  async function run(action: () => Promise<void>, successMessage: string) {
    setBusy(true)
    setFormError(null)
    setNotice(null)
    try {
      await action()
      setNotice(successMessage)
    } catch (e: unknown) {
      setFormError(e instanceof Error ? e.message : 'Request failed')
    } finally {
      setBusy(false)
    }
  }

  async function onSubmit(e: React.FormEvent) {
    e.preventDefault()
    if (!form.name.trim()) {
      setFormError('Give the deck a name.')
      return
    }
    const req = formToRequest(form)
    const id = editingId
    await run(async () => {
      if (id === null) {
        await createDeck(req)
        setForm(EMPTY_FORM)
      } else {
        await updateDeck(id, req)
      }
    }, id === null ? 'Deck created.' : 'Deck saved.')
  }

  async function onDelete(d: CardDeck) {
    // Deleting the deck being edited would leave the form pointing at a
    // missing row, so drop back to "create new".
    await run(async () => {
      await deleteDeck(d.id)
      if (editingId === d.id) resetForm()
    }, `Deleted “${d.name}”.`)
  }

  return (
    <div style={{ padding: 24, maxWidth: 1040, margin: '0 auto', display: 'grid', gap: 20 }}>
      <header style={{ display: 'flex', alignItems: 'baseline', gap: 12, flexWrap: 'wrap' }}>
        <h1 style={{ margin: 0 }}>Card decks</h1>
        <Link to="/lobbies">← Back to lobbies</Link>
      </header>

      <p style={{ margin: 0, opacity: 0.75 }}>
        Pick a deck skin, or build your own with custom card artwork. Decks are personal — only you see your
        selection.
      </p>

      {error ? (
        <div role="alert" style={{ color: '#b91c1c' }}>
          {error}
        </div>
      ) : null}

      <section style={cardStyle()}>
        <h2 style={{ marginTop: 0, fontSize: 18 }}>Your deck</h2>
        {loading ? <p style={{ opacity: 0.7 }}>Loading decks…</p> : null}
        <div style={{ display: 'flex', gap: 14, flexWrap: 'wrap' }}>
          {available.map((t) => {
            const isActive = t.ref === theme.ref
            return (
              <button
                key={t.ref || BUILTIN_DECK_PREFIX}
                type="button"
                disabled={busy}
                aria-pressed={isActive}
                onClick={() => void run(() => selectDeck(t.ref), `“${t.name}” is now your deck.`)}
                style={{
                  display: 'grid',
                  gap: 8,
                  justifyItems: 'center',
                  padding: 12,
                  borderRadius: 12,
                  cursor: busy ? 'not-allowed' : 'pointer',
                  background: isActive ? 'rgba(37,99,235,0.08)' : '#ffffff',
                  border: isActive ? '2px solid #2563eb' : '1px solid #cbd5e1',
                }}
              >
                <div style={{ display: 'flex', gap: 6 }}>
                  <CardIcon card={PREVIEW_CARDS[0]} theme={t} disabled />
                  <CardIcon card={PREVIEW_CARDS[1]} theme={t} disabled />
                  <CardBack theme={t} />
                </div>
                <span style={{ fontWeight: 700, fontSize: 13 }}>
                  {t.name}
                  {isActive ? ' ✓' : ''}
                </span>
              </button>
            )
          })}
        </div>
      </section>

      <section style={{ ...cardStyle(), display: 'grid', gap: 18 }}>
        <h2 style={{ margin: 0, fontSize: 18 }}>{editingId === null ? 'Create a deck' : 'Edit deck'}</h2>

        {/* auto-fit so the preview drops below the form instead of overflowing
            on narrow screens. */}
        <div
          style={{
            display: 'grid',
            gap: 18,
            gridTemplateColumns: 'repeat(auto-fit, minmax(min(260px, 100%), 1fr))',
          }}
        >
          <form onSubmit={(e) => void onSubmit(e)} style={{ display: 'grid', gap: 12 }}>
            <label style={labelStyle()}>
              Deck name
              <input
                value={form.name}
                onChange={(e) => set('name', e.target.value)}
                maxLength={60}
                placeholder="Neon Nights"
                style={inputStyle()}
                required
              />
            </label>

            <label style={labelStyle()}>
              Card face image URL template
              <input
                value={form.face_image_template}
                onChange={(e) => set('face_image_template', e.target.value)}
                placeholder="https://cdn.example.com/cards/{code}.svg"
                style={inputStyle()}
              />
              <span style={{ fontWeight: 400, fontSize: 12, opacity: 0.7 }}>
                Must be an https URL containing <code>{'{code}'}</code> (e.g. “AS”), or <code>{'{rank}'}</code> and{' '}
                <code>{'{suit}'}</code>. Leave blank to draw cards with suit symbols.
              </span>
            </label>

            <label style={labelStyle()}>
              Card back image URL
              <input
                value={form.back_image_url}
                onChange={(e) => set('back_image_url', e.target.value)}
                placeholder="https://cdn.example.com/cards/back.png"
                style={inputStyle()}
              />
            </label>

            <div style={{ display: 'flex', gap: 14, flexWrap: 'wrap' }}>
              <label style={labelStyle()}>
                Red suits
                <input
                  type="color"
                  value={form.red_suit_color}
                  onChange={(e) => set('red_suit_color', e.target.value)}
                />
              </label>
              <label style={labelStyle()}>
                Black suits
                <input
                  type="color"
                  value={form.black_suit_color}
                  onChange={(e) => set('black_suit_color', e.target.value)}
                />
              </label>
              <label style={labelStyle()}>
                Border
                <input type="color" value={form.border_color} onChange={(e) => set('border_color', e.target.value)} />
              </label>
            </div>

            {formError ? (
              <div role="alert" style={{ color: '#b91c1c', fontSize: 13 }}>
                {formError}
              </div>
            ) : null}
            {notice ? <div style={{ color: '#15803d', fontSize: 13 }}>{notice}</div> : null}

            <div style={{ display: 'flex', gap: 10 }}>
              <button type="submit" disabled={busy} style={{ fontWeight: 700 }}>
                {editingId === null ? 'Create deck' : 'Save changes'}
              </button>
              {editingId !== null ? (
                <button type="button" onClick={resetForm} disabled={busy}>
                  Cancel
                </button>
              ) : null}
            </div>
          </form>

          <div style={{ display: 'grid', gap: 8, justifyItems: 'center', alignContent: 'start' }}>
            <span style={{ fontSize: 13, fontWeight: 700, opacity: 0.8 }}>Preview</span>
            <div style={{ display: 'flex', gap: 6, flexWrap: 'wrap', justifyContent: 'center', maxWidth: 240 }}>
              {PREVIEW_CARDS.map((c) => (
                <CardIcon key={`${c.rank}${c.suit}`} card={c} theme={previewTheme} disabled />
              ))}
              <CardBack theme={previewTheme} />
            </div>
            <span style={{ fontSize: 12, opacity: 0.65, textAlign: 'center', maxWidth: 240 }}>
              Images that fail to load fall back to drawn cards.
            </span>
          </div>
        </div>
      </section>

      <section style={cardStyle()}>
        <h2 style={{ marginTop: 0, fontSize: 18 }}>Your custom decks</h2>
        {customDecks.length === 0 ? (
          <p style={{ opacity: 0.7, margin: 0 }}>No custom decks yet — create one above.</p>
        ) : (
          <ul style={{ listStyle: 'none', padding: 0, margin: 0, display: 'grid', gap: 10 }}>
            {customDecks.map((d) => (
              <li
                key={d.id}
                style={{
                  display: 'flex',
                  alignItems: 'center',
                  gap: 12,
                  flexWrap: 'wrap',
                  padding: '10px 12px',
                  borderRadius: 10,
                  border: '1px solid #e2e8f0',
                }}
              >
                <strong style={{ flex: '1 1 160px' }}>{d.name}</strong>
                <span style={{ fontSize: 12, opacity: 0.7, flex: '1 1 220px', wordBreak: 'break-all' }}>
                  {d.face_image_template || 'drawn faces'}
                </span>
                <button type="button" onClick={() => startEdit(d)} disabled={busy}>
                  Edit
                </button>
                <button type="button" onClick={() => void onDelete(d)} disabled={busy}>
                  Delete
                </button>
              </li>
            ))}
          </ul>
        )}
      </section>
    </div>
  )
}
