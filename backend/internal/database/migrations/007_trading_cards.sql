-- Trading cards table - defines all available trading cards
CREATE TABLE IF NOT EXISTS trading_cards (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  name TEXT NOT NULL,
  description TEXT NOT NULL,
  rarity TEXT NOT NULL CHECK(rarity IN ('common', 'uncommon', 'rare', 'epic', 'legendary')),
  artwork_url TEXT NOT NULL,
  category TEXT NOT NULL,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_trading_cards_rarity ON trading_cards(rarity);
CREATE INDEX IF NOT EXISTS idx_trading_cards_category ON trading_cards(category);

-- User trading cards - tracks which cards users have collected
CREATE TABLE IF NOT EXISTS user_trading_cards (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  user_id INTEGER NOT NULL,
  card_id INTEGER NOT NULL,
  acquired_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  quantity INTEGER NOT NULL DEFAULT 1,
  FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE ON UPDATE CASCADE,
  FOREIGN KEY(card_id) REFERENCES trading_cards(id) ON DELETE CASCADE ON UPDATE CASCADE
);

-- Each user can only have one entry per card (quantity tracks duplicates)
CREATE UNIQUE INDEX IF NOT EXISTS idx_user_trading_cards_user_card
ON user_trading_cards(user_id, card_id);

CREATE INDEX IF NOT EXISTS idx_user_trading_cards_user_id ON user_trading_cards(user_id);
CREATE INDEX IF NOT EXISTS idx_user_trading_cards_card_id ON user_trading_cards(card_id);

-- Card rewards - defines how cards can be earned
CREATE TABLE IF NOT EXISTS card_rewards (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  card_id INTEGER NOT NULL,
  reward_type TEXT NOT NULL CHECK(reward_type IN ('game_win', 'games_played', 'high_score', 'win_streak', 'special_event')),
  requirement_value INTEGER NOT NULL,
  requirement_data TEXT,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  FOREIGN KEY(card_id) REFERENCES trading_cards(id) ON DELETE CASCADE ON UPDATE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_card_rewards_card_id ON card_rewards(card_id);
CREATE INDEX IF NOT EXISTS idx_card_rewards_type ON card_rewards(reward_type);

-- Insert some starter trading cards
INSERT INTO trading_cards (name, description, rarity, artwork_url, category) VALUES
  ('Ace of Spades', 'The most iconic card in the deck', 'legendary', '/assets/cards/ace-spades.png', 'classic'),
  ('Royal Flush', 'The ultimate hand', 'legendary', '/assets/cards/royal-flush.png', 'achievement'),
  ('Perfect 29', 'The highest cribbage hand', 'legendary', '/assets/cards/perfect-29.png', 'achievement'),
  ('First Win', 'Your first victory', 'common', '/assets/cards/first-win.png', 'milestone'),
  ('Hat Trick', 'Win three games in a row', 'rare', '/assets/cards/hat-trick.png', 'streak'),
  ('Centurion', 'Play 100 games', 'epic', '/assets/cards/centurion.png', 'milestone'),
  ('Jack of All Trades', 'Win with every suit combination', 'rare', '/assets/cards/jack-all-trades.png', 'achievement'),
  ('Speed Demon', 'Win a game in under 10 minutes', 'uncommon', '/assets/cards/speed-demon.png', 'achievement'),
  ('Comeback Kid', 'Win after being down 20 points', 'rare', '/assets/cards/comeback-kid.png', 'achievement'),
  ('Shutout', 'Win without opponent scoring', 'epic', '/assets/cards/shutout.png', 'achievement');

-- Insert reward conditions for starter cards
INSERT INTO card_rewards (card_id, reward_type, requirement_value) VALUES
  (4, 'game_win', 1),           -- First Win: win 1 game
  (5, 'win_streak', 3),          -- Hat Trick: 3 game win streak
  (6, 'games_played', 100),      -- Centurion: play 100 games
  (8, 'special_event', 600),     -- Speed Demon: win in under 600 seconds
  (9, 'special_event', 20),      -- Comeback Kid: comeback from 20 points
  (10, 'special_event', 0);      -- Shutout: opponent score 0
