CREATE TABLE IF NOT EXISTS schema_migrations (
  version TEXT PRIMARY KEY,
  applied_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- users
CREATE TABLE IF NOT EXISTS users (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  username TEXT NOT NULL UNIQUE,
  password_hash TEXT NOT NULL,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  games_played INTEGER NOT NULL DEFAULT 0,
  games_won INTEGER NOT NULL DEFAULT 0
);

-- lobbies
CREATE TABLE IF NOT EXISTS lobbies (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  name TEXT NOT NULL,
  host_id INTEGER NOT NULL,
  max_players INTEGER NOT NULL,
  current_players INTEGER NOT NULL DEFAULT 0,
  status TEXT NOT NULL DEFAULT 'waiting' CHECK(status IN ('waiting', 'in_progress', 'finished')),
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  FOREIGN KEY(host_id) REFERENCES users(id) ON DELETE CASCADE ON UPDATE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_lobbies_host_id ON lobbies(host_id);

-- games
CREATE TABLE IF NOT EXISTS games (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  lobby_id INTEGER NOT NULL,
  status TEXT NOT NULL DEFAULT 'waiting' CHECK(status IN ('waiting', 'playing', 'finished')),
  current_player_id INTEGER,
  dealer_id INTEGER,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  finished_at TIMESTAMP,
  FOREIGN KEY(lobby_id) REFERENCES lobbies(id) ON DELETE CASCADE ON UPDATE CASCADE,
  FOREIGN KEY(current_player_id) REFERENCES users(id) ON DELETE SET NULL ON UPDATE CASCADE,
  FOREIGN KEY(dealer_id) REFERENCES users(id) ON DELETE SET NULL ON UPDATE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_games_lobby_id ON games(lobby_id);

-- game_players
CREATE TABLE IF NOT EXISTS game_players (
  game_id INTEGER NOT NULL,
  user_id INTEGER NOT NULL,
  position INTEGER NOT NULL,
  score INTEGER NOT NULL DEFAULT 0,
  hand TEXT NOT NULL DEFAULT '[]',
  crib_cards TEXT,
  is_bot BOOLEAN NOT NULL DEFAULT 0,
  bot_difficulty TEXT,
  PRIMARY KEY (game_id, user_id),
  FOREIGN KEY(game_id) REFERENCES games(id) ON DELETE CASCADE ON UPDATE CASCADE,
  FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE ON UPDATE CASCADE
);

-- Ensure each position within a game is assigned to at most one user.
CREATE UNIQUE INDEX IF NOT EXISTS idx_game_players_game_id_position
ON game_players(game_id, position);

CREATE INDEX IF NOT EXISTS idx_game_players_user_id ON game_players(user_id);

-- Keep lobbies.current_players consistent with game_players rows (via games.lobby_id).
-- This defends against drift and makes the denormalized count reliable.
CREATE TRIGGER IF NOT EXISTS trg_lobbies_current_players_after_game_players_insert
AFTER INSERT ON game_players
BEGIN
  UPDATE lobbies
    SET current_players = (
      SELECT COUNT(*)
      FROM game_players gp
      WHERE gp.game_id = (
        -- Count only the lobby's active game (waiting/playing), not historical games.
        SELECT id
        FROM games
        WHERE lobby_id = (SELECT lobby_id FROM games WHERE id = NEW.game_id)
          AND status IN ('waiting', 'playing')
        ORDER BY id DESC
        LIMIT 1
      )
    )
  WHERE id = (SELECT lobby_id FROM games WHERE id = NEW.game_id);
END;

CREATE TRIGGER IF NOT EXISTS trg_lobbies_current_players_after_game_players_delete
AFTER DELETE ON game_players
BEGIN
  UPDATE lobbies
    SET current_players = (
      SELECT COUNT(*)
      FROM game_players gp
      WHERE gp.game_id = (
        -- Count only the lobby's active game (waiting/playing), not historical games.
        SELECT id
        FROM games
        WHERE lobby_id = (SELECT lobby_id FROM games WHERE id = OLD.game_id)
          AND status IN ('waiting', 'playing')
        ORDER BY id DESC
        LIMIT 1
      )
    )
  WHERE id = (SELECT lobby_id FROM games WHERE id = OLD.game_id);
END;

-- game_moves
CREATE TABLE IF NOT EXISTS game_moves (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  game_id INTEGER NOT NULL,
  player_id INTEGER NOT NULL,
  move_type TEXT NOT NULL,
  card_played TEXT,
  score_claimed INTEGER,
  score_verified INTEGER,
  is_corrected BOOLEAN NOT NULL DEFAULT 0,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  FOREIGN KEY(game_id) REFERENCES games(id) ON DELETE CASCADE ON UPDATE CASCADE,
  FOREIGN KEY(player_id) REFERENCES users(id) ON DELETE CASCADE ON UPDATE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_game_moves_game_id_created_at ON game_moves(game_id, created_at);
CREATE INDEX IF NOT EXISTS idx_game_moves_player_id ON game_moves(player_id);

-- scoreboard
CREATE TABLE IF NOT EXISTS scoreboard (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  user_id INTEGER NOT NULL,
  game_id INTEGER NOT NULL,
  final_score INTEGER NOT NULL,
  position INTEGER NOT NULL,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE ON UPDATE CASCADE,
  FOREIGN KEY(game_id) REFERENCES games(id) ON DELETE CASCADE ON UPDATE CASCADE
);

-- Ensure each final standings position within a game is assigned to at most one user.
CREATE UNIQUE INDEX IF NOT EXISTS idx_scoreboard_game_id_position
ON scoreboard(game_id, position);

CREATE INDEX IF NOT EXISTS idx_scoreboard_user_id_created_at ON scoreboard(user_id, created_at);


