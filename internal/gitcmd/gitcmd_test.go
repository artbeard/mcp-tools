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

// PushTaskBranch не пускает master/protected; Git всё ещё банит push.
func TestPushTaskBranchGuards(t *testing.T) {
	_, err := gitcmd.PushTaskBranch("/tmp", "origin", "master", nil)
	if err == nil {
		t.Fatal("expected SafetyError for master")
	}
	if _, ok := err.(*gitcmd.SafetyError); !ok {
		t.Fatalf("got %T %v", err, err)
	}
	_, err = gitcmd.PushTaskBranch("/tmp", "origin", "IT-1", []string{"IT-1"})
	if err == nil {
		t.Fatal("expected SafetyError for protected")
	}
	_, err = gitcmd.Git("/tmp", 0, "push", "origin", "HEAD")
	if err == nil {
		t.Fatal("Git must still forbid push")
	}
}
