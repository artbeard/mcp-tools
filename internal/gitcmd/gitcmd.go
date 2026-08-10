// Package gitcmd - обёртка над git с запретом push (prepare-task и соседние CLI).
package gitcmd

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

var ForbiddenTaskBranches = map[string]struct{}{
	"master": {}, "main": {}, "dev": {},
}

// SafetyError - запрещённая операция (например push).
type SafetyError struct{ Msg string }

func (e *SafetyError) Error() string { return e.Msg }

// Result - stdout/stderr/code без panic.
type Result struct {
	Stdout     string
	Stderr     string
	ReturnCode int
}

func (r Result) Ok() bool { return r.ReturnCode == 0 }

func (r Result) ErrMsg() string {
	msg := strings.TrimSpace(r.Stderr)
	if msg == "" {
		msg = strings.TrimSpace(r.Stdout)
	}
	if msg == "" {
		msg = fmt.Sprintf("команда завершилась с кодом %d", r.ReturnCode)
	}
	return msg
}

// Run - произвольная команда с timeout.
func Run(cwd string, timeout time.Duration, command string, args ...string) Result {
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	cmd := exec.Command(command, args...)
	if cwd != "" {
		cmd.Dir = cwd
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Start()
	if err != nil {
		return Result{Stderr: err.Error(), ReturnCode: 1}
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case err := <-done:
		code := 0
		if err != nil {
			if ee, ok := err.(*exec.ExitError); ok {
				code = ee.ExitCode()
			} else {
				code = 1
			}
		}
		return Result{Stdout: stdout.String(), Stderr: stderr.String(), ReturnCode: code}
	case <-time.After(timeout):
		_ = cmd.Process.Kill()
		return Result{Stderr: fmt.Sprintf("таймаут после %s", timeout), ReturnCode: 1}
	}
}

// Git - git -C repo ...; arguments[0]==push → SafetyError.
func Git(repo string, timeout time.Duration, arguments ...string) (Result, error) {
	if len(arguments) > 0 && arguments[0] == "push" {
		return Result{}, &SafetyError{Msg: "git push запрещён в prepare-project-task"}
	}
	args := append([]string{"-C", repo}, arguments...)
	return Run("", timeout, "git", args...), nil
}

// MustGit - как Git, но SafetyError пробрасывается; для CLI удобнее Git.
func MustGit(repo string, timeout time.Duration, arguments ...string) Result {
	r, err := Git(repo, timeout, arguments...)
	if err != nil {
		return Result{Stderr: err.Error(), ReturnCode: 2}
	}
	return r
}

// ValidateTaskID - check-ref-format + запрет protected/base имён.
func ValidateTaskID(taskID string, protected []string, baseBranches []string) []string {
	var errors []string
	r := Run("", 30*time.Second, "git", "check-ref-format", "--branch", taskID)
	if !r.Ok() {
		errors = append(errors, fmt.Sprintf("некорректное имя ветки: %s", taskID))
	}
	if _, ok := ForbiddenTaskBranches[taskID]; ok {
		errors = append(errors, fmt.Sprintf("нельзя брать имя защищённой базовой ветки: %s", taskID))
		return errors
	}
	for _, p := range protected {
		if taskID == p {
			errors = append(errors, fmt.Sprintf("нельзя брать имя защищённой базовой ветки: %s", taskID))
			return errors
		}
	}
	for _, b := range baseBranches {
		if taskID == b {
			errors = append(errors, fmt.Sprintf("нельзя брать имя защищённой базовой ветки: %s", taskID))
			return errors
		}
	}
	return errors
}

// OperationMarkers - незавершённые git-операции в git-dir.
func OperationMarkers(repo string) []string {
	r, err := Git(repo, 30*time.Second, "rev-parse", "--absolute-git-dir")
	if err != nil {
		return []string{"UNKNOWN_GIT_DIR"}
	}
	if !r.Ok() {
		return []string{"UNKNOWN_GIT_DIR"}
	}
	gitDir := strings.TrimSpace(r.Stdout)
	markers := []string{"MERGE_HEAD", "CHERRY_PICK_HEAD", "REVERT_HEAD", "BISECT_LOG", "rebase-apply", "rebase-merge"}
	var found []string
	for _, m := range markers {
		if _, err := os.Stat(filepath.Join(gitDir, m)); err == nil {
			found = append(found, m)
		}
	}
	return found
}

// ShowRefExists - refs/heads/... локально.
func ShowRefExists(repo, ref string) bool {
	r, err := Git(repo, 30*time.Second, "show-ref", "--verify", "--quiet", ref)
	return err == nil && r.Ok()
}

// RemoteHeads - set of refs/heads/* с remote (ls-remote --heads).
func RemoteHeads(repo, remote string, refs ...string) (map[string]struct{}, error) {
	args := append([]string{"ls-remote", "--heads", remote}, refs...)
	r, err := Git(repo, 90*time.Second, args...)
	if err != nil {
		return nil, err
	}
	if !r.Ok() {
		return nil, fmt.Errorf("%s", r.ErrMsg())
	}
	out := map[string]struct{}{}
	for _, line := range strings.Split(r.Stdout, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || !strings.Contains(line, "\t") {
			continue
		}
		parts := strings.SplitN(line, "\t", 2)
		out[parts[1]] = struct{}{}
	}
	return out, nil
}
