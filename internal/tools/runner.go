package tools

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type ShellKind string

const (
	ShellBash ShellKind = "bash"
	ShellZsh  ShellKind = "zsh"
	ShellFish ShellKind = "fish"
	ShellSh   ShellKind = "sh"
)

// DetectShell reads $SHELL and returns the shell kind + its binary path.
func DetectShell() (ShellKind, string) {
	shellPath := os.Getenv("SHELL")
	if shellPath == "" {
		return ShellBash, "bash"
	}
	base := filepath.Base(shellPath)
	switch {
	case strings.Contains(base, "zsh"):
		return ShellZsh, shellPath
	case strings.Contains(base, "fish"):
		return ShellFish, shellPath
	case strings.Contains(base, "bash"):
		return ShellBash, shellPath
	default:
		return ShellSh, shellPath
	}
}

// BuildRunCmd builds an *exec.Cmd ready for interactive execution via tea.ExecProcess.
// For compiled languages (C, C++, Java) it compiles synchronously first.
// cleanup must be called after the process exits to remove temp binaries.
func (f *FS) BuildRunCmd(path, args string) (*exec.Cmd, func(), error) {
	p, err := f.SafePath(path)
	if err != nil {
		return nil, nil, err
	}

	ext := strings.ToLower(filepath.Ext(p))
	base := filepath.Base(p)
	dir := filepath.Dir(p)

	argList := []string{}
	if strings.TrimSpace(args) != "" {
		argList = strings.Fields(args)
	}

	_, shellBin := DetectShell()
	noop := func() {}

	var cmd *exec.Cmd

	switch ext {
	case ".py":
		cmd = exec.Command("python3", append([]string{p}, argList...)...)

	case ".js":
		cmd = exec.Command("node", append([]string{p}, argList...)...)

	case ".ts":
		cmd = exec.Command("npx", append([]string{"ts-node", p}, argList...)...)

	case ".sh", ".bash":
		cmd = exec.Command(shellBin, append([]string{p}, argList...)...)

	case ".zsh":
		cmd = exec.Command("zsh", append([]string{p}, argList...)...)

	case ".fish":
		cmd = exec.Command("fish", append([]string{p}, argList...)...)

	case ".rb":
		cmd = exec.Command("ruby", append([]string{p}, argList...)...)

	case ".go":
		cmd = exec.Command("go", append([]string{"run", p}, argList...)...)

	case ".c":
		bin, compileOut, cerr := compile(context.Background(), "gcc", p, dir)
		if cerr != nil {
			return nil, nil, fmt.Errorf("%s\n%w", compileOut, cerr)
		}
		cmd = exec.Command(bin, argList...)
		return finalize(cmd, dir, func() { os.Remove(bin) })

	case ".cpp":
		bin, compileOut, cerr := compile(context.Background(), "g++", p, dir)
		if cerr != nil {
			return nil, nil, fmt.Errorf("%s\n%w", compileOut, cerr)
		}
		cmd = exec.Command(bin, argList...)
		return finalize(cmd, dir, func() { os.Remove(bin) })

	case ".java":
		compileCmd := exec.Command("javac", p)
		compileCmd.Dir = dir
		out, cerr := compileCmd.CombinedOutput()
		if cerr != nil {
			return nil, nil, fmt.Errorf("javac: %s", strings.TrimSpace(string(out)))
		}
		className := strings.TrimSuffix(base, ".java")
		cmd = exec.Command("java", append([]string{className}, argList...)...)

	case ".json":
		if base != "package.json" {
			return nil, nil, fmt.Errorf("only package.json can be run, got: %s", base)
		}
		cmd = exec.Command("npm", "start")

	default:
		return nil, nil, fmt.Errorf("unsupported file type: %s", ext)
	}

	return finalize(cmd, dir, noop)
}

func finalize(cmd *exec.Cmd, dir string, cleanup func()) (*exec.Cmd, func(), error) {
	cmd.Dir = dir
	cmd.Env = os.Environ()
	return cmd, cleanup, nil
}

func compile(ctx context.Context, compiler, src, dir string) (bin, output string, err error) {
	bin = filepath.Join(dir, fmt.Sprintf(".themis_run_%d", time.Now().UnixNano()))
	out, err := exec.CommandContext(ctx, compiler, "-o", bin, src).CombinedOutput()
	if err != nil {
		return "", strings.TrimSpace(string(out)), fmt.Errorf("%s: %w", compiler, err)
	}
	return bin, "", nil
}
