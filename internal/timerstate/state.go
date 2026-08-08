// Package timerstate - локальный JSON-таймер (active + pending sessions).
package timerstate

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Active - запущенный таймер.
type Active struct {
	IssueKey  string `json:"issue_key"`
	StartedAt string `json:"started_at"`
	Comment   string `json:"comment"`
}

// Session - остановленный отрезок, ещё не списанный в worklog.
type Session struct {
	IssueKey         string `json:"issue_key"`
	StartedAt        string `json:"started_at"`
	StoppedAt        string `json:"stopped_at"`
	DurationSeconds  int    `json:"duration_seconds"`
	Comment          string `json:"comment"`
}

// State - содержимое state-файла.
type State struct {
	Active   *Active   `json:"active"`
	Sessions []Session `json:"sessions"`
}

// DefaultPath - ~/.local/state/mcp-tools/tracker-time.json
func DefaultPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".local", "state", "mcp-tools", "tracker-time.json"), nil
}

func legacyPaths() []string {
	home, err := os.UserHomeDir()
	if err != nil {
		return []string{"/tmp/.tracker-time.json"}
	}
	return []string{
		"/tmp/.tracker-time.json",
		filepath.Join(home, ".local", "state", "codex-writable", "tracker-time.json"),
	}
}

// Load читает state; если файла нет и path = default - пробует legacy.
func Load(path string) (*State, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, fmt.Errorf("state file: %w", err)
		}
		defaultPath, derr := DefaultPath()
		if derr == nil && path == defaultPath {
			for _, legacy := range legacyPaths() {
				raw, lerr := os.ReadFile(legacy)
				if lerr != nil {
					continue
				}
				var st State
				if err := json.Unmarshal(raw, &st); err != nil {
					return nil, fmt.Errorf("legacy state %s: %w", legacy, err)
				}
				if st.Sessions == nil {
					st.Sessions = []Session{}
				}
				return &st, nil
			}
		}
		return &State{Sessions: []Session{}}, nil
	}
	var st State
	if err := json.Unmarshal(data, &st); err != nil {
		return nil, fmt.Errorf("state file: %w", err)
	}
	if st.Sessions == nil {
		st.Sessions = []Session{}
	}
	return &st, nil
}

// Save пишет state (создаёт каталог).
func Save(path string, st *State) error {
	if st.Sessions == nil {
		st.Sessions = []Session{}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	raw = append(raw, '\n')
	return os.WriteFile(path, raw, 0o600)
}

// ParseTime - ISO из state (с Z или offset).
func ParseTime(value string) (time.Time, error) {
	if t, err := time.Parse(time.RFC3339Nano, value); err == nil {
		return t, nil
	}
	if t, err := time.Parse(time.RFC3339, value); err == nil {
		return t, nil
	}
	return time.Parse("2006-01-02T15:04:05.999999999Z07:00", value)
}

// FormatLocal - как Python format_local.
func FormatLocal(t time.Time) string {
	return t.Local().Format("2006-01-02 15:04:05 -0700")
}

// FormatWorklogStart - start для Tracker worklog API.
func FormatWorklogStart(t time.Time) string {
	return t.Local().Format("2006-01-02T15:04:05.000-0700")
}
