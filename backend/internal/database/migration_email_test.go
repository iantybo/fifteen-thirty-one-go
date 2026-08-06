package database

import (
	"path/filepath"
	"testing"
)

func TestMigrationAddsEmailColumn(t *testing.T) {
	db, err := OpenAndMigrate(filepath.Join(t.TempDir(), "t.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	if _, err := db.Exec(`INSERT INTO users(id, username, password_hash) VALUES (1, 'bob', 'h')`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE users SET email = ? WHERE id = 1`, "bob@example.com"); err != nil {
		t.Fatalf("email column missing after migrate: %v", err)
	}
	var got string
	if err := db.QueryRow(`SELECT email FROM users WHERE id = 1`).Scan(&got); err != nil {
		t.Fatal(err)
	}
	if got != "bob@example.com" {
		t.Fatal("email round-trip mismatch")
	}
	t.Log("migration applied; email round-trip succeeded")
}
