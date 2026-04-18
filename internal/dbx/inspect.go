package dbx

import (
	"context"
	"fmt"
)

func (d *DB) Tables(ctx context.Context) ([]string, error) {
	rows, _, err := d.Query(ctx, "SELECT name FROM sqlite_master WHERE type='table' ORDER BY name")
	if err != nil {
		return nil, fmt.Errorf("dbx: tables: %w", err)
	}
	var names []string
	for _, row := range rows {
		if name, ok := row["name"].(string); ok {
			names = append(names, name)
		}
	}
	return names, nil
}

func (d *DB) Schema(ctx context.Context, table string) (string, error) {
	rows, _, err := d.Query(ctx, "SELECT sql FROM sqlite_master WHERE type='table' AND name=?", table)
	if err != nil {
		return "", fmt.Errorf("dbx: schema: %w", err)
	}
	if len(rows) == 0 {
		return "", fmt.Errorf("dbx: table %q not found", table)
	}
	if s, ok := rows[0]["sql"].(string); ok {
		return s, nil
	}
	return "", fmt.Errorf("dbx: invalid schema for %q", table)
}

func (d *DB) TableInfo(ctx context.Context, table string) ([]map[string]interface{}, error) {
	rows, _, err := d.Query(ctx, fmt.Sprintf("PRAGMA table_info(%s)", table))
	if err != nil {
		return nil, fmt.Errorf("dbx: table_info: %w", err)
	}
	return rows, nil
}
