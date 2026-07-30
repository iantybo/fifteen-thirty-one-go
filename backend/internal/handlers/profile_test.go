package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	_ "github.com/mattn/go-sqlite3"
)

// newProfileTestDB creates an in-memory users table holding a single user whose
// email starts out as a known-good value.
func newProfileTestDB(t *testing.T) *sql.DB {
	t.Helper()

	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	if _, err := db.Exec(`CREATE TABLE users (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		username TEXT NOT NULL UNIQUE,
		email TEXT,
		password_hash TEXT NOT NULL,
		created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
		games_played INTEGER NOT NULL DEFAULT 0,
		games_won INTEGER NOT NULL DEFAULT 0
	)`); err != nil {
		t.Fatalf("create users: %v", err)
	}

	if _, err := db.Exec(
		`INSERT INTO users(id, username, email, password_hash) VALUES (1, 'alice', 'original@example.com', 'hash')`,
	); err != nil {
		t.Fatalf("seed user: %v", err)
	}

	return db
}

// doUpdateProfile invokes the handler with userID 1 already authenticated.
func doUpdateProfile(t *testing.T, db *sql.DB, body string) *httptest.ResponseRecorder {
	t.Helper()

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPut, "/me/profile", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set("userID", int64(1))

	UpdateProfileHandler(db)(c)
	return w
}

func currentEmail(t *testing.T, db *sql.DB) string {
	t.Helper()

	var email sql.NullString
	if err := db.QueryRow(`SELECT email FROM users WHERE id = 1`).Scan(&email); err != nil {
		t.Fatalf("read email: %v", err)
	}
	return email.String
}

func TestUpdateProfileHandlerRejectsInvalidEmail(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{"missing field", `{}`},
		{"empty", `{"email":""}`},
		{"whitespace only", `{"email":"   "}`},
		{"no at sign", `{"email":"notanemail"}`},
		{"no domain", `{"email":"user@"}`},
		{"no local part", `{"email":"@example.com"}`},
		{"spaces inside", `{"email":"user name@example.com"}`},
		{"display name form", `{"email":"Alice <alice@example.com>"}`},
		{"too long", `{"email":"` + strings.Repeat("a", 250) + `@example.com"}`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			db := newProfileTestDB(t)
			w := doUpdateProfile(t, db, tc.body)

			if w.Code != http.StatusBadRequest {
				t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
			}
			// The record must be untouched on a validation failure.
			if got := currentEmail(t, db); got != "original@example.com" {
				t.Error("email was modified despite validation failure")
			}
		})
	}
}

func TestUpdateProfileHandlerPersistsValidEmail(t *testing.T) {
	cases := []struct {
		name string
		body string
		want string
	}{
		{"plain", `{"email":"new@example.com"}`, "new@example.com"},
		{"surrounding whitespace trimmed", `{"email":"  spaced@example.com  "}`, "spaced@example.com"},
		{"plus addressing", `{"email":"user+tag@example.com"}`, "user+tag@example.com"},
		{"subdomain", `{"email":"user@mail.example.co.uk"}`, "user@mail.example.co.uk"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			db := newProfileTestDB(t)
			w := doUpdateProfile(t, db, tc.body)

			if w.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
			}
			if got := currentEmail(t, db); got != tc.want {
				t.Error("stored email does not match the submitted address")
			}

			var resp map[string]any
			if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			if resp["email"] != tc.want {
				t.Error("response email does not match the submitted address")
			}
		})
	}
}

func TestUpdateProfileHandlerResponseOmitsCredentials(t *testing.T) {
	db := newProfileTestDB(t)
	w := doUpdateProfile(t, db, `{"email":"new@example.com"}`)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
	if body := w.Body.String(); strings.Contains(body, "hash") || strings.Contains(body, "password") {
		t.Error("response leaks credential material")
	}

	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	allowed := map[string]bool{"id": true, "username": true, "email": true, "games_played": true, "games_won": true}
	for k := range resp {
		if !allowed[k] {
			t.Errorf("unexpected field %q in profile response", k)
		}
	}
}

func TestUpdateProfileHandlerRequiresAuth(t *testing.T) {
	db := newProfileTestDB(t)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPut, "/me/profile", strings.NewReader(`{"email":"new@example.com"}`))
	c.Request.Header.Set("Content-Type", "application/json")
	// No userID set on the context.

	UpdateProfileHandler(db)(c)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", w.Code, http.StatusUnauthorized)
	}
	if got := currentEmail(t, db); got != "original@example.com" {
		t.Error("email was modified for an unauthenticated request")
	}
}
