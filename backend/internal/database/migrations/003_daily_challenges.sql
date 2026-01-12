-- Migration for Daily Challenge Service tables
-- Run: sqlite3 backend/app.db < backend/internal/database/migrations/003_daily_challenges.sql

-- Table for storing challenge submissions
CREATE TABLE IF NOT EXISTS challenge_submissions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL,
    challenge_id VARCHAR(50) NOT NULL,
    answer TEXT NOT NULL,
    points_earned INTEGER NOT NULL DEFAULT 0,
    is_correct BOOLEAN NOT NULL DEFAULT 0,
    time_taken_seconds INTEGER,
    submitted_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    UNIQUE(user_id, challenge_id)
);

-- Table for storing user challenge statistics
CREATE TABLE IF NOT EXISTS user_challenge_stats (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL UNIQUE,
    total_challenges_completed INTEGER NOT NULL DEFAULT 0,
    total_challenges_correct INTEGER NOT NULL DEFAULT 0,
    current_streak INTEGER NOT NULL DEFAULT 0,
    longest_streak INTEGER NOT NULL DEFAULT 0,
    total_points INTEGER NOT NULL DEFAULT 0,
    last_completed_date TIMESTAMP,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

-- Indexes for performance
CREATE INDEX IF NOT EXISTS idx_challenge_submissions_user_id ON challenge_submissions(user_id);
CREATE INDEX IF NOT EXISTS idx_challenge_submissions_challenge_id ON challenge_submissions(challenge_id);
CREATE INDEX IF NOT EXISTS idx_challenge_submissions_leaderboard ON challenge_submissions(challenge_id, points_earned DESC, time_taken_seconds ASC);
CREATE INDEX IF NOT EXISTS idx_user_challenge_stats_user_id ON user_challenge_stats(user_id);
CREATE INDEX IF NOT EXISTS idx_user_challenge_stats_points ON user_challenge_stats(total_points DESC);
CREATE INDEX IF NOT EXISTS idx_user_challenge_stats_streak ON user_challenge_stats(longest_streak DESC);
