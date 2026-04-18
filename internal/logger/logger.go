// Package logger provides a structured, leveled logger for the agent runtime.
package logger

import (
	"fmt"
	"io"
	"os"
	"sync"
	"time"
)

// Level represents a log severity level.
type Level int

const (
	// LevelDebug is the most verbose level.
	LevelDebug Level = iota
	// LevelInfo is the default level for operational messages.
	LevelInfo
	// LevelWarn indicates a potential issue.
	LevelWarn
	// LevelError indicates a failure.
	LevelError
)

var levelNames = map[Level]string{
	LevelDebug: "DEBUG",
	LevelInfo:  "INFO",
	LevelWarn:  "WARN",
	LevelError: "ERROR",
}

// String returns the human-readable name for Level.
func (l Level) String() string {
	if n, ok := levelNames[l]; ok {
		return n
	}
	return "UNKNOWN"
}

// Logger is a concurrency-safe structured logger.
type Logger struct {
	mu     sync.Mutex
	level  Level
	out    io.Writer
	prefix string
}

// New creates a Logger that writes to the given writer at the specified level.
func New(out io.Writer, level Level) *Logger {
	return &Logger{out: out, level: level}
}

// Default returns a logger writing to stderr at info level.
func Default() *Logger {
	return New(os.Stderr, LevelInfo)
}

// WithPrefix returns a copy of the logger with the given prefix prepended.
func (l *Logger) WithPrefix(prefix string) *Logger {
	return &Logger{out: l.out, level: l.level, prefix: prefix}
}

// SetLevel changes the minimum log level.
func (l *Logger) SetLevel(level Level) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.level = level
}

func (l *Logger) log(level Level, msg string, args ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if level < l.level {
		return
	}

	ts := time.Now().Format("2006-01-02T15:04:05.000Z07:00")
	prefix := ""
	if l.prefix != "" {
		prefix = "[" + l.prefix + "] "
	}
	formatted := fmt.Sprintf(msg, args...)
	line := fmt.Sprintf("%s %s %s%s\n", ts, level.String(), prefix, formatted)
	_, _ = io.WriteString(l.out, line)
}

// Debug logs at debug level.
func (l *Logger) Debug(msg string, args ...any) { l.log(LevelDebug, msg, args...) }

// Info logs at info level.
func (l *Logger) Info(msg string, args ...any) { l.log(LevelInfo, msg, args...) }

// Warn logs at warn level.
func (l *Logger) Warn(msg string, args ...any) { l.log(LevelWarn, msg, args...) }

// Error logs at error level.
func (l *Logger) Error(msg string, args ...any) { l.log(LevelError, msg, args...) }
