import { useEffect, useMemo, useRef, useState } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { api } from '../api/client'
import type { Card, GameMove, GameSnapshot } from '../api/types'
import { useAuth } from '../auth/auth'
import { WsClient } from '../ws/wsClient'

function rankLabel(rank: number): string {
  return rank === 1 ? 'A' : rank === 11 ? 'J' : rank === 12 ? 'Q' : rank === 13 ? 'K' : String(rank)
}

function suitSymbol(suit: Card['suit']): string {
  switch (suit) {
    case 'S':
      return '♠'
    case 'H':
      return '♥'
    case 'D':
      return '♦'
    case 'C':
      return '♣'
  }
}

function suitColor(suit: Card['suit']): string {
  return suit === 'H' || suit === 'D' ? '#dc2626' : '#0f172a'
}

function cardToCode(c: Card): string {
  return `${rankLabel(c.rank)}${c.suit}`
}

function cardToString(c: Card): string {
  return `${rankLabel(c.rank)}${suitSymbol(c.suit)}`
}

function cardValue15(c: Card): number {
  // Pegging total uses 15-values: A=1, 2-10 as-is, J/Q/K=10.
  return c.rank >= 10 ? 10 : c.rank
}

function CardIcon({
  card,
  selected,
  disabled,
  onClick,
  title,
}: {
  card: Card
  selected?: boolean
  disabled?: boolean
  onClick?: () => void
  title?: string
}) {
  const rank = rankLabel(card.rank)
  const suit = suitSymbol(card.suit)
  const color = suitColor(card.suit)
  return (
    <button
      type="button"
      disabled={disabled}
      onClick={onClick}
      title={title}
      style={{
        width: 56,
        height: 72,
        padding: 0,
        borderRadius: 10,
        border: '1px solid #cbd5e1',
        background: selected ? '#2563eb' : '#ffffff',
        cursor: disabled ? 'not-allowed' : 'pointer',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        boxShadow: selected ? '0 0 0 2px rgba(37,99,235,0.25)' : undefined,
      }}
    >
      <div
        style={{
          width: '100%',
          height: '100%',
          position: 'relative',
          borderRadius: 10,
          background: selected ? '#2563eb' : '#ffffff',
        }}
      >
        <div
          style={{
            position: 'absolute',
            top: 6,
            left: 7,
            fontSize: 12,
            fontWeight: 700,
            lineHeight: 1,
            color: selected ? 'white' : color,
          }}
        >
          {rank}
        </div>
        <div
          style={{
            position: 'absolute',
            top: 22,
            left: 0,
            right: 0,
            textAlign: 'center',
            fontSize: 28,
            lineHeight: 1,
            color: selected ? 'white' : color,
          }}
        >
          {suit}
        </div>
        <div
          style={{
            position: 'absolute',
            bottom: 6,
            right: 7,
            fontSize: 12,
            fontWeight: 700,
            lineHeight: 1,
            transform: 'rotate(180deg)',
            color: selected ? 'white' : color,
          }}
        >
          {rank}
        </div>
      </div>
    </button>
  )
}

function ActionCard({
  label,
  disabled,
  onClick,
  title,
  accent,
}: {
  label: string
  disabled?: boolean
  onClick?: () => void
  title?: string
  accent?: 'primary' | 'danger'
}) {
  const bg = accent === 'primary' ? '#2563eb' : accent === 'danger' ? '#dc2626' : '#ffffff'
  const fg = accent ? '#ffffff' : '#0f172a'
  return (
    <button
      type="button"
      disabled={disabled}
      onClick={onClick}
      title={title}
      style={{
        width: 56,
        height: 72,
        padding: 0,
        borderRadius: 10,
        border: '1px solid #cbd5e1',
        background: bg,
        cursor: disabled ? 'not-allowed' : 'pointer',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        fontWeight: 900,
        letterSpacing: 0.8,
        color: fg,
        opacity: disabled ? 0.6 : 1,
      }}
    >
      {label}
    </button>
  )
}

function CardBack({ title }: { title?: string }) {
  return (
    <div
      title={title}
      style={{
        width: 28,
        height: 36,
        borderRadius: 8,
        border: '1px solid #cbd5e1',
        background:
          'repeating-linear-gradient(45deg, #1d4ed8 0px, #1d4ed8 6px, #2563eb 6px, #2563eb 12px)',
        boxShadow: '0 1px 2px rgba(0,0,0,0.12)',
      }}
    />
  )
}

function safeCountHandJSON(handJSON: string | undefined): number | null {
  if (!handJSON) return null
  const s = handJSON.trim()
  if (!s) return null
  try {
    const v: unknown = JSON.parse(s)
    return Array.isArray(v) ? v.length : null
  } catch {
    return null
  }
}

function PegTrack({
  players,
  scores,
  highlight,
}: {
  players: GameSnapshot['players']
  scores: number[] | undefined
  highlight?: { pos: number; delta?: number; kind?: 'score' | 'go' | 'play'; label?: string } | null
}) {
  const max = 121
  const endPad = 18
  const sorted = players.slice().sort((a, b) => a.position - b.position)
  const colors = ['#2563eb', '#dc2626', '#16a34a', '#7c3aed']

  const prevScoresRef = useRef<number[] | null>(null)
  const [deltas, setDeltas] = useState<Record<number, number>>({})

  useEffect(() => {
    if (!scores || scores.length === 0) return
    const prev = prevScoresRef.current
    if (!prev) {
      prevScoresRef.current = scores.slice()
      return
    }
    const nextDeltas: Record<number, number> = {}
    for (let i = 0; i < scores.length; i++) {
      const d = scores[i] - (prev[i] ?? 0)
      if (d !== 0) nextDeltas[i] = d
    }
    prevScoresRef.current = scores.slice()
    if (Object.keys(nextDeltas).length > 0) {
      setDeltas(nextDeltas)
      const t = window.setTimeout(() => setDeltas({}), 1100)
      return () => window.clearTimeout(t)
    }
  }, [scores])

  const posPct = (v: number) => `${Math.max(0, Math.min(max, v)) / max * 100}%`

  return (
    <div style={{ marginTop: 10, padding: 10, border: '1px solid #e2e8f0', borderRadius: 10 }}>
      <div style={{ fontWeight: 700, marginBottom: 8 }}>Peg board</div>
      <div
        style={{
          position: 'relative',
          height: 34,
          borderRadius: 999,
          background: '#f1f5f9',
          border: '1px solid #e2e8f0',
          overflow: 'hidden',
        }}
      >
        {/* Endcaps so 0/121 don't feel cramped */}
        <div style={{ position: 'absolute', left: 0, top: 0, bottom: 0, width: endPad, background: '#eef2ff' }} />
        <div style={{ position: 'absolute', right: 0, top: 0, bottom: 0, width: endPad, background: '#ecfeff' }} />
        {/* Finish marker */}
        <div style={{ position: 'absolute', right: 6, top: 8, fontSize: 12, opacity: 0.8 }} title="Finish">
          121
        </div>

        {/* Inner lane where ticks/pegs are positioned (gives padding at the ends) */}
        <div style={{ position: 'absolute', left: endPad, right: endPad, top: 0, bottom: 0 }}>
          {/* tick marks */}
          {Array.from({ length: Math.floor(max / 5) + 1 }).map((_, i) => {
            const v = i * 5
            const isTen = v % 10 === 0
            return (
              <div
                key={`tick:${v}`}
                style={{
                  position: 'absolute',
                  left: posPct(v),
                  top: 0,
                  bottom: 0,
                  width: 1,
                  background: isTen ? '#cbd5e1' : '#e2e8f0',
                  opacity: isTen ? 1 : 0.75,
                }}
                title={String(v)}
              />
            )
          })}

          {sorted.map((p, idx) => {
            const s = scores?.[p.position] ?? 0
            const c = colors[idx % colors.length]
            const showDelta = deltas[p.position]
            const isHighlight = highlight?.pos === p.position
            return (
              <div
                key={`peg:${p.position}`}
                style={{
                  position: 'absolute',
                  left: posPct(s),
                  top: 6 + (idx % 2) * 12,
                  transform: 'translateX(-50%)',
                  transition: 'left 420ms cubic-bezier(0.2, 0.8, 0.2, 1)',
                }}
              >
                <div style={{ position: 'relative' }}>
                  <div
                    style={{
                      width: 16,
                      height: 16,
                      borderRadius: 999,
                      background: c,
                      border: isHighlight ? '3px solid #f59e0b' : '2px solid #ffffff',
                      boxShadow: '0 1px 3px rgba(0,0,0,0.25)',
                    }}
                    title={`P${p.position}: ${s}`}
                  />
                  {/* Always show exact score next to the peg */}
                  <div
                    style={{
                      position: 'absolute',
                      top: -2,
                      left: 20,
                      fontSize: 11,
                      fontWeight: 800,
                      color: '#0f172a',
                      background: 'rgba(255,255,255,0.9)',
                      border: '1px solid #e2e8f0',
                      borderRadius: 999,
                      padding: '0 6px',
                    }}
                  >
                    {s}
                  </div>
                  {(typeof showDelta === 'number' && showDelta !== 0) || (isHighlight && highlight?.kind === 'go') ? (
                    <div
                      style={{
                        position: 'absolute',
                        top: -26,
                        left: '50%',
                        transform: 'translateX(-50%)',
                        padding: '2px 6px',
                        borderRadius: 999,
                        background: highlight?.kind === 'go' && isHighlight ? '#0f172a' : '#16a34a',
                        color: 'white',
                        fontSize: 12,
                        fontWeight: 800,
                        opacity: 0.95,
                        transition: 'opacity 250ms ease',
                        whiteSpace: 'nowrap',
                      }}
                    >
                      {highlight?.kind === 'go' && isHighlight ? 'GO' : `${showDelta > 0 ? '+' : ''}${showDelta}`}
                    </div>
                  ) : null}
                </div>
              </div>
            )
          })}
        </div>
      </div>
      {/* Vertical ruler labels so we can see the whole scale without horizontal crowding */}
      <div style={{ marginTop: 4, position: 'relative', height: 28, fontSize: 10, opacity: 0.85 }}>
        <div style={{ position: 'absolute', left: endPad, right: endPad, top: 0, bottom: 0 }}>
          {Array.from({ length: Math.floor(max / 5) + 1 }).map((_, i) => {
            const v = i * 5
            const isTen = v % 10 === 0
            return (
              <div
                key={`label:${v}`}
                style={{
                  position: 'absolute',
                  left: posPct(v),
                  transform: 'translateX(-50%)',
                  writingMode: 'vertical-rl',
                  textOrientation: 'mixed',
                  lineHeight: 1,
                  fontWeight: isTen ? 800 : 600,
                  color: isTen ? '#0f172a' : '#334155',
                  pointerEvents: 'none',
                }}
              >
                {v}
              </div>
            )
          })}
          {/* Ensure max is labeled even when it isn't a multiple of 5 */}
          {max % 5 !== 0 ? (
            <div
              style={{
                position: 'absolute',
                left: posPct(max),
                transform: 'translateX(-50%)',
                writingMode: 'vertical-rl',
                textOrientation: 'mixed',
                lineHeight: 1,
                fontWeight: 800,
                color: '#0f172a',
                pointerEvents: 'none',
              }}
            >
              {max}
            </div>
          ) : null}
        </div>
      </div>
    </div>
  )
}

function BreakdownView({
  title,
  b,
}: {
  title: string
  b:
    | { total: number; fifteens: number; pairs: number; runs: number; flush: number; nobs: number; reasons?: Record<string, number> }
    | undefined
}) {
  if (!b) return null
  const parts: Array<[string, number]> = [
    ['15s', b.fifteens] as [string, number],
    ['pairs', b.pairs] as [string, number],
    ['runs', b.runs] as [string, number],
    ['flush', b.flush] as [string, number],
    ['nobs', b.nobs] as [string, number],
  ].filter(([, v]) => v > 0)
  return (
    <div style={{ marginTop: 6 }}>
      <div style={{ fontWeight: 600 }}>
        {title}: +{b.total}
      </div>
      {parts.length > 0 ? (
        <div style={{ opacity: 0.9 }}>{parts.map(([k, v]) => `${k} ${v}`).join(' · ')}</div>
      ) : (
        <div style={{ opacity: 0.75 }}>(no points)</div>
      )}
    </div>
  )
}

export function GamePage() {
  const { id } = useParams()
  const gameId = Number(id)
  const isValidId = Number.isFinite(gameId) && gameId > 0
  const { user } = useAuth()
  const nav = useNavigate()
  const ws = useMemo(() => new WsClient(), [])
  const [status, setStatus] = useState<string>('disconnected')
  const [snap, setSnap] = useState<GameSnapshot | null>(null)
  const [loading, setLoading] = useState(false)
  const [err, setErr] = useState<string | null>(null)
  const [moveErr, setMoveErr] = useState<string | null>(null)
  const [moveBusy, setMoveBusy] = useState(false)
  const [selected, setSelected] = useState<Set<string>>(() => new Set())
  const [moves, setMoves] = useState<GameMove[] | null>(null)
  const [peggingCue, setPeggingCue] = useState<{ pos: number; kind: 'score' | 'go' | 'play'; delta?: number; card?: string } | null>(null)

  useEffect(() => {
    if (!user || !isValidId) return
    let cancelled = false

    async function fetchSnapshot() {
      setErr(null)
      try {
        setLoading(true)
        const snap = await api.getGame(gameId)
        if (!cancelled) setSnap(snap)
      } catch (e: unknown) {
        if (!cancelled) setErr(e instanceof Error ? e.message : 'failed to load game')
      } finally {
        if (!cancelled) setLoading(false)
      }
    }

    async function fetchMoves() {
      try {
        const res = await api.listGameMoves(gameId)
        if (!cancelled) setMoves(res.moves)
      } catch {
        // best-effort (used for visual cues only)
      }
    }

    // Fetch an initial snapshot immediately so the UI isn't blank until a WS update arrives.
    void fetchSnapshot()
    void fetchMoves()

    ws.connect(`game:${gameId}`)
    const offOpen = ws.on('ws_open', () => setStatus('connected'))
    const offClose = ws.on('ws_close', () => setStatus('disconnected'))
    // WS broadcasts a public snapshot (hands hidden). Treat updates as an invalidation signal
    // and re-fetch the user-specific snapshot via HTTP so "your hand" stays populated.
    const offUpdate = ws.on('game_update', () => {
      void fetchSnapshot()
      void fetchMoves()
    })
    return () => {
      cancelled = true
      offOpen()
      offClose()
      offUpdate()
      ws.disconnect()
    }
  }, [user, gameId, isValidId, ws])

  const myPos = snap?.players.find((p) => p.user_id === user?.id)?.position
  const state = snap?.state
  const stage = state?.stage
  const myHand = typeof myPos === 'number' ? state?.hands?.[myPos] ?? [] : []
  const discardCount = state?.rules?.max_players === 2 ? 2 : 1
  const isMyTurn = typeof myPos === 'number' && stage === 'pegging' && state?.current_index === myPos
  const peggingTotal = state?.pegging_total ?? 0
  const hasLegalPeggingPlay =
    stage === 'pegging' && myHand.some((c) => peggingTotal+cardValue15(c) <= 31)
  const canGo = isMyTurn && stage === 'pegging' && !moveBusy && !loading && !hasLegalPeggingPlay

  function playerLabel(pos: number | undefined): string {
    if (typeof pos !== 'number' || !snap) return ''
    const p = snap.players.find((pp) => pp.position === pos)
    if (!p) return `P${pos}`
    const isMe = p.user_id === user?.id
    return `P${pos}${isMe ? ' (you)' : ''}${p.is_bot ? ' 🤖' : ''}`
  }

  useEffect(() => {
    if (!moves || !snap || !snap.state || snap.state.stage !== 'pegging') return
    const latest = moves[0]
    if (!latest) return
    const pPos = snap.players.find((p) => p.user_id === latest.player_id)?.position
    if (typeof pPos !== 'number') return

    if (latest.move_type === 'go') {
      setPeggingCue({ pos: pPos, kind: 'go' })
      const t = window.setTimeout(() => setPeggingCue(null), 1000)
      return () => window.clearTimeout(t)
    }

    if (latest.move_type === 'play_card') {
      const delta = latest.score_verified ?? 0
      setPeggingCue({ pos: pPos, kind: delta > 0 ? 'score' : 'play', delta, card: latest.card_played })
      const t = window.setTimeout(() => setPeggingCue(null), 1200)
      return () => window.clearTimeout(t)
    }
  }, [moves, snap])

  async function submitMove(move: Parameters<(typeof api)['moveGame']>[1]) {
    if (!isValidId) return
    setMoveErr(null)
    setMoveBusy(true)
    try {
      await api.moveGame(gameId, move)
      setSelected(new Set())
      const next = await api.getGame(gameId)
      setSnap(next)
    } catch (e: unknown) {
      setMoveErr(e instanceof Error ? e.message : 'move failed')
    } finally {
      setMoveBusy(false)
    }
  }

  async function quit() {
    if (!isValidId) return
    setMoveErr(null)
    setMoveBusy(true)
    try {
      await api.quitGame(gameId)
      nav('/lobbies', { replace: true })
    } catch (e: unknown) {
      setMoveErr(e instanceof Error ? e.message : 'failed to quit')
    } finally {
      setMoveBusy(false)
    }
  }

  return (
    <div style={{ maxWidth: 900, margin: '24px auto', padding: '0 16px' }}>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'baseline', gap: 12 }}>
        <h1>Game {isValidId ? gameId : 'Invalid ID'}</h1>
        <div style={{ display: 'flex', gap: 10, alignItems: 'baseline' }}>
          <button type="button" onClick={quit} disabled={moveBusy || !isValidId}>
            🚪 Quit
          </button>
          <button type="button" onClick={() => nav('/lobbies', { replace: true })}>
            ⬅ Back to lobbies
          </button>
        </div>
      </div>
      <div>
        Status: {status} {loading ? '(loading snapshot…) ' : null}
      </div>
      {err && <div style={{ color: 'crimson', marginTop: 8 }}>{err}</div>}

      {!snap ? (
        <div style={{ marginTop: 16, opacity: 0.8 }}>{loading ? 'Loading…' : 'No snapshot yet.'}</div>
      ) : (
        <div style={{ marginTop: 16 }}>
          <h2>Players</h2>
          <ul>
            {snap.players
              .slice()
              .sort((a, b) => a.position - b.position)
              .map((p) => {
                const isMe = p.user_id === user?.id
                const score = state?.scores?.[p.position]
                const isDealer = state?.dealer_index === p.position
                const isTurn = stage === 'pegging' && state?.current_index === p.position
                const handCount = isMe ? null : safeCountHandJSON(p.hand)
                return (
                  <li key={`${p.game_id}:${p.user_id}`}>
                    <b>
                      P{p.position} user:{p.user_id}
                      {isMe ? ' (you)' : ''}
                      {p.is_bot ? ' 🤖' : ''}
                    </b>
                    {typeof score === 'number' ? ` — score ${score}` : ''}
                    {isDealer ? ' — dealer ★' : ''}
                    {isTurn ? ' — turn ▶' : ''}
                    {typeof handCount === 'number' ? (
                      <span style={{ marginLeft: 10, display: 'inline-flex', alignItems: 'center', gap: 6 }}>
                        <span style={{ opacity: 0.85 }}>hand</span>
                        <span style={{ display: 'inline-flex', gap: 4, alignItems: 'center' }}>
                          {Array.from({ length: Math.min(12, handCount) }).map((_, i) => (
                            <CardBack key={`hb:${p.position}:${i}`} title={`${handCount} cards`} />
                          ))}
                          {handCount > 12 ? <span style={{ opacity: 0.8 }}>+{handCount - 12}</span> : null}
                        </span>
                      </span>
                    ) : null}
                  </li>
                )
              })}
          </ul>

          <PegTrack
            players={snap.players}
            scores={state?.scores}
            highlight={peggingCue ? { pos: peggingCue.pos, kind: peggingCue.kind, delta: peggingCue.delta } : null}
          />

          <div
            style={{
              marginTop: 14,
              padding: 10,
              border: '1px solid #e2e8f0',
              borderRadius: 10,
              background: '#f8fafc',
              display: 'flex',
              flexWrap: 'wrap',
              gap: 12,
              alignItems: 'center',
            }}
          >
            {typeof state?.dealer_index === 'number' ? (
              <div style={{ fontWeight: 800 }}>Dealer: {playerLabel(state.dealer_index)}</div>
            ) : null}
            {stage === 'pegging' && typeof state?.current_index === 'number' ? (
              <div style={{ fontWeight: 800 }}>{isMyTurn ? 'Your turn' : `Turn: ${playerLabel(state.current_index)}`}</div>
            ) : null}
            {stage === 'pegging' ? (
              <div style={{ fontWeight: 900 }}>
                Total: {peggingTotal} / 31 <span style={{ opacity: 0.75 }}>(need {Math.max(0, 31 - peggingTotal)})</span>
              </div>
            ) : null}
            <div style={{ display: 'flex', gap: 8, alignItems: 'center' }}>
              <div style={{ fontWeight: 800 }}>Cut:</div>
              {state?.cut ? (
                <CardIcon card={state.cut} disabled title={cardToCode(state.cut)} />
              ) : (
                <div style={{ opacity: 0.8 }}>{stage === 'discard' ? '(not cut yet)' : '(none)'}</div>
              )}
            </div>
          </div>
          {stage === 'pegging' && (
            <div style={{ marginTop: 8 }}>
              <div style={{ fontWeight: 700 }}>Cards played this count</div>
              {(state?.pegging_seq ?? []).length === 0 ? (
                <div style={{ opacity: 0.8, marginTop: 6 }}>(none yet)</div>
              ) : (
                <div style={{ display: 'flex', gap: 8, flexWrap: 'wrap', marginTop: 8 }}>
                  {(state?.pegging_seq ?? []).map((c, i) => (
                    <CardIcon key={`pegseq:${i}:${cardToCode(c)}`} card={c} disabled title={cardToCode(c)} />
                  ))}
                </div>
              )}
              <div
                style={{
                  marginTop: 10,
                  padding: 10,
                  border: '1px solid #e2e8f0',
                  borderRadius: 10,
                  background: '#f8fafc',
                }}
              >
                <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'baseline', gap: 10 }}>
                  <div style={{ fontWeight: 800 }}>Pegging</div>
                  <details style={{ opacity: 0.9 }}>
                    <summary style={{ cursor: 'pointer' }}>?</summary>
                    <div style={{ marginTop: 6, maxWidth: 520 }}>
                      Score can happen during pegging (15, 31, pairs, runs). GO appears only when you have no legal play.
                    </div>
                  </details>
                </div>
                <div style={{ marginTop: 6, fontWeight: 800 }}>
                  {isMyTurn ? 'Your turn — play a card' : `Waiting for ${playerLabel(state?.current_index)}`}
                </div>
                <div style={{ marginTop: 4, opacity: 0.9 }}>
                  {canGo ? 'No legal play — say GO' : 'Play a card without going over 31.'}
                </div>
              </div>
              {peggingCue?.kind === 'score' && typeof peggingCue.delta === 'number' && peggingCue.delta > 0 ? (
                <div style={{ marginTop: 10, display: 'flex', alignItems: 'center', gap: 10 }}>
                  <div style={{ fontWeight: 900, color: '#16a34a' }}>+{peggingCue.delta}</div>
                  {peggingCue.card ? <ActionCard label={peggingCue.card} disabled title="Last scoring play" /> : null}
                  <div style={{ opacity: 0.85 }}>Scored during pegging</div>
                </div>
              ) : peggingCue?.kind === 'go' ? (
                <div style={{ marginTop: 10, display: 'flex', alignItems: 'center', gap: 10 }}>
                  <ActionCard label="GO" disabled accent="primary" title="Player said GO" />
                  <div style={{ opacity: 0.85 }}>No legal play</div>
                </div>
              ) : peggingCue?.kind === 'play' ? (
                <div style={{ marginTop: 10, display: 'flex', alignItems: 'center', gap: 10 }}>
                  {peggingCue.card ? <ActionCard label={peggingCue.card} disabled title="Last play" /> : null}
                  <div style={{ opacity: 0.85 }}>Played</div>
                </div>
              ) : null}
            </div>
          )}
          {stage === 'finished' && (
            <div style={{ marginTop: 12, padding: 12, border: '1px solid #cbd5e1', borderRadius: 8 }}>
              <div style={{ fontWeight: 700 }}>Game over</div>
              <div style={{ opacity: 0.85, marginTop: 6 }}>
                This game is finished. You can quit and start a new one from the lobbies screen.
              </div>
              {state?.history && state.history.length > 0 && (
                <div style={{ marginTop: 12 }}>
                  <div style={{ fontWeight: 700 }}>Scoring recap</div>
                  <div style={{ display: 'grid', gap: 10, marginTop: 8 }}>
                    {state.history
                      .slice()
                      .reverse()
                      .slice(0, 8)
                      .map((r) => (
                        <div
                          key={`round:${r.round}`}
                          style={{ padding: 10, border: '1px solid #e2e8f0', borderRadius: 8 }}
                        >
                          <div style={{ fontWeight: 700 }}>
                            Round {r.round} — dealer P{r.dealer_index} {r.cut ? `— cut ${cardToString(r.cut)}` : ''}
                          </div>
                          <div style={{ marginTop: 6 }}>
                            {snap.players
                              .slice()
                              .sort((a, b) => a.position - b.position)
                              .map((p) => {
                                const before = r.scores_before?.[p.position]
                                const after = r.scores_after?.[p.position]
                                const delta =
                                  typeof before === 'number' && typeof after === 'number' ? after - before : undefined
                                return (
                                  <div key={`r:${r.round}:p:${p.position}`} style={{ opacity: 0.92 }}>
                                    P{p.position}:{' '}
                                    {typeof delta === 'number' ? (delta >= 0 ? `+${delta}` : String(delta)) : ''}{' '}
                                    {typeof before === 'number' && typeof after === 'number'
                                      ? `(${before} → ${after})`
                                      : ''}
                                  </div>
                                )
                              })}
                          </div>
                        </div>
                      ))}
                  </div>
                </div>
              )}
              <div style={{ marginTop: 10, display: 'flex', gap: 10, flexWrap: 'wrap' }}>
                <button type="button" onClick={quit} disabled={moveBusy || !isValidId}>
                  Quit
                </button>
                <button type="button" onClick={() => nav('/lobbies', { replace: true })}>
                  Back to lobbies
                </button>
              </div>
            </div>
          )}

          {stage === 'counting' && (
            <div style={{ marginTop: 12, padding: 12, border: '1px solid #cbd5e1', borderRadius: 8 }}>
              <div style={{ fontWeight: 700 }}>Counting</div>
              <div style={{ opacity: 0.85, marginTop: 6 }}>Scores have been applied. Review the breakdown, then deal the next hand.</div>
              <div style={{ marginTop: 10 }}>
                <button
                  type="button"
                  disabled={moveBusy || loading}
                  onClick={async () => {
                    setMoveErr(null)
                    setMoveBusy(true)
                    try {
                      await api.nextHand(gameId)
                      const next = await api.getGame(gameId)
                      setSnap(next)
                    } catch (e: unknown) {
                      setMoveErr(e instanceof Error ? e.message : 'failed to deal next hand')
                    } finally {
                      setMoveBusy(false)
                    }
                  }}
                >
                  ▶ Next hand
                </button>
              </div>

              {state?.kept_hands && (
                <div style={{ marginTop: 12 }}>
                  <div style={{ fontWeight: 600 }}>Kept hands</div>
                  <div style={{ display: 'grid', gap: 12, marginTop: 8 }}>
                    {state.kept_hands.map((hand, idx) => (
                      <div key={`kept:${idx}`}>
                        <div style={{ fontWeight: 600, marginBottom: 6 }}>P{idx}</div>
                        <div style={{ display: 'flex', gap: 8, flexWrap: 'wrap' }}>
                          {hand.map((c, j) => (
                            <CardIcon key={`kh:${idx}:${j}:${cardToCode(c)}`} card={c} disabled title={cardToCode(c)} />
                          ))}
                        </div>
                        <BreakdownView title="Hand points" b={state.count_summary?.hands?.[String(idx)]} />
                      </div>
                    ))}
                  </div>
                </div>
              )}

              {state?.crib && (
                <div style={{ marginTop: 12 }}>
                  <div style={{ fontWeight: 600 }}>Crib</div>
                  <div style={{ display: 'flex', gap: 8, flexWrap: 'wrap', marginTop: 8 }}>
                    {state.crib.map((c, i) => (
                      <CardIcon key={`crib:${i}:${cardToCode(c)}`} card={c} disabled title={cardToCode(c)} />
                    ))}
                  </div>
                  <BreakdownView title="Crib points" b={state.count_summary?.crib} />
                </div>
              )}
            </div>
          )}

          {(stage === 'discard' || stage === 'pegging') && (
            <>
              <h2>Your hand</h2>
              {typeof myPos !== 'number' ? (
                <div style={{ opacity: 0.8 }}>You are not listed as a player in this snapshot.</div>
              ) : myHand.length === 0 ? (
                <div style={{ opacity: 0.8 }}>No cards visible (stage={stage}).</div>
              ) : (
                <div style={{ display: 'flex', flexWrap: 'wrap', gap: 8 }}>
                  {myHand.map((c) => {
                    const code = cardToCode(c)
                    const isSelected = stage === 'discard' && selected.has(code)
                    const isDiscard = stage === 'discard'
                    const isPegging = stage === 'pegging'

                    if (isDiscard) {
                      const selectionFull = selected.size >= discardCount
                      const canAdd = !isSelected && (!selectionFull || discardCount <= 0)
                      const canToggle = isSelected || canAdd
                      const disabled = moveBusy || !canToggle
                      return (
                        <div key={code} style={{ display: 'inline-block' }}>
                          <CardIcon
                            card={c}
                            selected={isSelected}
                            disabled={disabled}
                            onClick={() => {
                              if (disabled) return
                              setSelected((prev) => {
                                const next = new Set(prev)
                                if (next.has(code)) {
                                  next.delete(code)
                                } else {
                                  if (next.size >= discardCount) return next
                                  next.add(code)
                                }
                                return next
                              })
                            }}
                            title={
                              isSelected
                                ? 'Click to unselect'
                                : selected.size >= discardCount
                                  ? `Select exactly ${discardCount}; unselect a card first`
                                  : 'Click to select for discard'
                            }
                          />
                        </div>
                      )
                    }

                    // Pegging: play directly from the hand (single hand rendering; no duplicate "play" section).
                    const wouldExceed31 = isPegging && peggingTotal + cardValue15(c) > 31
                    const canPlay = isPegging && isMyTurn && !moveBusy && !loading && !wouldExceed31
                    const disabled = !canPlay
                    return (
                      <div key={code} style={{ display: 'inline-block' }}>
                        <CardIcon
                          card={c}
                          disabled={disabled}
                          onClick={canPlay ? () => submitMove({ type: 'play_card', card: code }) : undefined}
                          title={
                            !isPegging
                              ? undefined
                              : !isMyTurn
                                ? 'Not your turn'
                                : wouldExceed31
                                  ? `Would exceed 31 (total would be ${peggingTotal + cardValue15(c)})`
                                  : `Play ${code}`
                          }
                        />
                      </div>
                    )
                  })}
                </div>
              )}
            </>
          )}

          {stage === 'pegging' && myHand.length > 0 && (
            <div style={{ marginTop: 10, opacity: 0.85 }}>
              Tip: only playable cards are enabled; “Go” lights up only if you truly have no legal play.
            </div>
          )}

          <h2 style={{ marginTop: 16 }}>Actions</h2>
          {moveErr && <div style={{ color: 'crimson', marginTop: 8 }}>{moveErr}</div>}

          <div style={{ display: 'flex', flexWrap: 'wrap', gap: 10, marginTop: 8 }}>
            {stage === 'discard' && (
              <button
                type="button"
                disabled={moveBusy || selected.size !== discardCount}
                onClick={() => submitMove({ type: 'discard', cards: Array.from(selected) })}
                title={`Select exactly ${discardCount} card(s)`}
              >
                🗑 Discard ({selected.size}/{discardCount})
              </button>
            )}
          </div>

          {/* "Go" is shown as a card when it's truly needed (no legal play). */}
          {stage === 'pegging' && canGo && (
            <div style={{ marginTop: 10, display: 'flex', alignItems: 'center', gap: 10 }}>
              <ActionCard
                label="GO"
                accent="primary"
                onClick={() => submitMove({ type: 'go' })}
                title="No legal play — say GO"
              />
              <div style={{ opacity: 0.85 }}>No legal play — say GO</div>
            </div>
          )}

          {stage === 'pegging' && (
            <>
              {/* Play is performed by clicking a card directly in "Your hand" above. */}
            </>
          )}

          {/* (Actions + Play card moved above; keep below sections intact) */}

          {/*
            The rest of the component remains unchanged.
            (We left state/players/cut/finished UI above, and now render card icons for hand/actions.)
          */}
        </div>
      )}
    </div>
  )
}
