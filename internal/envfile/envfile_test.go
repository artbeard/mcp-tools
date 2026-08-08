package envfile_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/artbeard/mcp-tools/internal/envfile"
)

func TestLoad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tracker.env")
	content := "# comment\nTRACKER_OAUTH_TOKEN=abc\nTRACKER_ORG_ID=\"123\"\n\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	values, err := envfile.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if values["TRACKER_OAUTH_TOKEN"] != "abc" {
		t.Fatalf("token=%q", values["TRACKER_OAUTH_TOKEN"])
	}
	if values["TRACKER_ORG_ID"] != "123" {
		t.Fatalf("org=%q", values["TRACKER_ORG_ID"])
	}
}

// .env обязателен; .env.local опционален и перекрывает ключи.
func TestLoadDir(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte("A=1\nB=2\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	values, err := envfile.LoadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if values["A"] != "1" || values["B"] != "2" {
		t.Fatalf("without local: %#v", values)
	}

	if err := os.WriteFile(filepath.Join(dir, ".env.local"), []byte("B=local\nC=3\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	values, err = envfile.LoadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if values["A"] != "1" || values["B"] != "local" || values["C"] != "3" {
		t.Fatalf("with local: %#v", values)
	}
}

func TestLoadDirRequiresEnv(t *testing.T) {
	dir := t.TempDir()
	if _, err := envfile.LoadDir(dir); err == nil {
		t.Fatal("expected error without .env")
	}
}
