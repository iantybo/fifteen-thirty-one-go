CREATE TABLE IF NOT EXISTS game_events (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    game_id INTEGER NOT NULL REFERENCES games(id) ON DELETE CASCADE ON UPDATE CASCADE,
    event_type TEXT NOT NULL,
    player_id INTEGER REFERENCES users(id) ON DELETE SET NULL ON UPDATE CASCADE,
    metadata TEXT NOT NULL DEFAULT '{}',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_game_events_game_id ON game_events(game_id);
CREATE INDEX IF NOT EXISTS idx_game_events_event_type ON game_events(event_type);
