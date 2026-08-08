package tracker_test

import (
	"testing"

	"github.com/artbeard/mcp-tools/internal/tracker"
)

// Очередь: IT ок, it - нет.
func TestValidQueue(t *testing.T) {
	if !tracker.ValidQueue("IT") {
		t.Fatal("IT")
	}
	if tracker.ValidQueue("it") {
		t.Fatal("lowercase")
	}
}

// Assignee: без префикса всегда login; uid → число.
func TestParseUserSelector(t *testing.T) {
	got, err := tracker.ParseUserSelector("alice")
	if err != nil {
		t.Fatal(err)
	}
	if got.(map[string]any)["login"] != "alice" {
		t.Fatalf("%#v", got)
	}
	// в отличие от summonee: голые цифры тоже login-объект не строим как id
	got, err = tracker.ParseUserSelector("42")
	if err != nil {
		t.Fatal(err)
	}
	if got.(map[string]any)["login"] != "42" {
		t.Fatalf("plain digits should be login: %#v", got)
	}
}

// Теги: запятые, дубли режем.
func TestParseTags(t *testing.T) {
	got := tracker.ParseTags([]string{"a,b", "b", "c"})
	if len(got) != 3 || got[0] != "a" || got[1] != "b" || got[2] != "c" {
		t.Fatalf("%#v", got)
	}
}

// --field: JSON value или строка.
func TestParseExtraField(t *testing.T) {
	k, v, err := tracker.ParseExtraField(`parent={"key":"IT-1"}`)
	if err != nil || k != "parent" {
		t.Fatalf("%v %v %v", k, v, err)
	}
	m, ok := v.(map[string]any)
	if !ok || m["key"] != "IT-1" {
		t.Fatalf("%#v", v)
	}
	k, v, err = tracker.ParseExtraField("foo=bar")
	if err != nil || k != "foo" || v != "bar" {
		t.Fatalf("%v %v %v", k, v, err)
	}
}

// Payload: queue/summary/description + project/sp из field mapping.
func TestBuildCreateIssuePayload(t *testing.T) {
	sp := 3.5
	payload, err := tracker.BuildCreateIssuePayload(tracker.CreateIssueFields{
		Queue:       "it",
		Summary:     "  title  ",
		Description: "body",
		Project:     "CRM",
		Tags:        []string{"x"},
		SP:          &sp,
		Assignee:    "login:bob",
		Env:         map[string]string{},
	})
	if err != nil {
		t.Fatal(err)
	}
	if payload["queue"] != "IT" || payload["summary"] != "title" {
		t.Fatalf("%#v", payload)
	}
	if payload["project"] != "CRM" || payload["storyPoints"] != 3.5 {
		t.Fatalf("%#v", payload)
	}
	if payload["assignee"].(map[string]any)["login"] != "bob" {
		t.Fatalf("%#v", payload["assignee"])
	}
}
