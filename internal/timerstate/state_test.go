package timerstate_test

import (
	"path/filepath"
	"testing"

	"github.com/artbeard/mcp-tools/internal/timerstate"
)

// Save/Load roundtrip без active.
func TestSaveLoad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tracker-time.json")
	st := &timerstate.State{
		Sessions: []timerstate.Session{{
			IssueKey:        "IT-1",
			StartedAt:       "2026-01-01T10:00:00Z",
			StoppedAt:       "2026-01-01T10:05:00Z",
			DurationSeconds: 300,
			Comment:         "x",
		}},
	}
	if err := timerstate.Save(path, st); err != nil {
		t.Fatal(err)
	}
	got, err := timerstate.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.Active != nil || len(got.Sessions) != 1 || got.Sessions[0].IssueKey != "IT-1" {
		t.Fatalf("%#v", got)
	}
}

// Нет файла - пустой state, не ошибка.
func TestLoadMissing(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing.json")
	got, err := timerstate.Load(path)
	if err != nil || got.Active != nil || len(got.Sessions) != 0 {
		t.Fatalf("%v %#v", err, got)
	}
}
