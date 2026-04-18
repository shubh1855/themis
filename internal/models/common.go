package models

import "fmt"

// FileEntry represents a single item in a directory listing.
type FileEntry struct {
	Name  string `json:"name"`
	Path  string `json:"path"`
	IsDir bool   `json:"is_dir"`
	Size  int64  `json:"size"`
}

// ProcessResult represents the output of a command execution.
type ProcessResult struct {
	ExitCode int    `json:"exit_code"`
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr"`
	PID      int    `json:"pid,omitempty"`
}

// GitResult represents the output of a git operation.
type GitResult struct {
	Output string `json:"output"`
}

// DBResult represents the output of a database query.
type DBResult struct {
	Columns []string                 `json:"columns,omitempty"`
	Rows    []map[string]interface{} `json:"rows,omitempty"`
	Affected int64                   `json:"affected,omitempty"`
}

// TestResult represents a test/lint/benchmark run outcome.
type TestResult struct {
	Passed  bool   `json:"passed"`
	Output  string `json:"output"`
	Summary string `json:"summary,omitempty"`
}

// ArgString extracts a string argument by key from the tool args map.
func ArgString(args map[string]interface{}, key string) string {
	v, ok := args[key]
	if !ok {
		return ""
	}
	s, ok := v.(string)
	if !ok {
		return fmt.Sprintf("%v", v)
	}
	return s
}

// ArgInt extracts an integer argument by key from the tool args map.
func ArgInt(args map[string]interface{}, key string, def int) int {
	v, ok := args[key]
	if !ok {
		return def
	}
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	default:
		return def
	}
}

// ArgBool extracts a boolean argument by key from the tool args map.
func ArgBool(args map[string]interface{}, key string) bool {
	v, ok := args[key]
	if !ok {
		return false
	}
	b, ok := v.(bool)
	if !ok {
		return false
	}
	return b
}
