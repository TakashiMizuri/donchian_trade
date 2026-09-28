package flog

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEventWritesJSONL(t *testing.T) {
	dir := t.TempDir()
	l, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	l.Event("boot", map[string]any{"tag": "test", "n": 1})
	l.Event("bar", map[string]any{"symbol": "BTC", "skip": "ok"})

	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 1 {
		t.Fatalf("dir %+v err=%v", entries, err)
	}
	b, err := os.ReadFile(filepath.Join(dir, entries[0].Name()))
	if err != nil {
		t.Fatal(err)
	}
	if len(b) < 20 || b[len(b)-1] != '\n' {
		t.Fatalf("bad content %q", b)
	}
}
