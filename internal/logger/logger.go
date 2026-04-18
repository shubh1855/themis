package logger

import (
	"fmt"
	"io"
	"os"
	"sync"
	"time"
)

type Level int

const (
	LevelDebug Level = iota
	LevelInfo
	LevelWarn
	LevelError
)

var levelNames = map[Level]string{
	LevelDebug: "DEBUG",
	LevelInfo:  "INFO",
	LevelWarn:  "WARN",
	LevelError: "ERROR",
}

func (l Level) String() string {
	if n, ok := levelNames[l]; ok {
		return n
	}
	return "UNKNOWN"
}

type Logger struct {
	mu     sync.Mutex
	level  Level
	out    io.Writer
	prefix string
}

func New(out io.Writer, level Level) *Logger {
	return &Logger{out: out, level: level}
}

func Default() *Logger {
	return New(os.Stderr, LevelInfo)
}

func (l *Logger) WithPrefix(prefix string) *Logger {
	return &Logger{out: l.out, level: l.level, prefix: prefix}
}

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

func (l *Logger) Debug(msg string, args ...any) { l.log(LevelDebug, msg, args...) }

func (l *Logger) Info(msg string, args ...any) { l.log(LevelInfo, msg, args...) }

func (l *Logger) Warn(msg string, args ...any) { l.log(LevelWarn, msg, args...) }

func (l *Logger) Error(msg string, args ...any) { l.log(LevelError, msg, args...) }
