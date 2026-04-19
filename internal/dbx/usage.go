package dbx

import (
	"context"
	"fmt"
)

// InitUsage creates the usage_log table if it doesn't exist.
func (d *DB) InitUsage(ctx context.Context) error {
	_, err := d.db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS usage_log (
			id            INTEGER PRIMARY KEY AUTOINCREMENT,
			ts            TEXT    NOT NULL DEFAULT (datetime('now')),
			agent         TEXT    NOT NULL DEFAULT '',
			input_tokens  INTEGER NOT NULL DEFAULT 0,
			output_tokens INTEGER NOT NULL DEFAULT 0
		)
	`)
	if err != nil {
		return fmt.Errorf("dbx: init usage: %w", err)
	}
	return nil
}

// UsageEntry is a single recorded token-usage event.
type UsageEntry struct {
	ID           int
	TS           string
	Agent        string
	InputTokens  int
	OutputTokens int
}

// LogUsage records a token-usage entry.
func (d *DB) LogUsage(ctx context.Context, agent string, in, out int) error {
	_, err := d.db.ExecContext(ctx,
		"INSERT INTO usage_log(agent, input_tokens, output_tokens) VALUES(?, ?, ?)",
		agent, in, out)
	return err
}

// GetRecentUsage returns the most recent limit entries, newest first.
func (d *DB) GetRecentUsage(ctx context.Context, limit int) ([]UsageEntry, error) {
	rows, _, err := d.Query(ctx,
		"SELECT id, ts, agent, input_tokens, output_tokens FROM usage_log ORDER BY id DESC LIMIT ?",
		limit)
	if err != nil {
		return nil, err
	}
	out := make([]UsageEntry, 0, len(rows))
	for _, r := range rows {
		e := UsageEntry{}
		if v, ok := r["id"].(int64); ok {
			e.ID = int(v)
		}
		if v, ok := r["ts"].(string); ok {
			e.TS = v
		}
		if v, ok := r["agent"].(string); ok {
			e.Agent = v
		}
		if v, ok := r["input_tokens"].(int64); ok {
			e.InputTokens = int(v)
		}
		if v, ok := r["output_tokens"].(int64); ok {
			e.OutputTokens = int(v)
		}
		out = append(out, e)
	}
	return out, nil
}

// TotalUsage returns lifetime aggregate token counts.
func (d *DB) TotalUsage(ctx context.Context) (totalIn, totalOut int, err error) {
	rows, _, err := d.Query(ctx,
		"SELECT COALESCE(SUM(input_tokens),0) AS inp, COALESCE(SUM(output_tokens),0) AS outp FROM usage_log")
	if err != nil {
		return 0, 0, err
	}
	if len(rows) > 0 {
		if v, ok := rows[0]["inp"].(int64); ok {
			totalIn = int(v)
		}
		if v, ok := rows[0]["outp"].(int64); ok {
			totalOut = int(v)
		}
	}
	return totalIn, totalOut, nil
}
