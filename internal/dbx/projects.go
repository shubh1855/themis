package dbx

import (
	"context"
	"fmt"
	"time"
)

func (d *DB) InitProjects(ctx context.Context) error {
	_, err := d.db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS projects (
			id         INTEGER PRIMARY KEY AUTOINCREMENT,
			name       TEXT    NOT NULL,
			path       TEXT    NOT NULL DEFAULT '',
			created_at TEXT    NOT NULL DEFAULT (datetime('now')),
			updated_at TEXT    NOT NULL DEFAULT (datetime('now'))
		);
		CREATE TABLE IF NOT EXISTS chats (
			id         INTEGER PRIMARY KEY AUTOINCREMENT,
			project_id INTEGER REFERENCES projects(id),
			title      TEXT    NOT NULL,
			created_at TEXT    NOT NULL DEFAULT (datetime('now')),
			updated_at TEXT    NOT NULL DEFAULT (datetime('now'))
		);
		CREATE TABLE IF NOT EXISTS messages (
			id         INTEGER PRIMARY KEY AUTOINCREMENT,
			chat_id    INTEGER NOT NULL REFERENCES chats(id),
			role       TEXT    NOT NULL,
			content    TEXT    NOT NULL,
			created_at TEXT    NOT NULL DEFAULT (datetime('now'))
		);
	`)
	if err != nil {
		return fmt.Errorf("dbx: init projects: %w", err)
	}
	return nil
}

type Project struct {
	ID        int
	Name      string
	Path      string
	CreatedAt string
	UpdatedAt string
}

type Chat struct {
	ID        int
	ProjectID int
	Title     string
	CreatedAt string
	UpdatedAt string
}

func (d *DB) ListProjects(ctx context.Context) ([]Project, error) {
	rows, _, err := d.Query(ctx, "SELECT id, name, path, created_at, updated_at FROM projects ORDER BY updated_at DESC LIMIT 20")
	if err != nil {
		return nil, err
	}
	var out []Project
	for _, r := range rows {
		p := Project{}
		if v, ok := r["id"].(int64); ok {
			p.ID = int(v)
		}
		if v, ok := r["name"].(string); ok {
			p.Name = v
		}
		if v, ok := r["path"].(string); ok {
			p.Path = v
		}
		if v, ok := r["created_at"].(string); ok {
			p.CreatedAt = v
		}
		if v, ok := r["updated_at"].(string); ok {
			p.UpdatedAt = v
		}
		out = append(out, p)
	}
	return out, nil
}

func (d *DB) CreateProject(ctx context.Context, name, path string) (int64, error) {
	result, err := d.db.ExecContext(ctx, "INSERT INTO projects(name, path) VALUES(?, ?)", name, path)
	if err != nil {
		return 0, fmt.Errorf("dbx: create project: %w", err)
	}
	return result.LastInsertId()
}

func (d *DB) ListChats(ctx context.Context, projectID int) ([]Chat, error) {
	rows, _, err := d.Query(ctx, "SELECT id, project_id, title, created_at, updated_at FROM chats WHERE project_id = ? ORDER BY updated_at DESC LIMIT 20", projectID)
	if err != nil {
		return nil, err
	}
	var out []Chat
	for _, r := range rows {
		c := Chat{}
		if v, ok := r["id"].(int64); ok {
			c.ID = int(v)
		}
		if v, ok := r["project_id"].(int64); ok {
			c.ProjectID = int(v)
		}
		if v, ok := r["title"].(string); ok {
			c.Title = v
		}
		if v, ok := r["created_at"].(string); ok {
			c.CreatedAt = v
		}
		if v, ok := r["updated_at"].(string); ok {
			c.UpdatedAt = v
		}
		out = append(out, c)
	}
	return out, nil
}

func (d *DB) CreateChat(ctx context.Context, projectID int, title string) (int64, error) {
	result, err := d.db.ExecContext(ctx, "INSERT INTO chats(project_id, title) VALUES(?, ?)", projectID, title)
	if err != nil {
		return 0, fmt.Errorf("dbx: create chat: %w", err)
	}
	return result.LastInsertId()
}

func (d *DB) TouchProject(ctx context.Context, id int) error {
	_, err := d.db.ExecContext(ctx, "UPDATE projects SET updated_at = ? WHERE id = ?", time.Now().UTC().Format("2006-01-02 15:04:05"), id)
	return err
}

func (d *DB) TouchChat(ctx context.Context, id int) error {
	_, err := d.db.ExecContext(ctx, "UPDATE chats SET updated_at = ? WHERE id = ?", time.Now().UTC().Format("2006-01-02 15:04:05"), id)
	return err
}

func (d *DB) RecentChats(ctx context.Context) ([]Chat, error) {
	rows, _, err := d.Query(ctx, "SELECT id, project_id, title, created_at, updated_at FROM chats ORDER BY updated_at DESC LIMIT 10")
	if err != nil {
		return nil, err
	}
	var out []Chat
	for _, r := range rows {
		c := Chat{}
		if v, ok := r["id"].(int64); ok {
			c.ID = int(v)
		}
		if v, ok := r["project_id"].(int64); ok {
			c.ProjectID = int(v)
		}
		if v, ok := r["title"].(string); ok {
			c.Title = v
		}
		if v, ok := r["created_at"].(string); ok {
			c.CreatedAt = v
		}
		if v, ok := r["updated_at"].(string); ok {
			c.UpdatedAt = v
		}
		out = append(out, c)
	}
	return out, nil
}
