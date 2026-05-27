ALTER TABLE user_preferences
  ADD COLUMN card_theme TEXT NOT NULL DEFAULT 'classic'
  CHECK(card_theme IN ('classic', 'neon', 'minimal', 'emoji'));
