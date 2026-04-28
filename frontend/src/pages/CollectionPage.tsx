import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { api } from '../api/client'
import type { MyCardsResponse, UserCard } from '../api/types'

export function CollectionPage() {
  const [data, setData] = useState<MyCardsResponse | null>(null)
  const [err, setErr] = useState<string | null>(null)
  const [loading, setLoading] = useState(false)
  const [selling, setSelling] = useState<number | null>(null)

  useEffect(() => {
    let cancelled = false
    async function load() {
      setErr(null)
      setLoading(true)
      try {
        const res = await api.listMyCards()
        if (!cancelled) setData(res)
      } catch (e: unknown) {
        if (!cancelled) setErr(e instanceof Error ? e.message : 'failed to load collection')
      } finally {
        if (!cancelled) setLoading(false)
      }
    }
    void load()
    return () => {
      cancelled = true
    }
  }, [])

  async function onSell(card: UserCard) {
    setErr(null)
    setSelling(card.id)
    try {
      const res = await api.sellCard(card.id)
      setData((prev) =>
        prev
          ? {
              ...prev,
              coins: res.coins,
              cards: prev.cards.filter((c) => c.id !== card.id),
            }
          : prev,
      )
    } catch (e: unknown) {
      setErr(e instanceof Error ? e.message : 'failed to sell card')
    } finally {
      setSelling(null)
    }
  }

  const cards = data?.cards ?? []
  const coins = data?.coins ?? 0
  const sellPrice = data?.sell_price ?? 0

  return (
    <div style={{ maxWidth: 1000, margin: '24px auto', padding: '0 16px' }}>
      <header style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'baseline', gap: 16 }}>
        <h1>My Collection</h1>
        <div style={{ display: 'flex', gap: 12, alignItems: 'baseline' }}>
          <Link to="/lobbies">Lobbies</Link>
          <Link to="/leaderboard">Leaderboard</Link>
        </div>
      </header>

      <div style={{ display: 'flex', gap: 24, alignItems: 'baseline', margin: '8px 0 16px' }}>
        <div>
          <strong>Coins:</strong> {coins}
        </div>
        <div style={{ opacity: 0.75 }}>
          Each used card sells for {sellPrice} {sellPrice === 1 ? 'coin' : 'coins'}.
        </div>
      </div>

      {err && <div style={{ color: 'crimson', marginBottom: 12 }}>{err}</div>}
      {loading && <div>Loading collection…</div>}

      {!loading && !err && cards.length === 0 && (
        <div style={{ opacity: 0.8 }}>
          You don't own any cards yet. Finish a game to earn cards from the hands you played.
        </div>
      )}

      {!loading && cards.length > 0 && (
        <table style={{ width: '100%', borderCollapse: 'collapse' }}>
          <thead>
            <tr style={{ textAlign: 'left', borderBottom: '1px solid #ddd' }}>
              <th style={{ padding: '8px 6px' }}>Card</th>
              <th style={{ padding: '8px 6px' }}>From game</th>
              <th style={{ padding: '8px 6px' }}>Acquired</th>
              <th style={{ padding: '8px 6px' }}></th>
            </tr>
          </thead>
          <tbody>
            {cards.map((c) => (
              <tr key={c.id} style={{ borderBottom: '1px solid #f0f0f0' }}>
                <td
                  style={{
                    padding: '8px 6px',
                    fontFamily: 'ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace',
                    fontWeight: 600,
                  }}
                >
                  {c.card}
                </td>
                <td style={{ padding: '8px 6px' }}>
                  <Link to={`/games/${c.game_id}`}>#{c.game_id}</Link>
                </td>
                <td style={{ padding: '8px 6px', opacity: 0.8 }}>
                  {new Date(c.acquired_at).toLocaleString()}
                </td>
                <td style={{ padding: '8px 6px', textAlign: 'right' }}>
                  <button
                    type="button"
                    onClick={() => void onSell(c)}
                    disabled={selling !== null}
                  >
                    {selling === c.id ? 'Selling…' : `Sell for ${sellPrice}`}
                  </button>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </div>
  )
}
