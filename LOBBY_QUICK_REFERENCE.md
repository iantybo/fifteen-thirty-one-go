# Lobby Implementation - Quick Reference

## API Endpoints Summary

```
GET    /api/lobbies                    → List lobbies
POST   /api/lobbies                    → Create lobby
POST   /api/lobbies/{id}/join          → Join lobby
POST   /api/lobbies/{id}/add_bot       → Add AI player
GET    /ws                             → WebSocket upgrade
```

## Key Files

| File | Purpose | LOC |
|------|---------|-----|
| backend/internal/models/lobby.go | Lobby CRUD, DB queries | ~180 |
| backend/internal/handlers/lobby.go | HTTP handlers | ~650 |
| backend/pkg/websocket/hub.go | Room broadcasting | ~190 |
| backend/pkg/websocket/client.go | WS client pump loops | ~110 |
| frontend/src/pages/LobbiesPage.tsx | Lobby listing | ~85 |
| frontend/src/pages/CreateLobbyPage.tsx | Create form | ~70 |
| frontend/src/pages/LobbyDetailPage.tsx | Detail + join | ~50 |
| frontend/src/ws/wsClient.ts | WS client | ~100 |
| frontend/src/api/client.ts | HTTP client | ~105 |

## Database Schema (Essential)

```sql
lobbies
├─ id (PK)
├─ name
├─ host_id (FK → users)
├─ max_players
├─ current_players (denormalized, via triggers)
├─ status (waiting|in_progress|finished)
└─ created_at

games
├─ id (PK)
├─ lobby_id (FK → lobbies, 1:1)
├─ status
├─ current_player_id
├─ dealer_id
└─ state_json (full game state)

game_players
├─ game_id (FK → games)
├─ user_id (FK → users)
├─ position (0-3)
├─ score
├─ hand (JSON array)
├─ is_bot
└─ bot_difficulty
```

## HTTP Request/Response Examples

### Create Lobby
```bash
POST /api/lobbies
Content-Type: application/json
Authorization: Bearer <token>

{
  "name": "My Game",
  "max_players": 2
}

# Response (201 Created)
{
  "lobby": {
    "id": 42,
    "name": "My Game",
    "host_id": 1,
    "max_players": 2,
    "current_players": 1,
    "status": "waiting",
    "created_at": "2026-01-06T12:34:56Z"
  },
  "game": {
    "id": 99,
    "lobby_id": 42,
    "status": "waiting",
    "created_at": "2026-01-06T12:34:56Z"
  }
}
```

### List Lobbies
```bash
GET /api/lobbies?limit=50&offset=0
Authorization: Bearer <token>

# Response (200 OK)
{
  "lobbies": [
    {
      "id": 42,
      "name": "My Game",
      "host_id": 1,
      "max_players": 2,
      "current_players": 2,
      "status": "in_progress",
      "created_at": "2026-01-06T12:34:56Z"
    }
  ]
}
```

### Join Lobby
```bash
POST /api/lobbies/42/join
Authorization: Bearer <token>

# Response (200 OK)
{
  "lobby": {...},
  "game_id": 99,
  "already_joined": false,
  "joined_persisted": true,
  "realtime_sync": "ok"
}
```

### Add Bot
```bash
POST /api/lobbies/42/add_bot
Content-Type: application/json
Authorization: Bearer <token>

{
  "difficulty": "easy"
}

# Response (200 OK)
{
  "game_id": 99,
  "bot_user_id": 123,
  "bot_username": "bot_easy_42_abc123def"
}
```

## WebSocket Flow

```
1. Connect: GET /ws?room=game:99
   ↓ (receive)
2. {"type": "connected", "payload": {user_id: 1, room: "game:99"}}
   ↑ (send)
3. {"type": "join_room", "payload": {room: "game:99"}}
   ↓ (receive)
4. {"type": "joined_room", "payload": {room: "game:99"}}
   ↓ (receive when opponent moves)
5. {"type": "game_update", "payload": {game: {...}, players: [...], state: {...}}}
```

## State Transitions

### Lobby Status
```
waiting
  ↓ (game starts)
in_progress
  ↓ (game ends)
finished
```

### Game Status
```
waiting (players joining)
  ↓ (all players ready or timeout)
in_progress (dealing, pegging, counting)
  ↓ (winner reached 121 pts)
finished
```

## Common Flows

### Flow 1: Create & Play vs Bot
```
1. POST /api/lobbies (create 2-player lobby)
2. POST /api/lobbies/{id}/add_bot (add bot)
3. Navigate to game page
4. WS /ws?room=game:{gameId}
5. Fetch /api/games/{gameId}
6. Listen for game_update broadcasts
7. POST /api/games/{gameId}/move
8. Broadcast triggers re-fetch
```

### Flow 2: Join Existing Lobby
```
1. GET /api/lobbies (list)
2. Click "Open" on a lobby
3. Navigate to /lobbies/{id}
4. Click "Join"
5. POST /api/lobbies/{id}/join
6. Redirect to game page
7. Same WS flow as above
```

### Flow 3: Rejoin After Disconnect
```
1. WS disconnect
2. POST /api/lobbies/{id}/join (same player)
3. Server detects already_joined: true
4. Returns current game state
5. WS reconnect to game room
6. Fetch latest snapshot
```

## Error Responses

| Status | Message | Cause |
|--------|---------|-------|
| 400 | "invalid json" | Malformed JSON |
| 400 | "max_players must be 2-4" | Invalid player count |
| 400 | "name must be <= 100 characters" | Name too long |
| 400 | "lobby full" | Current >= max_players |
| 400 | "lobby not joinable" | Status != waiting |
| 404 | "lobby not found" | Lobby doesn't exist |
| 401 | "unauthorized" | Missing/invalid token |
| 403 | "only host can add bots" | Non-host adding bot |

## Key Design Decisions

### 1. One Game per Lobby
- Simplifies state management
- Clear 1:1 mapping
- Easier to reason about player positions

### 2. Denormalized Player Count
```go
lobbies.current_players  // maintained by triggers on game_players
```
**Why**: Avoid expensive COUNT queries during listing

### 3. HTTP + WS Hybrid
- HTTP: source of truth, idempotent, persistent
- WS: notifications, real-time updates, best-effort
- Client re-fetches on WS update (doesn't trust WS payload alone)

### 4. Transactional Joins
```go
TX.Update(lobbies SET current_players += 1)
TX.Insert(game_players)
TX.Insert/Update(hand)
TX.Commit()
```
**Why**: Atomic increment + player add prevents race conditions

### 5. Lock Ordering: DB → Memory
- DB lock acquired first
- Memory sync after commit
- Prevents deadlocks

### 6. Runtime State Persistence
- DB always authoritative
- Memory cache for performance
- Automatic recovery on server restart

### 7. No Lobby WS Broadcasts (currently)
- Lobbies are listed via HTTP only
- No "live join" notifications
- Simplifies initial implementation

## Frontend State Pattern

**No Redux** - just React hooks:
```typescript
const [lobbies, setLobbies] = useState<Lobby[]>([])
const [loading, setLoading] = useState(false)
const [err, setErr] = useState<string | null>(null)

useEffect(() => {
  api.listLobbies()
    .then(res => setLobbies(res.lobbies))
    .catch(e => setErr(e.message))
}, [])
```

## Room Naming Convention

```
lobby:global    ← All lobby announcements (not currently used)
game:1          ← Game 1 specific updates
game:2          ← Game 2 specific updates
...
game:N
```

## Debugging Tips

### Check Active Lobbies
```sql
SELECT id, name, current_players, max_players, status FROM lobbies WHERE status != 'finished';
```

### Check Game Players
```sql
SELECT gp.game_id, u.username, gp.position, gp.is_bot, gp.bot_difficulty 
FROM game_players gp 
JOIN users u ON gp.user_id = u.id 
WHERE gp.game_id = ?;
```

### Check WS Clients
- Look at `hub.rooms` map in memory (no direct query)
- Monitor goroutine count (each client = 2 goroutines)

### Check Game State
```sql
SELECT id, lobby_id, status, state_json FROM games WHERE id = ?;
```
Then `SELECT JSON_EXTRACT(state_json, '$.stage')` etc.

## Performance Notes

- Listing lobbies: O(n) but limited to 200 rows
- Creating lobby: O(1) + game init
- Joining: O(n) due to trigger recount
- WebSocket broadcast: O(m) where m = clients in room
- No pagination on moves or snapshots

