package database

import (
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"net/url"
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
	if fsPath, ok := filesystemPathFromDBPath(dbPath); ok {
		if err := os.MkdirAll(filepath.Dir(fsPath), 0o755); err != nil {
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
	if dbPath == ":memory:" {
		return "file::memory:?_foreign_keys=1&_busy_timeout=5000"
	}
	if strings.HasPrefix(dbPath, "file:") {
		base := dbPath
		query := ""
		if idx := strings.Index(dbPath, "?"); idx >= 0 {
			base = dbPath[:idx]
			query = dbPath[idx+1:]
		}
		q, err := url.ParseQuery(query)
		if err != nil {
			// Preserve user intent on malformed queries.
			return dbPath
		}
		if q.Get("_foreign_keys") == "" {
			q.Set("_foreign_keys", "1")
		}
		if q.Get("_busy_timeout") == "" {
			q.Set("_busy_timeout", "5000")
		}
		enc := q.Encode()
		if enc == "" {
			return base
		}
		return base + "?" + enc
	}
	return fmt.Sprintf("file:%s?_foreign_keys=1&_busy_timeout=5000", dbPath)
}

func looksLikeFilePath(p string) bool {
	// Treat any DSN that resolves to a filesystem path as file-backed.
	// This includes file: URIs like file:/path/to/db.sqlite or file:./data/db.sqlite?cache=shared.
	// Memory-backed DSNs (":memory:" / "file::memory:...") return false.
	_, ok := filesystemPathFromDBPath(p)
	return ok
}

// filesystemPathFromDBPath returns the underlying filesystem path for SQLite DSN-ish inputs.
// It strips the "file:" prefix and any query string (everything after '?').
// Returns ok=false for memory-backed databases (":memory:" or "file::memory:...") and for empty paths.
func filesystemPathFromDBPath(dbPath string) (path string, ok bool) {
	if dbPath == "" {
		return "", false
	}
	if dbPath == ":memory:" {
		return "", false
	}
	if strings.HasPrefix(dbPath, "file:") {
		rest := strings.TrimPrefix(dbPath, "file:")
		if i := strings.Index(rest, "?"); i >= 0 {
			rest = rest[:i]
		}
		// file::memory: (and variants) are not filesystem-backed.
		if rest == "" || rest == ":memory:" || strings.HasPrefix(rest, ":memory:") || strings.HasPrefix(rest, "::memory:") {
			return "", false
		}
		return rest, true
	}
	// Plain paths are treated as filesystem-backed.
	return dbPath, true
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

	stmts := splitSQLStatements(cleaned)
	for _, stmt := range stmts {
		stmt = strings.TrimSpace(stmt)
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
			// Explicitly handle and consume the newline (do not rely on outer loop increment).
			if i < len(s) && s[i] == '\n' {
				b.WriteByte('\n')
				i++
			}
			// The outer for-loop will increment i once more; offset that so we don't skip a character.
			i--
			continue
		}

		b.WriteByte(ch)
	}

	return b.String()
}

func splitSQLStatements(s string) []string {
	var out []string
	var b strings.Builder
	b.Grow(len(s))

	inSingle := false
	inDouble := false
	// SQLite triggers use BEGIN...END blocks that may contain semicolons.
	// Our migration runner splits on ';', so we must avoid splitting inside these blocks.
	//
	// We intentionally keep this heuristic small:
	// - detect CREATE TRIGGER ... BEGIN
	// - also handle optional qualifiers between CREATE and TRIGGER (e.g., CREATE TEMP TRIGGER)
	// - once inside BEGIN..END, ignore ';' until END is seen
	inTriggerDef := false
	blockDepth := 0
	var tok strings.Builder
	// Track the last two tokens (lowercased) to recognize "CREATE <qualifier?> TRIGGER".
	prevTok1 := ""
	prevTok2 := ""
	for i := 0; i < len(s); i++ {
		ch := s[i]

		if ch == '\'' && !inDouble {
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

		if !inSingle && !inDouble {
			// Tokenize outside quotes to detect BEGIN/END within CREATE TRIGGER blocks.
			isWord := (ch >= 'A' && ch <= 'Z') || (ch >= 'a' && ch <= 'z') || ch == '_'
			if isWord {
				tok.WriteByte(ch)
			} else if tok.Len() > 0 {
				t := strings.ToLower(tok.String())
				tok.Reset()

				// Track when we're inside a CREATE TRIGGER statement.
				if t == "trigger" && (prevTok1 == "create" || prevTok2 == "create") {
					inTriggerDef = true
				}
				// Track BEGIN..END blocks only for triggers.
				if inTriggerDef {
					// SQLite triggers use BEGIN..END, but trigger bodies can contain CASE..END
					// expressions. Treat CASE like a nested block so its END doesn't terminate
					// the trigger BEGIN..END scope.
					if t == "begin" || t == "case" {
						blockDepth++
					} else if t == "end" && blockDepth > 0 {
						blockDepth--
					}
				}
				// Shift token window.
				prevTok2 = prevTok1
				prevTok1 = t
			}
		}

		if !inSingle && !inDouble && ch == ';' && blockDepth == 0 {
			out = append(out, b.String())
			b.Reset()
			inTriggerDef = false
			prevTok1 = ""
			prevTok2 = ""
			continue
		}
		b.WriteByte(ch)
	}
	// Flush trailing token, if any.
	if !inSingle && !inDouble && tok.Len() > 0 {
		tok.Reset()
	}
	if b.Len() > 0 {
		out = append(out, b.String())
	}
	return out
}


