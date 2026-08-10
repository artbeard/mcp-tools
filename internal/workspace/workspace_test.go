package workspace_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/artbeard/mcp-tools/internal/workspace"
)

func writeManifest(t *testing.T, dir, body string) string {
	t.Helper()
	path := filepath.Join(dir, workspace.ManifestName)
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// Walk-up находит .workspace.yaml; AsPrepare даёт abs path.
func TestDiscoverAsPrepare(t *testing.T) {
	root := t.TempDir()
	nested := filepath.Join(root, "a", "b")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	_ = writeManifest(t, root, `
schema_version: 1
name: demo
type: monorepo
aliases: [d]
repositories:
  - id: app
    path: app
    remote: origin
    branches:
      base: master
git:
  protected_base_branches: [master, main]
`)
	_ = os.MkdirAll(filepath.Join(root, "app"), 0o755)

	m, err := workspace.Discover(nested, "")
	if err != nil {
		t.Fatal(err)
	}
	if m.Name != "demo" || m.Type != "monorepo" {
		t.Fatalf("%#v", m)
	}
	if err := workspace.AssertProjectName(m, "d"); err != nil {
		t.Fatal(err)
	}
	if err := workspace.AssertProjectName(m, "nope"); err == nil {
		t.Fatal("expected mismatch")
	}
	p := workspace.AsPrepare(m)
	if p.Key != "demo" || len(p.Repositories) != 1 {
		t.Fatalf("%#v", p)
	}
	if !filepath.IsAbs(p.Repositories[0].Path) {
		t.Fatalf("path not abs: %s", p.Repositories[0].Path)
	}
}

// base не master/main - ошибка.
func TestBadBaseBranch(t *testing.T) {
	root := t.TempDir()
	writeManifest(t, root, `
schema_version: 1
name: bad
type: monorepo
repositories:
  - id: app
    path: .
    branches:
      base: develop
`)
	if _, err := workspace.Load(root); err == nil {
		t.Fatal("expected error")
	}
}
