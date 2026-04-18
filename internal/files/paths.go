package files

import (
	"path/filepath"
	"strings"
)

// ResolvePath resolves a potentially relative path against the manager root.
func (m *Manager) ResolvePath(path string) string {
	if filepath.IsAbs(path) {
		return filepath.Clean(path)
	}
	return filepath.Clean(filepath.Join(m.Root, path))
}

// RelPath returns the path relative to the manager root.
func (m *Manager) RelPath(absPath string) string {
	rel, err := filepath.Rel(m.Root, absPath)
	if err != nil {
		return absPath
	}
	return rel
}

// IsWithinRoot checks if a path is within the manager root directory.
func (m *Manager) IsWithinRoot(path string) bool {
	abs := m.ResolvePath(path)
	rootAbs, err := filepath.Abs(m.Root)
	if err != nil {
		return false
	}
	return abs == rootAbs || strings.HasPrefix(abs, rootAbs+string(filepath.Separator))
}
