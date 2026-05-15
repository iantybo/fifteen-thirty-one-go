import { useCallback, useEffect, useMemo, useState } from 'react'
import type { CSSProperties } from 'react'
import { api } from '../api/client'
import type { FriendRequestView, FriendSummary, FriendsListResponse } from '../api/types'
import { useAuth } from '../auth/auth'

// FriendsPage renders three sections — accepted friends, incoming requests,
// outgoing requests — plus a block list. It calls the backend friends API and
// optimistically updates local state on common mutations.

const pageStyle: CSSProperties = { padding: '24px', maxWidth: 880, margin: '0 auto' }
const headerStyle: CSSProperties = { fontSize: 28, fontWeight: 800, margin: '0 0 24px 0' }
const sectionStyle: CSSProperties = {
  background: '#fff',
  border: '1px solid #e5e7eb',
  borderRadius: 12,
  padding: 20,
  marginBottom: 20,
  boxShadow: '0 1px 2px rgba(0,0,0,0.04)',
}
const sectionTitleStyle: CSSProperties = { fontSize: 18, fontWeight: 700, marginBottom: 12 }
const rowStyle: CSSProperties = {
  display: 'flex',
  alignItems: 'center',
  gap: 12,
  padding: '10px 8px',
  borderBottom: '1px solid #f3f4f6',
}
const lastRowStyle: CSSProperties = { ...rowStyle, borderBottom: 'none' }
const avatarStyle: CSSProperties = {
  width: 36,
  height: 36,
  borderRadius: '50%',
  background: '#e5e7eb',
  display: 'flex',
  alignItems: 'center',
  justifyContent: 'center',
  fontWeight: 700,
  color: '#374151',
}
const nameStyle: CSSProperties = { flex: 1, fontWeight: 600 }
const subtleStyle: CSSProperties = { color: '#6b7280', fontSize: 13 }
const dangerButtonStyle: CSSProperties = {
  padding: '6px 12px',
  background: '#fee2e2',
  color: '#991b1b',
  border: '1px solid #fecaca',
  borderRadius: 6,
  cursor: 'pointer',
}
const primaryButtonStyle: CSSProperties = {
  padding: '6px 12px',
  background: '#4a9eff',
  color: '#fff',
  border: 'none',
  borderRadius: 6,
  cursor: 'pointer',
  fontWeight: 600,
}
const ghostButtonStyle: CSSProperties = {
  padding: '6px 12px',
  background: 'transparent',
  color: '#374151',
  border: '1px solid #d1d5db',
  borderRadius: 6,
  cursor: 'pointer',
}
const formRowStyle: CSSProperties = { display: 'flex', gap: 8, marginTop: 12 }
const inputStyle: CSSProperties = {
  flex: 1,
  padding: '8px 12px',
  border: '1px solid #d1d5db',
  borderRadius: 6,
  fontSize: 14,
}
const errorStyle: CSSProperties = { color: '#b91c1c', marginTop: 8, fontSize: 14 }
const okStyle: CSSProperties = { color: '#047857', marginTop: 8, fontSize: 14 }

function initials(name: string): string {
  const parts = name.trim().split(/\s+/).slice(0, 2)
  return parts.map((p) => p[0]?.toUpperCase() ?? '').join('') || '?'
}

function formatDate(ts: string): string {
  const d = new Date(ts)
  if (Number.isNaN(d.getTime())) return ts
  return d.toLocaleDateString(undefined, { year: 'numeric', month: 'short', day: 'numeric' })
}

export function FriendsPage() {
  const { user } = useAuth()
  const [data, setData] = useState<FriendsListResponse | null>(null)
  const [blocked, setBlocked] = useState<FriendSummary[]>([])
  const [loading, setLoading] = useState(true)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [info, setInfo] = useState<string | null>(null)
  const [requestUserId, setRequestUserId] = useState('')

  const reload = useCallback(async () => {
    setLoading(true)
    try {
      const [list, blockedList] = await Promise.all([api.listFriends(), api.listBlocked()])
      setData(list)
      setBlocked(blockedList.blocked)
      setError(null)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'failed to load friends')
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    if (!user) return
    void reload()
  }, [user, reload])

  const sendRequest = async () => {
    const id = Number.parseInt(requestUserId, 10)
    if (!Number.isFinite(id) || id <= 0) {
      setError('Enter a valid user id')
      return
    }
    setBusy(true)
    setError(null)
    setInfo(null)
    try {
      await api.sendFriendRequest(id)
      setInfo('Request sent')
      setRequestUserId('')
      await reload()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'failed to send request')
    } finally {
      setBusy(false)
    }
  }

  const accept = async (req: FriendRequestView) => {
    setBusy(true)
    try {
      await api.acceptFriendRequest(req.id)
      await reload()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'failed to accept')
    } finally {
      setBusy(false)
    }
  }

  const decline = async (req: FriendRequestView) => {
    setBusy(true)
    try {
      await api.declineFriendRequest(req.id)
      await reload()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'failed to decline')
    } finally {
      setBusy(false)
    }
  }

  const remove = async (friend: FriendSummary) => {
    if (!window.confirm(`Remove ${friend.username} from your friends?`)) return
    setBusy(true)
    try {
      await api.removeFriend(friend.user_id)
      await reload()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'failed to remove')
    } finally {
      setBusy(false)
    }
  }

  const block = async (friend: FriendSummary) => {
    if (!window.confirm(`Block ${friend.username}? This also removes any friendship.`)) return
    setBusy(true)
    try {
      await api.blockUser(friend.user_id)
      await reload()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'failed to block')
    } finally {
      setBusy(false)
    }
  }

  const unblock = async (entry: FriendSummary) => {
    setBusy(true)
    try {
      await api.unblockUser(entry.user_id)
      await reload()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'failed to unblock')
    } finally {
      setBusy(false)
    }
  }

  const friends = data?.friends ?? []
  const incoming = data?.incoming ?? []
  const outgoing = data?.outgoing ?? []

  const counts = useMemo(
    () => ({ friends: friends.length, incoming: incoming.length, outgoing: outgoing.length, blocked: blocked.length }),
    [friends.length, incoming.length, outgoing.length, blocked.length],
  )

  if (!user) {
    return (
      <div style={pageStyle}>
        <h1 style={headerStyle}>Friends</h1>
        <div style={subtleStyle}>Sign in to manage your friends list.</div>
      </div>
    )
  }

  return (
    <div style={pageStyle}>
      <h1 style={headerStyle}>Friends</h1>

      <section style={sectionStyle}>
        <div style={sectionTitleStyle}>Send a Friend Request</div>
        <div style={subtleStyle}>Enter another player's user id to send them a request.</div>
        <div style={formRowStyle}>
          <input
            style={inputStyle}
            type="number"
            min={1}
            placeholder="User id"
            value={requestUserId}
            onChange={(e) => setRequestUserId(e.target.value)}
            disabled={busy}
          />
          <button style={primaryButtonStyle} onClick={sendRequest} disabled={busy || !requestUserId.trim()}>
            Send
          </button>
        </div>
        {error && <div style={errorStyle}>{error}</div>}
        {info && <div style={okStyle}>{info}</div>}
      </section>

      <section style={sectionStyle}>
        <div style={sectionTitleStyle}>
          Incoming requests {counts.incoming > 0 && <span style={subtleStyle}>({counts.incoming})</span>}
        </div>
        {loading ? (
          <div style={subtleStyle}>Loading…</div>
        ) : incoming.length === 0 ? (
          <div style={subtleStyle}>No incoming requests.</div>
        ) : (
          incoming.map((req, idx) => (
            <div key={req.id} style={idx === incoming.length - 1 ? lastRowStyle : rowStyle}>
              <div style={avatarStyle}>{initials(req.from_username)}</div>
              <div style={nameStyle}>
                {req.from_username}
                <div style={subtleStyle}>Sent {formatDate(req.created_at)}</div>
              </div>
              <button style={primaryButtonStyle} onClick={() => accept(req)} disabled={busy}>
                Accept
              </button>
              <button style={ghostButtonStyle} onClick={() => decline(req)} disabled={busy}>
                Decline
              </button>
            </div>
          ))
        )}
      </section>

      <section style={sectionStyle}>
        <div style={sectionTitleStyle}>
          Outgoing requests {counts.outgoing > 0 && <span style={subtleStyle}>({counts.outgoing})</span>}
        </div>
        {loading ? (
          <div style={subtleStyle}>Loading…</div>
        ) : outgoing.length === 0 ? (
          <div style={subtleStyle}>No outgoing requests.</div>
        ) : (
          outgoing.map((req, idx) => (
            <div key={req.id} style={idx === outgoing.length - 1 ? lastRowStyle : rowStyle}>
              <div style={avatarStyle}>{initials(req.from_username)}</div>
              <div style={nameStyle}>
                {req.from_username}
                <div style={subtleStyle}>Sent {formatDate(req.created_at)}</div>
              </div>
              <span style={subtleStyle}>Pending</span>
            </div>
          ))
        )}
      </section>

      <section style={sectionStyle}>
        <div style={sectionTitleStyle}>
          Friends {counts.friends > 0 && <span style={subtleStyle}>({counts.friends})</span>}
        </div>
        {loading ? (
          <div style={subtleStyle}>Loading…</div>
        ) : friends.length === 0 ? (
          <div style={subtleStyle}>No friends yet. Send a request above to get started.</div>
        ) : (
          friends.map((friend, idx) => (
            <div key={friend.user_id} style={idx === friends.length - 1 ? lastRowStyle : rowStyle}>
              <div style={avatarStyle}>{initials(friend.username)}</div>
              <div style={nameStyle}>
                {friend.username}
                <div style={subtleStyle}>Friends since {formatDate(friend.since)}</div>
              </div>
              <button style={ghostButtonStyle} onClick={() => remove(friend)} disabled={busy}>
                Remove
              </button>
              <button style={dangerButtonStyle} onClick={() => block(friend)} disabled={busy}>
                Block
              </button>
            </div>
          ))
        )}
      </section>

      <section style={sectionStyle}>
        <div style={sectionTitleStyle}>
          Blocked {counts.blocked > 0 && <span style={subtleStyle}>({counts.blocked})</span>}
        </div>
        {loading ? (
          <div style={subtleStyle}>Loading…</div>
        ) : blocked.length === 0 ? (
          <div style={subtleStyle}>No blocked users.</div>
        ) : (
          blocked.map((entry, idx) => (
            <div key={entry.user_id} style={idx === blocked.length - 1 ? lastRowStyle : rowStyle}>
              <div style={avatarStyle}>{initials(entry.username)}</div>
              <div style={nameStyle}>
                {entry.username}
                <div style={subtleStyle}>Blocked {formatDate(entry.since)}</div>
              </div>
              <button style={ghostButtonStyle} onClick={() => unblock(entry)} disabled={busy}>
                Unblock
              </button>
            </div>
          ))
        )}
      </section>
    </div>
  )
}
