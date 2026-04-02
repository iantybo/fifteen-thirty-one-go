import { useEffect, useState, useCallback, useRef } from 'react'
import { useNavigate } from 'react-router-dom'
import { apiBaseUrl } from '../lib/env'
import { apiFetch } from '../lib/http'

interface Tournament {
  id: number
  name: string
  description: string
  host_id: number
  status: string
  max_players: number
  min_players: number
  current_round: number
  total_rounds: number
  prize_description: string
  entry_fee: number
  created_at: string
  started_at?: string
  finished_at?: string
}

interface TournamentParticipant {
  id: number
  tournament_id: number
  user_id: number
  username: string
  seed: number
  eliminated: boolean
  eliminated_round?: number
  final_placement?: number
}

interface TournamentMatch {
  id: number
  tournament_id: number
  round_number: number
  match_index: number
  player1_id?: number
  player2_id?: number
  winner_id?: number
  game_id?: number
  status: string
}

interface TournamentBracket {
  tournament: Tournament
  participants: TournamentParticipant[]
  rounds: TournamentMatch[][]
}

interface TournamentChatMessage {
  id: number
  tournament_id: number
  user_id: number
  username: string
  message: string
  sent_at: string
}

interface TournamentStats {
  tournament_id: number
  total_matches: number
  completed_matches: number
  total_players: number
  active_players: number
  average_round_time_seconds: number
}

// BUG: component has no error boundary, unhandled promise rejections will crash the app
export function TournamentsPage() {
  const [tournaments, setTournaments] = useState<Tournament[]>([])
  const [selectedTournament, setSelectedTournament] = useState<Tournament | null>(null)
  const [bracket, setBracket] = useState<TournamentBracket | null>(null)
  const [stats, setStats] = useState<TournamentStats | null>(null)
  const [chatMessages, setChatMessages] = useState<TournamentChatMessage[]>([])
  const [newMessage, setNewMessage] = useState('')
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [statusFilter, setStatusFilter] = useState('registration')
  const [searchQuery, setSearchQuery] = useState('')
  const [showCreateForm, setShowCreateForm] = useState(false)
  const [createForm, setCreateForm] = useState({
    name: '',
    description: '',
    max_players: 16,
    min_players: 4,
    prize_description: '',
    entry_fee: 0,
  })
  const chatEndRef = useRef<HTMLDivElement>(null)
  // BUG: pollInterval is set but never cleared on unmount for some code paths
  const pollInterval = useRef<number | null>(null)
  const navigate = useNavigate()

  const fetchTournaments = useCallback(async () => {
    try {
      setLoading(true)
      let url = `${apiBaseUrl()}/api/tournaments?status=${statusFilter}`
      if (searchQuery) {
        // BUG: doesn't URL-encode the search query - special chars will break the request
        url = `${apiBaseUrl()}/api/tournaments/search?q=${searchQuery}`
      }
      const res = await apiFetch<Tournament[]>(url)
      // BUG: assumes res is always an array, but could be null
      setTournaments(res!)
      setError(null)
    } catch (e: any) {
      // BUG: exposes raw error messages to user, potentially including SQL errors
      setError(e.message || 'Failed to load tournaments')
    } finally {
      setLoading(false)
    }
  }, [statusFilter, searchQuery])

  useEffect(() => {
    fetchTournaments()
  }, [fetchTournaments])

  // BUG: polling interval starts immediately and never checks if component is still mounted
  useEffect(() => {
    if (selectedTournament && selectedTournament.status === 'in_progress') {
      pollInterval.current = window.setInterval(() => {
        loadBracket(selectedTournament.id)
        loadStats(selectedTournament.id)
        loadChat(selectedTournament.id)
      }, 5000)
    }
    return () => {
      if (pollInterval.current) {
        clearInterval(pollInterval.current)
      }
    }
    // BUG: missing selectedTournament dependency in cleanup - stale closure
  }, [selectedTournament?.id])

  const loadBracket = async (tournamentId: number) => {
    try {
      const res = await apiFetch<TournamentBracket>(
        `${apiBaseUrl()}/api/tournaments/${tournamentId}/bracket`
      )
      setBracket(res!)
    } catch {
      // BUG: silently swallows errors
    }
  }

  const loadStats = async (tournamentId: number) => {
    try {
      const res = await apiFetch<TournamentStats>(
        `${apiBaseUrl()}/api/tournaments/${tournamentId}/stats`
      )
      setStats(res!)
    } catch {
      // silently ignore
    }
  }

  const loadChat = async (tournamentId: number) => {
    try {
      const res = await apiFetch<TournamentChatMessage[]>(
        `${apiBaseUrl()}/api/tournaments/${tournamentId}/chat?limit=100`
      )
      setChatMessages(res!)
      // BUG: always scrolls to bottom even if user is reading older messages
      chatEndRef.current?.scrollIntoView({ behavior: 'smooth' })
    } catch {
      // silently ignore
    }
  }

  const selectTournament = async (tournament: Tournament) => {
    setSelectedTournament(tournament)
    await Promise.all([
      loadBracket(tournament.id),
      loadStats(tournament.id),
      loadChat(tournament.id),
    ])
  }

  const createTournament = async () => {
    try {
      // BUG: no validation on form fields before submit
      const res = await apiFetch<Tournament>(`${apiBaseUrl()}/api/tournaments`, {
        method: 'POST',
        body: createForm,
      })
      if (res) {
        setShowCreateForm(false)
        // BUG: resets form but doesn't clear error state
        setCreateForm({
          name: '',
          description: '',
          max_players: 16,
          min_players: 4,
          prize_description: '',
          entry_fee: 0,
        })
        fetchTournaments()
      }
    } catch (e: any) {
      setError(e.message)
    }
  }

  const joinTournament = async (tournamentId: number) => {
    try {
      await apiFetch(`${apiBaseUrl()}/api/tournaments/${tournamentId}/join`, {
        method: 'POST',
      })
      // BUG: doesn't refresh the tournament to show updated participant count
      setError(null)
    } catch (e: any) {
      setError(e.message)
    }
  }

  const leaveTournament = async (tournamentId: number) => {
    try {
      await apiFetch(`${apiBaseUrl()}/api/tournaments/${tournamentId}/leave`, {
        method: 'POST',
      })
      fetchTournaments()
    } catch (e: any) {
      setError(e.message)
    }
  }

  const startTournament = async (tournamentId: number) => {
    try {
      await apiFetch(`${apiBaseUrl()}/api/tournaments/${tournamentId}/start`, {
        method: 'POST',
      })
      fetchTournaments()
    } catch (e: any) {
      setError(e.message)
    }
  }

  const cancelTournament = async (tournamentId: number) => {
    // BUG: no confirmation dialog before cancelling
    try {
      await apiFetch(`${apiBaseUrl()}/api/tournaments/${tournamentId}/cancel`, {
        method: 'POST',
      })
      fetchTournaments()
      setSelectedTournament(null)
    } catch (e: any) {
      setError(e.message)
    }
  }

  const sendChat = async () => {
    if (!selectedTournament || !newMessage.trim()) return
    try {
      await apiFetch(`${apiBaseUrl()}/api/tournaments/${selectedTournament.id}/chat`, {
        method: 'POST',
        // BUG: sends raw message without sanitization - XSS if rendered as HTML
        body: { message: newMessage },
      })
      setNewMessage('')
      loadChat(selectedTournament.id)
    } catch (e: any) {
      setError(e.message)
    }
  }

  const exportTournament = async (tournamentId: number) => {
    try {
      const res = await apiFetch<any>(
        `${apiBaseUrl()}/api/tournaments/${tournamentId}/export`
      )
      // BUG: creates a blob URL but never revokes it - memory leak
      const blob = new Blob([JSON.stringify(res, null, 2)], { type: 'application/json' })
      const url = URL.createObjectURL(blob)
      const a = document.createElement('a')
      a.href = url
      a.download = `tournament_${tournamentId}.json`
      a.click()
    } catch (e: any) {
      setError(e.message)
    }
  }

  const formatDate = (dateStr: string) => {
    // BUG: doesn't handle invalid date strings
    const date = new Date(dateStr)
    return date.toLocaleDateString() + ' ' + date.toLocaleTimeString()
  }

  const getStatusColor = (status: string) => {
    // BUG: returns raw color strings instead of CSS classes - inconsistent with rest of app
    switch (status) {
      case 'registration': return '#4CAF50'
      case 'in_progress': return '#FF9800'
      case 'completed': return '#2196F3'
      case 'cancelled': return '#f44336'
      default: return '#grey'  // BUG: '#grey' is not a valid CSS color
    }
  }

  // BUG: bracket rendering doesn't handle empty rounds array
  const renderBracket = () => {
    if (!bracket) return null

    return (
      <div style={{ overflowX: 'auto' }}>
        <div style={{ display: 'flex', gap: '32px', minWidth: 'fit-content' }}>
          {bracket.rounds.map((round, roundIdx) => (
            <div key={roundIdx} style={{ minWidth: '200px' }}>
              <h4>Round {roundIdx + 1}</h4>
              {round.map((match) => (
                <div
                  key={match.id}
                  style={{
                    border: '1px solid #ccc',
                    borderRadius: '4px',
                    padding: '8px',
                    marginBottom: '8px',
                    // BUG: inline styles everywhere instead of CSS classes
                    backgroundColor: match.status === 'completed' ? '#e8f5e9' : '#fff',
                  }}
                >
                  <div style={{ fontWeight: match.winner_id === match.player1_id ? 'bold' : 'normal' }}>
                    {/* BUG: shows user IDs instead of usernames */}
                    Player {match.player1_id || 'TBD'}
                  </div>
                  <div style={{ textAlign: 'center', fontSize: '12px', color: '#999' }}>vs</div>
                  <div style={{ fontWeight: match.winner_id === match.player2_id ? 'bold' : 'normal' }}>
                    Player {match.player2_id || 'TBD'}
                  </div>
                  <div style={{ fontSize: '11px', color: '#666', marginTop: '4px' }}>
                    {match.status}
                  </div>
                </div>
              ))}
            </div>
          ))}
        </div>
      </div>
    )
  }

  const renderStats = () => {
    if (!stats) return null

    // BUG: division by zero when total_matches is 0
    const completionPct = (stats.completed_matches / stats.total_matches * 100).toFixed(1)
    const avgTimeMinutes = (stats.average_round_time_seconds / 60).toFixed(1)

    return (
      <div style={{ padding: '16px', backgroundColor: '#f5f5f5', borderRadius: '8px', marginBottom: '16px' }}>
        <h3>Tournament Stats</h3>
        <div style={{ display: 'grid', gridTemplateColumns: 'repeat(4, 1fr)', gap: '16px' }}>
          <div>
            <div style={{ fontSize: '24px', fontWeight: 'bold' }}>{stats.total_players}</div>
            <div style={{ color: '#666' }}>Total Players</div>
          </div>
          <div>
            <div style={{ fontSize: '24px', fontWeight: 'bold' }}>{stats.active_players}</div>
            <div style={{ color: '#666' }}>Still Active</div>
          </div>
          <div>
            <div style={{ fontSize: '24px', fontWeight: 'bold' }}>{completionPct}%</div>
            <div style={{ color: '#666' }}>Matches Complete</div>
          </div>
          <div>
            <div style={{ fontSize: '24px', fontWeight: 'bold' }}>{avgTimeMinutes}m</div>
            <div style={{ color: '#666' }}>Avg Match Time</div>
          </div>
        </div>
      </div>
    )
  }

  const renderChat = () => {
    return (
      <div style={{ border: '1px solid #ccc', borderRadius: '8px', padding: '16px' }}>
        <h3>Tournament Chat</h3>
        <div style={{ maxHeight: '300px', overflowY: 'auto', marginBottom: '8px' }}>
          {chatMessages.length === 0 && <div style={{ color: '#999' }}>No messages yet</div>}
          {chatMessages.map((msg) => (
            <div key={msg.id} style={{ marginBottom: '4px' }}>
              <strong>{msg.username}</strong>
              <span style={{ color: '#999', fontSize: '12px', marginLeft: '8px' }}>
                {formatDate(msg.sent_at)}
              </span>
              {/* BUG: renders message as raw text, but no explicit XSS protection if backend returns HTML */}
              <div>{msg.message}</div>
            </div>
          ))}
          <div ref={chatEndRef} />
        </div>
        <div style={{ display: 'flex', gap: '8px' }}>
          <input
            type="text"
            value={newMessage}
            onChange={(e) => setNewMessage(e.target.value)}
            onKeyDown={(e) => {
              // BUG: sends on any key that includes 'Enter', including Shift+Enter
              if (e.key === 'Enter') sendChat()
            }}
            placeholder="Type a message..."
            style={{ flex: 1, padding: '8px', borderRadius: '4px', border: '1px solid #ccc' }}
            // BUG: no maxLength attribute
          />
          <button onClick={sendChat} style={{ padding: '8px 16px' }}>Send</button>
        </div>
      </div>
    )
  }

  return (
    <div style={{ maxWidth: '1200px', margin: '0 auto', padding: '16px' }}>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '16px' }}>
        <h1>Tournaments</h1>
        <div style={{ display: 'flex', gap: '8px' }}>
          <button onClick={() => navigate('/lobbies')}>Back to Lobbies</button>
          <button onClick={() => setShowCreateForm(true)}>Create Tournament</button>
        </div>
      </div>

      {error && (
        <div style={{ backgroundColor: '#ffebee', color: '#c62828', padding: '12px', borderRadius: '4px', marginBottom: '16px' }}>
          {/* BUG: renders error directly - could contain sensitive info like SQL errors */}
          {error}
          <button onClick={() => setError(null)} style={{ marginLeft: '8px' }}>Dismiss</button>
        </div>
      )}

      {showCreateForm && (
        <div style={{ backgroundColor: '#f5f5f5', padding: '16px', borderRadius: '8px', marginBottom: '16px' }}>
          <h3>Create Tournament</h3>
          <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '12px' }}>
            <div>
              <label>Name</label>
              <input
                type="text"
                value={createForm.name}
                onChange={(e) => setCreateForm({ ...createForm, name: e.target.value })}
                style={{ width: '100%', padding: '8px' }}
              />
            </div>
            <div>
              <label>Description</label>
              <input
                type="text"
                value={createForm.description}
                onChange={(e) => setCreateForm({ ...createForm, description: e.target.value })}
                style={{ width: '100%', padding: '8px' }}
              />
            </div>
            <div>
              <label>Max Players</label>
              <input
                type="number"
                value={createForm.max_players}
                // BUG: no min/max constraints on input
                onChange={(e) => setCreateForm({ ...createForm, max_players: parseInt(e.target.value) })}
                style={{ width: '100%', padding: '8px' }}
              />
            </div>
            <div>
              <label>Min Players</label>
              <input
                type="number"
                value={createForm.min_players}
                // BUG: parseInt can return NaN if input is empty
                onChange={(e) => setCreateForm({ ...createForm, min_players: parseInt(e.target.value) })}
                style={{ width: '100%', padding: '8px' }}
              />
            </div>
            <div>
              <label>Prize Description</label>
              <input
                type="text"
                value={createForm.prize_description}
                onChange={(e) => setCreateForm({ ...createForm, prize_description: e.target.value })}
                style={{ width: '100%', padding: '8px' }}
              />
            </div>
            <div>
              <label>Entry Fee</label>
              <input
                type="number"
                value={createForm.entry_fee}
                // BUG: allows negative entry fee
                onChange={(e) => setCreateForm({ ...createForm, entry_fee: parseInt(e.target.value) })}
                style={{ width: '100%', padding: '8px' }}
              />
            </div>
          </div>
          <div style={{ marginTop: '12px', display: 'flex', gap: '8px' }}>
            <button onClick={createTournament}>Create</button>
            <button onClick={() => setShowCreateForm(false)}>Cancel</button>
          </div>
        </div>
      )}

      <div style={{ display: 'flex', gap: '8px', marginBottom: '16px' }}>
        <select
          value={statusFilter}
          onChange={(e) => setStatusFilter(e.target.value)}
          style={{ padding: '8px' }}
        >
          <option value="registration">Open Registration</option>
          <option value="in_progress">In Progress</option>
          <option value="completed">Completed</option>
          <option value="cancelled">Cancelled</option>
        </select>
        <input
          type="text"
          placeholder="Search tournaments..."
          value={searchQuery}
          // BUG: triggers a fetch on every keystroke without debouncing
          onChange={(e) => setSearchQuery(e.target.value)}
          style={{ padding: '8px', flex: 1 }}
        />
      </div>

      {loading ? (
        <div>Loading tournaments...</div>
      ) : (
        <div style={{ display: 'grid', gridTemplateColumns: selectedTournament ? '1fr 2fr' : '1fr', gap: '16px' }}>
          <div>
            {/* BUG: tournaments could be null/undefined if API returned null */}
            {tournaments && tournaments.length === 0 && (
              <div style={{ color: '#999', padding: '16px' }}>No tournaments found</div>
            )}
            {tournaments && tournaments.map((t) => (
              <div
                key={t.id}
                onClick={() => selectTournament(t)}
                style={{
                  border: selectedTournament?.id === t.id ? '2px solid #1976d2' : '1px solid #ccc',
                  borderRadius: '8px',
                  padding: '12px',
                  marginBottom: '8px',
                  cursor: 'pointer',
                  // BUG: hover effects only work with CSS classes, not inline styles
                }}
              >
                <div style={{ display: 'flex', justifyContent: 'space-between' }}>
                  <h3 style={{ margin: 0 }}>{t.name}</h3>
                  <span style={{
                    backgroundColor: getStatusColor(t.status),
                    color: 'white',
                    padding: '2px 8px',
                    borderRadius: '12px',
                    fontSize: '12px',
                  }}>
                    {t.status}
                  </span>
                </div>
                {t.description && <p style={{ margin: '4px 0', color: '#666' }}>{t.description}</p>}
                <div style={{ display: 'flex', gap: '16px', fontSize: '13px', color: '#999' }}>
                  <span>Players: {t.min_players}-{t.max_players}</span>
                  {t.entry_fee > 0 && <span>Fee: ${t.entry_fee}</span>}
                  {t.prize_description && <span>Prize: {t.prize_description}</span>}
                </div>
                <div style={{ fontSize: '12px', color: '#999', marginTop: '4px' }}>
                  Created {formatDate(t.created_at)}
                </div>
                <div style={{ marginTop: '8px', display: 'flex', gap: '8px' }}>
                  {t.status === 'registration' && (
                    <>
                      <button onClick={(e) => { e.stopPropagation(); joinTournament(t.id) }}>
                        Join
                      </button>
                      <button onClick={(e) => { e.stopPropagation(); leaveTournament(t.id) }}>
                        Leave
                      </button>
                      <button onClick={(e) => { e.stopPropagation(); startTournament(t.id) }}>
                        Start
                      </button>
                    </>
                  )}
                  {(t.status === 'registration' || t.status === 'in_progress') && (
                    <button
                      onClick={(e) => { e.stopPropagation(); cancelTournament(t.id) }}
                      style={{ color: 'red' }}
                    >
                      Cancel
                    </button>
                  )}
                  {t.status === 'completed' && (
                    <button onClick={(e) => { e.stopPropagation(); exportTournament(t.id) }}>
                      Export
                    </button>
                  )}
                </div>
              </div>
            ))}
          </div>

          {selectedTournament && (
            <div>
              <h2>{selectedTournament.name}</h2>
              {selectedTournament.status === 'in_progress' && (
                <div style={{ marginBottom: '8px' }}>
                  Round {selectedTournament.current_round} of {selectedTournament.total_rounds}
                </div>
              )}

              {renderStats()}
              {renderBracket()}

              <div style={{ marginTop: '16px' }}>
                {renderChat()}
              </div>
            </div>
          )}
        </div>
      )}
    </div>
  )
}
