-- Migration 007: Historical Hands Tracking
-- Tracks each hand played during a game for analysis and replay

-- Historical hands table - records every hand dealt during a game
CREATE TABLE IF NOT EXISTS historical_hands (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  game_id INTEGER NOT NULL,
  hand_number INTEGER NOT NULL,
  dealer_id INTEGER NOT NULL,
  starter_card TEXT,
  phase TEXT NOT NULL CHECK(phase IN ('discard', 'pegging', 'show', 'complete')),
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  completed_at TIMESTAMP,
  FOREIGN KEY(game_id) REFERENCES games(id) ON DELETE CASCADE ON UPDATE CASCADE,
  FOREIGN KEY(dealer_id) REFERENCES users(id) ON DELETE CASCADE ON UPDATE CASCADE
);

-- Ensure each hand number within a game is unique
CREATE UNIQUE INDEX IF NOT EXISTS idx_historical_hands_game_id_hand_number
ON historical_hands(game_id, hand_number);

CREATE INDEX IF NOT EXISTS idx_historical_hands_game_id_created_at ON historical_hands(game_id, created_at);
CREATE INDEX IF NOT EXISTS idx_historical_hands_dealer_id ON historical_hands(dealer_id);

-- Player hands within each historical hand
CREATE TABLE IF NOT EXISTS historical_hand_players (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  historical_hand_id INTEGER NOT NULL,
  user_id INTEGER NOT NULL,
  dealt_cards TEXT NOT NULL,
  discarded_cards TEXT,
  cards_played TEXT,
  hand_score INTEGER DEFAULT 0,
  crib_score INTEGER DEFAULT 0,
  pegging_score INTEGER DEFAULT 0,
  total_score INTEGER DEFAULT 0,
  is_dealer BOOLEAN NOT NULL DEFAULT 0,
  FOREIGN KEY(historical_hand_id) REFERENCES historical_hands(id) ON DELETE CASCADE ON UPDATE CASCADE,
  FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE ON UPDATE CASCADE
);

-- Ensure each player appears once per historical hand
CREATE UNIQUE INDEX IF NOT EXISTS idx_historical_hand_players_hand_user
ON historical_hand_players(historical_hand_id, user_id);

CREATE INDEX IF NOT EXISTS idx_historical_hand_players_user_id ON historical_hand_players(user_id);
CREATE INDEX IF NOT EXISTS idx_historical_hand_players_historical_hand_id ON historical_hand_players(historical_hand_id);

-- Pegging sequences within each hand
CREATE TABLE IF NOT EXISTS historical_hand_pegging (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  historical_hand_id INTEGER NOT NULL,
  sequence_number INTEGER NOT NULL,
  user_id INTEGER NOT NULL,
  card_played TEXT NOT NULL,
  running_count INTEGER NOT NULL,
  points_scored INTEGER DEFAULT 0,
  score_reason TEXT,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  FOREIGN KEY(historical_hand_id) REFERENCES historical_hands(id) ON DELETE CASCADE ON UPDATE CASCADE,
  FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE ON UPDATE CASCADE
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_historical_hand_pegging_hand_sequence
ON historical_hand_pegging(historical_hand_id, sequence_number);
CREATE INDEX IF NOT EXISTS idx_historical_hand_pegging_user_id ON historical_hand_pegging(user_id);
