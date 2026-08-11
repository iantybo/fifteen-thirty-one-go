-- Add target_score column to games table to support configurable winning scores
ALTER TABLE games ADD COLUMN target_score INTEGER NOT NULL DEFAULT 121;
