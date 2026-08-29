import { useCallback, useEffect, useMemo, useState } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { api } from '../api/client'
import type { Card, GameMove, ReplayPlayer, ReplayResponse, RoundSummary } from '../api/types'
import type React from 'react'

const CARD_W = 70
const CARD_H = 98
const CARD_R = 12

function rankLabel(rank: number): string {
  return rank === 1 ? 'A' : rank === 11 ? 'J' : rank === 12 ? 'Q' : rank === 13 ? 'K' : String(rank)
}

function suitSymbol(suit: Card['suit']): string {
  switch (suit) {
    case 'S': return '\u2660'
    case 'H': return '\u2665'
    case 'D': return '\u2666'
    case 'C': return '\u2663'
  }
}

function suitColor(suit: Card['suit']): string {
  return suit === 'H' || suit === 'D' ? '#dc2626' : '#0f172a'
}

function cardToString(c: Card): string {
  return `${rankLabel(c.rank)}${suitSymbol(c.suit)}`
}

function cardValue15(c: Card): number {
  return c.rank >= 10 ? 10 : c.rank
}

function parseCardCode(code: string): Card | null {
  if (!code || code.length < 2) return null
  const suitChar = code[code.length - 1]
  const rankStr = code.slice(0, -1)
  let rank: number
  switch (rankStr) {
    case 'A': rank = 1; break
    case 'J': rank = 11; break
    case 'Q': rank = 12; break
    case 'K': rank = 13; break
    default: rank = parseInt(rankStr, 10); break
  }
  if (isNaN(rank) || rank < 1 || rank > 13) return null
  if (!['S', 'H', 'D', 'C'].includes(suitChar)) return null
  return { rank, suit: suitChar as Card['suit'] }
}

function ReplayCardIcon({ card, highlight }: { card: Card; highlight?: boolean }) {
  const rank = rankLabel(card.rank)
  const suit = suitSymbol(card.suit)
  const color = suitColor(card.suit)
  return (
    <div
      style={{
        width: CARD_W,
        height: CARD_H,
        borderRadius: CARD_R,
        border: highlight ? '2px solid #facc15' : '1px solid #cbd5e1',
        background: '#ffffff',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        boxShadow: highlight ? '0 0 8px rgba(250,204,21,0.5)' : undefined,
        flexShrink: 0,
      }}
    >
      <div style={{ width: '100%', height: '100%', position: 'relative', borderRadius: CARD_R }}>
        <div style={{ position: 'absolute', top: 7, left: 8, fontSize: 14, fontWeight: 700, lineHeight: 1, color }}>{rank}</div>
        <div style={{ position: 'absolute', top: 30, left: 0, right: 0, textAlign: 'center', fontSize: 36, lineHeight: 1, color }}>{suit}</div>
        <div style={{ position: 'absolute', bottom: 7, right: 8, fontSize: 14, fontWeight: 700, lineHeight: 1, transform: 'rotate(180deg)', color }}>{rank}</div>
      </div>
    </div>
  )
}

function CardBack() {
  return (
    <div
      style={{
        width: CARD_W,
        height: CARD_H,
        borderRadius: CARD_R,
        border: '1px solid #cbd5e1',
        background: 'repeating-linear-gradient(45deg, #1d4ed8 0px, #1d4ed8 6px, #2563eb 6px, #2563eb 12px)',
        flexShrink: 0,
      }}
    />
  )
}

// ---------- Replay frame computation ----------

type ReplayFrame = {
  round: number
  stage: 'discard' | 'pegging' | 'counting' | 'initial'
  scores: number[]
  peggingTotal: number
  peggingSeq: Card[]
  discardsDone: boolean[]
  lastMove: GameMove | null
  annotation: string
  roundSummary: RoundSummary | null
}

function buildFrames(moves: GameMove[], players: ReplayPlayer[], rounds: RoundSummary[]): ReplayFrame[] {
  const n = players.length
  const frames: ReplayFrame[] = []

  // Initial frame
  frames.push({
    round: 1,
    stage: 'initial',
    scores: new Array(n).fill(0),
    peggingTotal: 0,
    peggingSeq: [],
    discardsDone: new Array(n).fill(false),
    lastMove: null,
    annotation: 'Game starts',
    roundSummary: null,
  })

  let round = 1
  let stage: ReplayFrame['stage'] = 'discard'
  const scores = new Array(n).fill(0)
  let peggingTotal = 0
  let peggingSeq: Card[] = []
  let discardsDone = new Array(n).fill(false)
  let passCount = 0

  const playerIndex = (userId: number) => {
    const p = players.find((p) => p.user_id === userId)
    return p ? p.position : -1
  }

  const playerName = (userId: number) => {
    const p = players.find((p) => p.user_id === userId)
    return p?.username ?? `Player ${userId}`
  }

  const roundSummaryMap = new Map<number, RoundSummary>()
  for (const r of rounds) {
    roundSummaryMap.set(r.round, r)
  }

  for (const move of moves) {
    const name = playerName(move.player_id)
    const pIdx = playerIndex(move.player_id)
    const verified = move.score_verified ?? 0

    if (move.move_type === 'discard') {
      stage = 'discard'
      if (pIdx >= 0 && pIdx < n) {
        discardsDone[pIdx] = true
      }
      const allDone = discardsDone.every(Boolean)
      frames.push({
        round,
        stage: 'discard',
        scores: [...scores],
        peggingTotal,
        peggingSeq: [...peggingSeq],
        discardsDone: [...discardsDone],
        lastMove: move,
        annotation: `${name} discarded${allDone ? ' (all discards done)' : ''}`,
        roundSummary: null,
      })
      if (allDone) {
        stage = 'pegging'
        peggingTotal = 0
        peggingSeq = []
        passCount = 0
      }
    } else if (move.move_type === 'play_card') {
      stage = 'pegging'
      const card = move.card_played ? parseCardCode(move.card_played) : null
      if (card) {
        peggingSeq.push(card)
        peggingTotal += cardValue15(card)
      }
      if (pIdx >= 0 && pIdx < n) {
        scores[pIdx] += verified
      }
      passCount = 0

      // Reset pegging sequence at 31
      if (peggingTotal >= 31) {
        const annotation = `${name} played ${card ? cardToString(card) : move.card_played ?? '?'}${verified > 0 ? ` for ${verified} pts` : ''} (31!)`
        frames.push({
          round,
          stage: 'pegging',
          scores: [...scores],
          peggingTotal,
          peggingSeq: [...peggingSeq],
          discardsDone: [...discardsDone],
          lastMove: move,
          annotation,
          roundSummary: null,
        })
        peggingTotal = 0
        peggingSeq = []
        passCount = 0
      } else {
        frames.push({
          round,
          stage: 'pegging',
          scores: [...scores],
          peggingTotal,
          peggingSeq: [...peggingSeq],
          discardsDone: [...discardsDone],
          lastMove: move,
          annotation: `${name} played ${card ? cardToString(card) : move.card_played ?? '?'}${verified > 0 ? ` for ${verified} pts` : ''}`,
          roundSummary: null,
        })
      }
    } else if (move.move_type === 'go') {
      stage = 'pegging'
      passCount++
      if (pIdx >= 0 && pIdx < n) {
        scores[pIdx] += verified
      }
      const annotation = `${name} says "Go"${verified > 0 ? ` (+${verified} last card)` : ''}`
      frames.push({
        round,
        stage: 'pegging',
        scores: [...scores],
        peggingTotal,
        peggingSeq: [...peggingSeq],
        discardsDone: [...discardsDone],
        lastMove: move,
        annotation,
        roundSummary: null,
      })
      // If all players have passed, reset sequence
      if (passCount >= n) {
        peggingTotal = 0
        peggingSeq = []
        passCount = 0
      }
    } else if (move.move_type.startsWith('count_')) {
      stage = 'counting'
      if (pIdx >= 0 && pIdx < n) {
        scores[pIdx] += verified
      }
      const kind = move.move_type.includes('crib') ? 'crib' : 'hand'
      const claimed = move.score_claimed ?? 0
      const mismatch = claimed !== verified ? ` (claimed ${claimed}, actual ${verified})` : ''
      frames.push({
        round,
        stage: 'counting',
        scores: [...scores],
        peggingTotal: 0,
        peggingSeq: [],
        discardsDone: [...discardsDone],
        lastMove: move,
        annotation: `${name} counts ${kind} for ${verified}${mismatch}`,
        roundSummary: roundSummaryMap.get(round) ?? null,
      })
    } else if (move.move_type === 'deal' || move.move_type === 'next_hand') {
      // Transition to new round
      round++
      stage = 'discard'
      peggingTotal = 0
      peggingSeq = []
      discardsDone = new Array(n).fill(false)
      passCount = 0
      frames.push({
        round,
        stage: 'discard',
        scores: [...scores],
        peggingTotal: 0,
        peggingSeq: [],
        discardsDone: [...discardsDone],
        lastMove: move,
        annotation: `Round ${round} begins`,
        roundSummary: null,
      })
    } else {
      // Unknown move type — still show it
      frames.push({
        round,
        stage,
        scores: [...scores],
        peggingTotal,
        peggingSeq: [...peggingSeq],
        discardsDone: [...discardsDone],
        lastMove: move,
        annotation: `${name}: ${move.move_type}`,
        roundSummary: null,
      })
    }
  }

  return frames
}

// ---------- Styles ----------

const feltBg: React.CSSProperties = {
  borderRadius: 16,
  padding: 14,
  border: '1px solid rgba(15, 23, 42, 0.12)',
  background:
    'radial-gradient(1000px 500px at 30% 20%, rgba(255,255,255,0.10), rgba(255,255,255,0) 60%), radial-gradient(900px 500px at 70% 70%, rgba(255,255,255,0.06), rgba(255,255,255,0) 55%), linear-gradient(180deg, #0b5d3c, #084a30)',
  color: 'rgba(255,255,255,0.92)',
  boxShadow: '0 18px 40px rgba(2,6,23,0.25)',
}

const panelStyle: React.CSSProperties = {
  padding: 12,
  borderRadius: 12,
  border: '1px solid rgba(255,255,255,0.15)',
  background: 'rgba(2,6,23,0.2)',
}

const SPEEDS = [
  { label: '0.5x', ms: 2000 },
  { label: '1x', ms: 1000 },
  { label: '2x', ms: 500 },
  { label: '4x', ms: 250 },
]

export function GameReplayPage() {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const gameId = Number(id)

  const [data, setData] = useState<ReplayResponse | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [loading, setLoading] = useState(true)

  const [stepIndex, setStepIndex] = useState(0)
  const [playing, setPlaying] = useState(false)
  const [speedIdx, setSpeedIdx] = useState(1) // default 1x

  useEffect(() => {
    if (!gameId || isNaN(gameId)) {
      setError('Invalid game ID')
      setLoading(false)
      return
    }
    let cancelled = false
    api.getGameReplay(gameId).then((res) => {
      if (cancelled) return
      setData(res)
      setLoading(false)
    }).catch((err) => {
      if (cancelled) return
      setError(err?.message ?? 'Failed to load replay')
      setLoading(false)
    })
    return () => { cancelled = true }
  }, [gameId])

  const frames = useMemo(() => {
    if (!data) return []
    return buildFrames(data.moves, data.players, data.rounds)
  }, [data])

  const frame = frames[stepIndex] ?? null

  // Auto-play
  useEffect(() => {
    if (!playing || frames.length === 0) return
    const interval = setInterval(() => {
      setStepIndex((prev) => {
        if (prev >= frames.length - 1) {
          setPlaying(false)
          return prev
        }
        return prev + 1
      })
    }, SPEEDS[speedIdx].ms)
    return () => clearInterval(interval)
  }, [playing, speedIdx, frames.length])

  // Keyboard controls
  const handleKey = useCallback(
    (e: KeyboardEvent) => {
      if (e.key === 'ArrowRight') {
        e.preventDefault()
        setStepIndex((prev) => Math.min(prev + 1, frames.length - 1))
      } else if (e.key === 'ArrowLeft') {
        e.preventDefault()
        setStepIndex((prev) => Math.max(prev - 1, 0))
      } else if (e.key === ' ') {
        e.preventDefault()
        setPlaying((p) => !p)
      }
    },
    [frames.length],
  )

  useEffect(() => {
    window.addEventListener('keydown', handleKey)
    return () => window.removeEventListener('keydown', handleKey)
  }, [handleKey])

  if (loading) {
    return (
      <div style={{ padding: 40, textAlign: 'center', color: '#64748b' }}>Loading replay...</div>
    )
  }

  if (error || !data) {
    return (
      <div style={{ padding: 40, textAlign: 'center' }}>
        <div style={{ color: '#dc2626', fontWeight: 700, marginBottom: 12 }}>{error ?? 'Failed to load'}</div>
        <button type="button" onClick={() => navigate(-1)} style={{ padding: '8px 16px', borderRadius: 8, border: '1px solid #cbd5e1', background: '#fff', cursor: 'pointer' }}>
          Go back
        </button>
      </div>
    )
  }

  const { players } = data
  const finalScores = frames.length > 0 ? frames[frames.length - 1].scores : players.map(() => 0)

  return (
    <div style={{ maxWidth: 900, margin: '0 auto', padding: '24px 16px' }}>
      {/* Header */}
      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: 16 }}>
        <div>
          <h1 style={{ margin: 0, fontSize: 22, fontWeight: 900 }}>Game Replay</h1>
          <div style={{ fontSize: 13, color: '#64748b', marginTop: 2 }}>
            {data.game.created_at ? new Date(data.game.created_at).toLocaleDateString() : ''} &middot; Game #{data.game.id}
          </div>
        </div>
        <button
          type="button"
          onClick={() => navigate(`/games/${gameId}`)}
          style={{
            padding: '6px 14px',
            borderRadius: 8,
            border: '1px solid #cbd5e1',
            background: '#fff',
            cursor: 'pointer',
            fontWeight: 600,
            fontSize: 13,
          }}
        >
          Back to game
        </button>
      </div>

      {/* Players bar */}
      <div style={{ display: 'flex', gap: 12, marginBottom: 16, flexWrap: 'wrap' }}>
        {players.map((p) => (
          <div
            key={p.user_id}
            style={{
              padding: '8px 14px',
              borderRadius: 10,
              background: '#f1f5f9',
              border: '1px solid #e2e8f0',
              fontWeight: 700,
              fontSize: 14,
            }}
          >
            <span style={{ opacity: 0.6, marginRight: 6 }}>P{p.position}</span>
            {p.username}
            {p.is_bot ? ' (Bot)' : ''}
            <span style={{ marginLeft: 8, fontWeight: 900, color: '#0f172a' }}>{finalScores[p.position]}</span>
          </div>
        ))}
      </div>

      {/* Board area */}
      <div style={feltBg}>
        {frame ? (
          <div style={{ minHeight: 260 }}>
            {/* Round & stage */}
            <div style={{ display: 'flex', gap: 14, alignItems: 'center', marginBottom: 12 }}>
              <span style={{ fontWeight: 900, fontSize: 15 }}>Round {frame.round}</span>
              <span
                style={{
                  fontSize: 11,
                  fontWeight: 800,
                  letterSpacing: 0.6,
                  textTransform: 'uppercase',
                  padding: '2px 8px',
                  borderRadius: 999,
                  background: 'rgba(255,255,255,0.15)',
                }}
              >
                {frame.stage}
              </span>
            </div>

            {/* Scores */}
            <div style={{ ...panelStyle, marginBottom: 12, display: 'flex', gap: 20 }}>
              {players.map((p) => (
                <div key={p.user_id} style={{ fontWeight: 700 }}>
                  <span style={{ opacity: 0.7 }}>{p.username}: </span>
                  <span style={{ fontWeight: 900, fontSize: 18 }}>{frame.scores[p.position]}</span>
                </div>
              ))}
            </div>

            {/* Pegging area */}
            {frame.stage === 'pegging' && (
              <div style={{ ...panelStyle, marginBottom: 12 }}>
                <div style={{ fontWeight: 800, marginBottom: 8, fontSize: 13, opacity: 0.8 }}>
                  Pegging &middot; Total: <span style={{ fontWeight: 900, fontSize: 16 }}>{frame.peggingTotal}</span>
                </div>
                <div style={{ display: 'flex', gap: 6, flexWrap: 'wrap' }}>
                  {frame.peggingSeq.length === 0 ? (
                    <span style={{ opacity: 0.5 }}>No cards played yet</span>
                  ) : (
                    frame.peggingSeq.map((c, i) => (
                      <ReplayCardIcon key={`peg-${i}`} card={c} highlight={i === frame.peggingSeq.length - 1} />
                    ))
                  )}
                </div>
              </div>
            )}

            {/* Counting summary */}
            {frame.stage === 'counting' && frame.roundSummary && (
              <div style={{ ...panelStyle, marginBottom: 12 }}>
                <div style={{ fontWeight: 800, marginBottom: 6, fontSize: 13, opacity: 0.8 }}>Counting breakdown</div>
                {frame.roundSummary.cut && (
                  <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginBottom: 8 }}>
                    <span style={{ opacity: 0.7 }}>Cut:</span>
                    <ReplayCardIcon card={frame.roundSummary.cut} />
                  </div>
                )}
                {frame.roundSummary.hands &&
                  Object.entries(frame.roundSummary.hands).map(([posStr, bd]) => {
                    const pos = parseInt(posStr, 10)
                    const p = players.find((p) => p.position === pos)
                    return (
                      <div key={posStr} style={{ marginBottom: 4 }}>
                        <span style={{ fontWeight: 700 }}>{p?.username ?? `P${pos}`}</span>: {bd.total} pts
                        {bd.fifteens > 0 ? ` (15s: ${bd.fifteens})` : ''}
                        {bd.pairs > 0 ? ` (pairs: ${bd.pairs})` : ''}
                        {bd.runs > 0 ? ` (runs: ${bd.runs})` : ''}
                        {bd.flush > 0 ? ` (flush: ${bd.flush})` : ''}
                        {bd.nobs > 0 ? ` (nobs: ${bd.nobs})` : ''}
                      </div>
                    )
                  })}
                {frame.roundSummary.crib && (
                  <div style={{ marginTop: 4 }}>
                    <span style={{ fontWeight: 700 }}>Crib</span>: {frame.roundSummary.crib.total} pts
                  </div>
                )}
              </div>
            )}

            {/* Annotation */}
            <div
              style={{
                padding: '10px 14px',
                borderRadius: 10,
                background: 'rgba(0,0,0,0.25)',
                fontWeight: 700,
                fontSize: 15,
                minHeight: 40,
                display: 'flex',
                alignItems: 'center',
              }}
            >
              {frame.annotation}
            </div>
          </div>
        ) : (
          <div style={{ opacity: 0.6 }}>No replay data</div>
        )}
      </div>

      {/* Controls */}
      <div
        style={{
          marginTop: 16,
          padding: '12px 16px',
          borderRadius: 12,
          background: '#f8fafc',
          border: '1px solid #e2e8f0',
          display: 'flex',
          alignItems: 'center',
          gap: 12,
          flexWrap: 'wrap',
        }}
      >
        {/* Prev */}
        <button
          type="button"
          disabled={stepIndex <= 0}
          onClick={() => setStepIndex((i) => Math.max(i - 1, 0))}
          style={{
            padding: '6px 12px',
            borderRadius: 6,
            border: '1px solid #cbd5e1',
            background: '#fff',
            cursor: stepIndex <= 0 ? 'not-allowed' : 'pointer',
            fontWeight: 700,
            opacity: stepIndex <= 0 ? 0.5 : 1,
          }}
        >
          Prev
        </button>

        {/* Play/Pause */}
        <button
          type="button"
          onClick={() => setPlaying((p) => !p)}
          style={{
            padding: '6px 16px',
            borderRadius: 6,
            border: 'none',
            background: playing ? '#dc2626' : '#2563eb',
            color: '#fff',
            fontWeight: 700,
            cursor: 'pointer',
          }}
        >
          {playing ? 'Pause' : 'Play'}
        </button>

        {/* Next */}
        <button
          type="button"
          disabled={stepIndex >= frames.length - 1}
          onClick={() => setStepIndex((i) => Math.min(i + 1, frames.length - 1))}
          style={{
            padding: '6px 12px',
            borderRadius: 6,
            border: '1px solid #cbd5e1',
            background: '#fff',
            cursor: stepIndex >= frames.length - 1 ? 'not-allowed' : 'pointer',
            fontWeight: 700,
            opacity: stepIndex >= frames.length - 1 ? 0.5 : 1,
          }}
        >
          Next
        </button>

        {/* Divider */}
        <div style={{ width: 1, height: 24, background: '#e2e8f0' }} />

        {/* Speed */}
        <div style={{ display: 'flex', gap: 4 }}>
          {SPEEDS.map((s, i) => (
            <button
              key={s.label}
              type="button"
              onClick={() => setSpeedIdx(i)}
              style={{
                padding: '4px 8px',
                borderRadius: 4,
                border: '1px solid #cbd5e1',
                background: i === speedIdx ? '#0f172a' : '#fff',
                color: i === speedIdx ? '#fff' : '#0f172a',
                fontWeight: 700,
                fontSize: 12,
                cursor: 'pointer',
              }}
            >
              {s.label}
            </button>
          ))}
        </div>

        {/* Divider */}
        <div style={{ width: 1, height: 24, background: '#e2e8f0' }} />

        {/* Step counter */}
        <div style={{ fontSize: 13, color: '#64748b', fontWeight: 600 }}>
          Move {stepIndex} / {frames.length - 1}
        </div>

        {/* Scrubber */}
        <input
          type="range"
          min={0}
          max={frames.length - 1}
          value={stepIndex}
          onChange={(e) => setStepIndex(Number(e.target.value))}
          style={{ flex: 1, minWidth: 100 }}
        />
      </div>

      {/* Keyboard hint */}
      <div style={{ marginTop: 8, fontSize: 12, color: '#94a3b8', textAlign: 'center' }}>
        Keyboard: Left/Right arrows to step, Space to play/pause
      </div>
    </div>
  )
}
