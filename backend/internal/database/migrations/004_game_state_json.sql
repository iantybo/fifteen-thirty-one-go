-- Persist full in-memory cribbage state for restart recovery.
-- Backwards compatible: existing rows get default empty JSON object.
ALTER TABLE games ADD COLUMN state_json TEXT NOT NULL DEFAULT '{}';

-- If any existing rows have the legacy empty-string sentinel, normalize to valid JSON.
UPDATE games SET state_json = '{}' WHERE state_json = '';


