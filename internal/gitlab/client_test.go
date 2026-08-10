package gitlab_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
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

// ParseRemoteURL для git@ / https.
func TestParseRemoteURL(t *testing.T) {
	r, err := gitlab.ParseRemoteURL("git@gitlab.example.com:group/proj.git")
	if err != nil {
		t.Fatal(err)
	}
	if r.Host != "gitlab.example.com" || r.ProjectPath != "group/proj" {
		t.Fatalf("%#v", r)
	}
	r, err = gitlab.ParseRemoteURL("https://gitlab.example.com/group/proj.git")
	if err != nil {
		t.Fatal(err)
	}
	if r.ProjectPath != "group/proj" {
		t.Fatalf("%#v", r)
	}
	if _, err := gitlab.ParseRemoteURL("not-a-remote"); err == nil {
		t.Fatal("expected error")
	}
}

// ValidateMRRecord проверяет path.
func TestValidateMRRecord(t *testing.T) {
	sum, err := gitlab.ValidateMRRecord(map[string]any{
		"iid":     float64(7),
		"web_url": "https://gitlab.example.com/group/proj/-/merge_requests/7",
		"title":   "t",
	}, "group/proj")
	if err != nil || sum.IID != 7 {
		t.Fatalf("%#v %v", sum, err)
	}
	if _, err := gitlab.ValidateMRRecord(map[string]any{
		"iid":     7,
		"web_url": "https://gitlab.example.com/other/-/merge_requests/7",
	}, "group/proj"); err == nil {
		t.Fatal("expected path mismatch")
	}
}

// Пагинация ListMRNotes / Commits / Discussions.
func TestListMRPaginated(t *testing.T) {
	var notePages, commitPages, discPages int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("PRIVATE-TOKEN") != "tok" {
			t.Fatalf("token: %q", r.Header.Get("PRIVATE-TOKEN"))
		}
		page := r.URL.Query().Get("page")
		switch {
		case strings.HasSuffix(r.URL.Path, "/notes"):
			notePages++
			if page == "1" || page == "" {
				w.Header().Set("X-Next-Page", "2")
				w.Header().Set("X-Total-Pages", "2")
				_ = json.NewEncoder(w).Encode([]map[string]any{{"id": 1, "body": "n1"}})
				return
			}
			_ = json.NewEncoder(w).Encode([]map[string]any{{"id": 2, "body": "n2"}})
		case strings.HasSuffix(r.URL.Path, "/commits"):
			commitPages++
			_ = json.NewEncoder(w).Encode([]map[string]any{{"id": "abc", "title": "c1"}})
		case strings.HasSuffix(r.URL.Path, "/discussions"):
			discPages++
			_ = json.NewEncoder(w).Encode([]map[string]any{{"id": "d1"}})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	c := &gitlab.Client{BaseURL: srv.URL, Token: "tok", HTTP: srv.Client()}
	notes, err := c.ListMRNotes("group/proj", 9)
	if err != nil {
		t.Fatal(err)
	}
	if len(notes) != 2 || notePages != 2 {
		t.Fatalf("notes=%d pages=%d", len(notes), notePages)
	}
	commits, err := c.ListMRCommits("group/proj", 9)
	if err != nil || len(commits) != 1 || commitPages != 1 {
		t.Fatalf("commits err=%v n=%d pages=%d", err, len(commits), commitPages)
	}
	discs, err := c.ListMRDiscussions("group/proj", 9)
	if err != nil || len(discs) != 1 || discPages != 1 {
		t.Fatalf("discussions err=%v n=%d pages=%d", err, len(discs), discPages)
	}
}

// FilterNotesByAuthors + ParseAuthorFilterCSV.
func TestFilterNotesByAuthors(t *testing.T) {
	notes := []map[string]any{
		{"id": 1, "author": map[string]any{"username": "alice", "id": float64(1)}},
		{"id": 2, "author": map[string]any{"username": "bob", "email": "bob@ex.com", "id": float64(2)}},
		{"id": 3, "author": map[string]any{"name": "Carol", "id": float64(3)}},
	}
	all := gitlab.FilterNotesByAuthors(notes, nil)
	if len(all) != 3 {
		t.Fatalf("nil filter: %d", len(all))
	}
	got := gitlab.FilterNotesByAuthors(notes, []string{"bob", "3"})
	if len(got) != 2 {
		t.Fatalf("filtered: %#v", got)
	}
	if gitlab.ParseAuthorFilterCSV(" a, b , ,c ") == nil {
		t.Fatal("csv")
	}
	parts := gitlab.ParseAuthorFilterCSV(" a, b , ,c ")
	if len(parts) != 3 || parts[0] != "a" || parts[2] != "c" {
		t.Fatalf("%#v", parts)
	}
	if gitlab.ParseAuthorFilterCSV("  ") != nil {
		t.Fatal("empty csv")
	}
}
