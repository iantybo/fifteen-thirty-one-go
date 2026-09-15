package models

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"testing"

	"fifteen-thirty-one-go/backend/internal/database"
)

func newTestDB(t *testing.T) *sql.DB {
	t.Helper()
	// A distinct shared-cache memory DB per test keeps them isolated.
	db, err := database.OpenAndMigrate("file:" + t.Name() + "?mode=memory&cache=shared")
	if err != nil {
		t.Fatalf("OpenAndMigrate: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	if _, err := db.Exec(`INSERT INTO users(id, username, password_hash) VALUES (1, 'alice', 'x'), (2, 'bob', 'y')`); err != nil {
		t.Fatalf("seed users: %v", err)
	}
	return db
}

func TestValidateImageURLRejectsUnsafeSchemes(t *testing.T) {
	// Deck URLs are rendered as image sources in browsers; only https is allowed.
	bad := []string{
		"javascript:alert(1)",
		"data:image/svg+xml;base64,PHN2Zz48L3N2Zz4=",
		"http://example.com/back.png",
		"blob:https://example.com/abc",
		"https://",
		"/relative/path.png",
	}
	for _, in := range bad {
		if _, err := validateImageURL(in); !errors.Is(err, ErrInvalidDeckImageURL) {
			t.Errorf("validateImageURL(%q) = %v, want ErrInvalidDeckImageURL", in, err)
		}
	}

	if got, err := validateImageURL("  https://cdn.example.com/back.png  "); err != nil || got != "https://cdn.example.com/back.png" {
		t.Errorf("validateImageURL(valid) = %q, %v", got, err)
	}
	// Empty means "no image", which is valid.
	if got, err := validateImageURL(""); err != nil || got != "" {
		t.Errorf("validateImageURL(empty) = %q, %v", got, err)
	}
}

func TestValidateFaceTemplate(t *testing.T) {
	if _, err := validateFaceTemplate("https://cdn.example.com/cards/fixed.svg"); !errors.Is(err, ErrInvalidDeckTemplate) {
		t.Error("template without a placeholder should be rejected")
	}
	if _, err := validateFaceTemplate("http://cdn.example.com/{code}.svg"); !errors.Is(err, ErrInvalidDeckTemplate) {
		t.Error("non-https template should be rejected")
	}
	for _, ok := range []string{
		"https://cdn.example.com/cards/{code}.svg",
		"https://cdn.example.com/cards/{rank}{suit}.png",
		"https://cdn.example.com/cards/{suit}/{rank}.webp",
	} {
		if got, err := validateFaceTemplate(ok); err != nil || got != ok {
			t.Errorf("validateFaceTemplate(%q) = %q, %v", ok, got, err)
		}
	}
}

func TestValidateHexColor(t *testing.T) {
	if got, err := validateHexColor("", "#dc2626"); err != nil || got != "#dc2626" {
		t.Errorf("empty color should fall back: got %q, %v", got, err)
	}
	if got, err := validateHexColor("#ABCDEF", "#000000"); err != nil || got != "#abcdef" {
		t.Errorf("color should normalize to lowercase: got %q, %v", got, err)
	}
	for _, bad := range []string{"red", "#12", "#12345", "#gggggg", "rgb(1,2,3)"} {
		if _, err := validateHexColor(bad, "#000000"); !errors.Is(err, ErrInvalidDeckColor) {
			t.Errorf("validateHexColor(%q) should fail, got %v", bad, err)
		}
	}
}

func TestValidateDeckName(t *testing.T) {
	if _, err := validateDeckName("   "); !errors.Is(err, ErrInvalidDeckName) {
		t.Error("blank name should be rejected")
	}
	if _, err := validateDeckName("bad\nname"); !errors.Is(err, ErrInvalidDeckName) {
		t.Error("control characters should be rejected")
	}
	long := make([]rune, maxDeckNameLen+1)
	for i := range long {
		long[i] = 'a'
	}
	if _, err := validateDeckName(string(long)); !errors.Is(err, ErrInvalidDeckName) {
		t.Error("over-long name should be rejected")
	}
	if got, err := validateDeckName("  Neon Nights  "); err != nil || got != "Neon Nights" {
		t.Errorf("valid name = %q, %v", got, err)
	}
}

func TestCardDeckCRUD(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)

	in := DeckInput{
		Name:              "Neon",
		BackImageURL:      "https://cdn.example.com/back.png",
		FaceImageTemplate: "https://cdn.example.com/{code}.svg",
		RedSuitColor:      "#FF00AA",
	}
	deck, err := CreateCardDeck(ctx, db, 1, in)
	if err != nil {
		t.Fatalf("CreateCardDeck: %v", err)
	}
	if deck.Name != "Neon" || deck.RedSuitColor != "#ff00aa" {
		t.Fatalf("unexpected deck: %+v", deck)
	}
	// Unset colors take documented defaults.
	if deck.BlackSuitColor != "#0f172a" || deck.BorderColor != "#cbd5e1" {
		t.Errorf("defaults not applied: %+v", deck)
	}

	// Duplicate names per owner are rejected...
	if _, err := CreateCardDeck(ctx, db, 1, in); !errors.Is(err, ErrDeckNameTaken) {
		t.Errorf("duplicate name = %v, want ErrDeckNameTaken", err)
	}
	// ...but a different owner may reuse the name.
	if _, err := CreateCardDeck(ctx, db, 2, in); err != nil {
		t.Errorf("other owner should be able to reuse name: %v", err)
	}

	// Another user's deck is invisible.
	if _, err := GetCardDeck(ctx, db, 2, deck.ID); !errors.Is(err, ErrDeckNotFound) {
		t.Errorf("cross-owner read = %v, want ErrDeckNotFound", err)
	}

	updated, err := UpdateCardDeck(ctx, db, 1, deck.ID, DeckInput{Name: "Neon v2", BorderColor: "#123456"})
	if err != nil {
		t.Fatalf("UpdateCardDeck: %v", err)
	}
	if updated.Name != "Neon v2" || updated.BorderColor != "#123456" {
		t.Fatalf("update not applied: %+v", updated)
	}
	// Cleared image fields come back as empty strings, not NULL scan errors.
	if updated.BackImageURL != "" || updated.FaceImageTemplate != "" {
		t.Errorf("images should be cleared: %+v", updated)
	}

	// A non-owner cannot update or delete.
	if _, err := UpdateCardDeck(ctx, db, 2, deck.ID, DeckInput{Name: "hijack"}); !errors.Is(err, ErrDeckNotFound) {
		t.Errorf("cross-owner update = %v, want ErrDeckNotFound", err)
	}
	if err := DeleteCardDeck(ctx, db, 2, deck.ID); !errors.Is(err, ErrDeckNotFound) {
		t.Errorf("cross-owner delete = %v, want ErrDeckNotFound", err)
	}

	decks, err := ListCardDecks(ctx, db, 1)
	if err != nil || len(decks) != 1 {
		t.Fatalf("ListCardDecks = %v, %v", decks, err)
	}

	if err := DeleteCardDeck(ctx, db, 1, deck.ID); err != nil {
		t.Fatalf("DeleteCardDeck: %v", err)
	}
	if decks, err := ListCardDecks(ctx, db, 1); err != nil || len(decks) != 0 {
		t.Fatalf("after delete: %v, %v", decks, err)
	}
}

func TestSetActiveDeck(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)

	deck, err := CreateCardDeck(ctx, db, 1, DeckInput{Name: "Mine"})
	if err != nil {
		t.Fatalf("CreateCardDeck: %v", err)
	}

	// A built-in reference is accepted.
	prefs, err := SetActiveDeck(ctx, db, 1, BuiltinDeckPrefix+"midnight")
	if err != nil || prefs.ActiveDeck != BuiltinDeckPrefix+"midnight" {
		t.Fatalf("builtin selection = %+v, %v", prefs, err)
	}
	// Auto-count default is preserved when only the deck is set.
	if prefs.AutoCountMode != "suggest" {
		t.Errorf("auto_count_mode = %q, want suggest", prefs.AutoCountMode)
	}

	// The default deck normalizes to the empty string.
	if prefs, err = SetActiveDeck(ctx, db, 1, BuiltinDeckPrefix+DefaultDeckID); err != nil || prefs.ActiveDeck != "" {
		t.Fatalf("default selection = %+v, %v", prefs, err)
	}

	// Own custom deck is accepted.
	own := "1"
	if deck.ID != 1 {
		t.Fatalf("expected first deck id 1, got %d", deck.ID)
	}
	if prefs, err = SetActiveDeck(ctx, db, 1, own); err != nil || prefs.ActiveDeck != own {
		t.Fatalf("custom selection = %+v, %v", prefs, err)
	}

	// Unknown built-in and malformed references are rejected.
	for _, bad := range []string{BuiltinDeckPrefix + "nope", "abc", "-1", "0"} {
		if _, err := SetActiveDeck(ctx, db, 1, bad); !errors.Is(err, ErrInvalidDeckRef) {
			t.Errorf("SetActiveDeck(%q) = %v, want ErrInvalidDeckRef", bad, err)
		}
	}
	// Another user's deck cannot be selected.
	if _, err := SetActiveDeck(ctx, db, 2, own); !errors.Is(err, ErrDeckNotFound) {
		t.Errorf("cross-owner selection = %v, want ErrDeckNotFound", err)
	}

	// Deleting the active deck clears the selection rather than dangling.
	if err := DeleteCardDeck(ctx, db, 1, deck.ID); err != nil {
		t.Fatalf("DeleteCardDeck: %v", err)
	}
	prefs, err = GetUserPreferences(db, 1)
	if err != nil {
		t.Fatalf("GetUserPreferences: %v", err)
	}
	if prefs.ActiveDeck != "" {
		t.Errorf("active deck after delete = %q, want empty", prefs.ActiveDeck)
	}
}

func TestSetActiveDeckPreservesAutoCountMode(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)

	if _, err := SetUserAutoCountModeAndGetPreferencesTx(db, 1, "auto"); err != nil {
		t.Fatalf("set auto count: %v", err)
	}
	prefs, err := SetActiveDeck(ctx, db, 1, BuiltinDeckPrefix+"forest")
	if err != nil {
		t.Fatalf("SetActiveDeck: %v", err)
	}
	if prefs.AutoCountMode != "auto" {
		t.Errorf("auto_count_mode = %q, want auto (deck change must not reset it)", prefs.AutoCountMode)
	}
	if prefs.ActiveDeck != BuiltinDeckPrefix+"forest" {
		t.Errorf("active_deck = %q", prefs.ActiveDeck)
	}
}

func TestActiveDeckRefStoredCanonically(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)

	deck, err := CreateCardDeck(ctx, db, 1, DeckInput{Name: "Mine"})
	if err != nil {
		t.Fatalf("CreateCardDeck: %v", err)
	}

	// A non-canonical numeric ref must be stored in canonical decimal form,
	// otherwise DeleteCardDeck's string comparison misses it and the
	// preference is left dangling at a deleted deck.
	prefs, err := SetActiveDeck(ctx, db, 1, "0000001")
	if err != nil {
		t.Fatalf("SetActiveDeck: %v", err)
	}
	if prefs.ActiveDeck != "1" {
		t.Fatalf("active_deck = %q, want canonical %q", prefs.ActiveDeck, "1")
	}

	if err := DeleteCardDeck(ctx, db, 1, deck.ID); err != nil {
		t.Fatalf("DeleteCardDeck: %v", err)
	}
	after, err := GetUserPreferences(db, 1)
	if err != nil {
		t.Fatalf("GetUserPreferences: %v", err)
	}
	if after.ActiveDeck != "" {
		t.Errorf("active_deck = %q after delete, want cleared", after.ActiveDeck)
	}
}

func TestCreateCardDeckEnforcesPerUserLimit(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)

	for i := 0; i < maxDecksPerUser; i++ {
		if _, err := CreateCardDeck(ctx, db, 1, DeckInput{Name: fmt.Sprintf("deck-%d", i)}); err != nil {
			t.Fatalf("CreateCardDeck(%d): %v", i, err)
		}
	}
	if _, err := CreateCardDeck(ctx, db, 1, DeckInput{Name: "one-too-many"}); !errors.Is(err, ErrTooManyDecks) {
		t.Errorf("create past limit = %v, want ErrTooManyDecks", err)
	}
	// The cap is per user, so another account is unaffected.
	if _, err := CreateCardDeck(ctx, db, 2, DeckInput{Name: "fresh"}); err != nil {
		t.Errorf("other user should not be capped: %v", err)
	}

	// Listing stays bounded by the same limit.
	decks, err := ListCardDecks(ctx, db, 1)
	if err != nil {
		t.Fatalf("ListCardDecks: %v", err)
	}
	if len(decks) > maxDecksPerUser {
		t.Errorf("ListCardDecks returned %d decks, want <= %d", len(decks), maxDecksPerUser)
	}
}

func TestSentinelErrorsSurviveWrapping(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)

	// Domain sentinels must stay recognizable to errors.Is so handlers keep
	// mapping them to 404/409 rather than 500.
	if _, err := GetCardDeck(ctx, db, 1, 4242); !errors.Is(err, ErrDeckNotFound) {
		t.Errorf("GetCardDeck(missing) = %v, want ErrDeckNotFound", err)
	}
	if _, err := CreateCardDeck(ctx, db, 1, DeckInput{Name: "dup"}); err != nil {
		t.Fatalf("CreateCardDeck: %v", err)
	}
	if _, err := CreateCardDeck(ctx, db, 1, DeckInput{Name: "dup"}); !errors.Is(err, ErrDeckNameTaken) {
		t.Errorf("duplicate = %v, want ErrDeckNameTaken", err)
	}
}

func TestCanceledContextIsPropagated(t *testing.T) {
	db := newTestDB(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// With a canceled context the DB call must fail rather than run anyway.
	if _, err := ListCardDecks(ctx, db, 1); !errors.Is(err, context.Canceled) {
		t.Errorf("ListCardDecks(canceled) = %v, want context.Canceled", err)
	}
}
