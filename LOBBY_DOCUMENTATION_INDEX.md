# Lobby Implementation Documentation Index

## Overview

This directory contains comprehensive documentation about the lobby system implementation for the Fifteen Thirty-One Cribbage game platform.

**Last Updated**: January 6, 2026  
**Codebase**: Go backend + React/TypeScript frontend  
**Database**: SQLite with triggers

## Documents

### 1. LOBBY_IMPLEMENTATION.md (Main Reference)
**Size**: 31 KB, 837 lines  
**Purpose**: Complete technical deep-dive

**Contents**:
- Section 1: Backend Lobby Implementation (data model, HTTP endpoints, transactions)
- Section 2: Frontend Lobby Implementation (pages, API client, types)
- Section 3: WebSocket Message Structure (connection flow, message format, broadcasting)
- Section 4: State Management (frontend hooks, backend two-tier system)
- Section 5: UI/UX for Lobby (layouts, visual feedback, missing features)
- Section 6: WebSocket Flow Diagram (complete flow from browser to database)
- Section 7: Features Matrix (implemented vs missing)
- Section 8: Critical Code Locations (file references with key functions)
- Section 9: Error Handling & Edge Cases (race conditions, retries, recovery)
- Section 10: Performance Characteristics (complexity analysis, scalability)
- Section 11: Security Considerations (auth, authorization, input validation, SQL injection)
- Section 12: Testing Observations (current state, manual flow)
- Section 13: Recommendations for Enhancement (must-have, nice-to-have, technical debt)
- Section 14: Summary Table (layer breakdown by technology)

**Best For**: Understanding the complete architecture, diving into specific components, tracing data flows

### 2. LOBBY_QUICK_REFERENCE.md (Lookup Guide)
**Size**: 7.2 KB, 325 lines  
**Purpose**: Fast lookups during development

**Contents**:
- API Endpoints Summary (4 endpoints with quick descriptions)
- Key Files (with line counts and purposes)
- Database Schema (essential tables with key fields)
- HTTP Request/Response Examples (curl-style examples for all endpoints)
- WebSocket Flow (step-by-step message sequence)
- State Transitions (lobby and game status flows)
- Common Flows (3 typical user journeys)
- Error Responses (status codes and causes)
- Key Design Decisions (explained with rationale)
- Frontend State Pattern (React hooks example)
- Room Naming Convention (WebSocket room names)
- Debugging Tips (SQL queries, goroutine monitoring)
- Performance Notes (complexity of common operations)

**Best For**: Quick lookups, API testing, onboarding new developers, debugging

## Architecture Overview

### High Level
```
Frontend (React/TS)
  ├─ LobbiesPage (list, create shortcut)
  ├─ CreateLobbyPage (form)
  ├─ LobbyDetailPage (join button)
  └─ API Client + WS Client
        ↓ HTTP
Backend (Go/Gin)
  ├─ HTTP Handlers (list, create, join, add_bot)
  ├─ WebSocket Hub (broadcasting)
  └─ Game Manager (in-memory state)
        ↓ SQL
Database (SQLite)
  ├─ lobbies (core data)
  ├─ games (1:1 per lobby)
  ├─ game_players (roster with positions)
  └─ Triggers (maintain current_players count)
```

### Critical Flows

**Create Lobby**
```
POST /api/lobbies
  → TX: INSERT lobbies, games, game_players
  → Initialize cribbage.State (Deal)
  → Persist state_json, hand
  → Install in defaultGameManager
  → Response: {lobby, game}
```

**Join Lobby**
```
POST /api/lobbies/{id}/join
  → TX: UPDATE lobbies SET current_players += 1
  → TX: INSERT game_players
  → TX: Persist hand from state_json
  → Sync runtime state (best-effort)
  → Response: {lobby, game_id, already_joined?, realtime_sync?}
```

**Add Bot**
```
POST /api/lobbies/{id}/add_bot
  → TX: INSERT users (bot)
  → TX: INSERT game_players
  → TX: Persist bot hand
  → Broadcast game_update
  → Response: {game_id, bot_user_id, bot_username}
```

**Game Update (Real-time)**
```
Player joins game room
  → WS /ws?room=game:123
  → Receive "connected" ack
  → Send "join_room" message
  → Listen for game_update broadcasts
  → On update: Re-fetch game snapshot via HTTP
```

## Key Design Decisions

1. **One Game per Lobby** - Simplifies state management with clear 1:1 mapping
2. **Denormalized Player Count** - Avoids COUNT queries via SQLite triggers
3. **HTTP + WS Hybrid** - HTTP is source of truth, WS provides notifications
4. **Transactional Joins** - Atomic increment + player add prevents race conditions
5. **Lock Ordering** - DB lock acquired first, memory sync after commit
6. **Two-Tier State** - DB authoritative, memory cache for performance
7. **No Lobby WS Broadcasts** - HTTP polling simplifies initial implementation

## File Navigation

### Backend
- `backend/internal/models/lobby.go` - Core lobby CRUD functions
- `backend/internal/handlers/lobby.go` - HTTP handlers for all 4 endpoints
- `backend/internal/handlers/websocket.go` - WS upgrade and message routing
- `backend/internal/handlers/hub.go` - Broadcasting coordinator
- `backend/pkg/websocket/hub.go` - Hub implementation (rooms, clients)
- `backend/pkg/websocket/client.go` - Client pumps (read/write loops)
- `backend/internal/database/migrations/001_init.sql` - Base schema, triggers
- `backend/internal/database/migrations/006_lobby_improvements.sql` - Lobby enhancements (chat, spectators, presence)

### Frontend
- `frontend/src/pages/LobbiesPage.tsx` - Main lobby list page
- `frontend/src/pages/CreateLobbyPage.tsx` - Create form
- `frontend/src/pages/LobbyDetailPage.tsx` - Detail + join
- `frontend/src/pages/GamePage.tsx` - Game board (receives game_update broadcasts)
- `frontend/src/api/client.ts` - HTTP API methods
- `frontend/src/api/types.ts` - TypeScript types
- `frontend/src/ws/wsClient.ts` - Custom WebSocket client

## Common Tasks

### I want to...

**Understand how lobbies are created**
→ Read LOBBY_IMPLEMENTATION.md Section 1.2 (CreateLobbyHandler) + Quick Reference "Flow 1"

**Debug a join failure**
→ Check Quick Reference "Error Responses" table + Section 9 (Edge Cases)

**Add a new WebSocket message type**
→ See LOBBY_IMPLEMENTATION.md Section 3.3 (Message Format) + quickref "WebSocket Flow"

**Understand state recovery after restart**
→ LOBBY_IMPLEMENTATION.md Section 4.3 + Section 9.3 (Server Restart)

**Write a test for lobby creation**
→ Read Section 8 (Code Locations) for test entry points + Section 12 (Testing Observations)

**Performance tune lobby listing**
→ Check Section 10 (Performance) + Quick Reference "Performance Notes"

**Add live lobby update notifications**
→ See Section 13 (Recommendations) - marked as "Must-Have"

**Monitor WebSocket connections**
→ Quick Reference "Debugging Tips" section

## API Quick Reference

```bash
# List lobbies
GET /api/lobbies?limit=50&offset=0
Authorization: Bearer <token>

# Create lobby
POST /api/lobbies
{"name": "My Game", "max_players": 2}

# Join lobby
POST /api/lobbies/42/join

# Add bot
POST /api/lobbies/42/add_bot
{"difficulty": "easy"}

# WebSocket
GET /ws?room=game:99
Upgrade: websocket
```

## Database Queries

### Check Active Lobbies
```sql
SELECT id, name, current_players, max_players, status 
FROM lobbies 
WHERE status != 'finished';
```

### Check Game Players
```sql
SELECT gp.game_id, u.username, gp.position, gp.is_bot, gp.bot_difficulty 
FROM game_players gp 
JOIN users u ON gp.user_id = u.id 
WHERE gp.game_id = ?;
```

### Check Game State
```sql
SELECT id, lobby_id, status, 
       JSON_EXTRACT(state_json, '$.stage') as stage,
       state_version
FROM games 
WHERE id = ?;
```

See Quick Reference for more debugging queries.

## Performance Characteristics

| Operation | Complexity | Notes |
|-----------|-----------|-------|
| List lobbies | O(n) | Limited to 200 rows by LIMIT |
| Create lobby | O(1) | Single INSERT per table + game init |
| Join lobby | O(n) | Atomic UPDATE + trigger recount |
| Add bot | O(n) | CREATE user + UPDATE + broadcast |
| WS broadcast | O(m) | m = clients in room, single pass |
| Room join | O(1) | Map operation |

## Security Checklist

- [x] Token-based authentication required
- [x] Authorization checks (host-only bot addition)
- [x] Input validation (name length, player count)
- [x] Parameterized SQL queries (no injection)
- [x] WebSocket origin validation
- [ ] Rate limiting on lobby creation
- [ ] DDoS protection (not implemented)
- [ ] Bot account lockout (not implemented)

## Recommendations

### Must-Have (MVP+)
1. Live lobby updates via WebSocket
2. Lobby detail page improvements
3. Better error messages

### Nice-to-Have (Polish)
4. Lobby chat
5. Player kick functionality
6. Spectator mode
7. Difficulty selection UI

### Technical Debt
8. Consolidate WebSocket messages schema
9. Add integration tests
10. Logging improvements
11. Database connection pooling

See LOBBY_IMPLEMENTATION.md Section 13 for detailed recommendations.

## Related Documentation

- Game implementation: (if exists) GAME_IMPLEMENTATION.md
- Database schema: backend/internal/database/migrations/
- Frontend setup: frontend/README.md
- Backend setup: backend/README.md

## Version History

| Date | Version | Changes |
|------|---------|---------|
| 2026-01-06 | 1.0 | Initial comprehensive documentation |

## Questions?

Refer to the main LOBBY_IMPLEMENTATION.md document. It contains:
- Complete code examples
- Error handling details
- Data flow diagrams
- Security analysis
- Performance analysis
- Test guidance

