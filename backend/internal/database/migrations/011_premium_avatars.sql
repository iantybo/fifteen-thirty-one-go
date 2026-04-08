-- Premium subscriptions and avatar support
CREATE TABLE IF NOT EXISTS premium_subscriptions (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  user_id INTEGER NOT NULL,
  stripe_customer_id TEXT NOT NULL,
  stripe_subscription_id TEXT,
  status TEXT NOT NULL DEFAULT 'active' CHECK(status IN ('active', 'canceled', 'past_due', 'unpaid')),
  current_period_start TIMESTAMP NOT NULL,
  current_period_end TIMESTAMP NOT NULL,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE ON UPDATE CASCADE,
  UNIQUE(user_id)
);

CREATE INDEX IF NOT EXISTS idx_premium_subscriptions_user_id ON premium_subscriptions(user_id);
CREATE INDEX IF NOT EXISTS idx_premium_subscriptions_stripe_customer_id ON premium_subscriptions(stripe_customer_id);
CREATE INDEX IF NOT EXISTS idx_premium_subscriptions_stripe_subscription_id ON premium_subscriptions(stripe_subscription_id);

-- Add avatar_url column to users table
ALTER TABLE users ADD COLUMN avatar_url TEXT;

-- Add is_premium computed column (virtual column for easier querying)
-- Note: SQLite doesn't support computed columns directly, so we'll use a view or check in application code
