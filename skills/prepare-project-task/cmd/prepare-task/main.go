// CLI: подготовка локальных task-веток по .workspace.yaml (бывший prepare_task_branch.py).
// Режимы: --list / --check TASK / --prepare TASK. git push запрещён.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/artbeard/mcp-tools/internal/gitcmd"
	"github.com/artbeard/mcp-tools/internal/workspace"
)

func main() {
	list := flag.Bool("list", false, "показать манифест")
	check := flag.String("check", "", "только preflight для TASK_ID")
	prepare := flag.String("prepare", "", "создать ветки для TASK_ID")
	project := flag.String("project", "", "имя/alias проекта из манифеста")
	cwd := flag.String("cwd", "", "старт discovery (по умолчанию getwd)")
	manifest := flag.String("manifest", "", "явный путь к .workspace.yaml")
	flag.Parse()

	modes := 0
	if *list {
		modes++
	}
	if *check != "" {
		modes++
	}
	if *prepare != "" {
		modes++
	}
	if modes != 1 {
		fmt.Fprintf(os.Stderr, "использование: prepare-task (--list | --check TASK | --prepare TASK) [--project NAME] [--cwd DIR] [--manifest PATH]\n")
		os.Exit(2)
	}

	code := run(*list, *check, *prepare, *project, *cwd, *manifest)
	os.Exit(code)
}

func run(list bool, checkTask, prepareTask, project, cwd, manifestPath string) int {
	m, err := workspace.Discover(cwd, manifestPath)
	if err != nil {
		return emit(map[string]any{"status": "failed", "errors": []string{err.Error()}, "push_allowed": false}, 2)
	}
	if err := workspace.AssertProjectName(m, project); err != nil {
		return emit(map[string]any{"status": "failed", "errors": []string{err.Error()}, "push_allowed": false}, 2)
	}
	if list {
		payload := workspace.ListPayload(m)
		payload["push_allowed"] = false
		return emit(payload, 0)
	}

	proj := workspace.AsPrepare(m)
	taskID := checkTask
	action := "check"
	if prepareTask != "" {
		taskID = prepareTask
		action = "prepare"
	}

	bases := make([]string, 0, len(proj.Repositories))
	for _, r := range proj.Repositories {
		bases = append(bases, r.BaseBranch)
	}
	taskErrors := gitcmd.ValidateTaskID(taskID, m.ProtectedBaseBranches, bases)
	if len(taskErrors) > 0 {
		return emit(map[string]any{
			"status":       "failed",
			"action":       action,
			"project":      proj.Key,
			"task_id":      taskID,
			"errors":       taskErrors,
			"push_allowed": false,
		}, 2)
	}

	checks := preflight(proj, taskID)
	checkErrors := flattenRepoErrors(checks)
	if len(checkErrors) > 0 || checkTask != "" {
		status := "ok"
		exit := 0
		if len(checkErrors) > 0 {
			status = "failed"
			exit = 1
		}
		return emit(map[string]any{
			"status":       status,
			"action":       "check",
			"project":      proj.Key,
			"project_type": proj.Type,
			"task_id":      taskID,
			"repositories": checks,
			"errors":       checkErrors,
			"push_allowed": false,
		}, exit)
	}

	fetches, fetchErrors := fetchAndVerify(proj)
	if len(fetchErrors) > 0 {
		return emit(map[string]any{
			"status":                "failed",
			"action":                "prepare",
			"phase":                 "fetch_and_verify",
			"project":               proj.Key,
			"project_type":          proj.Type,
			"task_id":               taskID,
			"repositories":          fetches,
			"errors":                fetchErrors,
			"working_trees_changed": false,
			"push_allowed":          false,
		}, 1)
	}

	prepared, prepareErrors := prepareRepos(proj, taskID)
	status := "ok"
	exit := 0
	if len(prepareErrors) > 0 {
		status = "failed"
		exit = 3
	}
	return emit(map[string]any{
		"status":         status,
		"action":         "prepare",
		"phase":          "create_branches",
		"project":        proj.Key,
		"project_type":   proj.Type,
		"task_id":        taskID,
		"repositories":   prepared,
		"errors":         prepareErrors,
		"push_performed": false,
		"push_allowed":   false,
	}, exit)
}

func emit(payload map[string]any, exitCode int) int {
	enc := json.NewEncoder(os.Stdout)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	_ = enc.Encode(payload)
	return exitCode
}

func flattenRepoErrors(checks []map[string]any) []string {
	var out []string
	for _, item := range checks {
		label, _ := item["label"].(string)
		errs, _ := item["errors"].([]string)
		for _, e := range errs {
			out = append(out, label+": "+e)
		}
	}
	if out == nil {
		out = []string{}
	}
	return out
}

// preflight - все репы, без мутаций.
func preflight(proj workspace.PrepareProject, taskID string) []map[string]any {
	out := make([]map[string]any, 0, len(proj.Repositories))
	for _, item := range proj.Repositories {
		out = append(out, preflightRepo(item, taskID))
	}
	return out
}

func preflightRepo(item workspace.PrepareRepo, taskID string) map[string]any {
	repository, _ := filepath.Abs(item.Path)
	result := map[string]any{
		"label":       item.Label,
		"path":        repository,
		"base_branch": item.BaseBranch,
		"remote":      item.Remote,
		"ready":       false,
		"errors":      []string{},
	}
	var errors []string

	inside, err := gitcmd.Git(repository, 30*time.Second, "rev-parse", "--is-inside-work-tree")
	if err != nil {
		errors = append(errors, err.Error())
		result["errors"] = errors
		return result
	}
	if !inside.Ok() || strings.TrimSpace(inside.Stdout) != "true" {
		errors = append(errors, "это не git-репозиторий")
		result["errors"] = errors
		return result
	}

	branch, _ := gitcmd.Git(repository, 30*time.Second, "branch", "--show-current")
	current := ""
	if branch.Ok() {
		current = strings.TrimSpace(branch.Stdout)
	}
	if current == "" {
		result["current_branch"] = nil
		errors = append(errors, "отсоединённый HEAD (detached) - небезопасно")
	} else {
		result["current_branch"] = current
	}

	status, _ := gitcmd.Git(repository, 30*time.Second, "status", "--porcelain=v1", "--untracked-files=all")
	if !status.Ok() {
		errors = append(errors, status.ErrMsg())
	} else if strings.TrimSpace(status.Stdout) != "" {
		result["changes"] = strings.Split(strings.TrimSuffix(status.Stdout, "\n"), "\n")
		errors = append(errors, "в репозитории есть незакоммиченные или неотслеживаемые изменения")
	}

	markers := gitcmd.OperationMarkers(repository)
	if len(markers) > 0 {
		result["in_progress"] = markers
		errors = append(errors, "идёт незавершённая git-операция (merge/rebase/...)")
	}

	remote, _ := gitcmd.Git(repository, 30*time.Second, "remote", "get-url", item.Remote)
	if !remote.Ok() {
		errors = append(errors, "нет remote: "+item.Remote)
	}

	baseRef := "refs/heads/" + item.BaseBranch
	if !gitcmd.ShowRefExists(repository, baseRef) {
		errors = append(errors, "локальной базовой ветки нет: "+item.BaseBranch)
	}
	taskRef := "refs/heads/" + taskID
	if gitcmd.ShowRefExists(repository, taskRef) {
		errors = append(errors, "локальная task-ветка уже есть: "+taskID)
	}

	if remote.Ok() {
		heads, err := gitcmd.RemoteHeads(repository, item.Remote, baseRef, "refs/heads/"+taskID)
		if err != nil {
			errors = append(errors, fmt.Sprintf("не удалось посмотреть remote %s: %v", item.Remote, err))
		} else {
			if _, ok := heads[baseRef]; !ok {
				errors = append(errors, fmt.Sprintf("на remote нет базовой ветки: %s/%s", item.Remote, item.BaseBranch))
			}
			if _, ok := heads["refs/heads/"+taskID]; ok {
				errors = append(errors, fmt.Sprintf("на remote уже есть task-ветка: %s/%s", item.Remote, taskID))
			}
		}
	}

	result["errors"] = errors
	result["ready"] = len(errors) == 0
	return result
}

func fetchAndVerify(proj workspace.PrepareProject) ([]map[string]any, []string) {
	results := make([]map[string]any, 0, len(proj.Repositories))
	var errors []string
	for _, item := range proj.Repositories {
		repository, _ := filepath.Abs(item.Path)
		remoteRef := fmt.Sprintf("refs/remotes/%s/%s", item.Remote, item.BaseBranch)
		refspec := fmt.Sprintf("refs/heads/%s:%s", item.BaseBranch, remoteRef)
		fetched, err := gitcmd.Git(repository, 120*time.Second, "fetch", "--no-tags", item.Remote, refspec)
		entry := map[string]any{"label": item.Label, "path": repository, "fetched": false}
		if err != nil {
			entry["error"] = err.Error()
			errors = append(errors, item.Label+": не удалось сделать fetch: "+err.Error())
			results = append(results, entry)
			continue
		}
		entry["fetched"] = fetched.Ok()
		if !fetched.Ok() {
			msg := fetched.ErrMsg()
			entry["error"] = msg
			errors = append(errors, item.Label+": не удалось сделать fetch: "+msg)
		}
		results = append(results, entry)
	}
	if len(errors) > 0 {
		return results, errors
	}

	for i, item := range proj.Repositories {
		repository, _ := filepath.Abs(item.Path)
		remoteRef := fmt.Sprintf("refs/remotes/%s/%s", item.Remote, item.BaseBranch)
		localRef := "refs/heads/" + item.BaseBranch
		ancestor, err := gitcmd.Git(repository, 60*time.Second, "merge-base", "--is-ancestor", localRef, remoteRef)
		if err != nil || !ancestor.Ok() {
			msg := fmt.Sprintf("локальную %s нельзя fast-forward до %s/%s", item.BaseBranch, item.Remote, item.BaseBranch)
			results[i]["fast_forward_ready"] = false
			results[i]["error"] = msg
			errors = append(errors, item.Label+": "+msg)
		} else {
			results[i]["fast_forward_ready"] = true
		}
	}
	return results, errors
}

func prepareRepos(proj workspace.PrepareProject, taskID string) ([]map[string]any, []string) {
	results := make([]map[string]any, 0, len(proj.Repositories))
	var errors []string
	for _, item := range proj.Repositories {
		repository, _ := filepath.Abs(item.Path)
		entry := map[string]any{"label": item.Label, "path": repository, "branch": nil}
		base := item.BaseBranch
		remoteRef := fmt.Sprintf("refs/remotes/%s/%s", item.Remote, base)

		checkedOut, err := gitcmd.Git(repository, 60*time.Second, "switch", base)
		if err != nil || !checkedOut.Ok() {
			msg := checkedOut.ErrMsg()
			if err != nil {
				msg = err.Error()
			}
			entry["error"] = msg
			errors = append(errors, item.Label+": не удалось переключиться на ветку: "+msg)
			results = append(results, entry)
			break
		}

		updated, err := gitcmd.Git(repository, 60*time.Second, "merge", "--ff-only", remoteRef)
		if err != nil || !updated.Ok() {
			msg := updated.ErrMsg()
			if err != nil {
				msg = err.Error()
			}
			entry["error"] = msg
			errors = append(errors, item.Label+": fast-forward не удался: "+msg)
			results = append(results, entry)
			break
		}

		created, err := gitcmd.Git(repository, 60*time.Second, "switch", "-c", taskID, base)
		if err != nil || !created.Ok() {
			msg := created.ErrMsg()
			if err != nil {
				msg = err.Error()
			}
			entry["error"] = msg
			errors = append(errors, item.Label+": не удалось создать ветку: "+msg)
			results = append(results, entry)
			break
		}

		entry["branch"] = taskID
		entry["base_branch"] = base
		entry["published"] = false
		results = append(results, entry)
	}
	return results, errors
}
