// Package codestyle - поиск и чтение правил из skills/codestyle/content.
package codestyle

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Priority - critical | normal.
type Priority string

const (
	PriorityCritical Priority = "critical" // обязательно прочитать перед кодом
	PriorityNormal   Priority = "normal"   // по релевантности задачи
)

// Entry - одна запись mcp-meta.json.
type Entry struct {
	File     string   `json:"file"`     // имя .md (например dto.md)
	Category string   `json:"category"` // general|backend|frontend|patterns
	Tags     []string `json:"tags"`     // ключевые слова для scoring
	Tasks    []string `json:"tasks"`    // типичные формулировки задач
	Priority Priority `json:"priority"` // critical|normal
	Summary  string   `json:"summary"`  // краткое описание правила
}

// Meta - корневой mcp-meta.json.
type Meta struct {
	Version string  `json:"version"` // версия индекса
	Updated string  `json:"updated"` // дата обновления индекса
	Entries []Entry `json:"entries"` // все правила для find-rules
}

// ContentDir - корень content (рядом .codestyle/, mcp-meta.json).
type ContentDir string

// DefaultContentDir - <repo>/skills/codestyle/content.
func DefaultContentDir(repoRoot string) ContentDir {
	return ContentDir(filepath.Join(repoRoot, "skills", "codestyle", "content"))
}

func (c ContentDir) metaPath() string {
	return filepath.Join(string(c), "mcp-meta.json")
}

func (c ContentDir) rulesDir() string {
	return filepath.Join(string(c), ".codestyle")
}

// LoadMeta читает mcp-meta.json.
func (c ContentDir) LoadMeta() (*Meta, error) {
	raw, err := os.ReadFile(c.metaPath())
	if err != nil {
		return nil, fmt.Errorf("mcp-meta.json: %w", err)
	}
	var meta Meta
	if err := json.Unmarshal(raw, &meta); err != nil {
		return nil, fmt.Errorf("mcp-meta.json: %w", err)
	}
	if len(meta.Entries) == 0 {
		return nil, fmt.Errorf("mcp-meta.json: пустой entries")
	}
	return &meta, nil
}

// EntryURI - codestyle://<category>/<file-without-.md>.
func EntryURI(e Entry) string {
	base := strings.TrimSuffix(e.File, ".md")
	base = strings.TrimSuffix(base, ".MD")
	return "codestyle://" + e.Category + "/" + base
}
