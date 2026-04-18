// Package dbx provides database helpers with SQLite as the primary backend.
package dbx

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	_ "modernc.org/sqlite" // Pure Go SQLite driver
)

const queryTimeout = 30 * time.Second

// DB wraps a sql.DB with helper methods for agent tool use.
type DB struct {
	db   *sql.DB
	path string
}

// Open opens a SQLite database at the given file path.
func Open(path string) (*DB, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("dbx: open %q: %w", path, err)
	}
	// Verify connection
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("dbx: ping %q: %w", path, err)
	}
	return &DB{db: db, path: path}, nil
}

// Close closes the database connection.
func (d *DB) Close() error {
	return d.db.Close()
}

// Query executes a SQL query and returns rows as maps.
func (d *DB) Query(ctx context.Context, query string, args ...interface{}) ([]map[string]interface{}, []string, error) {
	ctx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()

	// Determine if it's a SELECT or a mutating query
	trimmed := strings.TrimSpace(strings.ToUpper(query))
	if strings.HasPrefix(trimmed, "SELECT") || strings.HasPrefix(trimmed, "PRAGMA") {
		return d.queryRows(ctx, query, args...)
	}

	result, err := d.db.ExecContext(ctx, query, args...)
	if err != nil {
		return nil, nil, fmt.Errorf("dbx: exec: %w", err)
	}
	affected, _ := result.RowsAffected()
	return []map[string]interface{}{
		{"affected_rows": affected},
	}, []string{"affected_rows"}, nil
}

func (d *DB) queryRows(ctx context.Context, query string, args ...interface{}) ([]map[string]interface{}, []string, error) {
	rows, err := d.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, nil, fmt.Errorf("dbx: query: %w", err)
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return nil, nil, fmt.Errorf("dbx: columns: %w", err)
	}

	var results []map[string]interface{}
	for rows.Next() {
		values := make([]interface{}, len(cols))
		valuePtrs := make([]interface{}, len(cols))
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			return nil, nil, fmt.Errorf("dbx: scan: %w", err)
		}

		row := make(map[string]interface{})
		for i, col := range cols {
			row[col] = values[i]
		}
		results = append(results, row)
	}

	return results, cols, rows.Err()
}
