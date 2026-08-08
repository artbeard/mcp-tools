package tracker_test

import (
	"testing"

	"github.com/artbeard/mcp-tools/internal/tracker"
)

// Проверяем, что --target находится по алиасам (регистр/синонимы),
// а неизвестный статус не матчится.
func TestMatchTransition(t *testing.T) {
	transitions := []map[string]any{
		{
			"id":      "open",
			"display": "Открыть",
			"to":      map[string]any{"display": "Открыто", "key": "open"},
		},
		{
			"id":      "inProgress",
			"display": "Взять в работу",
			"to":      map[string]any{"display": "В работе", "key": "inProgress"},
		},
	}
	match := tracker.MatchTransition(transitions, "открыто")
	if match == nil || match["id"] != "open" {
		t.Fatalf("expected open, got %#v", match)
	}
	match = tracker.MatchTransition(transitions, "in progress")
	if match == nil || match["id"] != "inProgress" {
		t.Fatalf("expected inProgress, got %#v", match)
	}
	if tracker.MatchTransition(transitions, "готово") != nil {
		t.Fatal("expected no match")
	}
}
