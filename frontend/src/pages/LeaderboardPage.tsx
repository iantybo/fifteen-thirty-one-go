import { useEffect, useMemo, useState } from 'react'
import { Link } from 'react-router-dom'
import { api } from '../api/client'
import type { LeaderboardPlayer, LeaderboardResponse } from '../api/types'

function pct(x: number) {
  if (!Number.isFinite(x)) return '—'
  return `${(x * 100).toFixed(1)}%`
}

function sparkline(values: number[]) {
  const blocks = '▁▂▃▄▅▆▇█'
  if (values.length === 0) return ''
  const clamped = values.map((v) => Math.max(0, Math.min(1, v)))
  return clamped
    .map((v) => {
      const idx = Math.round(v * (blocks.length - 1))
      return blocks[idx] ?? blocks[0]
    })
    .join('')
}

export function LeaderboardPage() {
  const [data, setData] = useState<LeaderboardResponse | null>(null)
  const [days, setDays] = useState(30)
  const [err, setErr] = useState<string | null>(null)
  const [loading, setLoading] = useState(false)

  useEffect(() => {
    let cancelled = false
    async function load() {
      setErr(null)
      setLoading(true)
      try {
        const res = await api.getLeaderboard(days)
        if (!cancelled) setData(res)
      } catch (e: unknown) {
        if (!cancelled) setErr(e instanceof Error ? e.message : 'failed to load leaderboard')
      } finally {
        if (!cancelled) setLoading(false)
      }
    }
    void load()
    return () => {
      cancelled = true
    }
  }, [days])

  const items = useMemo(() => {
    const it = data?.items ?? []
    // Backend already sorts, but keep it deterministic.
    return [...it].sort((a, b) => {
      if ((a.games_played === 0) !== (b.games_played === 0)) return b.games_played - a.games_played
      if (a.win_rate !== b.win_rate) return b.win_rate - a.win_rate
      if (a.games_played !== b.games_played) return b.games_played - a.games_played
      return a.username.localeCompare(b.username)
    })
  }, [data])

  function renderTrend(p: LeaderboardPlayer) {
    const vals = (p.series ?? []).map((pt) => pt.win_rate)
    const s = sparkline(vals)
    if (!s) return '—'
    return s
  }

  return (
    <div style={{ maxWidth: 1000, margin: '24px auto', padding: '0 16px' }}>
      <header style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'baseline', gap: 16 }}>
        <h1>Leaderboard</h1>
        <div style={{ display: 'flex', gap: 12, alignItems: 'baseline' }}>
          <Link to="/lobbies">Lobbies</Link>
        </div>
      </header>

      <div style={{ display: 'flex', gap: 12, alignItems: 'center', margin: '8px 0 16px' }}>
        <label>
          Window:{' '}
          <select value={days} onChange={(e) => setDays(Number(e.target.value))}>
            <option value={7}>7 days</option>
            <option value={30}>30 days</option>
            <option value={90}>90 days</option>
            <option value={365}>365 days</option>
          </select>
        </label>
        {data && (
          <span style={{ opacity: 0.75 }}>
            Trend shows cumulative win rate over the last {data.days} days.
          </span>
        )}
      </div>

      {err && <div style={{ color: 'crimson' }}>{err}</div>}
      {loading && <div>Loading leaderboard...</div>}

      {!loading && !err && (
        <table style={{ width: '100%', borderCollapse: 'collapse' }}>
          <thead>
            <tr style={{ textAlign: 'left', borderBottom: '1px solid #ddd' }}>
              <th style={{ padding: '8px 6px' }}>Player</th>
              <th style={{ padding: '8px 6px' }}>Games</th>
              <th style={{ padding: '8px 6px' }}>Wins</th>
              <th style={{ padding: '8px 6px' }}>Win rate</th>
              <th style={{ padding: '8px 6px' }}>Trend</th>
            </tr>
          </thead>
          <tbody>
            {items.map((p) => (
              <tr key={p.user_id} style={{ borderBottom: '1px solid #f0f0f0' }}>
                <td style={{ padding: '8px 6px', fontWeight: 600 }}>{p.username}</td>
                <td style={{ padding: '8px 6px' }}>{p.games_played}</td>
                <td style={{ padding: '8px 6px' }}>{p.games_won}</td>
                <td style={{ padding: '8px 6px' }}>{pct(p.win_rate)}</td>
                <td style={{ padding: '8px 6px', fontFamily: 'ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace' }}>
                  {renderTrend(p)}
                </td>
              </tr>
            ))}
            {items.length === 0 && (
              <tr>
                <td colSpan={5} style={{ padding: '12px 6px', opacity: 0.8 }}>
                  No players yet.
                </td>
              </tr>
            )}
          </tbody>
        </table>
      )}
    </div>
  )
}


