package dbx

import (
	"context"
	"database/sql"
	"fmt"
)

// InitSettings ensures the key-value settings table exists.
func (d *DB) InitSettings(ctx context.Context) error {
	_, err := d.db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS settings (
			key   TEXT PRIMARY KEY,
			value TEXT NOT NULL
		)
	`)
	if err != nil {
		return fmt.Errorf("dbx: init settings: %w", err)
	}
	return nil
}

// GetSetting returns the stored value for key. ok is false if the key is absent.
func (d *DB) GetSetting(ctx context.Context, key string) (value string, ok bool, err error) {
	row := d.db.QueryRowContext(ctx, `SELECT value FROM settings WHERE key = ?`, key)
	err = row.Scan(&value)
	if err == sql.ErrNoRows {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("dbx: get setting %q: %w", key, err)
	}
	return value, true, nil
}

// SetSetting upserts a value for key.
func (d *DB) SetSetting(ctx context.Context, key, value string) error {
	_, err := d.db.ExecContext(ctx, `
		INSERT INTO settings(key, value) VALUES(?, ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value
	`, key, value)
	if err != nil {
		return fmt.Errorf("dbx: set setting %q: %w", key, err)
	}
	return nil
}
