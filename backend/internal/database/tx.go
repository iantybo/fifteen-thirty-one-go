package database

import "database/sql"

// WithTx executes fn inside a database transaction. If fn returns an error or
// panics, the transaction is rolled back; otherwise it is committed. The
// caller receives the value returned by fn (or nil on error).
func WithTx(db *sql.DB, fn func(*sql.Tx) error) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()
	if err := fn(tx); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	committed = true
	return nil
}
