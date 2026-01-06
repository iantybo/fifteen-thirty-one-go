-- Migration 006: Lobby Improvements (Yahoo Games inspired)
-- Adds chat, spectators, presence, profiles, and enhanced lobby features

-- Add new columns to lobbies table
ALTER TABLE lobbies ADD COLUMN skill_level TEXT DEFAULT 'social' CHECK(skill_level IN ('social', 'intermediate', 'advanced'));
ALTER TABLE lobbies ADD COLUMN is_private BOOLEAN DEFAULT 0;
ALTER TABLE lobbies ADD COLUMN password_hash TEXT;
ALTER TABLE lobbies ADD COLUMN description TEXT;
ALTER TABLE lobbies ADD COLUMN allow_spectators BOOLEAN DEFAULT 1;

-- Add new columns to users table for profiles
ALTER TABLE users ADD COLUMN avatar_url TEXT;
ALTER TABLE users ADD COLUMN skill_rating INTEGER DEFAULT 1000;
ALTER TABLE users ADD COLUMN bio TEXT;
ALTER TABLE users ADD COLUMN last_seen TIMESTAMP DEFAULT CURRENT_TIMESTAMP;

-- Lobby chat messages
CREATE TABLE IF NOT EXISTS lobby_messages (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  lobby_id INTEGER NOT NULL,
  user_id INTEGER,
  username TEXT NOT NULL,
  message TEXT NOT NULL,
  message_type TEXT NOT NULL DEFAULT 'chat' CHECK(message_type IN ('chat', 'system', 'join', 'leave')),
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  FOREIGN KEY(lobby_id) REFERENCES lobbies(id) ON DELETE CASCADE ON UPDATE CASCADE,
  FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE SET NULL ON UPDATE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_lobby_messages_lobby_id_created_at ON lobby_messages(lobby_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_lobby_messages_user_id ON lobby_messages(user_id);

-- Spectators for lobbies
CREATE TABLE IF NOT EXISTS lobby_spectators (
  lobby_id INTEGER NOT NULL,
  user_id INTEGER NOT NULL,
  joined_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (lobby_id, user_id),
  FOREIGN KEY(lobby_id) REFERENCES lobbies(id) ON DELETE CASCADE ON UPDATE CASCADE,
  FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE ON UPDATE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_lobby_spectators_user_id ON lobby_spectators(user_id);

-- User presence tracking
CREATE TABLE IF NOT EXISTS user_presence (
  user_id INTEGER PRIMARY KEY,
  status TEXT NOT NULL DEFAULT 'online' CHECK(status IN ('online', 'away', 'in_game', 'offline')),
  last_active TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  current_lobby_id INTEGER,
  FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE ON UPDATE CASCADE,
  FOREIGN KEY(current_lobby_id) REFERENCES lobbies(id) ON DELETE SET NULL ON UPDATE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_user_presence_status ON user_presence(status);
CREATE INDEX IF NOT EXISTS idx_user_presence_current_lobby_id ON user_presence(current_lobby_id);

-- Trigger to auto-update user presence on lobby join
CREATE TRIGGER IF NOT EXISTS trg_update_presence_on_join
AFTER INSERT ON game_players
BEGIN
  INSERT INTO user_presence (user_id, status, current_lobby_id)
  VALUES (
    NEW.user_id,
    'in_game',
    (SELECT lobby_id FROM games WHERE id = NEW.game_id)
  )
  ON CONFLICT(user_id) DO UPDATE SET
    status = 'in_game',
    current_lobby_id = (SELECT lobby_id FROM games WHERE id = NEW.game_id),
    last_active = CURRENT_TIMESTAMP;
END;

-- Trigger to auto-update user presence on lobby leave
CREATE TRIGGER IF NOT EXISTS trg_update_presence_on_leave
AFTER DELETE ON game_players
WHEN NOT EXISTS (SELECT 1 FROM game_players WHERE user_id = OLD.user_id)
BEGIN
  UPDATE user_presence
  SET status = 'online',
      current_lobby_id = NULL,
      last_active = CURRENT_TIMESTAMP
  WHERE user_id = OLD.user_id;
END;
