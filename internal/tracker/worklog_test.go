package tracker_test

import (
	"testing"

	"github.com/artbeard/mcp-tools/internal/tracker"
)

// Человекочитаемая длительность и ISO для worklog.
func TestDurationHelpers(t *testing.T) {
	if got := tracker.SecondsToHuman(3660); got != "1 ч 1 мин" {
		t.Fatalf("human 3660: %q", got)
	}
	if got := tracker.SecondsToHuman(7200); got != "2 ч" {
		t.Fatalf("human 7200: %q", got)
	}
	if got := tracker.SecondsToISODuration(90); got != "PT2M" {
		t.Fatalf("iso 90: %q", got)
	}
	if got := tracker.SecondsToISODuration(3660); got != "PT1H1M" {
		t.Fatalf("iso 3660: %q", got)
	}
}

// Округление: round-to 1 - минимум 60с; иначе шаг N минут.
func TestRoundSeconds(t *testing.T) {
	if got := tracker.RoundSeconds(10, 1); got != 60 {
		t.Fatalf("min: %d", got)
	}
	if got := tracker.RoundSeconds(200, 5); got != 300 {
		t.Fatalf("round 5: %d", got)
	}
	if got := tracker.RoundSeconds(100, 5); got != 300 {
		t.Fatalf("round up to step: %d", got)
	}
}
