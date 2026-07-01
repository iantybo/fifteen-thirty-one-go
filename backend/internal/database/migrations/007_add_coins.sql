-- Add coins column to users table
ALTER TABLE users ADD COLUMN coins INTEGER NOT NULL DEFAULT 0 CHECK(coins >= 0);
