import type { GameSnapshot } from '../../api/types'
import type { ScoreBreakdown, PlayerProfileState } from './playerProfileUtils'
import { playerInitials, profilePalette } from './playerProfileUtils'

export type { ScoreBreakdown, PlayerProfileState } from './playerProfileUtils'

export function ScoreBreakdownLine({ b }: { b: ScoreBreakdown | undefined }) {
  if (!b) return <span style={{ opacity: 0.8 }}>(no breakdown)</span>
  const parts: Array<[string, number]> = (
    [
      ['15s', b.fifteens],
      ['pairs', b.pairs],
      ['runs', b.runs],
      ['flush', b.flush],
      ['nobs', b.nobs],
    ] as Array<[string, number]>
  ).filter(([, v]) => v > 0)
  return (
    <span style={{ opacity: 0.92 }}>
      <span style={{ fontWeight: 900 }}>+{b.total}</span>
      {parts.length > 0 ? <span style={{ opacity: 0.9 }}> ({parts.map(([k, v]) => `${k} ${v}`).join(' · ')})</span> : null}
    </span>
  )
}

export function PlayerProfileCard({
  player,
  profile,
  isDealer,
  isYou,
}: {
  player: GameSnapshot['players'][number]
  profile: PlayerProfileState | undefined
  isDealer: boolean
  isYou: boolean
}) {
  const palette = profilePalette(player.position)
  const stats = profile?.stats
  const gamesPlayed = stats?.games_played ?? 0
  const wins = stats?.games_won ?? 0
  const losses = Math.max(0, gamesPlayed - wins)
  const winRate = gamesPlayed > 0 ? Math.round((wins / gamesPlayed) * 100) : null
  const rank =
    gamesPlayed >= 20 ? (winRate !== null && winRate >= 60 ? 'Ace' : winRate !== null && winRate >= 45 ? 'Pro' : 'Regular') : 'Rookie'

  return (
    <div
      style={{
        padding: 12,
        borderRadius: 16,
        border: `1px solid ${palette.border}`,
        background: `linear-gradient(145deg, ${palette.bg1}, ${palette.bg2})`,
        boxShadow: '0 12px 24px rgba(15, 23, 42, 0.15)',
        position: 'relative',
        overflow: 'hidden',
        minHeight: 150,
      }}
    >
      <div
        style={{
          position: 'absolute',
          right: -20,
          top: -20,
          width: 80,
          height: 80,
          borderRadius: 999,
          background: 'rgba(255,255,255,0.35)',
        }}
      />
      <div style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
        <div
          style={{
            width: 44,
            height: 44,
            borderRadius: 999,
            background: 'rgba(15,23,42,0.88)',
            color: '#f8fafc',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            fontWeight: 900,
            letterSpacing: 0.6,
            boxShadow: '0 6px 14px rgba(15,23,42,0.22)',
            textTransform: 'uppercase',
          }}
        >
          {playerInitials(player.username)}
        </div>
        <div style={{ flex: 1, minWidth: 0 }}>
          <div style={{ fontWeight: 900, fontSize: 16, color: '#0f172a', overflow: 'hidden', textOverflow: 'ellipsis' }}>
            {player.username} {player.is_bot ? '🤖' : ''}
          </div>
          <div style={{ display: 'flex', gap: 6, flexWrap: 'wrap', marginTop: 4 }}>
            <span
              style={{
                fontSize: 11,
                fontWeight: 800,
                letterSpacing: 0.6,
                textTransform: 'uppercase',
                padding: '2px 6px',
                borderRadius: 999,
                background: 'rgba(15,23,42,0.08)',
                color: '#0f172a',
              }}
            >
              P{player.position}
            </span>
            {isYou ? (
              <span
                style={{
                  fontSize: 11,
                  fontWeight: 800,
                  letterSpacing: 0.6,
                  textTransform: 'uppercase',
                  padding: '2px 6px',
                  borderRadius: 999,
                  background: 'rgba(14,116,144,0.18)',
                  color: '#0e7490',
                }}
              >
                You
              </span>
            ) : null}
            {isDealer ? (
              <span
                style={{
                  fontSize: 11,
                  fontWeight: 800,
                  letterSpacing: 0.6,
                  textTransform: 'uppercase',
                  padding: '2px 6px',
                  borderRadius: 999,
                  background: 'rgba(250,204,21,0.25)',
                  color: '#92400e',
                }}
              >
                Dealer
              </span>
            ) : null}
            <span
              style={{
                fontSize: 11,
                fontWeight: 800,
                letterSpacing: 0.6,
                textTransform: 'uppercase',
                padding: '2px 6px',
                borderRadius: 999,
                background: 'rgba(255,255,255,0.75)',
                color: palette.accent,
              }}
            >
              {rank}
            </span>
          </div>
        </div>
      </div>
      <div style={{ marginTop: 12 }}>
        {profile?.loading ? (
          <div style={{ fontWeight: 700, opacity: 0.75 }}>Loading stats...</div>
        ) : profile?.error ? (
          <div style={{ fontWeight: 700, color: '#b91c1c' }}>Stats unavailable</div>
        ) : (
          <div style={{ display: 'grid', gridTemplateColumns: 'repeat(3, minmax(0, 1fr))', gap: 6 }}>
            <div style={{ background: 'rgba(255,255,255,0.7)', borderRadius: 10, padding: '6px 8px', textAlign: 'center' }}>
              <div style={{ fontSize: 11, fontWeight: 800, letterSpacing: 0.5, color: '#475569' }}>Wins</div>
              <div style={{ fontSize: 16, fontWeight: 900 }}>{wins}</div>
            </div>
            <div style={{ background: 'rgba(255,255,255,0.7)', borderRadius: 10, padding: '6px 8px', textAlign: 'center' }}>
              <div style={{ fontSize: 11, fontWeight: 800, letterSpacing: 0.5, color: '#475569' }}>Losses</div>
              <div style={{ fontSize: 16, fontWeight: 900 }}>{losses}</div>
            </div>
            <div style={{ background: 'rgba(255,255,255,0.7)', borderRadius: 10, padding: '6px 8px', textAlign: 'center' }}>
              <div style={{ fontSize: 11, fontWeight: 800, letterSpacing: 0.5, color: '#475569' }}>Win %</div>
              <div style={{ fontSize: 16, fontWeight: 900 }}>{winRate === null ? '—' : `${winRate}%`}</div>
            </div>
          </div>
        )}
      </div>
    </div>
  )
}
