package gitlab_test

import (
	"testing"

	"github.com/artbeard/mcp-tools/internal/gitlab"
)

// Разбор web-URL MR.
func TestParseMRURL(t *testing.T) {
	ref, err := gitlab.ParseMRURL("https://gitlab.example.com/group/proj/-/merge_requests/42?tab=overview")
	if err != nil {
		t.Fatal(err)
	}
	if ref.BaseURL != "https://gitlab.example.com" || ref.ProjectPath != "group/proj" || ref.IID != 42 {
		t.Fatalf("%#v", ref)
	}
	if _, err := gitlab.ParseMRURL("not-a-url"); err == nil {
		t.Fatal("expected error")
	}
}

// Поиск строки в unified diff.
func TestFindDiffLine(t *testing.T) {
	changes := map[string]any{
		"changes": []any{
			map[string]any{
				"new_path": "a.go",
				"old_path": "a.go",
				"diff": "@@ -1,3 +1,4 @@\n line1\n-line2\n+line2changed\n+line3\n line4\n",
			},
		},
	}
	got, err := gitlab.FindDiffLine(changes, "a.go", 2, "new")
	if err != nil {
		t.Fatal(err)
	}
	if got.DiffMarker != "added" {
		t.Fatalf("%#v", got)
	}
	if _, err := gitlab.FindDiffLine(changes, "missing.go", 1, "new"); err == nil {
		t.Fatal("expected missing file")
	}
}
