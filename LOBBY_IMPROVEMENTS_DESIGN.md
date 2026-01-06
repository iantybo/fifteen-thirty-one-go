# Lobby Improvements Design - Yahoo Games Inspired

## Overview
This document outlines improvements to the lobby experience inspired by Yahoo Games features.

## Key Yahoo Games Features
1. **Ante Room System** - Pre-game lobbies with skill levels (social, intermediate, advanced)
2. **Spectator Mode** - Watch games without playing
3. **Chat Functionality** - Public lobby chat and private messages
4. **Player Profiles** - Usernames, avatars, stats visibility
5. **Real-time Updates** - Live lobby list and player status
6. **Room Categories** - Filter by skill level, game type
7. **Game Rules Access** - Help/rules visible in lobby

## Proposed Improvements

### 1. Database Schema Enhancements

#### New Tables
```sql
-- Lobby chat messages
CREATE TABLE lobby_messages (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  lobby_id INTEGER NOT NULL,
  user_id INTEGER,
  username TEXT NOT NULL,
  message TEXT NOT NULL,
  message_type TEXT NOT NULL DEFAULT 'chat' CHECK(message_type IN ('chat', 'system', 'join', 'leave')),
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  FOREIGN KEY(lobby_id) REFERENCES lobbies(id) ON DELETE CASCADE,
  FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE SET NULL
);

-- Spectators
CREATE TABLE lobby_spectators (
  lobby_id INTEGER NOT NULL,
  user_id INTEGER NOT NULL,
  joined_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (lobby_id, user_id),
  FOREIGN KEY(lobby_id) REFERENCES lobbies(id) ON DELETE CASCADE,
  FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE
);

-- Player presence
CREATE TABLE user_presence (
  user_id INTEGER PRIMARY KEY,
  status TEXT NOT NULL DEFAULT 'online' CHECK(status IN ('online', 'away', 'in_game', 'offline')),
  last_seen TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  current_lobby_id INTEGER,
  FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE,
  FOREIGN KEY(current_lobby_id) REFERENCES lobbies(id) ON DELETE SET NULL
);
```

#### Enhanced Lobbies Table
```sql
ALTER TABLE lobbies ADD COLUMN skill_level TEXT DEFAULT 'social' CHECK(skill_level IN ('social', 'intermediate', 'advanced'));
ALTER TABLE lobbies ADD COLUMN is_private BOOLEAN DEFAULT 0;
ALTER TABLE lobbies ADD COLUMN password_hash TEXT;
ALTER TABLE lobbies ADD COLUMN description TEXT;
ALTER TABLE lobbies ADD COLUMN allow_spectators BOOLEAN DEFAULT 1;
```

#### Enhanced Users Table
```sql
ALTER TABLE users ADD COLUMN avatar_url TEXT;
ALTER TABLE users ADD COLUMN skill_rating INTEGER DEFAULT 1000;
ALTER TABLE users ADD COLUMN bio TEXT;
```

### 2. Backend Features

#### WebSocket Events (New)
- `lobby:list_updated` - Broadcast when lobby list changes
- `lobby:player_joined` - Player joins lobby
- `lobby:player_left` - Player leaves lobby
- `lobby:spectator_joined` - Spectator joins
- `lobby:spectator_left` - Spectator leaves
- `lobby:chat_message` - Chat message in lobby
- `lobby:status_changed` - Lobby status update
- `player:presence_changed` - Player online status

#### API Endpoints (New)
- `POST /api/lobbies/{id}/chat` - Send chat message
- `GET /api/lobbies/{id}/chat` - Get chat history
- `POST /api/lobbies/{id}/spectate` - Join as spectator
- `DELETE /api/lobbies/{id}/spectate` - Leave spectator mode
- `GET /api/lobbies/{id}/spectators` - List spectators
- `PUT /api/users/presence` - Update user presence
- `GET /api/users/{id}/profile` - Get user profile
- `PUT /api/users/profile` - Update own profile

#### Handlers
- `lobby_chat_handler.go` - Chat functionality
- `spectator_handler.go` - Spectator mode
- `presence_handler.go` - User presence tracking
- `profile_handler.go` - User profiles

### 3. Frontend Features

#### New Components
- `LobbyChat.tsx` - Chat interface component
- `PlayerCard.tsx` - Player profile card with avatar and stats
- `SpectatorPanel.tsx` - Spectator list and controls
- `LobbyFilters.tsx` - Filter/search lobbies
- `PlayerPresence.tsx` - Online status indicator
- `QuickJoin.tsx` - One-click join available room
- `RoomSettings.tsx` - Lobby settings display

#### Enhanced Components
- `LobbiesPage.tsx` - Add filters, live updates, better UI
- `LobbyDetailPage.tsx` - Add chat, spectators, player cards
- `CreateLobbyPage.tsx` - Add skill level, privacy settings

#### Real-time Features
- Live lobby list updates via WebSocket
- Chat message delivery
- Player join/leave notifications
- Spectator count updates
- Presence status updates

### 4. UI/UX Improvements

#### Lobby List Page
- Card-based layout with status indicators
- Filter by: skill level, status, player count
- Search by lobby name
- Quick join button for open rooms
- Player avatars preview
- Live player count updates
- Status badges (waiting, in progress, full)

#### Lobby Detail Page
- Two-panel layout: left (players/spectators), right (chat)
- Player cards with:
  - Avatar
  - Username
  - Stats (games won/played)
  - Ready status
  - Kick button (host only)
- Spectator list (collapsible)
- Chat panel with:
  - Message history
  - Input field
  - System messages (joins/leaves)
  - Timestamps
- Room settings panel:
  - Skill level
  - Max players
  - Description
  - Rules link

#### Player Profiles
- Avatar upload
- Bio/description
- Stats display
- Skill rating
- Recent games

### 5. Security Considerations

- Rate limiting on chat messages (5 msgs/10 sec)
- Profanity filter
- Report/block functionality
- Password hashing for private rooms
- Spectator permissions (can't access player hands)
- CSRF protection on all endpoints

### 6. Performance Optimizations

- Chat message pagination (last 50 messages)
- Lobby list caching (Redis)
- WebSocket connection pooling
- Database indexes on new tables
- Debounced presence updates

## Implementation Priority

### Phase 1 (High Priority)
1. Real-time lobby list updates
2. Lobby chat functionality
3. Enhanced lobby UI with filters
4. Player presence indicators

### Phase 2 (Medium Priority)
5. Spectator mode
6. Player profiles with avatars
7. Private/password-protected rooms
8. Skill level system

### Phase 3 (Nice to Have)
9. Private messaging
10. Friend lists
11. Achievements/badges
12. Game rules viewer

## Migration Plan

1. Create migration `006_lobby_improvements.sql` with new tables/columns
2. Update backend models and handlers
3. Add WebSocket event types
4. Implement API endpoints
5. Create frontend components
6. Update existing components
7. Add tests
8. Deploy with feature flags
