package handlers

import "testing"

// parseClientID / isValidClientID parse the shared "c:<gameId>:<userId>:<seq>"
// contract with frontend/src/sync/clientId.ts. These tests exercise the happy
// path and the malformed cases the frontend's parseClientId rejects.

func TestParseClientID(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		id     string
		wantG  int64
		wantU  int64
		wantS  int64
		wantOK bool
	}{
		{"valid", "c:1:9:3", 1, 9, 3, true},
		{"valid multi-digit", "c:123:456:789", 123, 456, 789, true},
		{"valid zeros", "c:0:0:0", 0, 0, 0, true},
		{"wrong prefix", "x:1:9:3", 0, 0, 0, false},
		{"empty prefix", ":1:9:3", 0, 0, 0, false},
		{"too few segments", "c:1:9", 0, 0, 0, false},
		{"too many segments", "c:1:9:3:4", 0, 0, 0, false},
		{"non-numeric game", "c:a:9:3", 0, 0, 0, false},
		{"non-numeric user", "c:1:b:3", 0, 0, 0, false},
		{"non-numeric seq", "c:1:9:z", 0, 0, 0, false},
		{"empty string", "", 0, 0, 0, false},
		{"prefix only", "c", 0, 0, 0, false},
		{"blank seq", "c:1:9:", 0, 0, 0, false},
		{"blank game", "c::9:3", 0, 0, 0, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			g, u, s, ok := parseClientID(tc.id)
			if ok != tc.wantOK {
				t.Fatalf("parseClientID(%q) ok = %v, want %v", tc.id, ok, tc.wantOK)
			}
			if !tc.wantOK {
				return
			}
			if g != tc.wantG || u != tc.wantU || s != tc.wantS {
				t.Errorf("parseClientID(%q) = (%d, %d, %d), want (%d, %d, %d)",
					tc.id, g, u, s, tc.wantG, tc.wantU, tc.wantS)
			}
		})
	}
}

func TestIsValidClientID(t *testing.T) {
	t.Parallel()

	cases := []struct {
		id   string
		want bool
	}{
		{"c:1:9:3", true},
		{"c:0:0:0", true},
		{"x:1:9:3", false},
		{"c:1:9", false},
		{"c:1:9:3:4", false},
		{"c:1:9:z", false},
		{"", false},
	}
	for _, tc := range cases {
		t.Run(tc.id, func(t *testing.T) {
			t.Parallel()
			if got := isValidClientID(tc.id); got != tc.want {
				t.Errorf("isValidClientID(%q) = %v, want %v", tc.id, got, tc.want)
			}
		})
	}
}
