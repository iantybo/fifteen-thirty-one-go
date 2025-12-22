PRAGMA foreign_keys = ON;

CREATE TABLE IF NOT EXISTS user_preferences (
  user_id INTEGER PRIMARY KEY,
  auto_count_mode TEXT NOT NULL DEFAULT 'suggest', -- off|suggest|auto
  updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  FOREIGN KEY(user_id) REFERENCES users(id)
);


