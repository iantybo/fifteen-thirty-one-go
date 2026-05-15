import { useCallback, useEffect, useMemo, useState } from 'react'
import type { CSSProperties } from 'react'
import { api } from '../api/client'
import type { Achievement, AchievementsSnapshot, UnlockedAchievement } from '../api/types'
import { useAuth } from '../auth/auth'

// AchievementsPage renders unlocked and locked achievements for the current
// user. Includes a "Re-evaluate" button that recomputes the achievement set
// from current stats — useful right after a finished game.

const pageStyle: CSSProperties = { padding: '24px', maxWidth: 980, margin: '0 auto' }
const headerStyle: CSSProperties = { fontSize: 28, fontWeight: 800, margin: '0 0 8px 0' }
const subtitleStyle: CSSProperties = { color: '#6b7280', marginBottom: 24 }

const toolbarStyle: CSSProperties = {
  display: 'flex',
  alignItems: 'center',
  gap: 12,
  marginBottom: 16,
}
const primaryButtonStyle: CSSProperties = {
  padding: '8px 14px',
  background: '#4a9eff',
  color: '#fff',
  border: 'none',
  borderRadius: 6,
  cursor: 'pointer',
  fontWeight: 600,
}
const sectionStyle: CSSProperties = {
  background: '#fff',
  border: '1px solid #e5e7eb',
  borderRadius: 12,
  padding: 20,
  marginBottom: 20,
  boxShadow: '0 1px 2px rgba(0,0,0,0.04)',
}
const sectionTitleStyle: CSSProperties = { fontSize: 18, fontWeight: 700, marginBottom: 12 }
const subtleStyle: CSSProperties = { color: '#6b7280', fontSize: 13 }
const gridStyle: CSSProperties = {
  display: 'grid',
  gridTemplateColumns: 'repeat(auto-fill, minmax(220px, 1fr))',
  gap: 12,
}
const tierBronze: CSSProperties = { background: 'linear-gradient(135deg,#fde68a,#fbbf24)', color: '#7c2d12' }
const tierSilver: CSSProperties = { background: 'linear-gradient(135deg,#e5e7eb,#9ca3af)', color: '#1f2937' }
const tierGold: CSSProperties = { background: 'linear-gradient(135deg,#fef08a,#eab308)', color: '#713f12' }
const tierPlatinum: CSSProperties = { background: 'linear-gradient(135deg,#e0e7ff,#a5b4fc)', color: '#312e81' }

function tierStyle(tier: string): CSSProperties {
  switch (tier) {
    case 'silver':
      return tierSilver
    case 'gold':
      return tierGold
    case 'platinum':
      return tierPlatinum
    case 'bronze':
    default:
      return tierBronze
  }
}

function cardStyle(unlocked: boolean): CSSProperties {
  return {
    padding: 16,
    borderRadius: 12,
    border: '1px solid #e5e7eb',
    background: unlocked ? '#ffffff' : '#f9fafb',
    opacity: unlocked ? 1 : 0.7,
    position: 'relative',
    display: 'flex',
    flexDirection: 'column',
    gap: 8,
    minHeight: 130,
  }
}

const iconBadgeStyle: CSSProperties = {
  width: 44,
  height: 44,
  borderRadius: '50%',
  display: 'flex',
  alignItems: 'center',
  justifyContent: 'center',
  fontSize: 24,
  fontWeight: 800,
}

const titleStyle: CSSProperties = { fontSize: 16, fontWeight: 700 }
const descStyle: CSSProperties = { color: '#374151', fontSize: 13 }
const metaStyle: CSSProperties = { color: '#6b7280', fontSize: 12 }
const tierChipStyle: CSSProperties = {
  display: 'inline-block',
  padding: '2px 8px',
  borderRadius: 999,
  fontSize: 11,
  fontWeight: 700,
  textTransform: 'uppercase',
  letterSpacing: 0.4,
}

const errorStyle: CSSProperties = { color: '#b91c1c', marginBottom: 12 }
const okStyle: CSSProperties = { color: '#047857', marginBottom: 12 }

function formatDate(ts: string): string {
  const d = new Date(ts)
  if (Number.isNaN(d.getTime())) return ts
  return d.toLocaleDateString(undefined, { year: 'numeric', month: 'short', day: 'numeric' })
}

type UnlockedCardProps = { ach: UnlockedAchievement }
function UnlockedCard({ ach }: UnlockedCardProps) {
  return (
    <div style={cardStyle(true)}>
      <div style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
        <div style={{ ...iconBadgeStyle, ...tierStyle(ach.tier) }}>{ach.icon}</div>
        <div>
          <div style={titleStyle}>{ach.title}</div>
          <span style={{ ...tierChipStyle, ...tierStyle(ach.tier) }}>{ach.tier}</span>
        </div>
      </div>
      <div style={descStyle}>{ach.description}</div>
      <div style={metaStyle}>Unlocked {formatDate(ach.unlocked_at)}</div>
    </div>
  )
}

type LockedCardProps = { ach: Achievement }
function LockedCard({ ach }: LockedCardProps) {
  return (
    <div style={cardStyle(false)}>
      <div style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
        <div style={{ ...iconBadgeStyle, background: '#e5e7eb', color: '#6b7280' }}>🔒</div>
        <div>
          <div style={titleStyle}>{ach.title}</div>
          <span style={{ ...tierChipStyle, background: '#e5e7eb', color: '#6b7280' }}>{ach.tier}</span>
        </div>
      </div>
      <div style={descStyle}>{ach.description}</div>
    </div>
  )
}

export function AchievementsPage() {
  const { user } = useAuth()
  const [snap, setSnap] = useState<AchievementsSnapshot | null>(null)
  const [loading, setLoading] = useState(true)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [info, setInfo] = useState<string | null>(null)

  const reload = useCallback(async () => {
    setLoading(true)
    try {
      const data = await api.getMyAchievements()
      setSnap(data)
      setError(null)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'failed to load achievements')
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    if (!user) return
    void reload()
  }, [user, reload])

  const evaluate = async () => {
    setBusy(true)
    setInfo(null)
    setError(null)
    try {
      const res = await api.evaluateMyAchievements()
      if (res.newly_unlocked.length === 0) {
        setInfo('No new achievements unlocked.')
      } else {
        setInfo(
          `Unlocked ${res.newly_unlocked.length} achievement${res.newly_unlocked.length === 1 ? '' : 's'}: ${res.newly_unlocked
            .map((a) => a.title)
            .join(', ')}`,
        )
      }
      await reload()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'failed to evaluate')
    } finally {
      setBusy(false)
    }
  }

  const unlockedCount = snap?.unlocked.length ?? 0
  const totalCount = (snap?.unlocked.length ?? 0) + (snap?.locked.length ?? 0)
  const percent = useMemo(() => {
    if (totalCount === 0) return 0
    return Math.round((unlockedCount / totalCount) * 100)
  }, [unlockedCount, totalCount])

  if (!user) {
    return (
      <div style={pageStyle}>
        <h1 style={headerStyle}>Achievements</h1>
        <div style={subtleStyle}>Sign in to see your achievements.</div>
      </div>
    )
  }

  return (
    <div style={pageStyle}>
      <h1 style={headerStyle}>Achievements</h1>
      <div style={subtitleStyle}>
        {unlockedCount} of {totalCount} unlocked ({percent}%).
      </div>

      <div style={toolbarStyle}>
        <button style={primaryButtonStyle} onClick={evaluate} disabled={busy || loading}>
          {busy ? 'Evaluating…' : 'Re-evaluate'}
        </button>
        <span style={subtleStyle}>
          Re-evaluation recomputes your achievement set from your current scoreboard stats. It is safe to run anytime.
        </span>
      </div>

      {error && <div style={errorStyle}>{error}</div>}
      {info && <div style={okStyle}>{info}</div>}

      <section style={sectionStyle}>
        <div style={sectionTitleStyle}>Unlocked</div>
        {loading ? (
          <div style={subtleStyle}>Loading…</div>
        ) : unlockedCount === 0 ? (
          <div style={subtleStyle}>No achievements unlocked yet — play some games!</div>
        ) : (
          <div style={gridStyle}>
            {snap!.unlocked.map((a) => (
              <UnlockedCard key={a.id} ach={a} />
            ))}
          </div>
        )}
      </section>

      <section style={sectionStyle}>
        <div style={sectionTitleStyle}>Locked</div>
        {loading ? (
          <div style={subtleStyle}>Loading…</div>
        ) : (snap?.locked.length ?? 0) === 0 ? (
          <div style={subtleStyle}>You've unlocked everything. Nicely done!</div>
        ) : (
          <div style={gridStyle}>
            {snap!.locked.map((a) => (
              <LockedCard key={a.id} ach={a} />
            ))}
          </div>
        )}
      </section>
    </div>
  )
}
