package files

import (
	"path/filepath"
	"strings"
)

func (m *Manager) ResolvePath(path string) string {
	if filepath.IsAbs(path) {
		return filepath.Clean(path)
	}
	return filepath.Clean(filepath.Join(m.Root, path))
}

func (m *Manager) RelPath(absPath string) string {
	rel, err := filepath.Rel(m.Root, absPath)
	if err != nil {
		return absPath
	}
	return rel
}

func (m *Manager) IsWithinRoot(path string) bool {
	abs := m.ResolvePath(path)
	rootAbs, err := filepath.Abs(m.Root)
	if err != nil {
		return false
	}
	return abs == rootAbs || strings.HasPrefix(abs, rootAbs+string(filepath.Separator))
}
