-- Add in-game currency to users.
ALTER TABLE users ADD COLUMN coins INTEGER NOT NULL DEFAULT 0;

-- Track cards owned by users. A card is awarded ("used") when a game the user
-- participated in finishes, one row per distinct card the user played during
-- that game. sold_at is nullable: NULL means the user still owns the card.
CREATE TABLE IF NOT EXISTS user_cards (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  user_id INTEGER NOT NULL,
  game_id INTEGER NOT NULL,
  card TEXT NOT NULL,
  acquired_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  sold_at TIMESTAMP,
  sold_price INTEGER,
  FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE ON UPDATE CASCADE,
  FOREIGN KEY(game_id) REFERENCES games(id) ON DELETE CASCADE ON UPDATE CASCADE,
  CHECK ((sold_at IS NULL) = (sold_price IS NULL))
);

-- A given (user, game, card) is awarded at most once — keeps finalize idempotent.
CREATE UNIQUE INDEX IF NOT EXISTS idx_user_cards_user_game_card
ON user_cards(user_id, game_id, card);

CREATE INDEX IF NOT EXISTS idx_user_cards_user_sold ON user_cards(user_id, sold_at);
