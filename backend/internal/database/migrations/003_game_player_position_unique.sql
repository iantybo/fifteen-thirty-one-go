PRAGMA foreign_keys = ON;

CREATE UNIQUE INDEX IF NOT EXISTS idx_game_players_game_id_position
ON game_players(game_id, position);


