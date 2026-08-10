package codestyle

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var uriRe = regexp.MustCompile(`^codestyle://[A-Za-z0-9_\-./]+$`)

// ReadRuleOutput - ответ read-rule.
type ReadRuleOutput struct {
	URI     string `json:"uri"`
	File    string `json:"file"`
	Content string `json:"content"`
}

// ReadRule читает .md по URI codestyle://category/file.
func (c ContentDir) ReadRule(uri string) (*ReadRuleOutput, error) {
	uri = strings.TrimSpace(uri)
	if !uriRe.MatchString(uri) {
		return nil, fmt.Errorf("URI должен быть вида codestyle://<category>/<file>")
	}
	relPath := strings.TrimPrefix(uri, "codestyle://")
	rulesDir := c.rulesDir()

	// codestyle://INDEX → корневой INDEX.md (как в документации / resources)
	candidates := []string{filepath.Join(rulesDir, relPath+".md")}
	if relPath == "INDEX" || relPath == "general/INDEX" {
		candidates = append(candidates, filepath.Join(rulesDir, "INDEX.md"))
	}

	var cleanFull string
	cleanRules, err := filepath.Abs(rulesDir)
	if err != nil {
		return nil, err
	}
	var pickedRel string
	for _, fullPath := range candidates {
		abs, err := filepath.Abs(fullPath)
		if err != nil {
			continue
		}
		rel, err := filepath.Rel(cleanRules, abs)
		if err != nil || strings.HasPrefix(rel, "..") {
			continue
		}
		if _, err := os.Stat(abs); err == nil {
			cleanFull = abs
			pickedRel = rel
			break
		}
	}
	if cleanFull == "" {
		meta, merr := c.LoadMeta()
		examples := []string{"codestyle://INDEX", "codestyle://backend/dto"}
		if merr == nil && len(meta.Entries) > 0 {
			examples = examples[:0]
			for i, e := range meta.Entries {
				if i >= 10 {
					break
				}
				examples = append(examples, EntryURI(e))
			}
		}
		return nil, fmt.Errorf("правило %q не найдено; сначала find-rules. Примеры: %s", uri, strings.Join(examples, ", "))
	}

	raw, err := os.ReadFile(cleanFull)
	if err != nil {
		return nil, fmt.Errorf("чтение %s: %w", cleanFull, err)
	}

	parts := strings.Split(filepath.ToSlash(pickedRel), "/")
	fileLabel := filepath.ToSlash(pickedRel)
	if len(parts) >= 2 {
		fileLabel = parts[len(parts)-2] + "/" + parts[len(parts)-1]
	}

	return &ReadRuleOutput{
		URI:     uri,
		File:    fileLabel,
		Content: string(raw),
	}, nil
}

// ListEntries - все URI из meta.
func ListEntries(meta *Meta) []FindRulesMatch {
	out := make([]FindRulesMatch, 0, len(meta.Entries))
	for _, e := range meta.Entries {
		out = append(out, FindRulesMatch{
			URI:      EntryURI(e),
			Category: e.Category,
			File:     e.File,
			Priority: e.Priority,
			Summary:  e.Summary,
		})
	}
	return out
}
