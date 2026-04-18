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

func (f *FS) RunFile(path, args string) (string, error) {
	p, err := f.SafePath(path)
	if err != nil {
		return "", err
	}

	ext := strings.ToLower(filepath.Ext(p))
	base := filepath.Base(p)
	dir := filepath.Dir(p)

	argList := []string{}
	if strings.TrimSpace(args) != "" {
		argList = strings.Fields(args)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var cmd *exec.Cmd

	switch ext {
	case ".py":
		cmd = exec.CommandContext(ctx, "python3", append([]string{p}, argList...)...)

	case ".js":
		cmd = exec.CommandContext(ctx, "node", append([]string{p}, argList...)...)

	case ".ts":
		cmd = exec.CommandContext(ctx, "npx", append([]string{"ts-node", p}, argList...)...)

	case ".sh":
		cmd = exec.CommandContext(ctx, "bash", append([]string{p}, argList...)...)

	case ".rb":
		cmd = exec.CommandContext(ctx, "ruby", append([]string{p}, argList...)...)

	case ".go":
		cmd = exec.CommandContext(ctx, "go", append([]string{"run", p}, argList...)...)

	case ".c":
		bin, compileOut, cerr := compile(ctx, "gcc", p, dir)
		if cerr != nil {
			return compileOut, cerr
		}
		defer os.Remove(bin)
		cmd = exec.CommandContext(ctx, bin, argList...)

	case ".cpp":
		bin, compileOut, cerr := compile(ctx, "g++", p, dir)
		if cerr != nil {
			return compileOut, cerr
		}
		defer os.Remove(bin)
		cmd = exec.CommandContext(ctx, bin, argList...)

	case ".java":
		compileCmd := exec.CommandContext(ctx, "javac", p)
		compileCmd.Dir = dir
		out, cerr := compileCmd.CombinedOutput()
		if cerr != nil {
			return string(out), fmt.Errorf("javac: %w", cerr)
		}
		className := strings.TrimSuffix(base, ".java")
		cmd = exec.CommandContext(ctx, "java", append([]string{className}, argList...)...)

	case ".json":
		if base != "package.json" {
			return "", fmt.Errorf("only package.json can be run, got: %s", base)
		}
		cmd = exec.CommandContext(ctx, "npm", "start")

	default:
		return "", fmt.Errorf("unsupported file type: %s", ext)
	}

	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	output := strings.TrimSpace(string(out))
	if err != nil {
		if output != "" {
			return output, fmt.Errorf("exit error: %w", err)
		}
		return "", err
	}
	return output, nil
}

func compile(ctx context.Context, compiler, src, dir string) (bin, output string, err error) {
	bin = filepath.Join(dir, fmt.Sprintf(".themis_run_%d", time.Now().UnixNano()))
	out, err := exec.CommandContext(ctx, compiler, "-o", bin, src).CombinedOutput()
	if err != nil {
		return "", string(out), fmt.Errorf("%s: %w", compiler, err)
	}
	return bin, "", nil
}
