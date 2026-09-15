-- Signups: expressions of interest captured before (or independently of) a
-- full user account. Distinct from `users`, which backs authentication.

CREATE TABLE IF NOT EXISTS signups (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  -- Stored lowercased and trimmed so uniqueness is case-insensitive.
  email TEXT NOT NULL UNIQUE,
  name TEXT NOT NULL,
  -- Free-text note from the signup form (optional).
  note TEXT,
  status TEXT NOT NULL DEFAULT 'pending' CHECK(status IN ('pending', 'invited', 'accepted', 'rejected')),
  -- Set once a signup is converted into a real account.
  user_id INTEGER,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE SET NULL ON UPDATE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_signups_status ON signups(status);
CREATE INDEX IF NOT EXISTS idx_signups_created_at ON signups(created_at);

-- Keep updated_at current on row modifications (SQLite does not auto-update DEFAULT values).
CREATE TRIGGER IF NOT EXISTS signups_set_updated_at
AFTER UPDATE ON signups
FOR EACH ROW
WHEN NEW.updated_at = OLD.updated_at
BEGIN
  UPDATE signups
  SET updated_at = CURRENT_TIMESTAMP
  WHERE id = NEW.id;
END;
