package security

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

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

	if target != rootAbs && !strings.HasPrefix(target, rootAbs+string(os.PathSeparator)) {
		return "", fmt.Errorf("sanitize: path %q escapes root %q", path, root)
	}

	return target, nil
}

func SafeFilename(name string) string {
	name = strings.ReplaceAll(name, "\x00", "")
	name = filepath.Base(name)
	name = strings.ReplaceAll(name, "/", "_")
	name = strings.ReplaceAll(name, "\\", "_")
	name = strings.TrimLeft(name, ".")

	if name == "" || name == "." || name == ".." {
		return "unnamed"
	}
	return name
}

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

func SanitizeArgs(args []string) error {
	for _, arg := range args {
		if err := SanitizeCommand(arg); err != nil {
			return err
		}
	}
	return nil
}
