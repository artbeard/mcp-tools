package gitcmd_test

import (
	"testing"

	"github.com/artbeard/mcp-tools/internal/gitcmd"
)

// push запрещён на уровне обёртки.
func TestGitForbidsPush(t *testing.T) {
	_, err := gitcmd.Git("/tmp", 0, "push", "origin", "HEAD")
	if err == nil {
		t.Fatal("expected SafetyError")
	}
	if _, ok := err.(*gitcmd.SafetyError); !ok {
		t.Fatalf("got %T %v", err, err)
	}
}

// Запрещённые имена task branch.
func TestValidateTaskID(t *testing.T) {
	errs := gitcmd.ValidateTaskID("master", nil, nil)
	if len(errs) == 0 {
		t.Fatal("master should be forbidden")
	}
	errs = gitcmd.ValidateTaskID("IT-123", []string{"master", "main"}, []string{"master"})
	if len(errs) != 0 {
		t.Fatalf("%v", errs)
	}
}
