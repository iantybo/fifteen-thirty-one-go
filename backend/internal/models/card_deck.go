package models

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// Deck validation errors. Handlers map these to 400 responses.
var (
	ErrInvalidDeckName     = errors.New("invalid deck name")
	ErrInvalidDeckImageURL = errors.New("invalid deck image url")
	ErrInvalidDeckColor    = errors.New("invalid deck color")
	ErrInvalidDeckTemplate = errors.New("invalid face image template")
	ErrDeckNameTaken       = errors.New("deck name already taken")
	ErrDeckNotFound        = errors.New("deck not found")
	ErrInvalidDeckRef      = errors.New("invalid deck reference")
	ErrTooManyDecks        = errors.New("too many decks")
)

const (
	maxDeckNameLen = 60
	maxDeckURLLen  = 2048
	// BuiltinDeckPrefix marks an active_deck value that refers to a built-in deck
	// rather than a row in card_decks.
	BuiltinDeckPrefix = "builtin:"
	// DefaultDeckID is the built-in deck used when a user has made no selection.
	DefaultDeckID = "classic"
	// maxDecksPerUser bounds both creation and listing so a single account
	// cannot grow an unbounded deck collection or response.
	maxDecksPerUser = 50
)

// BuiltinDeck is a server-defined deck skin that every user can select.
type BuiltinDeck struct {
	ID                string `json:"id"`
	Name              string `json:"name"`
	BackImageURL      string `json:"back_image_url,omitempty"`
	FaceImageTemplate string `json:"face_image_template,omitempty"`
	RedSuitColor      string `json:"red_suit_color"`
	BlackSuitColor    string `json:"black_suit_color"`
	BorderColor       string `json:"border_color"`
}

// builtinDecks are rendered entirely from vector shapes in the frontend (no image
// fetches), so they always work offline. Custom decks supply image URLs instead.
var builtinDecks = []BuiltinDeck{
	{
		ID:             DefaultDeckID,
		Name:           "Classic",
		RedSuitColor:   "#dc2626",
		BlackSuitColor: "#0f172a",
		BorderColor:    "#cbd5e1",
	},
	{
		ID:             "midnight",
		Name:           "Midnight",
		RedSuitColor:   "#f472b6",
		BlackSuitColor: "#1e293b",
		BorderColor:    "#475569",
	},
	{
		ID:             "forest",
		Name:           "Forest",
		RedSuitColor:   "#b91c1c",
		BlackSuitColor: "#14532d",
		BorderColor:    "#86efac",
	},
	{
		ID:             "high-contrast",
		Name:           "High Contrast",
		RedSuitColor:   "#b30000",
		BlackSuitColor: "#000000",
		BorderColor:    "#000000",
	},
}

// BuiltinDecks returns a copy of the built-in deck list.
func BuiltinDecks() []BuiltinDeck {
	out := make([]BuiltinDeck, len(builtinDecks))
	copy(out, builtinDecks)
	return out
}

// IsBuiltinDeckID reports whether id names a built-in deck.
func IsBuiltinDeckID(id string) bool {
	for i := range builtinDecks {
		if builtinDecks[i].ID == id {
			return true
		}
	}
	return false
}

// CardDeck is a user-created deck skin.
type CardDeck struct {
	ID                int64     `json:"id"`
	OwnerID           int64     `json:"owner_id"`
	Name              string    `json:"name"`
	BackImageURL      string    `json:"back_image_url,omitempty"`
	FaceImageTemplate string    `json:"face_image_template,omitempty"`
	RedSuitColor      string    `json:"red_suit_color"`
	BlackSuitColor    string    `json:"black_suit_color"`
	BorderColor       string    `json:"border_color"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

// DeckInput carries the mutable fields of a deck. Nil string pointers mean
// "leave unchanged" on update; empty strings clear the value.
type DeckInput struct {
	Name              string
	BackImageURL      string
	FaceImageTemplate string
	RedSuitColor      string
	BlackSuitColor    string
	BorderColor       string
}

// validateDeckName trims and length-checks a deck name.
func validateDeckName(name string) (string, error) {
	n := strings.TrimSpace(name)
	if n == "" || len([]rune(n)) > maxDeckNameLen {
		return "", ErrInvalidDeckName
	}
	// Control characters would break rendering and log output.
	for _, r := range n {
		if r < 0x20 || r == 0x7f {
			return "", ErrInvalidDeckName
		}
	}
	return n, nil
}

// validateImageURL enforces an https-only absolute URL. Deck URLs are rendered
// as image sources in other users' browsers, so schemes that can execute script
// (javascript:, data:, blob:) or leak over plaintext (http:) are rejected.
func validateImageURL(raw string) (string, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return "", nil
	}
	if len(s) > maxDeckURLLen {
		return "", ErrInvalidDeckImageURL
	}
	u, err := url.Parse(s)
	if err != nil {
		return "", ErrInvalidDeckImageURL
	}
	if !strings.EqualFold(u.Scheme, "https") {
		return "", ErrInvalidDeckImageURL
	}
	if u.Host == "" {
		return "", ErrInvalidDeckImageURL
	}
	return u.String(), nil
}

// validateFaceTemplate checks that a face template is a valid https URL and
// contains at least one card placeholder, otherwise every card would render the
// same image.
func validateFaceTemplate(raw string) (string, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return "", nil
	}
	// Validate the URL with placeholders substituted, since "{" and "}" are not
	// legal URL characters and would fail a strict parse.
	probe := strings.NewReplacer("{rank}", "A", "{suit}", "S", "{code}", "AS").Replace(s)
	if _, err := validateImageURL(probe); err != nil {
		return "", ErrInvalidDeckTemplate
	}
	if !strings.Contains(s, "{rank}") && !strings.Contains(s, "{suit}") && !strings.Contains(s, "{code}") {
		return "", ErrInvalidDeckTemplate
	}
	return s, nil
}

// validateHexColor accepts #rgb, #rrggbb, and #rrggbbaa.
func validateHexColor(raw, fallback string) (string, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return fallback, nil
	}
	if s[0] != '#' {
		return "", ErrInvalidDeckColor
	}
	hex := s[1:]
	if len(hex) != 3 && len(hex) != 6 && len(hex) != 8 {
		return "", ErrInvalidDeckColor
	}
	for i := 0; i < len(hex); i++ {
		c := hex[i]
		isHex := (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')
		if !isHex {
			return "", ErrInvalidDeckColor
		}
	}
	return strings.ToLower(s), nil
}

// normalizeDeckInput validates and normalizes every field of a deck payload.
func normalizeDeckInput(in DeckInput) (DeckInput, error) {
	var out DeckInput
	var err error

	if out.Name, err = validateDeckName(in.Name); err != nil {
		return DeckInput{}, err
	}
	if out.BackImageURL, err = validateImageURL(in.BackImageURL); err != nil {
		return DeckInput{}, err
	}
	if out.FaceImageTemplate, err = validateFaceTemplate(in.FaceImageTemplate); err != nil {
		return DeckInput{}, err
	}
	if out.RedSuitColor, err = validateHexColor(in.RedSuitColor, "#dc2626"); err != nil {
		return DeckInput{}, err
	}
	if out.BlackSuitColor, err = validateHexColor(in.BlackSuitColor, "#0f172a"); err != nil {
		return DeckInput{}, err
	}
	if out.BorderColor, err = validateHexColor(in.BorderColor, "#cbd5e1"); err != nil {
		return DeckInput{}, err
	}
	return out, nil
}

// isUniqueViolation reports whether err is a SQLite UNIQUE constraint failure.
// Matching on the driver message keeps models free of a driver dependency.
func isUniqueViolation(err error) bool {
	return err != nil && strings.Contains(strings.ToLower(err.Error()), "unique constraint failed")
}

const deckColumns = `id, owner_id, name,
	COALESCE(back_image_url, ''), COALESCE(face_image_template, ''),
	red_suit_color, black_suit_color, border_color, created_at, updated_at`

func scanDeck(s interface {
	Scan(dest ...any) error
}) (*CardDeck, error) {
	var d CardDeck
	err := s.Scan(&d.ID, &d.OwnerID, &d.Name, &d.BackImageURL, &d.FaceImageTemplate,
		&d.RedSuitColor, &d.BlackSuitColor, &d.BorderColor, &d.CreatedAt, &d.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &d, nil
}

// ListCardDecks returns every deck owned by userID, name-ordered.
// Results are capped at maxDecksPerUser so the response stays bounded.
func ListCardDecks(ctx context.Context, db *sql.DB, userID int64) ([]CardDeck, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT `+deckColumns+` FROM card_decks WHERE owner_id = ? ORDER BY name COLLATE NOCASE LIMIT ?`,
		userID, maxDecksPerUser)
	if err != nil {
		return nil, fmt.Errorf("query card decks (user_id=%d): %w", userID, err)
	}
	defer rows.Close()

	decks := []CardDeck{}
	for rows.Next() {
		d, err := scanDeck(rows)
		if err != nil {
			return nil, fmt.Errorf("scan card deck (user_id=%d): %w", userID, err)
		}
		decks = append(decks, *d)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate card decks (user_id=%d): %w", userID, err)
	}
	return decks, nil
}

// getCardDeckTx loads a deck owned by userID through an open transaction.
func getCardDeckTx(ctx context.Context, tx *sql.Tx, userID, deckID int64) (*CardDeck, error) {
	row := tx.QueryRowContext(ctx, `SELECT `+deckColumns+` FROM card_decks WHERE id = ? AND owner_id = ?`, deckID, userID)
	d, err := scanDeck(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrDeckNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("scan card deck in tx (id=%d user_id=%d): %w", deckID, userID, err)
	}
	return d, nil
}

// GetCardDeck loads a single deck owned by userID.
func GetCardDeck(ctx context.Context, db *sql.DB, userID, deckID int64) (*CardDeck, error) {
	row := db.QueryRowContext(ctx, `SELECT `+deckColumns+` FROM card_decks WHERE id = ? AND owner_id = ?`, deckID, userID)
	d, err := scanDeck(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrDeckNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("scan card deck (id=%d user_id=%d): %w", deckID, userID, err)
	}
	return d, nil
}

// CreateCardDeck validates and inserts a new deck for userID. The per-user
// deck count is checked inside the transaction so concurrent creates cannot
// both slip past the limit.
func CreateCardDeck(ctx context.Context, db *sql.DB, userID int64, in DeckInput) (*CardDeck, error) {
	n, err := normalizeDeckInput(in)
	if err != nil {
		return nil, err
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin create deck tx (user_id=%d): %w", userID, err)
	}
	defer func() { _ = tx.Rollback() }()

	var count int64
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM card_decks WHERE owner_id = ?`, userID).Scan(&count); err != nil {
		return nil, fmt.Errorf("count card decks (user_id=%d): %w", userID, err)
	}
	if count >= maxDecksPerUser {
		return nil, ErrTooManyDecks
	}

	res, err := tx.ExecContext(ctx,
		`INSERT INTO card_decks(owner_id, name, back_image_url, face_image_template,
			red_suit_color, black_suit_color, border_color)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		userID, n.Name, nullIfEmpty(n.BackImageURL), nullIfEmpty(n.FaceImageTemplate),
		n.RedSuitColor, n.BlackSuitColor, n.BorderColor,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, ErrDeckNameTaken
		}
		return nil, fmt.Errorf("insert card deck (user_id=%d): %w", userID, err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("last insert id for card deck (user_id=%d): %w", userID, err)
	}

	deck, err := scanDeck(tx.QueryRowContext(ctx,
		`SELECT `+deckColumns+` FROM card_decks WHERE id = ? AND owner_id = ?`, id, userID))
	if err != nil {
		return nil, fmt.Errorf("read back created deck (id=%d user_id=%d): %w", id, userID, err)
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit create deck tx (user_id=%d): %w", userID, err)
	}
	return deck, nil
}

// UpdateCardDeck replaces the mutable fields of an existing deck.
func UpdateCardDeck(ctx context.Context, db *sql.DB, userID, deckID int64, in DeckInput) (*CardDeck, error) {
	n, err := normalizeDeckInput(in)
	if err != nil {
		return nil, err
	}

	res, err := db.ExecContext(ctx,
		`UPDATE card_decks
		 SET name = ?, back_image_url = ?, face_image_template = ?,
		     red_suit_color = ?, black_suit_color = ?, border_color = ?,
		     updated_at = CURRENT_TIMESTAMP
		 WHERE id = ? AND owner_id = ?`,
		n.Name, nullIfEmpty(n.BackImageURL), nullIfEmpty(n.FaceImageTemplate),
		n.RedSuitColor, n.BlackSuitColor, n.BorderColor, deckID, userID,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, ErrDeckNameTaken
		}
		return nil, fmt.Errorf("update card deck (id=%d user_id=%d): %w", deckID, userID, err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("update card deck rows affected (id=%d user_id=%d): %w", deckID, userID, err)
	}
	if affected == 0 {
		return nil, ErrDeckNotFound
	}
	return GetCardDeck(ctx, db, userID, deckID)
}

// DeleteCardDeck removes a deck. If it was the user's active deck, the selection
// falls back to the default built-in deck so nobody is left pointing at a
// deleted row.
func DeleteCardDeck(ctx context.Context, db *sql.DB, userID, deckID int64) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin delete deck tx (id=%d user_id=%d): %w", deckID, userID, err)
	}
	defer func() { _ = tx.Rollback() }()

	res, err := tx.ExecContext(ctx, `DELETE FROM card_decks WHERE id = ? AND owner_id = ?`, deckID, userID)
	if err != nil {
		return fmt.Errorf("delete card deck (id=%d user_id=%d): %w", deckID, userID, err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("delete card deck rows affected (id=%d user_id=%d): %w", deckID, userID, err)
	}
	if affected == 0 {
		return ErrDeckNotFound
	}

	if _, err := tx.ExecContext(ctx,
		`UPDATE user_preferences SET active_deck = NULL, updated_at = CURRENT_TIMESTAMP
		 WHERE user_id = ? AND active_deck = ?`,
		userID, strconv.FormatInt(deckID, 10),
	); err != nil {
		return fmt.Errorf("clear active deck (id=%d user_id=%d): %w", deckID, userID, err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit delete deck tx (id=%d user_id=%d): %w", deckID, userID, err)
	}
	return nil
}

// SetActiveDeck records the user's deck selection. ref is either
// "builtin:<id>" or the decimal id of a deck the user owns; an empty ref
// resets to the default deck.
func SetActiveDeck(ctx context.Context, db *sql.DB, userID int64, ref string) (*UserPreferences, error) {
	// Begin the transaction before validating so the ownership check and the
	// upsert observe the same snapshot.
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin active_deck tx (user_id=%d): %w", userID, err)
	}
	defer func() { _ = tx.Rollback() }()

	normalized, err := normalizeDeckRef(ctx, tx, userID, ref)
	if err != nil {
		return nil, err
	}

	if _, err := tx.ExecContext(ctx,
		`INSERT INTO user_preferences(user_id, active_deck) VALUES (?, ?)
		 ON CONFLICT(user_id) DO UPDATE SET active_deck = excluded.active_deck, updated_at = CURRENT_TIMESTAMP`,
		userID, nullIfEmpty(normalized),
	); err != nil {
		return nil, fmt.Errorf("upsert active_deck (user_id=%d): %w", userID, err)
	}

	p, err := scanPreferencesRow(tx.QueryRowContext(ctx, preferencesSelect, userID))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// Unreachable after an upsert, but keep the write and return intent.
			if err := tx.Commit(); err != nil {
				return nil, fmt.Errorf("commit active_deck tx (user_id=%d): %w", userID, err)
			}
			p := defaultPreferences(userID)
			p.ActiveDeck = normalized
			return p, nil
		}
		return nil, fmt.Errorf("read back preferences (user_id=%d): %w", userID, err)
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit active_deck tx (user_id=%d): %w", userID, err)
	}
	return p, nil
}

// normalizeDeckRef validates a deck reference and returns its canonical form.
// An empty result means "use the default deck". Ownership is checked through
// the caller's transaction so a concurrent delete cannot leave a dangling
// active_deck reference.
func normalizeDeckRef(ctx context.Context, tx *sql.Tx, userID int64, ref string) (string, error) {
	s := strings.TrimSpace(ref)
	if s == "" || s == BuiltinDeckPrefix+DefaultDeckID {
		return "", nil
	}
	if after, ok := strings.CutPrefix(s, BuiltinDeckPrefix); ok {
		if !IsBuiltinDeckID(after) {
			return "", ErrInvalidDeckRef
		}
		return s, nil
	}
	id, err := strconv.ParseInt(s, 10, 64)
	if err != nil || id <= 0 {
		return "", ErrInvalidDeckRef
	}
	// Verify ownership so a user cannot select someone else's deck.
	if _, err := getCardDeckTx(ctx, tx, userID, id); err != nil {
		if errors.Is(err, ErrDeckNotFound) {
			return "", ErrDeckNotFound
		}
		return "", err
	}
	// Return the canonical decimal form: DeleteCardDeck clears the selection by
	// string comparison, so a non-canonical ref like "007" would dangle.
	return strconv.FormatInt(id, 10), nil
}

func nullIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}
