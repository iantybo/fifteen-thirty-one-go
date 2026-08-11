-- Migration: Add email and display_name fields to users table
-- Purpose: Enable users to add profile settings like name and email addresses

-- Add email field (optional, unique when set)
ALTER TABLE users ADD COLUMN email TEXT UNIQUE;

-- Add display_name field (optional, for display purposes)
ALTER TABLE users ADD COLUMN display_name TEXT;

-- Create index on email for faster lookups
CREATE INDEX idx_users_email ON users(email) WHERE email IS NOT NULL;
