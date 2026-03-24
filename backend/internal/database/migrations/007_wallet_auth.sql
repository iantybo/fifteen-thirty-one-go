-- Add optional wallet address for wallet-based authentication.
ALTER TABLE users ADD COLUMN wallet_address TEXT UNIQUE;
CREATE INDEX IF NOT EXISTS idx_users_wallet_address ON users(wallet_address);

-- Down / rollback (manual): SQLite cannot drop columns in older versions without rebuild;
-- for rollback on modern SQLite:
--   DROP INDEX IF EXISTS idx_users_wallet_address;
--   ALTER TABLE users DROP COLUMN wallet_address;
