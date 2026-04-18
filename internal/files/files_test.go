package files_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/syn3rgy2026/UntrainedModels_Syn3rgy_SatyamUttamPandey/internal/files"
)

func testManager(t *testing.T) *files.Manager {
	t.Helper()
	return files.NewManager(t.TempDir())
}

func TestWriteAndReadFile(t *testing.T) {
	m := testManager(t)
	if err := m.WriteFile("test.txt", "hello world"); err != nil {
		t.Fatalf("write failed: %v", err)
	}

	content, err := m.ReadFile("test.txt")
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}
	if content != "hello world" {
		t.Errorf("expected 'hello world', got %q", content)
	}
}

func TestEditFile_ExactMatch(t *testing.T) {
	m := testManager(t)
	original := "line1\nline2\nline3\n"
	if err := m.WriteFile("test.txt", original); err != nil {
		t.Fatal(err)
	}

	if err := m.EditFile("test.txt", "line2", "REPLACED"); err != nil {
		t.Fatalf("edit failed: %v", err)
	}

	content, _ := m.ReadFile("test.txt")
	if content != "line1\nREPLACED\nline3\n" {
		t.Errorf("unexpected content: %q", content)
	}
}

func TestEditFile_NotFound(t *testing.T) {
	m := testManager(t)
	if err := m.WriteFile("test.txt", "hello"); err != nil {
		t.Fatal(err)
	}

	err := m.EditFile("test.txt", "nonexistent", "replacement")
	if err == nil {
		t.Error("expected error when old_string not found")
	}
}

func TestCreateFile_AlreadyExists(t *testing.T) {
	m := testManager(t)
	if err := m.WriteFile("exists.txt", "content"); err != nil {
		t.Fatal(err)
	}

	err := m.CreateFile("exists.txt", "new content")
	if err == nil {
		t.Error("expected error when file already exists")
	}
}

func TestPathTraversal(t *testing.T) {
	m := testManager(t)

	_, err := m.ReadFile("../../../etc/passwd")
	if err == nil {
		t.Error("expected path traversal to be blocked")
	}
}

func TestListDir(t *testing.T) {
	m := testManager(t)
	m.WriteFile("a.txt", "a")
	m.WriteFile("b.txt", "b")
	m.Mkdir("subdir")

	entries, err := m.ListDir(".")
	if err != nil {
		t.Fatalf("list failed: %v", err)
	}
	if len(entries) < 3 {
		t.Errorf("expected at least 3 entries, got %d", len(entries))
	}
}

func TestTree(t *testing.T) {
	m := testManager(t)
	m.WriteFile("a.txt", "a")
	m.Mkdir("sub")
	m.WriteFile(filepath.Join("sub", "b.txt"), "b")

	tree, err := m.Tree(".", 3)
	if err != nil {
		t.Fatalf("tree failed: %v", err)
	}
	if tree == "" {
		t.Error("expected non-empty tree output")
	}
}

func TestMoveFile(t *testing.T) {
	m := testManager(t)
	m.WriteFile("src.txt", "content")

	if err := m.MoveFile("src.txt", "dst.txt"); err != nil {
		t.Fatalf("move failed: %v", err)
	}

	if m.Exists("src.txt") {
		t.Error("source should not exist after move")
	}
	content, _ := m.ReadFile("dst.txt")
	if content != "content" {
		t.Error("moved file content mismatch")
	}
}

func TestCopyFile(t *testing.T) {
	m := testManager(t)
	m.WriteFile("src.txt", "content")

	if err := m.CopyFile("src.txt", "copy.txt"); err != nil {
		t.Fatalf("copy failed: %v", err)
	}

	if !m.Exists("src.txt") {
		t.Error("source should still exist after copy")
	}
	content, _ := m.ReadFile("copy.txt")
	if content != "content" {
		t.Error("copied file content mismatch")
	}
}

func TestDeleteFile(t *testing.T) {
	m := testManager(t)
	m.WriteFile("delete_me.txt", "content")

	if err := m.DeleteFile("delete_me.txt"); err != nil {
		t.Fatalf("delete failed: %v", err)
	}
	if m.Exists("delete_me.txt") {
		t.Error("file should not exist after delete")
	}
}

func TestAtomicWrite(t *testing.T) {
	m := testManager(t)

	// Write should be atomic (no .tmp file left behind)
	if err := m.WriteFile("atomic.txt", "content"); err != nil {
		t.Fatal(err)
	}

	// Check no temp file exists
	root := m.Root
	entries, _ := os.ReadDir(root)
	for _, e := range entries {
		if filepath.Ext(e.Name()) == ".tmp" {
			t.Errorf("temp file left behind: %s", e.Name())
		}
	}
}
