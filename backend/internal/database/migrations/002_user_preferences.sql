CREATE TABLE IF NOT EXISTS user_preferences (
  user_id INTEGER PRIMARY KEY,
  auto_count_mode TEXT NOT NULL DEFAULT 'suggest' CHECK(auto_count_mode IN ('off', 'suggest', 'auto')), -- off|suggest|auto
  updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE ON UPDATE CASCADE
);

-- Keep updated_at current on row modifications (SQLite does not auto-update DEFAULT values).
CREATE TRIGGER IF NOT EXISTS user_preferences_set_updated_at
AFTER UPDATE ON user_preferences
FOR EACH ROW
WHEN NEW.updated_at = OLD.updated_at
BEGIN
  UPDATE user_preferences
  SET updated_at = CURRENT_TIMESTAMP
  WHERE user_id = NEW.user_id;
END;


