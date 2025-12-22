-- Persist full in-memory cribbage state for restart recovery.
-- Backwards compatible: existing rows get default empty string.
ALTER TABLE games ADD COLUMN state_json TEXT NOT NULL DEFAULT '';


