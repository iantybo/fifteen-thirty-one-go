-- Custom card decks: users can define their own deck skins (card face images,
-- card back image, and accent colors) and select one as active.

CREATE TABLE IF NOT EXISTS card_decks (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  owner_id INTEGER NOT NULL,
  name TEXT NOT NULL,
  -- Image for the card back, shown for face-down cards.
  back_image_url TEXT,
  -- Base URL template for card faces, e.g. https://cdn/cards/{rank}{suit}.svg
  -- Supported placeholders: {rank}, {suit}, {code}.
  face_image_template TEXT,
  -- Accent colors used when an image is unavailable (fallback rendering).
  red_suit_color TEXT NOT NULL DEFAULT '#dc2626',
  black_suit_color TEXT NOT NULL DEFAULT '#0f172a',
  border_color TEXT NOT NULL DEFAULT '#cbd5e1',
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  FOREIGN KEY(owner_id) REFERENCES users(id) ON DELETE CASCADE ON UPDATE CASCADE,
  -- Deck names are unique per owner so the UI can address a deck by name.
  UNIQUE(owner_id, name)
);

CREATE INDEX IF NOT EXISTS idx_card_decks_owner_id ON card_decks(owner_id);

-- Keep updated_at current on row modifications (SQLite does not auto-update DEFAULT values).
CREATE TRIGGER IF NOT EXISTS card_decks_set_updated_at
AFTER UPDATE ON card_decks
FOR EACH ROW
WHEN NEW.updated_at = OLD.updated_at
BEGIN
  UPDATE card_decks
  SET updated_at = CURRENT_TIMESTAMP
  WHERE id = NEW.id;
END;

-- Active deck selection. NULL/absent means the built-in "classic" deck.
-- Built-in decks are referenced by the 'builtin:<id>' string form; custom decks
-- by their numeric card_decks.id. Stored as TEXT to hold both.
ALTER TABLE user_preferences ADD COLUMN active_deck TEXT;
