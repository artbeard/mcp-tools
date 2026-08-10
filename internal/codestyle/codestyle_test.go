package codestyle

import (
	"os"
	"path/filepath"
	"testing"
)

// Тесты tokenize / FindRules / ReadRule: кириллица, ranking critical, валидация task,
// чтение реального content, защита от path traversal.

// Токены из ru/en строки: lower-case, без пунктуации, длина >= 2.
func TestTokenizeCyrillic(t *testing.T) {
	got := Tokenize("Новый Symfony-контроллер с DTO!")
	want := map[string]bool{"новый": true, "symfony": true, "контроллер": true, "dto": true}
	for _, tok := range got {
		if !want[tok] {
			t.Fatalf("unexpected token %q in %v", tok, got)
		}
	}
	if len(got) != 4 {
		t.Fatalf("got %v", got)
	}
}

// FindRules: в топе critical, в matches есть controller и dto по запросу про контроллер/DTO.
func TestFindRulesRanking(t *testing.T) {
	meta := &Meta{Entries: []Entry{
		{File: "controller.md", Category: "backend", Tags: []string{"controller", "route"}, Tasks: []string{"новый контроллер"}, Priority: PriorityCritical, Summary: "Symfony controller"},
		{File: "naming.md", Category: "general", Tags: []string{"naming"}, Tasks: []string{"именование"}, Priority: PriorityNormal, Summary: "naming rules"},
		{File: "dto.md", Category: "backend", Tags: []string{"dto"}, Tasks: []string{"DTO payload"}, Priority: PriorityCritical, Summary: "DTO class"},
	}}
	out, err := FindRules(meta, FindRulesInput{Task: "новый Symfony контроллер с DTO", MaxResults: 8})
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Matches) < 2 {
		t.Fatalf("matches=%v", out.Matches)
	}
	if out.Matches[0].Priority != PriorityCritical {
		t.Fatalf("first should be critical: %+v", out.Matches[0])
	}
	uris := map[string]bool{}
	for _, m := range out.Matches {
		uris[m.URI] = true
	}
	if !uris["codestyle://backend/controller"] || !uris["codestyle://backend/dto"] {
		t.Fatalf("expected controller+dto, got %+v", out.Matches)
	}
}

// task короче 3 символов - ошибка.
func TestFindRulesShortTask(t *testing.T) {
	_, err := FindRules(&Meta{Entries: []Entry{{File: "a.md", Category: "general"}}}, FindRulesInput{Task: "ab"})
	if err == nil {
		t.Fatal("expected error")
	}
}

// ReadRule по живому skills/codestyle/content: dto + INDEX + fallback general/INDEX.
func TestReadRuleRealContent(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	c := DefaultContentDir(root)
	if _, err := c.LoadMeta(); err != nil {
		t.Skip("content not synced:", err)
	}
	out, err := c.ReadRule("codestyle://backend/dto")
	if err != nil {
		t.Fatal(err)
	}
	if out.Content == "" || out.File == "" {
		t.Fatalf("%+v", out)
	}
	rootIdx, err := c.ReadRule("codestyle://INDEX")
	if err != nil {
		t.Fatal(err)
	}
	if rootIdx.Content == "" {
		t.Fatal("empty INDEX")
	}
	viaGeneral, err := c.ReadRule("codestyle://general/INDEX")
	if err != nil {
		t.Fatal(err)
	}
	if viaGeneral.Content == "" {
		t.Fatal("empty general/INDEX fallback")
	}
}

// URI с .. за пределы .codestyle отклоняется.
func TestReadRuleTraversal(t *testing.T) {
	dir := t.TempDir()
	c := ContentDir(dir)
	if err := os.MkdirAll(filepath.Join(dir, ".codestyle", "backend"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".codestyle", "backend", "dto.md"), []byte("ok"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := c.ReadRule("codestyle://../etc/passwd")
	if err == nil {
		t.Fatal("expected reject")
	}
}
