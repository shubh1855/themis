package gitx_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/syn3rgy2026/UntrainedModels_Syn3rgy_SatyamUttamPandey/internal/gitx"
)

func setupGitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	run := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=Test",
			"GIT_AUTHOR_EMAIL=test@test.com",
			"GIT_COMMITTER_NAME=Test",
			"GIT_COMMITTER_EMAIL=test@test.com",
		)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %s", args, out)
		}
	}

	run("init")
	run("config", "user.email", "test@test.com")
	run("config", "user.name", "Test")

	os.WriteFile(filepath.Join(dir, "README.md"), []byte("# Test"), 0644)
	run("add", ".")
	run("commit", "-m", "initial")

	return dir
}

func TestGitStatus(t *testing.T) {
	dir := setupGitRepo(t)
	g := gitx.New(dir)

	status, err := g.Status(context.Background())
	if err != nil {
		t.Fatalf("status failed: %v", err)
	}

	if status != "" {
		t.Logf("status output: %q", status)
	}

	os.WriteFile(filepath.Join(dir, "new.txt"), []byte("new"), 0644)

	status, err = g.Status(context.Background())
	if err != nil {
		t.Fatalf("status failed: %v", err)
	}

	if status == "" {
		t.Error("expected dirty status with untracked file")
	}
}

func TestGitLog(t *testing.T) {
	dir := setupGitRepo(t)
	g := gitx.New(dir)

	log, err := g.Log(context.Background(), 5)
	if err != nil {
		t.Fatalf("log failed: %v", err)
	}

	if log == "" {
		t.Error("expected non-empty log")
	}
}

func TestGitBranch(t *testing.T) {
	dir := setupGitRepo(t)
	g := gitx.New(dir)

	branches, err := g.Branch(context.Background())
	if err != nil {
		t.Fatalf("branch failed: %v", err)
	}

	if branches == "" {
		t.Error("expected at least one branch")
	}
}
