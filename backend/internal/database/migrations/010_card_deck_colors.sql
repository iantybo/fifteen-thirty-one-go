-- Add card_deck_colors column to user_preferences
-- Stores JSON object with suit colors: {"H": "#dc2626", "D": "#dc2626", "C": "#0f172a", "S": "#0f172a"}
-- NULL means use default colors
ALTER TABLE user_preferences ADD COLUMN card_deck_colors TEXT;
