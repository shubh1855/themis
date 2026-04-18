package security

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// SanitizePath resolves a path within a root directory, preventing traversal attacks.
// Returns the absolute, cleaned path or an error if traversal was detected.
func SanitizePath(root, path string) (string, error) {
	if root == "" {
		return "", fmt.Errorf("sanitize: empty root directory")
	}

	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return "", fmt.Errorf("sanitize: invalid root %q: %w", root, err)
	}

	var target string
	if filepath.IsAbs(path) {
		target = filepath.Clean(path)
	} else {
		target = filepath.Clean(filepath.Join(rootAbs, path))
	}

	// Ensure target is within root
	if target != rootAbs && !strings.HasPrefix(target, rootAbs+string(os.PathSeparator)) {
		return "", fmt.Errorf("sanitize: path %q escapes root %q", path, root)
	}

	return target, nil
}

// SafeFilename removes directory separators and null bytes from a filename,
// returning a clean filename safe for use in any directory.
func SafeFilename(name string) string {
	// Remove null bytes
	name = strings.ReplaceAll(name, "\x00", "")
	// Get only the base filename
	name = filepath.Base(name)
	// Remove path separators that might survive
	name = strings.ReplaceAll(name, "/", "_")
	name = strings.ReplaceAll(name, "\\", "_")
	// Remove leading dots to prevent hidden files
	name = strings.TrimLeft(name, ".")

	if name == "" || name == "." || name == ".." {
		return "unnamed"
	}
	return name
}

// SanitizeCommand checks a command string for injection patterns.
// Returns an error if suspicious patterns are found.
func SanitizeCommand(cmd string) error {
	dangerous := []string{
		"$(", "`",
		"&&", "||",
		";",
		"|",
		">", ">>",
		"<", "<<",
		"\\n", "\\r",
	}

	for _, pat := range dangerous {
		if strings.Contains(cmd, pat) {
			return fmt.Errorf("sanitize: dangerous pattern %q in command", pat)
		}
	}

	return nil
}

// SanitizeArgs checks each argument for shell injection patterns.
func SanitizeArgs(args []string) error {
	for _, arg := range args {
		if err := SanitizeCommand(arg); err != nil {
			return err
		}
	}
	return nil
}
