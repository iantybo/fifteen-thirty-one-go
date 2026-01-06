# Lobby Implementation Analysis - Fifteen Thirty-One

## Executive Summary

The lobby system is a **minimal but functional** real-time multiplayer game matchmaking system built with:
- **Backend**: Go with Gin framework + SQLite database + Gorilla WebSocket
- **Frontend**: React + TypeScript with custom WebSocket client
- **Real-time communication**: Room-based broadcasting via WebSocket hub
- **State management**: HTTP API with optimistic WebSocket updates

The implementation focuses on **core functionality over features** - no chat, no advanced filtering, no complex UI. Players can list, create, join lobbies and add bots, then transition directly to gameplay.

---

## 1. Backend Lobby Implementation

### 1.1 Data Model (lobby.go)

**Lobby Table Schema:**
```sql
CREATE TABLE lobbies (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  name TEXT NOT NULL,
  host_id INTEGER NOT NULL,
  max_players INTEGER NOT NULL,
  current_players INTEGER NOT NULL DEFAULT 0,
  status TEXT NOT NULL DEFAULT 'waiting' CHECK(status IN ('waiting', 'in_progress', 'finished')),
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  FOREIGN KEY(host_id) REFERENCES users(id)
);
```

**Core Structures:**
```go
type Lobby struct {
  ID             int64     `json:"id"`
  Name           string    `json:"name"`
  HostID         int64     `json:"host_id"`
  MaxPlayers     int64     `json:"max_players"`
  CurrentPlayers int64     `json:"current_players"`
  Status         string    `json:"status"` // waiting|in_progress|finished
  CreatedAt      time.Time `json:"created_at"`
}
```

**Key Design Decisions:**
- `current_players` is **denormalized** - maintained via SQLite triggers on `game_players` table
- Status lifecycle: `waiting` → `in_progress` → `finished`
- One game per lobby (1:1 relationship via `games.lobby_id`)
- Host is the lobby creator and typically position 0 in the game

### 1.2 HTTP API Endpoints (lobby.go)

#### GET /api/lobbies
Lists active lobbies (status != 'finished')
```go
func ListLobbiesHandler(db *sql.DB) gin.HandlerFunc
```
- **Query params**: `limit` (default 50, max 200), `offset` (default 0)
- **Response**: `{ "lobbies": Lobby[] }`
- **Ordering**: by `created_at DESC`
- **Auth**: Required (auth middleware)

#### POST /api/lobbies
Creates a new lobby and associated game
```go
func CreateLobbyHandler(db *sql.DB) gin.HandlerFunc
```
- **Request**:
  ```json
  {
    "name": "string",
    "max_players": 2|3|4
  }
  ```
- **Response**:
  ```json
  {
    "lobby": Lobby,
    "game": Game
  }
  ```
- **Side Effects**:
  1. Insert lobby with creator as host (position 0)
  2. Create game with status 'waiting'
  3. Add host as game_players at position 0
  4. Initialize cribbage engine state (Deal())
  5. Persist initial hand and game state to DB
  6. Install game state in runtime memory manager

#### POST /api/lobbies/{id}/join
Joins an existing lobby
```go
func JoinLobbyHandler(db *sql.DB) gin.HandlerFunc
```
- **Response**:
  ```json
  {
    "lobby": Lobby,
    "game_id": number,
    "already_joined": boolean,  // idempotent - player may rejoin
    "joined_persisted": boolean,
    "realtime_sync": string     // "ok" or "failed"
  }
  ```
- **Transactional behavior**:
  - Increment `lobby.current_players` atomically
  - Add player to game_players with auto-assigned position
  - Persist initial hand from game state
  - Sync runtime state (best-effort)
- **Idempotency**: If user already in game, return 200 without re-incrementing
- **Failure modes**: 
  - `ErrNotFound`: Lobby doesn't exist
  - `ErrLobbyFull`: current_players >= max_players
  - `ErrLobbyNotJoinable`: Lobby status != 'waiting'

#### POST /api/lobbies/{id}/add_bot
Adds an AI player to the lobby
```go
func AddBotToLobbyHandler(db *sql.DB) gin.HandlerFunc
```
- **Request** (optional):
  ```json
  { "difficulty": "easy"|"medium"|"hard" }
  ```
- **Response**:
  ```json
  {
    "game_id": number,
    "bot_user_id": number,
    "bot_username": string
  }
  ```
- **Restrictions**: Only lobby host can add bots
- **Implementation Details**:
  - Create temporary bot user (auto-generated unique username)
  - Add to game_players with auto-assigned position
  - Persist bot's hand from game state
  - Triggers `current_players` update via SQLite trigger
  - Broadcasts `game_update` to all clients in the game room

### 1.3 Transaction Handling

**JoinLobby uses nested transactions:**
```
TX1: BeginTx()
  ├─ UpdateLobbies SET current_players += 1 (guarded by status & count)
  ├─ AddGamePlayerAutoPositionTx(TX1) → allocates position
  ├─ UpdatePlayerHandIfEmptyTx(TX1) → persists initial hand
  └─ Commit()
```

**Why transactional?**
- Prevents orphaned lobby/game records
- Ensures atomic increment + player addition
- Avoids "lobby full" race conditions
- Allows rollback if hand persistence fails

**Lock Ordering (DB → Memory):**
- DB transaction acquired first (respects SQLite's locking)
- Runtime state sync happens AFTER commit (uses best-effort sync function)
- Prevents deadlocks and keeps locks short

### 1.4 Lobby Lifecycle & Triggers

**SQLite Triggers:**
```sql
trg_lobbies_current_players_after_game_players_insert
trg_lobbies_current_players_after_game_players_delete
```

**Behavior:**
- When `game_players` row inserted/deleted, recount active players in the lobby's latest game
- Only counts players in games with status IN ('waiting', 'in_progress')
- Handles bot additions automatically (no HTTP call needed to update count)

---

## 2. Frontend Lobby Implementation

### 2.1 Pages & Navigation

#### LobbiesPage.tsx
Main lobby listing and entry point
- **Features**:
  - Load and list active lobbies
  - "Create" link (→ CreateLobbyPage)
  - "Play vs Computer" quick-start button (creates 2-player lobby + adds bot)
  - "Logout" button
  - Error/loading states
- **State**:
  ```typescript
  lobbies: Lobby[]
  loading: boolean
  err: string | null
  quickBusy: boolean // during vs-bot creation
  ```
- **API calls**:
  - `api.listLobbies()` on component mount
  - `api.createLobby() → api.addBotToLobby()` for vs-bot flow

#### CreateLobbyPage.tsx
Lobby creation form
- **Form inputs**:
  - `name`: string (auto-trimmed, max 100 chars, defaults to "Lobby")
  - `max_players`: dropdown (2, 3, 4)
- **Validation**: Client-side (name non-empty) + server-side (max_players 2-4)
- **Flow**:
  1. User submits form
  2. Call `api.createLobby()`
  3. Navigate to GamePage with returned `game.id`
  4. GamePage loads lobby + game snapshot
- **UX**: Button shows "Creating..." during request

#### LobbyDetailPage.tsx
Individual lobby view (when accessed via direct link)
- **Route**: `/lobbies/:id`
- **Single action**: "Join lobby" button
- **Flow**:
  1. User clicks join
  2. Call `api.joinLobby(lobbyId)`
  3. Navigate to GamePage with returned `game_id`
- **Idempotency**: Clicking join multiple times is safe (returns "already_joined": true)

### 2.2 API Client (client.ts)

**Lobby-related methods:**
```typescript
listLobbies(): Promise<{ lobbies: Lobby[] }>
createLobby(req: CreateLobbyRequest): Promise<{ lobby: Lobby; game: Game }>
joinLobby(lobbyId: number): Promise<{
  lobby: Lobby
  game_id: number
  already_joined?: boolean
  realtime_sync?: string
}>
addBotToLobby(lobbyId: number, req?: AddBotRequest): Promise<{
  game_id: number
  bot_user_id: number
  bot_username: string
}>
```

**Error Handling:**
```typescript
try {
  const res = await api.createLobby({...})
} catch (e: unknown) {
  setErr(e instanceof Error ? e.message : 'failed to create lobby')
}
```

### 2.3 Types (types.ts)

```typescript
export type Lobby = {
  id: number
  name: string
  host_id: number
  max_players: number
  current_players: number
  status: 'waiting' | 'in_progress' | 'finished'
  created_at: string
}

export type Game = {
  id: number
  lobby_id: number
  status: 'waiting' | 'in_progress' | 'finished'
  current_player_id?: number
  dealer_id?: number
  created_at: string
  finished_at?: string
}
```

---

## 3. WebSocket Message Structure

### 3.1 Connection Flow

**Client → Backend:**
```javascript
// 1. Upgrade HTTP to WebSocket
GET /ws?room=lobby:global HTTP/1.1
Upgrade: websocket
Authorization: Bearer <token>

// 2. Backend upgrades and registers client
// 3. Client receives "connected" ack
{
  "type": "connected",
  "payload": {
    "user_id": 123,
    "room": "lobby:global"
  },
  "timestamp": "2026-01-06T..."
}

// 4. Client explicitly joins game room (redundancy)
{
  "type": "join_room",
  "payload": { "room": "game:42" }
}

// 5. Server acks
{
  "type": "joined_room",
  "payload": { "room": "game:42" }
}
```

### 3.2 Room Structure

**Naming convention:**
- `lobby:global` - All lobby-related broadcasts (initial default)
- `game:123` - Game-specific room (one per game)

**Default room:** `lobby:global` if not specified in query param

### 3.3 Message Format

**All WS messages follow this envelope:**
```json
{
  "type": "string",
  "payload": any,
  "timestamp": "RFC3339Nano"
}
```

**Inbound message structure (client → server):**
```typescript
interface InboundMessage {
  type: string
  payload: Record<string, any>
}
```

### 3.4 Game Update Broadcasting

**When broadcasts occur:**
- After `AddBotToLobbyHandler` completes → `broadcastGameUpdate(db, gameID)`
- After successful move → `broadcastGameUpdate(db, gameID)`

**Broadcast function:**
```go
func broadcastGameUpdate(db *sql.DB, gameID int64) {
  hub.Broadcast("game:"+gameID, "game_update", BuildGameSnapshotPublic(db, gameID))
}
```

**Message sent to all clients in `game:123` room:**
```json
{
  "type": "game_update",
  "payload": {
    "game": Game,
    "players": GamePlayer[],
    "state": CribbageState
  },
  "timestamp": "..."
}
```

### 3.5 Lobby-Specific WebSocket Messages

**Current implementation**: **No lobby-specific WS messages**

Lobbies rely purely on HTTP polling:
- Client calls `listLobbies()` on page load
- No auto-refresh mechanism
- No live player-joined notifications

**Implication**: When a player joins a lobby, other players viewing the lobby list won't see the `current_players` count update until they manually refresh.

---

## 4. State Management

### 4.1 Frontend State Management

**No Redux/Zustand** - Uses React hooks exclusively

**Local component state (hooks):**

**LobbiesPage:**
```typescript
const [lobbies, setLobbies] = useState<Lobby[]>([])
const [loading, setLoading] = useState(false)
const [err, setErr] = useState<string | null>(null)
const [quickBusy, setQuickBusy] = useState(false)
```

**LobbyDetailPage:**
```typescript
const [err, setErr] = useState<string | null>(null)
const [busy, setBusy] = useState(false)
```

**CreateLobbyPage:**
```typescript
const [name, setName] = useState('Lobby')
const [maxPlayers, setMaxPlayers] = useState(2)
const [err, setErr] = useState<string | null>(null)
const [busy, setBusy] = useState(false)
```

**GamePage:**
```typescript
const [snap, setSnap] = useState<GameSnapshot | null>(null)
const [loading, setLoading] = useState(false)
const [err, setErr] = useState<string | null>(null)
const [moveBusy, setMoveBusy] = useState(false)
const [status, setStatus] = useState<string>('disconnected')
```

### 4.2 Data Flow Pattern

**Optimistic + polling:**
1. Client makes HTTP request
2. On success, update local state immediately
3. For game state: WebSocket broadcasts invalidation signal
4. Client re-fetches game snapshot via HTTP (not from WS payload)

**Example:**
```typescript
// In GamePage.tsx
const offUpdate = ws.on('game_update', () => {
  void fetchSnapshot()    // Re-fetch via HTTP, not WS
  void fetchMoves()
})
```

### 4.3 Backend State Management

**Two-tier state system:**

**Tier 1: Database (source of truth)**
- All lobby/game data persisted
- Authoritative for history and recovery

**Tier 2: In-memory game engine (runtime state)**
```go
// GameManager (singleton)
defaultGameManager  // map[gameID]*cribbage.State

// Lock-protected access
st, unlock, ok := defaultGameManager.GetLocked(gameID)
defer unlock()
// use st
defaultGameManager.Set(gameID, st)
```

**When runtime state is missing:**
- Reload from DB `state_json` field
- Restore cards, hands, scoring info
- Persist back to memory for next operation

**Hand persistence:**
- Persisted separately in `game_players.hand` (JSON)
- Restored on player rejoin for UI convenience
- Not authoritative (game state_json is)

---

## 5. UI/UX for Lobby

### 5.1 Current UI Architecture

**Simple, functional design:**

**LobbiesPage Layout:**
```
┌────────────────────────────────────────────┐
│ <h1>Lobbies</h1>  [Create] [Play vs Bot] [Logout] │
├────────────────────────────────────────────┤
│                                            │
│  Lobby Name — 2/4 — waiting [Open]        │
│  Another Lobby — 1/2 — waiting [Open]     │
│                                            │
└────────────────────────────────────────────┘
```

**CreateLobbyPage Layout:**
```
┌────────────────────────────────────────────┐
│ <h1>Create lobby</h1>                      │
├────────────────────────────────────────────┤
│ Name:        [Lobby________]               │
│ Max players: [▼ 2]                         │
│                      [Create]              │
└────────────────────────────────────────────┘
```

**LobbyDetailPage Layout:**
```
┌────────────────────────────────────────────┐
│ <h1>Lobby 123</h1>                         │
│              [Join lobby]                  │
│ error message (if any)                     │
└────────────────────────────────────────────┘
```

### 5.2 Visual Feedback

- **Loading states**: Text "Loading lobbies...", "Creating…", "Joining…"
- **Errors**: Red text (color: crimson)
- **Button states**: `disabled={busy || loading}`
- **No animations**: Minimal CSS, focus on responsiveness

### 5.3 What's Missing (by design)

- No live player counts
- No chat or messaging
- No spectator mode
- No difficulty settings in UI (hardcoded to "easy" for vs-bot)
- No lobby chat or team chat
- No kick/remove player functionality
- No password/private lobbies
- No custom game settings UI

---

## 6. Complete WebSocket Flow Diagram

```
┌─────────────────────────────────────────────────────────────────┐
│                        Browser (Client)                         │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  1. Page load: fetch /api/lobbies → [Lobbies...]               │
│  2. User clicks "Create" → navigate to CreateLobbyPage         │
│  3. Submit form → POST /api/lobbies → {lobby, game}            │
│  4. Redirect to GamePage                                        │
│  5. WS connect to /ws?room=game:42                              │
│  6. Send { type: "join_room", payload: {room: "game:42"} }     │
│  7. Fetch GET /api/games/42 (game snapshot)                    │
│  8. Listen for ws.on("game_update", ...)                        │
│     → Triggers re-fetch of /api/games/42                       │
│  9. User plays card: POST /api/games/42/move                    │
│  10. Server broadcasts game_update to all in game:42            │
│  11. Other clients' game_update listeners fire                  │
│  12. Re-fetch game snapshot                                     │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────┐
│                    Go Server (Backend)                          │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  ListLobbiesHandler(db)                                         │
│  │├─ Query: SELECT * FROM lobbies WHERE status != 'finished'   │
│  │└─ Response: {lobbies: [...]}                                │
│                                                                 │
│  CreateLobbyHandler(db)                                         │
│  │├─ TX: INSERT lobbies, games, game_players                   │
│  │├─ Initialize cribbage.State (Deal)                          │
│  │├─ Persist state_json, hand JSON                             │
│  │├─ Install in defaultGameManager                             │
│  │└─ Response: {lobby, game}                                   │
│                                                                 │
│  JoinLobbyHandler(db)                                           │
│  │├─ TX: UPDATE lobbies SET current_players += 1               │
│  │├─ TX: INSERT game_players                                   │
│  │├─ TX: Persist hand JSON                                     │
│  │├─ Sync runtime state (best-effort)                          │
│  │└─ Response: {lobby, game_id, ...}                           │
│                                                                 │
│  AddBotToLobbyHandler(db)                                       │
│  │├─ TX: INSERT users (bot)                                    │
│  │├─ TX: INSERT game_players (bot)                             │
│  │├─ TX: Persist bot hand                                      │
│  │├─ broadcastGameUpdate() → Hub.Broadcast(...)                │
│  │└─ Response: {game_id, bot_user_id, bot_username}            │
│                                                                 │
│  WebSocketHandler()                                             │
│  │├─ Parse token, extract user_id                              │
│  │├─ Upgrade HTTP → WebSocket                                  │
│  │├─ Create ws.Client, register with Hub                       │
│  │├─ Run client.WritePump() (bg)                               │
│  │├─ Run client.ReadPump() (bg)                                │
│  │└─ Send "connected" ack                                      │
│                                                                 │
│  Hub.Broadcast(room, type, payload)                            │
│  │├─ Find all clients in room                                  │
│  │├─ Serialize payload to JSON                                 │
│  │├─ Send to each client (non-blocking)                        │
│  │└─ Remove dead clients                                       │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────┐
│                    SQLite Database                              │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  tables:                                                        │
│  ├─ users (id, username, password_hash, ...)                  │
│  ├─ lobbies (id, name, host_id, max_players,                  │
│  │           current_players, status, created_at)             │
│  ├─ games (id, lobby_id, status, current_player_id, ...)      │
│  ├─ game_players (game_id, user_id, position, score,          │
│  │               hand, is_bot, bot_difficulty)                │
│  ├─ game_moves (id, game_id, player_id, move_type, ...)       │
│  └─ scoreboard (id, user_id, game_id, final_score, ...)       │
│                                                                 │
│  triggers:                                                      │
│  ├─ trg_lobbies_current_players_after_game_players_insert      │
│  └─ trg_lobbies_current_players_after_game_players_delete      │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

---

## 7. Key Features Currently Implemented

| Feature | Status | Implementation |
|---------|--------|-----------------|
| List active lobbies | ✓ | HTTP GET /api/lobbies |
| Create lobby | ✓ | HTTP POST /api/lobbies |
| Join lobby | ✓ | HTTP POST /api/lobbies/{id}/join |
| Add AI player | ✓ | HTTP POST /api/lobbies/{id}/add_bot |
| Idempotent join | ✓ | Detects existing player, returns 200 |
| WebSocket real-time | ✓ | game_update broadcasts to game room |
| Bot difficulties | ✓ | easy, medium, hard (API only) |
| Lobby status tracking | ✓ | waiting → in_progress → finished |
| Player count sync | ✓ | SQLite triggers on game_players |
| Transaction safety | ✓ | Atomic join + hand persist |
| Runtime state recovery | ✓ | Reload from DB on server restart |
| **Chat in lobby** | ✗ | Not implemented |
| **Live lobby updates** | ✗ | No WS broadcasts for lobby changes |
| **Spectator mode** | ✗ | No spectator implementation |
| **Kick player** | ✗ | No player removal |
| **Custom settings UI** | ✗ | Hardcoded game rules |
| **Lobby password** | ✗ | All lobbies public |

---

## 8. Critical Code Locations

| Component | File | Key Functions |
|-----------|------|-----------------|
| **Lobby model** | `backend/internal/models/lobby.go` | `CreateLobby`, `JoinLobby`, `JoinLobbyTx`, `ListLobbies`, `SetLobbyStatus` |
| **Lobby HTTP handlers** | `backend/internal/handlers/lobby.go` | `ListLobbiesHandler`, `CreateLobbyHandler`, `JoinLobbyHandler`, `AddBotToLobbyHandler` |
| **WebSocket core** | `backend/pkg/websocket/hub.go`, `client.go` | `Hub.Broadcast`, `Client.ReadPump`, `Client.WritePump` |
| **WebSocket handler** | `backend/internal/handlers/websocket.go` | `WebSocketHandler`, `handleWSMessage` |
| **Broadcasting** | `backend/internal/handlers/hub.go` | `broadcastGameUpdate` |
| **Frontend API** | `frontend/src/api/client.ts` | `api.listLobbies`, `api.createLobby`, `api.joinLobby`, `api.addBotToLobby` |
| **Lobbies page** | `frontend/src/pages/LobbiesPage.tsx` | Component, list logic, vs-bot shortcut |
| **Create lobby page** | `frontend/src/pages/CreateLobbyPage.tsx` | Form validation, submission |
| **Lobby detail page** | `frontend/src/pages/LobbyDetailPage.tsx` | Join button, navigation |
| **WS client** | `frontend/src/ws/wsClient.ts` | `WsClient.connect`, `.send`, `.on`, room management |
| **Game page** | `frontend/src/pages/GamePage.tsx` | Game snapshot fetch, WS update listener |
| **Database schema** | `backend/internal/database/migrations/001_init.sql` | Lobby/game tables, triggers |

---

## 9. Error Handling & Edge Cases

### 9.1 Join Lobby Edge Cases

**Scenario**: User joins while lobby reaches max_players

**Behavior** (via transaction isolation):
1. User A tries join (4 expected max_players)
2. Concurrent: User B tries join (3 current_players)
3. User A's UPDATE: current_players < max_players (4 < 4) → succeeds, increment to 4
4. User B's UPDATE: current_players < max_players (4 < 4) fails → check status
5. Return: `ErrLobbyFull` (current_players >= max_players now)

**Transaction prevents race condition**: SQLite's SERIALIZABLE isolation ensures only one update succeeds.

### 9.2 Bot Addition When Lobby Full

```go
nextPos, err := models.AddGamePlayerAutoPositionTx(tx, gameID, botID, l.MaxPlayers, true, &botDiff)
if err != nil && strings.Contains(err.Error(), "could not allocate position") {
  c.JSON(http.StatusBadRequest, gin.H{"error": "lobby full"})
  return
}
```

**Behavior**: Returns 400 with "lobby full" message if no position available.

### 9.3 Server Restart (Runtime State Lost)

**Flow**:
1. Player joins game after server restart
2. `defaultGameManager.GetLocked(gameID)` returns (nil, false)
3. `syncRuntimeStateFromDB()` unmarshals from DB `state_json`
4. Restores full game state into memory
5. Next operation uses in-memory state

**Assumption**: `state_json` is always valid (set during CreateLobby).

### 9.4 Idempotent Rejoin

```typescript
// Client joins, connection drops, rejoins
const res = await api.joinLobby(lobbyId)
// Returns {already_joined: true, realtime_sync: "ok"}
// No error, player transitions to GamePage
```

**Server-side check**:
```go
var existingPos sql.NullInt64
if err := tx.QueryRow(
  `SELECT position FROM game_players WHERE game_id = ? AND user_id = ?`, gameID, userID
).Scan(&existingPos); err == nil {
  // Already joined, sync runtime state, return 200
  resp := gin.H{"already_joined": true, "realtime_sync": "ok"}
  c.JSON(http.StatusOK, resp)
  return
}
```

---

## 10. Performance Characteristics

| Operation | Complexity | Notes |
|-----------|-----------|-------|
| List lobbies | O(n) | Full table scan, limited by LIMIT |
| Create lobby | O(1) | Single INSERT per table + game init |
| Join lobby | O(n) | Atomic UPDATE + trigger recount |
| Add bot | O(n) | CREATE user + UPDATE + broadcast |
| WebSocket broadcast | O(m) | m = clients in room, single pass |
| Room join/leave | O(1) | Map operations |

**Scalability:**
- SQLite: Single-threaded, suitable for ~100 concurrent connections
- WebSocket hub: Unbounded room capacity (in-memory map)
- No pagination on move history (GamePage fetches all moves)
- No pagination on game snapshots

---

## 11. Security Considerations

### 11.1 Authentication
- **Token-based** (JWT/custom)
- Required for all lobbies API
- Required for WebSocket upgrade
- Extracted and validated in middleware

### 11.2 Authorization
- **Host-only bot addition**: `if l.HostID != userID`
- **No lobby privacy**: All lobbies public to authenticated users
- **No admin functions**: No lobby deletion, moderation

### 11.3 Input Validation
```go
// CreateLobby
req.Name = strings.TrimSpace(req.Name)
if req.Name == "" { req.Name = "Lobby" }
if len(req.Name) > 100 { return error }
if req.MaxPlayers < 2 || req.MaxPlayers > 4 { return error }
```

### 11.4 SQL Injection
- **Parameterized queries** throughout (? placeholders)
- No string concatenation in SQL

### 11.5 WebSocket Security
```go
// Origin validation
CheckOrigin: func(r *http.Request) bool {
  origin := r.Header.Get("Origin")
  if origin == "" { return true }  // Non-browser clients OK
  if cfgDevAllowAll() { return true }
  if cfgIsDev() { return isLocalhostOrigin(origin) || isAllowedOrigin(origin) }
  return isAllowedOrigin(origin)
}
```

---

## 12. Testing Observations

**No unit tests visible in codebase for lobby logic.**

**Manual test flow:**
1. Open two browser windows
2. Window A: Create lobby (2 players)
3. Window B: List lobbies, click Open, Join
4. Both: Navigate to GamePage, play game
5. Verify WebSocket updates propagate

---

## 13. Recommendations for Future Enhancement

### Must-Have (for MVP+)
1. **Live lobby updates**: Broadcast `lobby_update` when player joins/quits
2. **Lobby detail page improvements**: Show player list, update counts real-time
3. **Better error messages**: More user-friendly, less generic

### Nice-to-Have (for polish)
4. **Lobby chat**: Simple message broadcast in `lobby:global` room
5. **Player kick**: Host can remove players before game starts
6. **Spectator mode**: Non-players can watch games
7. **Difficulty selection UI**: UI for easy/medium/hard bots
8. **Lobby search/filter**: By name, player count, created date
9. **Stats on lobby list**: Win/loss records visible

### Technical Debt
10. **Consolidate WebSocket messages**: Define message schema (OpenAPI/Protobuf)
11. **Add integration tests**: Full flow from lobby creation to game finish
12. **Logging improvements**: Structured logs, request tracing
13. **Database connection pooling**: Tune pool size for production

---

## 14. Summary Table

```
┌──────────────────────┬─────────────────┬──────────────────────────────┐
│ Layer                │ Technology      │ Status                       │
├──────────────────────┼─────────────────┼──────────────────────────────┤
│ Frontend Pages       │ React + TS      │ 3 pages (Lobbies, Create,    │
│                      │                 │ Detail)                      │
│ Frontend State       │ React Hooks     │ Local component state only   │
│ Frontend WS Client   │ Gorilla API     │ Room-based, event emitter    │
│ API Client           │ Fetch + Zod     │ Type-safe, error handling    │
│ Backend HTTP         │ Gin             │ 4 endpoints (list, create,   │
│                      │                 │ join, add_bot)               │
│ Backend WS           │ Gorilla         │ Hub + Client + ReadPump      │
│ Game State Mgmt      │ In-memory map   │ Lock-protected, DB recovery  │
│ Database             │ SQLite          │ Schema + triggers + indexes  │
│ Transaction Model    │ Serializable    │ Atomic join + hand persist   │
│ Broadcasting         │ Channel-based   │ Async, non-blocking          │
└──────────────────────┴─────────────────┴──────────────────────────────┘
```

