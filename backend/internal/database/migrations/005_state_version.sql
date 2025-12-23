-- Optimistic concurrency token for state_json updates.
-- Incremented on each state_json write; used to detect concurrent moves without holding in-memory locks during DB I/O.
ALTER TABLE games ADD COLUMN state_version INTEGER NOT NULL DEFAULT 0;


