package flog

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Logger appends one JSON object per line to a daily-rotated file under Dir.
type Logger struct {
	Dir  string
	mu   sync.Mutex
	day  string
	file *os.File
}

func New(dir string) (*Logger, error) {
	if dir == "" {
		dir = "./data/logs"
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	l := &Logger{Dir: dir}
	if err := l.rotate(time.Now().UTC()); err != nil {
		return nil, err
	}
	return l, nil
}

func (l *Logger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.file == nil {
		return nil
	}
	err := l.file.Close()
	l.file = nil
	return err
}

func (l *Logger) rotate(now time.Time) error {
	day := now.Format("20060102")
	if l.file != nil && l.day == day {
		return nil
	}
	if l.file != nil {
		_ = l.file.Close()
		l.file = nil
	}
	path := filepath.Join(l.Dir, fmt.Sprintf("bot-%s.jsonl", day))
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	l.file = f
	l.day = day
	return nil
}

// Event writes a structured analysis event. kind is required; fields are merged in.
func (l *Logger) Event(kind string, fields map[string]any) {
	if l == nil {
		return
	}
	now := time.Now().UTC()
	row := map[string]any{
		"ts":   now.Format(time.RFC3339Nano),
		"kind": kind,
	}
	for k, v := range fields {
		if k == "ts" || k == "kind" {
			continue
		}
		row[k] = v
	}
	b, err := json.Marshal(row)
	if err != nil {
		return
	}
	b = append(b, '\n')

	l.mu.Lock()
	defer l.mu.Unlock()
	if err := l.rotate(now); err != nil {
		return
	}
	_, _ = l.file.Write(b)
}

// Nop returns a logger that discards events (tests).
func Nop() *Logger { return nil }
