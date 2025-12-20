package database

import (
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	_ "github.com/mattn/go-sqlite3"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

func OpenAndMigrate(dbPath string) (*sql.DB, error) {
	if dbPath == "" {
		return nil, fmt.Errorf("DATABASE_PATH is required")
	}

	// Ensure parent directory exists for file-backed DBs.
	if looksLikeFilePath(dbPath) {
		if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
			return nil, fmt.Errorf("mkdir db dir: %w", err)
		}
	}

	db, err := sql.Open("sqlite3", sqliteDSN(dbPath))
	if err != nil {
		return nil, fmt.Errorf("sql open: %w", err)
	}
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("db ping: %w", err)
	}
	if err := migrate(db); err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}

func sqliteDSN(dbPath string) string {
	// foreign_keys=on ensures FK constraints are enforced at the connection level.
	// _busy_timeout reduces spurious SQLITE_BUSY for concurrent reads/writes.
	if strings.HasPrefix(dbPath, "file:") || dbPath == ":memory:" {
		return dbPath
	}
	return fmt.Sprintf("file:%s?_foreign_keys=on&_busy_timeout=5000", dbPath)
}

func looksLikeFilePath(p string) bool {
	if p == ":memory:" {
		return false
	}
	if strings.HasPrefix(p, "file:") {
		return false
	}
	return true
}

func migrate(db *sql.DB) error {
	// Create schema_migrations (if not created by the first migration).
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (version TEXT PRIMARY KEY, applied_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP)`); err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}

	applied, err := loadAppliedVersions(db)
	if err != nil {
		return err
	}

	migs, err := listMigrationFiles(migrationsFS, "migrations")
	if err != nil {
		return err
	}

	for _, m := range migs {
		if applied[m] {
			continue
		}
		body, err := fs.ReadFile(migrationsFS, "migrations/"+m)
		if err != nil {
			return fmt.Errorf("read migration %s: %w", m, err)
		}

		tx, err := db.Begin()
		if err != nil {
			return fmt.Errorf("begin tx: %w", err)
		}
		if err := execSQLScript(tx, string(body)); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("apply migration %s: %w", m, err)
		}
		if _, err := tx.Exec(`INSERT INTO schema_migrations(version) VALUES (?)`, m); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("record migration %s: %w", m, err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit migration %s: %w", m, err)
		}
	}

	return nil
}

func loadAppliedVersions(db *sql.DB) (map[string]bool, error) {
	rows, err := db.Query(`SELECT version FROM schema_migrations`)
	if err != nil {
		return nil, fmt.Errorf("list applied migrations: %w", err)
	}
	defer rows.Close()

	out := map[string]bool{}
	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err != nil {
			return nil, fmt.Errorf("scan migration version: %w", err)
		}
		out[v] = true
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate migration versions: %w", err)
	}
	return out, nil
}

func listMigrationFiles(fsys fs.FS, dir string) ([]string, error) {
	entries, err := fs.ReadDir(fsys, dir)
	if err != nil {
		return nil, fmt.Errorf("readdir %s: %w", dir, err)
	}
	var files []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if strings.HasSuffix(name, ".sql") {
			files = append(files, name)
		}
	}
	sort.Strings(files)
	return files, nil
}

type sqlExecer interface {
	Exec(query string, args ...any) (sql.Result, error)
}

func execSQLScript(exec sqlExecer, script string) error {
	// Very small migration runner:
	// - strips line comments (only when not inside quotes)
	// - splits on ';'
	// This is sufficient for our simple schema files.
	cleaned := stripLineCommentsOutsideQuotes(script)

	parts := strings.Split(cleaned, ";")
	for _, p := range parts {
		stmt := strings.TrimSpace(p)
		if stmt == "" {
			continue
		}
		if _, err := exec.Exec(stmt); err != nil {
			return err
		}
	}
	return nil
}

func stripLineCommentsOutsideQuotes(s string) string {
	var b strings.Builder
	b.Grow(len(s))

	inSingle := false
	inDouble := false
	for i := 0; i < len(s); i++ {
		ch := s[i]

		// Handle quote toggles + escaped quote by doubling.
		if ch == '\'' && !inDouble {
			// SQL escapes single quote as ''.
			if inSingle && i+1 < len(s) && s[i+1] == '\'' {
				b.WriteByte(ch)
				b.WriteByte(ch)
				i++
				continue
			}
			inSingle = !inSingle
			b.WriteByte(ch)
			continue
		}
		if ch == '"' && !inSingle {
			if inDouble && i+1 < len(s) && s[i+1] == '"' {
				b.WriteByte(ch)
				b.WriteByte(ch)
				i++
				continue
			}
			inDouble = !inDouble
			b.WriteByte(ch)
			continue
		}

		// Line comment start: "--" when not inside quotes.
		if !inSingle && !inDouble && ch == '-' && i+1 < len(s) && s[i+1] == '-' {
			// Skip until newline (but keep the newline).
			for i < len(s) && s[i] != '\n' {
				i++
			}
			if i < len(s) && s[i] == '\n' {
				b.WriteByte('\n')
			}
			continue
		}

		b.WriteByte(ch)
	}

	return b.String()
}


