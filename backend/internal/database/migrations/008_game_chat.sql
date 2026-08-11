-- Migration 008: In-game chat messages
-- Adds game-scoped chat history for players.

CREATE TABLE IF NOT EXISTS game_messages (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  game_id INTEGER NOT NULL,
  user_id INTEGER,
  username TEXT NOT NULL,
  message TEXT NOT NULL,
  message_type TEXT NOT NULL DEFAULT 'chat' CHECK(message_type IN ('chat', 'system')),
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  FOREIGN KEY(game_id) REFERENCES games(id) ON DELETE CASCADE ON UPDATE CASCADE,
  FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE SET NULL ON UPDATE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_game_messages_game_id_created_at ON game_messages(game_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_game_messages_user_id ON game_messages(user_id);


