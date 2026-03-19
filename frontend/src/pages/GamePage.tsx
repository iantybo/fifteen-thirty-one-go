import { useEffect, useMemo, useRef, useState } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { api } from '../api/client'
import type { GameMove, GameSnapshot } from '../api/types'
import { useAuth } from '../auth/auth'
import { WsClient } from '../ws/wsClient'
import type React from 'react'
import { PEG_BOARD_THEMES, usePegBoardTheme } from './game/pegBoardThemes'
import { cardToCode, cardToString, cardValue15 } from './game/cardUtils'
import { CardIcon, ActionCard, CardBack } from './game/CardComponents'
import { PlayerProfileCard, ScoreBreakdownLine } from './game/PlayerProfileCard'
import type { PlayerProfileState } from './game/PlayerProfileCard'
import { PegTrack } from './game/PegTrack'

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

function feltPanelStyle(): React.CSSProperties {
  return {
    borderRadius: 16,
    padding: 14,
    border: '1px solid rgba(15, 23, 42, 0.12)',
    background:
      'radial-gradient(1000px 500px at 30% 20%, rgba(255,255,255,0.10), rgba(255,255,255,0) 60%), radial-gradient(900px 500px at 70% 70%, rgba(255,255,255,0.06), rgba(255,255,255,0) 55%), linear-gradient(180deg, #0b5d3c, #084a30)',
    color: 'rgba(255,255,255,0.92)',
    boxShadow: '0 18px 40px rgba(2,6,23,0.25)',
    // Keep the playing surface footprint stable across stages (no jump when sections appear/disappear).
    // Fixed height + internal scrolling prevents hand size / stage changes from resizing the table.
    height: 760,
    display: 'grid',
    gridTemplateRows: '140px 1fr 140px 240px',
    gap: 12,
    overflow: 'hidden',
  }
}

// (BreakdownView removed; counting/finished UI now focuses on recap + readiness inside the table.)

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
  const [peggingCue, setPeggingCue] = useState<{ pos: number; kind: 'play'; delta?: number; card?: string } | null>(null)
  const [cutPulse, setCutPulse] = useState(false)
  const [profilesByUserId, setProfilesByUserId] = useState<Record<number, PlayerProfileState>>({})
  const [pegBoardCustomizeOpen, setPegBoardCustomizeOpen] = useState(false)
  const pegBoardCustomizeRef = useRef<HTMLDivElement>(null)
  const [pegBoardTheme, setPegBoardTheme] = usePegBoardTheme()
  const profileFetchRef = useRef<Set<number>>(new Set())
  const lastCueMoveIDRef = useRef<number | null>(null)

  useEffect(() => {
    if (!pegBoardCustomizeOpen) return
    function handleClick(e: MouseEvent) {
      if (pegBoardCustomizeRef.current?.contains(e.target as Node)) return
      setPegBoardCustomizeOpen(false)
    }
    document.addEventListener('mousedown', handleClick)
    return () => document.removeEventListener('mousedown', handleClick)
  }, [pegBoardCustomizeOpen])

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

  useEffect(() => {
    setProfilesByUserId({})
    profileFetchRef.current = new Set()
  }, [gameId])

  useEffect(() => {
    if (!snap) return
    let cancelled = false
    const pending = new Set<number>()
    for (const player of snap.players) {
      const userId = player.user_id
      if (profileFetchRef.current.has(userId)) continue
      profileFetchRef.current.add(userId)
      pending.add(userId)
      setProfilesByUserId((prev) => ({ ...prev, [userId]: { loading: true } }))
      api
        .getUserStats(userId)
        .then((stats) => {
          pending.delete(userId)
          if (cancelled) return
          setProfilesByUserId((prev) => ({ ...prev, [userId]: { loading: false, stats } }))
        })
        .catch((e: unknown) => {
          pending.delete(userId)
          if (cancelled) return
          const msg = e instanceof Error ? e.message : 'failed to load stats'
          setProfilesByUserId((prev) => ({ ...prev, [userId]: { loading: false, error: msg } }))
        })
    }
    return () => {
      cancelled = true
      for (const id of pending) {
        profileFetchRef.current.delete(id)
      }
    }
  }, [snap])

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
  const cutCode = state?.cut ? cardToCode(state.cut) : null
  const prevCutRef = useRef<string | null>(null)

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
    if (typeof latest.id === 'number' && lastCueMoveIDRef.current === latest.id) return
    if (typeof latest.id === 'number') lastCueMoveIDRef.current = latest.id
    const pPos = snap.players.find((p) => p.user_id === latest.player_id)?.position
    if (typeof pPos !== 'number') return

    if (latest.move_type === 'play_card') {
      setPeggingCue({ pos: pPos, kind: 'play', card: latest.card_played })
      const t = window.setTimeout(() => setPeggingCue(null), 1200)
      return () => window.clearTimeout(t)
    }
  }, [moves, snap])

  useEffect(() => {
    const prev = prevCutRef.current
    prevCutRef.current = cutCode
    // Animate when the cut first appears or changes.
    if (cutCode && cutCode !== prev) {
      // Avoid synchronous setState in effects (lint): schedule on next tick.
      const t0 = window.setTimeout(() => setCutPulse(true), 0)
      const t1 = window.setTimeout(() => setCutPulse(false), 360)
      return () => {
        window.clearTimeout(t0)
        window.clearTimeout(t1)
      }
    }
  }, [cutCode])

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
      <div style={{ display: 'flex', alignItems: 'center', gap: 10, marginTop: 6 }}>
        <div
          title={`Connection: ${status}`}
          style={{
            width: 10,
            height: 10,
            borderRadius: 999,
            background: status === 'connected' ? '#22c55e' : '#f97316',
            boxShadow: status === 'connected' ? '0 0 0 3px rgba(34,197,94,0.18)' : '0 0 0 3px rgba(249,115,22,0.18)',
          }}
        />
        <div style={{ opacity: 0.8 }}>{loading ? 'Loading…' : ''}</div>
      </div>
      {err && <div style={{ color: 'crimson', marginTop: 8 }}>{err}</div>}

      {!snap ? (
        <div style={{ marginTop: 16, opacity: 0.8 }}>{loading ? 'Loading…' : 'No snapshot yet.'}</div>
      ) : (
        <div style={{ marginTop: 16 }}>
          <div style={{ marginBottom: 16 }}>
            <div style={{ fontWeight: 900, marginBottom: 8, opacity: 0.95 }}>Player profiles</div>
            <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(220px, 1fr))', gap: 12 }}>
              {snap.players
                .slice()
                .sort((a, b) => a.position - b.position)
                .map((p) => (
                  <PlayerProfileCard
                    key={`profile:${p.user_id}`}
                    player={p}
                    profile={profilesByUserId[p.user_id]}
                    isDealer={p.position === state?.dealer_index}
                    isYou={p.user_id === user?.id}
                  />
                ))}
            </div>
          </div>
          {(() => {
            const sortedPlayers = snap.players.slice().sort((a, b) => a.position - b.position)
            const opp = sortedPlayers.find((p) => p.user_id !== user?.id) ?? sortedPlayers[0]
            const oppCount =
              typeof opp?.hand_count === 'number'
                ? opp.hand_count
                : opp && opp.user_id !== user?.id
                  ? safeCountHandJSON(opp.hand) ?? 0
                  : 0
            return (
              <div style={feltPanelStyle()}>
                {/* Opponent zone */}
                <div style={{ height: 140 }}>
                  <div style={{ fontWeight: 900 }}>
                    Opponent {opp?.is_bot ? '🤖' : ''}{' '}
                    <span style={{ opacity: 0.85, fontWeight: 700 }}>
                      {typeof opp?.position === 'number' ? `P${opp.position}` : ''}
                    </span>
                    {typeof opp?.position === 'number' && opp.position === state?.dealer_index ? (
                      <span title="Dealer" style={{ marginLeft: 6, color: '#facc15' }}>
                        👑
                      </span>
                    ) : null}
                  </div>
                  <div
                    style={{
                      marginTop: 10,
                      display: 'flex',
                      gap: 8,
                      alignItems: 'center',
                      overflowX: 'auto',
                      overflowY: 'hidden',
                      paddingBottom: 4,
                    }}
                  >
                    {Array.from({ length: oppCount }).map((_, i) => (
                      <CardBack key={`opp:${i}`} title={`${oppCount} cards`} />
                    ))}
                  </div>
                </div>

                {/* Board */}
                <div style={{ display: 'flex', alignItems: 'flex-start', gap: 14, flexWrap: 'wrap' }}>
                  <div style={{ flex: '1 1 520px', minWidth: 360, position: 'relative' }} ref={pegBoardCustomizeRef}>
                    <PegTrack
                      players={snap.players}
                      scores={state?.scores}
                      theme={pegBoardTheme}
                      headerAction={
                        <>
                          <button
                            type="button"
                            onClick={() => setPegBoardCustomizeOpen((o) => !o)}
                            style={{
                              fontSize: 12,
                              padding: '4px 10px',
                              borderRadius: 6,
                              border: '1px solid rgba(255,255,255,0.3)',
                              background: 'rgba(255,255,255,0.12)',
                              color: 'rgba(255,255,255,0.95)',
                              cursor: 'pointer',
                              fontWeight: 600,
                            }}
                          >
                            Customize
                          </button>
                          {pegBoardCustomizeOpen && (
                            <div
                              role="dialog"
                              aria-label="Peg board theme"
                              style={{
                                position: 'absolute',
                                top: '100%',
                                right: 0,
                                marginTop: 4,
                                padding: 8,
                                borderRadius: 8,
                                background: '#1e293b',
                                border: '1px solid #334155',
                                boxShadow: '0 10px 25px rgba(0,0,0,0.35)',
                                zIndex: 20,
                                minWidth: 140,
                              }}
                            >
                              <div style={{ fontSize: 11, fontWeight: 700, marginBottom: 6, color: 'rgba(255,255,255,0.7)' }}>
                                Theme
                              </div>
                              {PEG_BOARD_THEMES.map((t) => (
                                <button
                                  key={t.id}
                                  type="button"
                                  onClick={() => {
                                    setPegBoardTheme(t.id)
                                    setPegBoardCustomizeOpen(false)
                                  }}
                                  style={{
                                    display: 'block',
                                    width: '100%',
                                    textAlign: 'left',
                                    padding: '6px 8px',
                                    marginBottom: 2,
                                    borderRadius: 4,
                                    border: 'none',
                                    background: pegBoardTheme.id === t.id ? 'rgba(255,255,255,0.2)' : 'transparent',
                                    color: 'rgba(255,255,255,0.95)',
                                    cursor: 'pointer',
                                    fontSize: 13,
                                  }}
                                >
                                  {t.name}
                                </button>
                              ))}
                            </div>
                          )}
                        </>
                      }
                    />
                  </div>
                  <div style={{ flex: '0 0 auto' }}>
                    <div style={{ fontWeight: 900, marginBottom: 6, opacity: 0.9 }}>Cut</div>
                    {state?.cut ? (
                      <div
                        style={{
                          display: 'inline-block',
                          transform: cutPulse ? 'translateY(-3px) scale(1.06)' : 'translateY(0) scale(1)',
                          transition: 'transform 260ms ease, filter 260ms ease',
                          filter: cutPulse ? 'drop-shadow(0 10px 18px rgba(245,158,11,0.55))' : 'none',
                        }}
                      >
                        <CardIcon card={state.cut} disabled title={cardToCode(state.cut)} />
                      </div>
                    ) : (
                      <div style={{ opacity: 0.85 }}>{stage === 'discard' ? '(not cut yet)' : '(none)'}</div>
                    )}
                  </div>
                </div>

                {/* Reserve a fixed footer row so the surface doesn't jump between stages. */}
                <div style={{ height: 140 }}>
                  <div style={{ fontWeight: 900, marginBottom: 6, opacity: 0.9 }}>Played</div>
                  {stage === 'pegging' ? (
                    (state?.pegging_seq ?? []).length === 0 ? (
                      <div style={{ opacity: 0.85 }}>(none yet)</div>
                    ) : (
                      <div style={{ display: 'flex', gap: 8, overflowX: 'auto', overflowY: 'hidden', paddingBottom: 4 }}>
                        {(state?.pegging_seq ?? []).map((c, i) => (
                          <CardIcon key={`pegseq:${i}:${cardToCode(c)}`} card={c} disabled title={cardToCode(c)} />
                        ))}
                      </div>
                    )
                  ) : (
                    <div style={{ opacity: 0.75 }}>&nbsp;</div>
                  )}
                </div>

                <div style={{ height: 240, paddingTop: 10, borderTop: '1px solid rgba(255,255,255,0.16)' }}>
                  {stage === 'counting' ? (
                    <div style={{ height: '100%', overflow: 'auto', paddingRight: 6 }}>
                      <div style={{ fontWeight: 900, marginBottom: 6, opacity: 0.95 }}>Counting</div>
                      <div style={{ opacity: 0.9, marginBottom: 10 }}>
                        Scores have been applied. Review the breakdown, then ready up for the next hand.
                      </div>

                      <div style={{ display: 'flex', gap: 10, flexWrap: 'wrap', alignItems: 'center' }}>
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
                          {state?.ready_next_hand?.[myPos ?? -1] ? '✅ Ready (click to unready)' : '▶ Ready for next hand'}
                        </button>
                        {moveErr ? <div style={{ color: '#fecaca', fontWeight: 700 }}>{moveErr}</div> : null}
                      </div>

                      {Array.isArray(state?.kept_hands) && state.kept_hands.length > 0 && (
                        <div style={{ marginTop: 8 }}>
                          <div style={{ fontWeight: 800, opacity: 0.95 }}>Kept hands</div>
                          <div style={{ display: 'grid', gap: 10, marginTop: 8 }}>
                            {state.kept_hands.map((hand, idx) => {
                              const isDealer = idx === state?.dealer_index
                              const b = state?.count_summary?.hands?.[String(idx)]
                              return (
                                <div
                                  key={`count:hand:${idx}`}
                                  style={{
                                    padding: 10,
                                    border: '1px solid rgba(255,255,255,0.18)',
                                    borderRadius: 12,
                                    background: 'rgba(2,6,23,0.20)',
                                  }}
                                >
                                  <div style={{ display: 'flex', justifyContent: 'space-between', gap: 10, alignItems: 'baseline' }}>
                                    <div style={{ fontWeight: 900 }}>
                                      P{idx} {idx === myPos ? '(you)' : ''}{' '}
                                      {isDealer ? (
                                        <span title="Dealer" style={{ color: '#facc15' }}>
                                          👑
                                        </span>
                                      ) : null}
                                    </div>
                                    <div style={{ textAlign: 'right' }}>
                                      <ScoreBreakdownLine b={b} />
                                    </div>
                                  </div>
                                  <div style={{ marginTop: 8, display: 'flex', gap: 10, overflowX: 'auto', paddingBottom: 4 }}>
                                    {hand.map((c, j) => (
                                      <CardIcon key={`kh:${idx}:${j}:${cardToCode(c)}`} card={c} disabled title={cardToCode(c)} />
                                    ))}
                                  </div>
                                </div>
                              )
                            })}
                          </div>
                        </div>
                      )}

                      {Array.isArray(state?.crib) && state.crib.length > 0 && (
                        <div style={{ marginTop: 10 }}>
                          <div style={{ fontWeight: 800, opacity: 0.95 }}>Crib</div>
                          <div
                            style={{
                              marginTop: 8,
                              padding: 10,
                              border: '1px solid rgba(255,255,255,0.18)',
                              borderRadius: 12,
                              background: 'rgba(2,6,23,0.20)',
                            }}
                          >
                            <div style={{ display: 'flex', justifyContent: 'space-between', gap: 10, alignItems: 'baseline' }}>
                              <div style={{ fontWeight: 900, opacity: 0.95 }}>Dealer's crib</div>
                              <div style={{ textAlign: 'right' }}>
                                <ScoreBreakdownLine b={state?.count_summary?.crib} />
                              </div>
                            </div>
                            <div style={{ marginTop: 8, display: 'flex', gap: 10, overflowX: 'auto', paddingBottom: 4 }}>
                              {state.crib.map((c, i) => (
                                <CardIcon key={`crib:${i}:${cardToCode(c)}`} card={c} disabled title={cardToCode(c)} />
                              ))}
                            </div>
                          </div>
                        </div>
                      )}

                      {Array.isArray(state?.ready_next_hand) && state.ready_next_hand.length > 0 && (
                        <div
                          style={{
                            marginTop: 10,
                            padding: 10,
                            border: '1px solid rgba(255,255,255,0.18)',
                            borderRadius: 12,
                            background: 'rgba(2,6,23,0.20)',
                          }}
                        >
                          <div style={{ fontWeight: 800 }}>Next hand readiness</div>
                          <div style={{ marginTop: 8, display: 'grid', gap: 4 }}>
                            {snap.players
                              .slice()
                              .sort((a, b) => a.position - b.position)
                              .filter((p) => !p.is_bot)
                              .map((p) => {
                                const ready = !!state.ready_next_hand?.[p.position]
                                const isDealer = p.position === state?.dealer_index
                                return (
                                  <div key={`ready:${p.position}`} style={{ display: 'flex', justifyContent: 'space-between' }}>
                                    <div style={{ opacity: 0.9 }}>
                                      {playerLabel(p.position)}{' '}
                                      {isDealer ? (
                                        <span title="Dealer" style={{ color: '#facc15' }}>
                                          👑
                                        </span>
                                      ) : null}
                                    </div>
                                    <div style={{ fontWeight: 900, color: ready ? '#22c55e' : '#f87171' }}>
                                      {ready ? 'Ready' : 'Waiting'}
                                    </div>
                                  </div>
                                )
                              })}
                          </div>
                        </div>
                      )}
                    </div>
                  ) : stage === 'finished' ? (
                    <div style={{ height: '100%', overflow: 'auto', paddingRight: 6 }}>
                      <div style={{ fontWeight: 900, marginBottom: 6, opacity: 0.95 }}>Game over</div>
                      <div style={{ opacity: 0.9 }}>
                        This game is finished. You can quit and start a new one from the lobbies screen.
                      </div>
                      {state?.history && state.history.length > 0 && (
                        <details style={{ marginTop: 10 }}>
                          <summary style={{ fontWeight: 800, cursor: 'pointer' }}>Scoring recap</summary>
                          <div style={{ marginTop: 10, maxHeight: 180, overflowY: 'auto', paddingRight: 6 }}>
                            <div style={{ display: 'grid', gap: 10 }}>
                              {state.history
                                .slice()
                                .reverse()
                                .slice(0, 12)
                                .map((r) => (
                                  <div
                                    key={`round:${r.round}`}
                                    style={{
                                      padding: 10,
                                      border: '1px solid rgba(255,255,255,0.18)',
                                      borderRadius: 12,
                                      background: 'rgba(2,6,23,0.20)',
                                      overflowWrap: 'anywhere',
                                    }}
                                  >
                                    <div style={{ fontWeight: 800 }}>
                                      Round {r.round} — dealer P{r.dealer_index} {r.cut ? `— cut ${cardToString(r.cut)}` : ''}
                                    </div>
                                    <div style={{ marginTop: 6, opacity: 0.92 }}>
                                      {snap.players
                                        .slice()
                                        .sort((a, b) => a.position - b.position)
                                        .map((p) => {
                                          const before = r.scores_before?.[p.position]
                                          const after = r.scores_after?.[p.position]
                                          const delta =
                                            typeof before === 'number' && typeof after === 'number' ? after - before : undefined
                                          return (
                                            <div key={`r:${r.round}:p:${p.position}`}>
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
                        </details>
                      )}
                    </div>
                  ) : (
                    <>
                      <div style={{ fontWeight: 900, marginBottom: 6, opacity: 0.95 }}>
                        Your hand{' '}
                        {typeof myPos === 'number' && myPos === state?.dealer_index ? (
                          <span title="Dealer" style={{ color: '#facc15' }}>
                            👑
                          </span>
                        ) : null}
                      </div>
                      {stage === 'discard' || stage === 'pegging' ? (
                        typeof myPos !== 'number' ? (
                          <div style={{ opacity: 0.85 }}>You are not listed as a player in this snapshot.</div>
                        ) : myHand.length === 0 ? (
                          <div style={{ opacity: 0.85 }}>No cards visible.</div>
                        ) : (
                          <>
                            <div style={{ display: 'flex', gap: 10, alignItems: 'flex-start', overflowX: 'auto', overflowY: 'hidden', paddingBottom: 6 }}>
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

                                const wouldExceed31 = isPegging && peggingTotal + cardValue15(c) > 31
                                const canPlay = isPegging && isMyTurn && !moveBusy && !loading && !wouldExceed31
                                const disabled = !canPlay
                                return (
                                  <div key={code} style={{ display: 'inline-block' }}>
                                    <CardIcon
                                      card={c}
                                      disabled={disabled}
                                      muted={disabled}
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

                              {/* Actions are part of the hand (immediately after your cards). */}
                              {stage === 'discard' && (
                                <div style={{ display: 'inline-block', marginLeft: 6, flex: '0 0 auto' }}>
                                  <ActionCard
                                    label="🗑"
                                    accent="primary"
                                    disabled={moveBusy || selected.size !== discardCount}
                                    onClick={() => submitMove({ type: 'discard', cards: Array.from(selected) })}
                                    title={`Discard (${selected.size}/${discardCount})`}
                                  />
                                </div>
                              )}
                              {stage === 'pegging' && canGo && (
                                <div style={{ display: 'inline-block', marginLeft: 6, flex: '0 0 auto' }}>
                                  <ActionCard
                                    label="GO"
                                    accent="primary"
                                    disabled={moveBusy || loading}
                                    onClick={() => submitMove({ type: 'go' })}
                                    title="No legal play — say GO"
                                  />
                                </div>
                              )}
                            </div>

                            {moveErr ? (
                              <div style={{ color: '#fecaca', marginTop: 10, fontWeight: 700 }}>{moveErr}</div>
                            ) : null}
                          </>
                        )
                      ) : (
                        <div style={{ opacity: 0.75 }}>&nbsp;</div>
                      )}
                    </>
                  )}
                </div>
              </div>
            )
          })()}

          {/* Finished/counting controls now live inside the felt table. */}

          {/* No pegging tutorial text in the main UI; see the collapsed Info section below if needed. */}

          {/* No separate Actions section: in-game actions live next to your hand. */}

          {/* Collapsed info/log drawer (debug + explanations live here, not in the main game UI). */}
          <details style={{ marginTop: 18 }}>
            <summary style={{ cursor: 'pointer', opacity: 0.85 }}>Info / Log</summary>
            <div style={{ marginTop: 10, padding: 10, border: '1px solid #e2e8f0', borderRadius: 10, background: '#f8fafc' }}>
              <div style={{ display: 'grid', gap: 6, opacity: 0.9 }}>
                <div>
                  <b>Stage</b>: {stage}
                </div>
                {stage === 'pegging' ? (
                  <>
                    <div>
                      <b>Turn</b>: {isMyTurn ? 'you' : playerLabel(state?.current_index)}
                    </div>
                    <div>
                      <b>Total</b>: {peggingTotal}/31
                    </div>
                    <div>
                      <b>Hint</b>: {canGo ? 'No legal play; use the GO card in your hand.' : 'Click a card in your hand to play.'}
                    </div>
                  </>
                ) : null}
                {peggingCue ? (
                  <div>
                    <b>Last event</b>:{' '}
                    {peggingCue.kind === 'play' ? `${peggingCue.card ?? ''} played` : ''}
                  </div>
                ) : null}
              </div>
            </div>
          </details>

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
