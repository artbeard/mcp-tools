package codestyle

// find.go: поиск правил по описанию задачи.
// Токенизирует запрос, scoring по tags/tasks/summary из mcp-meta, возвращает
// URI + priority + summary (без тела .md - его читает read.go).

import (
	"fmt"
	"math"
	"regexp"
	"sort"
	"strings"
)

// FindRulesInput - вход find-rules.
type FindRulesInput struct {
	Task       string
	MaxResults int
}

// FindRulesMatch - один матч.
type FindRulesMatch struct {
	URI          string   `json:"uri"`           // codestyle://category/file
	Category     string   `json:"category"`      // general|backend|frontend|patterns
	File         string   `json:"file"`          // имя .md
	Priority     Priority `json:"priority"`      // critical|normal
	Summary      string   `json:"summary"`       // краткое описание
	Score        float64  `json:"score"`         // релевантность (keyword scoring)
	MatchedTerms []string `json:"matched_terms"` // совпавшие токены запроса
}

// FindRulesOutput - ответ find-rules (1:1 с TS).
type FindRulesOutput struct {
	Task    string           `json:"task"`    // исходный запрос
	Matches []FindRulesMatch `json:"matches"` // ранжированный список URI
	Hint    string           `json:"hint"`    // подсказка агенту (что читать дальше)
}

var nonAlnum = regexp.MustCompile(`[^\p{L}\p{N}]+`)

// Tokenize - lower-case, не-буквенно-цифровые → пробел, токены len>=2 (кириллица ок).
func Tokenize(text string) []string {
	lower := strings.ToLower(text)
	spaced := nonAlnum.ReplaceAllString(lower, " ")
	parts := strings.Fields(spaced)
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if len([]rune(p)) >= 2 {
			out = append(out, p)
		}
	}
	return out
}

func scoreEntry(entry Entry, queryTokens []string) (score float64, matched []string) {
	haystack := map[string]struct{}{}
	add := func(text string) {
		for _, tok := range Tokenize(text) {
			haystack[tok] = struct{}{}
		}
	}
	for _, tag := range entry.Tags {
		add(tag)
	}
	for _, task := range entry.Tasks {
		add(task)
	}
	add(entry.Summary)
	add(entry.File)

	matchedSet := map[string]struct{}{}
	for _, qt := range queryTokens {
		if _, ok := haystack[qt]; ok {
			if entry.Priority == PriorityCritical {
				score += 3
			} else {
				score += 2
			}
			matchedSet[qt] = struct{}{}
			continue
		}
		for ht := range haystack {
			if len([]rune(ht)) >= 4 && (strings.Contains(ht, qt) || strings.Contains(qt, ht)) {
				score += 1
				matchedSet[qt] = struct{}{}
				break
			}
		}
	}
	if entry.Priority == PriorityCritical {
		score += 0.5
	}
	matched = make([]string, 0, len(matchedSet))
	for t := range matchedSet {
		matched = append(matched, t)
	}
	sort.Strings(matched)
	return score, matched
}

func round1(v float64) float64 {
	return math.Round(v*10) / 10
}

// FindRules ранжирует правила по ключевым словам из mcp-meta (логика как в find-rules.ts).
func FindRules(meta *Meta, in FindRulesInput) (*FindRulesOutput, error) {
	task := strings.TrimSpace(in.Task)
	if len([]rune(task)) < 3 {
		return nil, fmt.Errorf("task должен быть не короче 3 символов")
	}
	max := in.MaxResults
	if max <= 0 {
		max = 8
	}
	if max > 15 {
		max = 15
	}

	queryTokens := Tokenize(task)
	type scored struct {
		entry   Entry
		score   float64
		matched []string
	}
	var rows []scored
	for _, e := range meta.Entries {
		s, m := scoreEntry(e, queryTokens)
		if s > 0 {
			rows = append(rows, scored{entry: e, score: s, matched: m})
		}
	}
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].score != rows[j].score {
			return rows[i].score > rows[j].score
		}
		if rows[i].entry.Priority != rows[j].entry.Priority {
			return rows[i].entry.Priority == PriorityCritical
		}
		return rows[i].entry.File < rows[j].entry.File
	})
	if len(rows) > max {
		rows = rows[:max]
	}

	matches := make([]FindRulesMatch, 0, len(rows))
	for _, r := range rows {
		matches = append(matches, FindRulesMatch{
			URI:          EntryURI(r.entry),
			Category:     r.entry.Category,
			File:         r.entry.File,
			Priority:     r.entry.Priority,
			Summary:      r.entry.Summary,
			Score:        round1(r.score),
			MatchedTerms: r.matched,
		})
	}

	hint := "Ничего не нашлось. Попробуй переформулировать задачу с конкретными терминами (controller, entity, store, migration, и т.д.) или начни с codestyle://INDEX."
	if len(matches) > 0 {
		hint = "Дальше: для каждого URI с priority=\"critical\" вызови tool codestyle_read_rule(uri), чтобы получить контент правила. Грузи только релевантное - экономь контекст."
	}

	return &FindRulesOutput{Task: task, Matches: matches, Hint: hint}, nil
}
