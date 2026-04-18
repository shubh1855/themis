package files

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/syn3rgy2026/UntrainedModels_Syn3rgy_SatyamUttamPandey/internal/models"
)

// ListDir lists the contents of a directory.
func (m *Manager) ListDir(path string) ([]models.FileEntry, error) {
	p, err := m.safePath(path)
	if err != nil {
		return nil, fmt.Errorf("files: listdir: %w", err)
	}

	entries, err := os.ReadDir(p)
	if err != nil {
		return nil, fmt.Errorf("files: listdir %q: %w", path, err)
	}

	result := make([]models.FileEntry, 0, len(entries))
	for _, e := range entries {
		info, err := e.Info()
		if err != nil {
			continue
		}
		result = append(result, models.FileEntry{
			Name:  e.Name(),
			Path:  filepath.Join(path, e.Name()),
			IsDir: e.IsDir(),
			Size:  info.Size(),
		})
	}
	return result, nil
}

// Tree builds a text tree representation of a directory up to maxDepth.
func (m *Manager) Tree(path string, maxDepth int) (string, error) {
	p, err := m.safePath(path)
	if err != nil {
		return "", fmt.Errorf("files: tree: %w", err)
	}

	var sb strings.Builder
	sb.WriteString(filepath.Base(p) + "/\n")
	if err := m.buildTree(&sb, p, "", maxDepth, 0); err != nil {
		return "", err
	}
	return sb.String(), nil
}

func (m *Manager) buildTree(sb *strings.Builder, dir, prefix string, maxDepth, currentDepth int) error {
	if currentDepth >= maxDepth {
		return nil
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}

	for i, e := range entries {
		isLast := i == len(entries)-1
		connector := "├── "
		if isLast {
			connector = "└── "
		}

		name := e.Name()
		if e.IsDir() {
			name += "/"
		}
		sb.WriteString(prefix + connector + name + "\n")

		if e.IsDir() {
			childPrefix := prefix + "│   "
			if isLast {
				childPrefix = prefix + "    "
			}
			childPath := filepath.Join(dir, e.Name())
			if err := m.buildTree(sb, childPath, childPrefix, maxDepth, currentDepth+1); err != nil {
				continue
			}
		}
	}
	return nil
}

// Glob returns files matching a glob pattern within the root.
func (m *Manager) Glob(pattern string) ([]string, error) {
	p, err := m.safePath(pattern)
	if err != nil {
		// If pattern itself can't be sanitized, try matching from root
		p = filepath.Join(m.Root, pattern)
	}

	matches, err := filepath.Glob(p)
	if err != nil {
		return nil, fmt.Errorf("files: glob %q: %w", pattern, err)
	}

	// Convert to relative paths
	result := make([]string, 0, len(matches))
	for _, match := range matches {
		rel := m.RelPath(match)
		result = append(result, rel)
	}
	return result, nil
}
