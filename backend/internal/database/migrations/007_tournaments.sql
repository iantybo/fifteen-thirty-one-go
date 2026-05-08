CREATE TABLE IF NOT EXISTS tournaments (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  name TEXT NOT NULL,
  created_by INTEGER,
  status TEXT NOT NULL DEFAULT 'open' CHECK(status IN ('open', 'in_progress', 'completed', 'cancelled')),
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  starts_at TIMESTAMP,
  FOREIGN KEY(created_by) REFERENCES users(id) ON DELETE SET NULL ON UPDATE CASCADE
);

CREATE TABLE IF NOT EXISTS tournament_participants (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  tournament_id INTEGER NOT NULL,
  user_id INTEGER,
  joined_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  seed INTEGER,
  eliminated_at TIMESTAMP,
  UNIQUE (tournament_id, user_id),
  FOREIGN KEY(tournament_id) REFERENCES tournaments(id) ON DELETE CASCADE ON UPDATE CASCADE,
  FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE SET NULL ON UPDATE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_tournament_participants_tournament_id
ON tournament_participants(tournament_id);

CREATE INDEX IF NOT EXISTS idx_tournament_participants_user_id
ON tournament_participants(user_id);
