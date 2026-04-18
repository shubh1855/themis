package models

import "fmt"

type FileEntry struct {
	Name  string `json:"name"`
	Path  string `json:"path"`
	IsDir bool   `json:"is_dir"`
	Size  int64  `json:"size"`
}

type ProcessResult struct {
	ExitCode int    `json:"exit_code"`
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr"`
	PID      int    `json:"pid,omitempty"`
}

type GitResult struct {
	Output string `json:"output"`
}

type DBResult struct {
	Columns []string                 `json:"columns,omitempty"`
	Rows    []map[string]interface{} `json:"rows,omitempty"`
	Affected int64                   `json:"affected,omitempty"`
}

type TestResult struct {
	Passed  bool   `json:"passed"`
	Output  string `json:"output"`
	Summary string `json:"summary,omitempty"`
}

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
