package models

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestNormalizeSignupEmail(t *testing.T) {
	// Addresses are lowercased so the UNIQUE constraint is case-insensitive.
	if got, err := normalizeSignupEmail("  Alice@Example.COM "); err != nil || got != "alice@example.com" {
		t.Errorf("normalizeSignupEmail = %q, %v; want alice@example.com", got, err)
	}

	bad := []string{
		"",
		"not-an-email",
		"no-at-sign.com",
		"@example.com",
		"alice@",
		"alice@localhost",  // no dot in domain
		"Bob <bob@x.com>",  // display-name form
		"a@b.com, c@d.com", // list
		strings.Repeat("a", 250) + "@example.com", // over length
	}
	for _, in := range bad {
		if _, err := normalizeSignupEmail(in); !errors.Is(err, ErrInvalidSignupEmail) {
			t.Errorf("normalizeSignupEmail(%q) = %v, want ErrInvalidSignupEmail", in, err)
		}
	}
}

func TestNormalizeSignupInput(t *testing.T) {
	out, err := normalizeSignupInput(SignupInput{Email: "A@B.com", Name: "  Ada  ", Note: " hi\nthere "})
	if err != nil {
		t.Fatalf("normalizeSignupInput: %v", err)
	}
	if out.Email != "a@b.com" || out.Name != "Ada" || out.Note != "hi\nthere" {
		t.Fatalf("unexpected normalization: %+v", out)
	}

	if _, err := normalizeSignupInput(SignupInput{Email: "a@b.com", Name: "   "}); !errors.Is(err, ErrInvalidSignupName) {
		t.Error("blank name should be rejected")
	}
	if _, err := normalizeSignupInput(SignupInput{Email: "a@b.com", Name: "bad\x07name"}); !errors.Is(err, ErrInvalidSignupName) {
		t.Error("control chars in name should be rejected")
	}
	long := strings.Repeat("x", maxSignupNoteLen+1)
	if _, err := normalizeSignupInput(SignupInput{Email: "a@b.com", Name: "Ada", Note: long}); !errors.Is(err, ErrInvalidSignupNote) {
		t.Error("over-long note should be rejected")
	}
}

func TestSignupCRUD(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)

	s, err := CreateSignup(ctx, db, SignupInput{Email: "Ada@Example.com", Name: "Ada", Note: "keen"})
	if err != nil {
		t.Fatalf("CreateSignup: %v", err)
	}
	if s.Email != "ada@example.com" || s.Status != SignupPending {
		t.Fatalf("unexpected signup: %+v", s)
	}
	if s.UserID != nil {
		t.Errorf("new signup should not be linked to a user: %+v", s.UserID)
	}

	// Duplicate email is rejected, case-insensitively.
	if _, err := CreateSignup(ctx, db, SignupInput{Email: "ADA@example.com", Name: "Ada again"}); !errors.Is(err, ErrSignupEmailTaken) {
		t.Errorf("duplicate email = %v, want ErrSignupEmailTaken", err)
	}

	got, err := GetSignup(ctx, db, s.ID)
	if err != nil || got.ID != s.ID {
		t.Fatalf("GetSignup = %+v, %v", got, err)
	}
	if _, err := GetSignup(ctx, db, 9999); !errors.Is(err, ErrSignupNotFound) {
		t.Errorf("missing signup = %v, want ErrSignupNotFound", err)
	}

	updated, err := UpdateSignupStatus(ctx, db, s.ID, SignupInvited)
	if err != nil || updated.Status != SignupInvited {
		t.Fatalf("UpdateSignupStatus = %+v, %v", updated, err)
	}
	if _, err := UpdateSignupStatus(ctx, db, s.ID, "bogus"); !errors.Is(err, ErrInvalidSignupStatus) {
		t.Errorf("bad status = %v, want ErrInvalidSignupStatus", err)
	}
	if _, err := UpdateSignupStatus(ctx, db, 9999, SignupAccepted); !errors.Is(err, ErrSignupNotFound) {
		t.Errorf("missing signup update = %v, want ErrSignupNotFound", err)
	}
}

func TestListSignupsFilterAndBound(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)

	for i := 0; i < 5; i++ {
		if _, err := CreateSignup(ctx, db, SignupInput{
			Email: string(rune('a'+i)) + "@example.com",
			Name:  "User",
		}); err != nil {
			t.Fatalf("CreateSignup(%d): %v", i, err)
		}
	}
	// Move one to invited so the filter has something to exclude.
	all, err := ListSignups(ctx, db, "", 0)
	if err != nil || len(all) != 5 {
		t.Fatalf("ListSignups(all) = %d rows, %v", len(all), err)
	}
	if _, err := UpdateSignupStatus(ctx, db, all[0].ID, SignupInvited); err != nil {
		t.Fatalf("UpdateSignupStatus: %v", err)
	}

	pending, err := ListSignups(ctx, db, SignupPending, 0)
	if err != nil || len(pending) != 4 {
		t.Fatalf("ListSignups(pending) = %d rows, %v", len(pending), err)
	}
	invited, err := ListSignups(ctx, db, SignupInvited, 0)
	if err != nil || len(invited) != 1 {
		t.Fatalf("ListSignups(invited) = %d rows, %v", len(invited), err)
	}

	// An invalid status filter is a client error, not a silent empty list.
	if _, err := ListSignups(ctx, db, "nope", 0); !errors.Is(err, ErrInvalidSignupStatus) {
		t.Errorf("bad status filter = %v, want ErrInvalidSignupStatus", err)
	}

	// limit is honored and clamped.
	two, err := ListSignups(ctx, db, "", 2)
	if err != nil || len(two) != 2 {
		t.Fatalf("ListSignups(limit=2) = %d rows, %v", len(two), err)
	}
	huge, err := ListSignups(ctx, db, "", 10_000)
	if err != nil || len(huge) != 5 {
		t.Fatalf("ListSignups(limit=huge) = %d rows, %v", len(huge), err)
	}

	counts, err := CountSignupsByStatus(ctx, db)
	if err != nil {
		t.Fatalf("CountSignupsByStatus: %v", err)
	}
	if counts[SignupPending] != 4 || counts[SignupInvited] != 1 {
		t.Errorf("counts = %+v", counts)
	}
}

func TestSignupCanceledContext(t *testing.T) {
	db := newTestDB(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, err := ListSignups(ctx, db, "", 0); !errors.Is(err, context.Canceled) {
		t.Errorf("ListSignups(canceled) = %v, want context.Canceled", err)
	}
}
