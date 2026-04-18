package tools

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
)

type FS struct {
	Root string
}

func NewFS(root string) *FS {
	return &FS{Root: root}
}

func (f *FS) SafePath(path string) (string, error) {
	rootAbs, err := filepath.Abs(f.Root)
	if err != nil {
		return "", err
	}

	target := path
	if !filepath.IsAbs(path) {
		target = filepath.Join(rootAbs, path)
	}

	targetAbs, err := filepath.Abs(target)
	if err != nil {
		return "", err
	}

	if targetAbs != rootAbs &&
		!strings.HasPrefix(targetAbs, rootAbs+string(os.PathSeparator)) {
		return "", os.ErrPermission
	}

	return targetAbs, nil
}

func (f *FS) ReadFile(path string) (string, error) {
	p, err := f.SafePath(path)
	if err != nil {
		return "", err
	}

	b, err := os.ReadFile(p)
	if err != nil {
		return "", err
	}

	return string(b), nil
}

func (f *FS) WriteFile(path, content string) error {
	p, err := f.SafePath(path)
	if err != nil {
		return err
	}

	parent := filepath.Dir(p)
	if err := os.MkdirAll(parent, 0755); err != nil {
		return err
	}

	return os.WriteFile(p, []byte(content), 0644)
}

func (f *FS) AppendFile(path, content string) error {
	p, err := f.SafePath(path)
	if err != nil {
		return err
	}

	parent := filepath.Dir(p)
	if err := os.MkdirAll(parent, 0755); err != nil {
		return err
	}

	file, err := os.OpenFile(p, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = file.WriteString(content)
	return err
}

func (f *FS) CreateFile(path, content string) error {
	p, err := f.SafePath(path)
	if err != nil {
		return err
	}

	if _, err := os.Stat(p); err == nil {
		return errors.New("file already exists")
	}

	return f.WriteFile(path, content)
}

func (f *FS) Mkdir(path string) error {
	p, err := f.SafePath(path)
	if err != nil {
		return err
	}

	return os.MkdirAll(p, 0755)
}

func (f *FS) EditFile(path, oldString, newString string) error {
	content, err := f.ReadFile(path)
	if err != nil {
		return err
	}
	if !strings.Contains(content, oldString) {
		return errors.New("old_string not found in " + path)
	}
	return f.WriteFile(path, strings.Replace(content, oldString, newString, 1))
}

func (f *FS) Exists(path string) bool {
	p, err := f.SafePath(path)
	if err != nil {
		return false
	}

	_, err = os.Stat(p)
	return err == nil
}
