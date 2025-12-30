import { useEffect, useMemo, useState } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { api } from '../api/client'
import type { Card, GameSnapshot } from '../api/types'
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

    // Fetch an initial snapshot immediately so the UI isn't blank until a WS update arrives.
    void fetchSnapshot()

    ws.connect(`game:${gameId}`)
    const offOpen = ws.on('ws_open', () => setStatus('connected'))
    const offClose = ws.on('ws_close', () => setStatus('disconnected'))
    // WS broadcasts a public snapshot (hands hidden). Treat updates as an invalidation signal
    // and re-fetch the user-specific snapshot via HTTP so "your hand" stays populated.
    const offUpdate = ws.on('game_update', () => {
      void fetchSnapshot()
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
                  </li>
                )
              })}
          </ul>

          <h2>State</h2>
          <div>Stage: {stage}</div>
          <div>Dealer: {state?.dealer_index}</div>
          <div>Current: {state?.current_index}</div>
          <div>
            Pegging total: {state?.pegging_total} {stage === 'pegging' ? `(need ${Math.max(0, 31 - peggingTotal)})` : ''}
          </div>
          <div style={{ display: 'flex', gap: 10, alignItems: 'center', marginTop: 8 }}>
            <div style={{ minWidth: 40, fontWeight: 600 }}>Cut</div>
            {state?.cut ? (
              <CardIcon card={state.cut} disabled title={cardToCode(state.cut)} />
            ) : (
              <div style={{ opacity: 0.8 }}>{stage === 'discard' ? '(not cut yet)' : '(none)'}</div>
            )}
          </div>
          {stage === 'pegging' && (
            <div style={{ marginTop: 8 }}>
              <div style={{ fontWeight: 600 }}>Pegging sequence</div>
              <div style={{ opacity: 0.9 }}>
                {(state?.pegging_seq ?? []).length === 0
                  ? '(empty)'
                  : (state?.pegging_seq ?? []).map(cardToString).join(' , ')}
              </div>
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

          <h2>Your hand</h2>
          {typeof myPos !== 'number' ? (
            <div style={{ opacity: 0.8 }}>You are not listed as a player in this snapshot.</div>
          ) : myHand.length === 0 ? (
            <div style={{ opacity: 0.8 }}>No cards visible (stage={stage}).</div>
          ) : (
            <div style={{ display: 'flex', flexWrap: 'wrap', gap: 8 }}>
              {myHand.map((c) => {
                const code = cardToCode(c)
                const isSelected = selected.has(code)
                const canSelect = stage === 'discard'
                const selectionFull = selected.size >= discardCount
                const canAdd = !isSelected && (!selectionFull || discardCount <= 0)
                const canToggle = isSelected || canAdd
                return (
                  <div key={code} style={{ display: 'inline-block' }}>
                    <CardIcon
                      card={c}
                      selected={isSelected}
                      disabled={!canSelect || moveBusy || !canToggle}
                      onClick={() => {
                        if (!canSelect) return
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
                        stage === 'discard'
                          ? isSelected
                            ? 'Click to unselect'
                            : selected.size >= discardCount
                              ? `Select exactly ${discardCount}; unselect a card first`
                              : 'Click to select for discard'
                          : undefined
                      }
                    />
                  </div>
                )
              })}
            </div>
          )}

          {stage === 'pegging' && myHand.length > 0 && (
            <div style={{ marginTop: 10, opacity: 0.85 }}>
              Tip: only playable cards are enabled; “Go” lights up only if you truly have no legal play.
            </div>
          )}

          <h2 style={{ marginTop: 16 }}>Actions</h2>
          {moveErr && <div style={{ color: 'crimson', marginTop: 8 }}>{moveErr}</div>}

          <div style={{ display: 'flex', flexWrap: 'wrap', gap: 10, marginTop: 8 }}>
            <button
              type="button"
              disabled={moveBusy || stage !== 'discard' || selected.size !== discardCount}
              onClick={() => submitMove({ type: 'discard', cards: Array.from(selected) })}
              title={stage !== 'discard' ? 'Not in discard stage' : `Select exactly ${discardCount} card(s)`}
            >
              🗑 Discard ({selected.size}/{discardCount})
            </button>

            <button
              type="button"
              disabled={!canGo}
              onClick={() => submitMove({ type: 'go' })}
              title={
                stage !== 'pegging'
                  ? 'Not in pegging stage'
                  : !isMyTurn
                    ? 'Not your turn'
                    : hasLegalPeggingPlay
                      ? 'You have a legal play'
                      : 'No legal play; go'
              }
            >
              ⏭ Go
            </button>
          </div>

          {stage === 'pegging' && (
            <>
              <h3 style={{ marginTop: 12 }}>Play a card</h3>
              <div style={{ display: 'flex', flexWrap: 'wrap', gap: 8 }}>
                {myHand.map((c) => {
                  const code = cardToCode(c)
                  const wouldExceed31 = peggingTotal+cardValue15(c) > 31
                  const canPlay = isMyTurn && !moveBusy && !loading && !wouldExceed31
                  return (
                    <CardIcon
                      key={`play:${code}`}
                      card={c}
                      disabled={!canPlay}
                      onClick={() => submitMove({ type: 'play_card', card: code })}
                      title={
                        stage !== 'pegging'
                          ? 'Not in pegging stage'
                          : !isMyTurn
                            ? 'Not your turn'
                            : wouldExceed31
                              ? `Would exceed 31 (total would be ${peggingTotal + cardValue15(c)})`
                              : `Play ${code}`
                      }
                    />
                  )
                })}
              </div>
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
