package dbx

import (
	"context"
	"fmt"
	"os"
	"strings"
)

func (d *DB) Migrate(ctx context.Context, sqlScript string) error {
	statements := splitStatements(sqlScript)

	tx, err := d.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("dbx: begin tx: %w", err)
	}

	for i, stmt := range statements {
		stmt = strings.TrimSpace(stmt)
		if stmt == "" {
			continue
		}
		if _, err := tx.ExecContext(ctx, stmt); err != nil {
			tx.Rollback()
			return fmt.Errorf("dbx: migrate statement %d: %w", i+1, err)
		}
	}

	return tx.Commit()
}

func (d *DB) MigrateFile(ctx context.Context, path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("dbx: read migration %q: %w", path, err)
	}
	return d.Migrate(ctx, string(data))
}

func splitStatements(sql string) []string {
	return strings.Split(sql, ";")
}
