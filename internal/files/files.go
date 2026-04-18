// Package files provides filesystem operations with path safety and atomic writes.
package files

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/syn3rgy2026/UntrainedModels_Syn3rgy_SatyamUttamPandey/internal/security"
)

// Manager provides safe filesystem operations rooted at a base directory.
type Manager struct {
	Root string
}

// NewManager creates a file manager rooted at the given directory.
func NewManager(root string) *Manager {
	return &Manager{Root: root}
}

// safePath resolves and validates a path within the root directory.
func (m *Manager) safePath(path string) (string, error) {
	return security.SanitizePath(m.Root, path)
}

// ReadFile reads the entire contents of a file.
func (m *Manager) ReadFile(path string) (string, error) {
	p, err := m.safePath(path)
	if err != nil {
		return "", fmt.Errorf("files: read: %w", err)
	}
	data, err := os.ReadFile(p)
	if err != nil {
		return "", fmt.Errorf("files: read %q: %w", path, err)
	}
	return string(data), nil
}

// WriteFile writes content to a file atomically by writing to a temp file first.
func (m *Manager) WriteFile(path, content string) error {
	p, err := m.safePath(path)
	if err != nil {
		return fmt.Errorf("files: write: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
		return fmt.Errorf("files: mkdir: %w", err)
	}
	// Atomic write: write to temp, then rename
	tmp := p + ".tmp"
	if err := os.WriteFile(tmp, []byte(content), 0644); err != nil {
		return fmt.Errorf("files: write tmp: %w", err)
	}
	if err := os.Rename(tmp, p); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("files: rename: %w", err)
	}
	return nil
}

// AppendFile appends content to a file, creating it if necessary.
func (m *Manager) AppendFile(path, content string) error {
	p, err := m.safePath(path)
	if err != nil {
		return fmt.Errorf("files: append: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
		return fmt.Errorf("files: mkdir: %w", err)
	}
	f, err := os.OpenFile(p, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("files: append open: %w", err)
	}
	defer f.Close()
	_, err = f.WriteString(content)
	return err
}

// EditFile replaces the first occurrence of oldStr with newStr in a file.
func (m *Manager) EditFile(path, oldStr, newStr string) error {
	content, err := m.ReadFile(path)
	if err != nil {
		return err
	}
	if !strings.Contains(content, oldStr) {
		return fmt.Errorf("files: edit %q: old_string not found", path)
	}
	updated := strings.Replace(content, oldStr, newStr, 1)
	return m.WriteFile(path, updated)
}

// MoveFile moves a file from src to dst.
func (m *Manager) MoveFile(src, dst string) error {
	s, err := m.safePath(src)
	if err != nil {
		return fmt.Errorf("files: move src: %w", err)
	}
	d, err := m.safePath(dst)
	if err != nil {
		return fmt.Errorf("files: move dst: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(d), 0755); err != nil {
		return fmt.Errorf("files: move mkdir: %w", err)
	}
	return os.Rename(s, d)
}

// CopyFile copies a file from src to dst.
func (m *Manager) CopyFile(src, dst string) error {
	s, err := m.safePath(src)
	if err != nil {
		return fmt.Errorf("files: copy src: %w", err)
	}
	d, err := m.safePath(dst)
	if err != nil {
		return fmt.Errorf("files: copy dst: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(d), 0755); err != nil {
		return fmt.Errorf("files: copy mkdir: %w", err)
	}
	in, err := os.Open(s)
	if err != nil {
		return fmt.Errorf("files: copy open: %w", err)
	}
	defer in.Close()

	out, err := os.Create(d)
	if err != nil {
		return fmt.Errorf("files: copy create: %w", err)
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}

// DeleteFile removes a file.
func (m *Manager) DeleteFile(path string) error {
	p, err := m.safePath(path)
	if err != nil {
		return fmt.Errorf("files: delete: %w", err)
	}
	return os.Remove(p)
}

// Mkdir creates a directory and all parents.
func (m *Manager) Mkdir(path string) error {
	p, err := m.safePath(path)
	if err != nil {
		return fmt.Errorf("files: mkdir: %w", err)
	}
	return os.MkdirAll(p, 0755)
}

// CreateFile creates a new file; fails if the file already exists.
func (m *Manager) CreateFile(path, content string) error {
	p, err := m.safePath(path)
	if err != nil {
		return fmt.Errorf("files: create: %w", err)
	}
	if _, err := os.Stat(p); err == nil {
		return errors.New("files: file already exists: " + path)
	}
	return m.WriteFile(path, content)
}

// Exists checks whether a path exists within the root.
func (m *Manager) Exists(path string) bool {
	p, err := m.safePath(path)
	if err != nil {
		return false
	}
	_, err = os.Stat(p)
	return err == nil
}
