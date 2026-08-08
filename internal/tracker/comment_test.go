package tracker_test

import (
	"testing"

	"github.com/artbeard/mcp-tools/internal/tracker"
)

// Разбор --summonee: префиксы login/id/cloud/uid, голые цифры→id, иначе login;
// неизвестный префикс - ошибка.
func TestParseSummonee(t *testing.T) {
	cases := []struct {
		in   string
		key  string
		want any
	}{
		{"login:alice", "login", "alice"},
		{"id:abc", "id", "abc"},
		{"cloud:xyz", "cloudUid", "xyz"},
		{"42", "id", "42"},
		{"bob", "login", "bob"},
	}
	for _, tc := range cases {
		got, err := tracker.ParseSummonee(tc.in)
		if err != nil {
			t.Fatalf("%q: %v", tc.in, err)
		}
		m, ok := got.(map[string]any)
		if !ok || m[tc.key] != tc.want {
			t.Fatalf("%q: got %#v", tc.in, got)
		}
	}
	uid, err := tracker.ParseSummonee("uid:7")
	if err != nil {
		t.Fatal(err)
	}
	if uid.(map[string]any)["uid"] != 7 {
		t.Fatalf("uid: %#v", uid)
	}
	if _, err := tracker.ParseSummonee("foo:bar"); err == nil {
		t.Fatal("expected unsupported selector error")
	}
}

// Ключ задачи: QUEUE-123 ок, нижний регистр - нет.
func TestValidIssueKey(t *testing.T) {
	if !tracker.ValidIssueKey("IT-123") {
		t.Fatal("IT-123 should be valid")
	}
	if tracker.ValidIssueKey("it-123") {
		t.Fatal("lowercase invalid")
	}
}
