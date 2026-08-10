// CLI: merge remote task → local/remote dev (бывший deploy_task_to_dev.py).
// Режимы: --list / --check TASK / --deploy TASK / --continue TASK.
// Push только через PushDevBranch (refs/heads/dev:refs/heads/dev).
//
// Exit codes:
//   0 - ok (list / check без конфликтов / deploy push ok / quirk local_merge_created)
//   1 - static preflight / fetch / plan hard-fail
//   2 - setup (манифест, task id)
//   3 - mid-deploy fail (merge / race / push)
//   4 - needs_resolution (ожидаемый resumable outcome)
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/artbeard/mcp-tools/internal/gitcmd"
	"github.com/artbeard/mcp-tools/internal/workspace"
)

var shaOnlyRe = regexp.MustCompile(`^[0-9a-f]{40,64}$`)

func main() {
	list := flag.Bool("list", false, "показать манифест")
	check := flag.String("check", "", "только plan (без merge/push)")
	deploy := flag.String("deploy", "", "merge + push origin/dev")
	cont := flag.String("continue", "", "докоммитить MERGE_HEAD и продолжить deploy")
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
	if *deploy != "" {
		modes++
	}
	if *cont != "" {
		modes++
	}
	if modes != 1 {
		fmt.Fprintf(os.Stderr, "использование: deploy-task (--list | --check TASK | --deploy TASK | --continue TASK) [--project NAME] [--cwd DIR] [--manifest PATH]\n")
		os.Exit(2)
	}

	os.Exit(run(*list, *check, *deploy, *cont, *project, *cwd, *manifest))
}

// run - оркестрация: preflight → (continue?) → fetch → plan → check|conflict|merges → race → push.
func run(list bool, checkTask, deployTask, continueTask, project, cwd, manifestPath string) int {
	m, err := workspace.Discover(cwd, manifestPath)
	if err != nil {
		return emit(map[string]any{"status": "failed", "errors": []string{err.Error()}, "push_target": "origin/dev"}, 2)
	}
	if err := workspace.AssertProjectName(m, project); err != nil {
		return emit(map[string]any{"status": "failed", "errors": []string{err.Error()}, "push_target": "origin/dev"}, 2)
	}
	if list {
		payload := workspace.ListPayload(m)
		payload["push_target"] = "origin/dev"
		return emit(payload, 0)
	}

	proj := workspace.AsDeploy(m)
	taskID := checkTask
	if deployTask != "" {
		taskID = deployTask
	}
	if continueTask != "" {
		taskID = continueTask
	}

	taskErrors := gitcmd.ValidateTaskID(taskID, m.ProtectedBranches, nil)
	if len(taskErrors) > 0 {
		return emit(map[string]any{
			"status": "failed", "errors": taskErrors, "push_target": "origin/dev",
		}, 2)
	}

	checks, checkErrors := staticPreflight(proj, continueTask != "")
	if len(checkErrors) > 0 {
		return emit(map[string]any{
			"status": "failed", "action": "check", "project": proj.Key, "task_id": taskID,
			"repositories": checks, "errors": checkErrors, "push_started": false,
		}, 1)
	}

	// --continue: докоммитить MERGE_HEAD или сразу fallthrough в полный пайплайн.
	if continueTask != "" {
		continued, code := continueActiveMerge(proj, taskID)
		if continued != nil {
			return emit(continued, code)
		}
	}

	fetches, fetchErrors := refreshRefs(proj, taskID)
	if len(fetchErrors) > 0 {
		return emit(map[string]any{
			"status": "failed", "action": "check", "phase": "fetch", "project": proj.Key, "task_id": taskID,
			"repositories": fetches, "errors": fetchErrors, "push_started": false,
		}, 1)
	}

	plan, planErrors := integrationPlan(proj, taskID, fetches)
	if len(planErrors) > 0 {
		return emit(map[string]any{
			"status": "failed", "action": "check", "phase": "plan", "project": proj.Key, "task_id": taskID,
			"repositories": plan, "errors": planErrors, "push_started": false,
		}, 1)
	}

	var conflicts []map[string]any
	for _, item := range plan {
		if needs, _ := item["needs_resolution"].(bool); needs {
			conflicts = append(conflicts, item)
		}
	}
	if conflicts == nil {
		conflicts = []map[string]any{}
	}

	if checkTask != "" {
		status := "ok"
		exit := 0
		if len(conflicts) > 0 {
			status = "needs_resolution"
			exit = 4
		}
		conflictSummaries := make([]map[string]any, 0, len(conflicts))
		for _, item := range conflicts {
			files, _ := item["conflict_files"].([]string)
			if files == nil {
				files = []string{}
			}
			conflictSummaries = append(conflictSummaries, map[string]any{
				"label": item["label"], "files": files,
			})
		}
		return emit(map[string]any{
			"status": status, "action": "check", "project": proj.Key, "project_type": proj.Type,
			"task_id": taskID, "repositories": plan, "conflicts": conflictSummaries, "push_started": false,
		}, exit)
	}

	// Predicted conflicts → begin_conflict только на первой conflicting репе.
	if len(conflicts) > 0 {
		label, _ := conflicts[0]["label"].(string)
		var configItem workspace.DeployRepo
		found := false
		for _, r := range proj.Repositories {
			if r.Label == label {
				configItem = r
				found = true
				break
			}
		}
		if !found {
			return emit(map[string]any{
				"status": "failed", "errors": []string{fmt.Sprintf("не найден репозиторий: %s", label)},
				"push_target": "origin/dev",
			}, 2)
		}
		payload, code := beginConflict(proj, taskID, configItem)
		return emit(payload, code)
	}

	localMerges, mergeErrors := createLocalMerges(proj, taskID, plan)
	if len(mergeErrors) > 0 {
		return emit(map[string]any{
			"status": "failed", "action": "deploy", "phase": "create_local_merges",
			"project": proj.Key, "task_id": taskID, "repositories": localMerges,
			"errors": mergeErrors, "push_started": false,
		}, 3)
	}

	raceErrors := verifyRemoteUnchanged(proj, plan)
	if len(raceErrors) > 0 {
		return emit(map[string]any{
			"status": "failed", "action": "deploy", "phase": "remote_race_check",
			"project": proj.Key, "task_id": taskID, "local_merges": localMerges,
			"errors": raceErrors, "push_started": false,
		}, 3)
	}

	pushes, summary, pushErrors := pushAllDev(proj, plan)
	status := "ok"
	exit := 0
	if len(pushErrors) > 0 {
		status = "failed"
		exit = 3
	}
	return emit(map[string]any{
		"status": status, "action": "deploy", "phase": "push_dev",
		"project": proj.Key, "project_type": proj.Type, "task_id": taskID,
		"local_merges": localMerges, "repositories": pushes, "push_summary": summary,
		"errors": pushErrors, "push_target": "origin/dev",
	}, exit)
}

func emit(payload map[string]any, code int) int {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	_ = enc.Encode(payload)
	return code
}

func gitDir(repository string) string {
	r, err := gitcmd.Git(repository, 30*time.Second, "rev-parse", "--absolute-git-dir")
	if err != nil || !r.Ok() {
		return ""
	}
	return strings.TrimSpace(r.Stdout)
}

func mergeActive(repository string) bool {
	dir := gitDir(repository)
	if dir == "" {
		return false
	}
	_, err := os.Stat(filepath.Join(dir, "MERGE_HEAD"))
	return err == nil
}

// otherOperations - незавершённые ops кроме MERGE_HEAD (его разрешаем на --continue).
func otherOperations(repository string) []string {
	dir := gitDir(repository)
	if dir == "" {
		return []string{"UNKNOWN_GIT_DIR"}
	}
	names := []string{"CHERRY_PICK_HEAD", "REVERT_HEAD", "BISECT_LOG", "rebase-apply", "rebase-merge"}
	var found []string
	for _, name := range names {
		if _, err := os.Stat(filepath.Join(dir, name)); err == nil {
			found = append(found, name)
		}
	}
	return found
}

// staticPreflight - clean / detached / ops; dirty OK только при allowMerge+MERGE_HEAD.
func staticPreflight(project workspace.DeployProject, allowMerge bool) ([]map[string]any, []string) {
	var results []map[string]any
	var errors []string
	activeCount := 0
	for _, item := range project.Repositories {
		repository, _ := filepath.Abs(item.Path)
		entry := map[string]any{"label": item.Label, "path": repository, "ready": false, "errors": []string{}}
		errs := []string{}

		inside, err := gitcmd.Git(repository, 30*time.Second, "rev-parse", "--is-inside-work-tree")
		if err != nil || !inside.Ok() {
			errs = append(errs, "это не Git work tree")
			entry["errors"] = errs
			results = append(results, entry)
			for _, e := range errs {
				errors = append(errors, fmt.Sprintf("%s: %s", item.Label, e))
			}
			continue
		}

		branchRes, _ := gitcmd.Git(repository, 30*time.Second, "branch", "--show-current")
		branch := strings.TrimSpace(branchRes.Stdout)
		if branch == "" {
			entry["current_branch"] = nil
			errs = append(errs, "detached HEAD небезопасен")
		} else {
			entry["current_branch"] = branch
		}

		active := mergeActive(repository)
		entry["merge_active"] = active
		if active {
			activeCount++
			if !allowMerge {
				errs = append(errs, "уже идёт merge; используй --continue")
			}
			if branch != item.DevBranch {
				errs = append(errs, "активный merge не на ветке dev")
			}
		}

		ops := otherOperations(repository)
		if len(ops) > 0 {
			entry["in_progress"] = ops
			errs = append(errs, "неподдерживаемая Git-операция уже идёт")
		}

		status, _ := gitcmd.Git(repository, 30*time.Second, "status", "--porcelain=v1", "--untracked-files=all")
		if !status.Ok() {
			errs = append(errs, status.ErrMsg())
		} else if strings.TrimSpace(status.Stdout) != "" && !(allowMerge && active) {
			entry["changes"] = strings.Split(strings.TrimSpace(status.Stdout), "\n")
			errs = append(errs, "репозиторий не чистый")
		}

		entry["errors"] = errs
		entry["ready"] = len(errs) == 0
		results = append(results, entry)
		for _, e := range errs {
			errors = append(errors, fmt.Sprintf("%s: %s", item.Label, e))
		}
	}
	if activeCount > 1 {
		errors = append(errors, "больше одной репы с активным merge")
	}
	if errors == nil {
		errors = []string{}
	}
	return results, errors
}

// refreshRefs - ls-remote base+dev+task → fetch трёх tracking refs.
func refreshRefs(project workspace.DeployProject, taskID string) ([]map[string]any, []string) {
	var results []map[string]any
	var errors []string
	for _, item := range project.Repositories {
		repository, _ := filepath.Abs(item.Path)
		remote := item.Remote
		dev, base := item.DevBranch, item.BaseBranch
		baseRef := "refs/heads/" + base
		devRef := "refs/heads/" + dev
		taskRef := "refs/heads/" + taskID

		entry := map[string]any{"label": item.Label, "path": repository, "fetched": false}
		ls, err := gitcmd.Git(repository, 120*time.Second, "ls-remote", "--heads", remote, baseRef, devRef, taskRef)
		if err != nil || !ls.Ok() {
			msg := "не удалось проверить remote"
			if err != nil {
				msg = err.Error()
			} else {
				msg = ls.ErrMsg()
			}
			entry["error"] = msg
			errors = append(errors, fmt.Sprintf("%s: не удалось проверить remote: %s", item.Label, msg))
			results = append(results, entry)
			continue
		}
		refs := map[string]string{}
		for _, line := range strings.Split(ls.Stdout, "\n") {
			line = strings.TrimSpace(line)
			if line == "" || !strings.Contains(line, "\t") {
				continue
			}
			parts := strings.SplitN(line, "\t", 2)
			refs[parts[1]] = parts[0]
		}
		if _, ok := refs[baseRef]; !ok {
			msg := fmt.Sprintf("remote base-ветки нет: %s", base)
			entry["error"] = msg
			errors = append(errors, fmt.Sprintf("%s: %s", item.Label, msg))
			results = append(results, entry)
			continue
		}
		if _, ok := refs[devRef]; !ok {
			msg := "remote ветки dev нет"
			entry["error"] = msg
			errors = append(errors, fmt.Sprintf("%s: %s", item.Label, msg))
			results = append(results, entry)
			continue
		}
		if _, ok := refs[taskRef]; !ok {
			msg := fmt.Sprintf("remote task-ветки нет: %s", taskID)
			entry["error"] = msg
			errors = append(errors, fmt.Sprintf("%s: %s", item.Label, msg))
			results = append(results, entry)
			continue
		}
		entry["observed_remote_dev_sha"] = refs[devRef]
		entry["remote_base_sha"] = refs[baseRef]
		entry["remote_task_sha"] = refs[taskRef]

		fetched, _ := gitcmd.Git(repository, 180*time.Second, "fetch", "--no-tags", remote,
			fmt.Sprintf("%s:refs/remotes/%s/%s", baseRef, remote, base),
			fmt.Sprintf("%s:refs/remotes/%s/%s", devRef, remote, dev),
			fmt.Sprintf("%s:refs/remotes/%s/%s", taskRef, remote, taskID),
		)
		if !fetched.Ok() {
			msg := fetched.ErrMsg()
			entry["error"] = msg
			errors = append(errors, fmt.Sprintf("%s: fetch не удался: %s", item.Label, msg))
		} else {
			entry["fetched"] = true
		}
		results = append(results, entry)
	}
	if errors == nil {
		errors = []string{}
	}
	return results, errors
}

// conflictFilesFromMergeTree - имена файлов из merge-tree --name-only --messages.
func conflictFilesFromMergeTree(output string) []string {
	seen := map[string]struct{}{}
	var files []string
	for _, line := range strings.Split(output, "\n") {
		value := strings.TrimSpace(line)
		if value == "" || shaOnlyRe.MatchString(value) || strings.HasPrefix(value, "CONFLICT ") || strings.HasPrefix(value, "Auto-merging ") {
			continue
		}
		if !strings.Contains(value, "\t") && !strings.HasPrefix(value, "100644 ") && !strings.HasPrefix(value, "100755 ") && !strings.HasPrefix(value, "160000 ") {
			if _, ok := seen[value]; !ok {
				seen[value] = struct{}{}
				files = append(files, value)
			}
		}
	}
	if files == nil {
		files = []string{}
	}
	sort.Strings(files)
	return files
}

// taskHistoryGuard - общая история с remote base + нет merge-коммитов вне base (контаминация).
func taskHistoryGuard(repository, baseRef, taskRef string) (map[string]any, []string) {
	result := map[string]any{"base_ref": baseRef, "task_ref": taskRef}
	var errors []string
	mergeBase, _ := gitcmd.Git(repository, 60*time.Second, "merge-base", baseRef, taskRef)
	if !mergeBase.Ok() || strings.TrimSpace(mergeBase.Stdout) == "" {
		errors = append(errors, "task-ветка не имеет общей истории с remote master/main base")
		return result, errors
	}
	result["branch_point"] = strings.TrimSpace(mergeBase.Stdout)
	merges, _ := gitcmd.Git(repository, 60*time.Second, "rev-list", "--merges", baseRef+".."+taskRef)
	if !merges.Ok() {
		errors = append(errors, fmt.Sprintf("не удалось проверить merge-историю task: %s", merges.ErrMsg()))
		return result, errors
	}
	var unexpected []string
	for _, line := range strings.Split(merges.Stdout, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			unexpected = append(unexpected, line)
		}
	}
	if unexpected == nil {
		unexpected = []string{}
	}
	result["unexpected_merge_commits"] = unexpected
	if len(unexpected) > 0 {
		errors = append(errors,
			"в task-ветке есть merge-коммиты вне remote master/main; возможно попадание коммитов из dev, пересобери ветку от текущего base")
	}
	return result, errors
}

// integrationPlan - already_deployed / prepared_local / merge-tree conflicts.
func integrationPlan(project workspace.DeployProject, taskID string, fetches []map[string]any) ([]map[string]any, []string) {
	var results []map[string]any
	var errors []string
	for i, item := range project.Repositories {
		fetched := fetches[i]
		repository, _ := filepath.Abs(item.Path)
		dev, base, remote := item.DevBranch, item.BaseBranch, item.Remote
		devRef := "refs/heads/" + dev
		remoteBase := fmt.Sprintf("refs/remotes/%s/%s", remote, base)
		remoteDev := fmt.Sprintf("refs/remotes/%s/%s", remote, dev)
		remoteTask := fmt.Sprintf("refs/remotes/%s/%s", remote, taskID)

		entry := map[string]any{"label": item.Label, "path": repository, "mergeable": false, "conflict_files": []string{}}
		history, historyErrors := taskHistoryGuard(repository, remoteBase, remoteTask)
		entry["history_guard"] = history
		if len(historyErrors) > 0 {
			entry["error"] = historyErrors[0]
			for _, msg := range historyErrors {
				errors = append(errors, fmt.Sprintf("%s: %s", item.Label, msg))
			}
			results = append(results, entry)
			continue
		}

		localDev, _ := gitcmd.Git(repository, 30*time.Second, "show-ref", "--verify", "--hash", devRef)
		if !localDev.Ok() {
			entry["error"] = "локальной ветки dev нет"
			errors = append(errors, fmt.Sprintf("%s: локальной ветки dev нет", item.Label))
			results = append(results, entry)
			continue
		}

		remoteContainsTask, _ := gitcmd.Git(repository, 60*time.Second, "merge-base", "--is-ancestor", remoteTask, remoteDev)
		if remoteContainsTask.Ok() {
			entry["mergeable"] = true
			entry["already_deployed"] = true
			entry["observed_remote_dev_sha"] = fetched["observed_remote_dev_sha"]
			results = append(results, entry)
			continue
		}

		localContainsTask, _ := gitcmd.Git(repository, 60*time.Second, "merge-base", "--is-ancestor", remoteTask, devRef)
		remoteIsAncestor, _ := gitcmd.Git(repository, 60*time.Second, "merge-base", "--is-ancestor", remoteDev, devRef)
		localIsAncestor, _ := gitcmd.Git(repository, 60*time.Second, "merge-base", "--is-ancestor", devRef, remoteDev)

		if localContainsTask.Ok() && remoteIsAncestor.Ok() {
			entry["mergeable"] = true
			entry["prepared_local"] = true
			entry["observed_remote_dev_sha"] = fetched["observed_remote_dev_sha"]
			results = append(results, entry)
			continue
		}
		if !localIsAncestor.Ok() {
			msg := "локальный dev имеет коммиты, которых нет в remote, и не содержит merge задачи"
			entry["error"] = msg
			errors = append(errors, fmt.Sprintf("%s: %s", item.Label, msg))
			results = append(results, entry)
			continue
		}

		mergeTree, _ := gitcmd.Git(repository, 120*time.Second, "merge-tree", "--write-tree", "--name-only", "--messages", remoteDev, remoteTask)
		entry["observed_remote_dev_sha"] = fetched["observed_remote_dev_sha"]
		if mergeTree.Ok() {
			entry["mergeable"] = true
		} else {
			entry["needs_resolution"] = true
			entry["conflict_files"] = conflictFilesFromMergeTree(mergeTree.Stdout)
			diag := mergeTree.Stdout + mergeTree.Stderr
			if len(diag) > 8000 {
				diag = diag[len(diag)-8000:]
			}
			entry["diagnostics"] = diag
		}
		results = append(results, entry)
	}
	if errors == nil {
		errors = []string{}
	}
	return results, errors
}

func resolutionPayload(project workspace.DeployProject, taskID string, item workspace.DeployRepo, diagnostics string) map[string]any {
	repository, _ := filepath.Abs(item.Path)
	conflictsRes, _ := gitcmd.Git(repository, 30*time.Second, "diff", "--name-only", "--diff-filter=U")
	unstagedRes, _ := gitcmd.Git(repository, 30*time.Second, "diff", "--name-only")
	conflicts := splitNonEmpty(conflictsRes.Stdout)
	unstaged := splitNonEmpty(unstagedRes.Stdout)
	if len(diagnostics) > 8000 {
		diagnostics = diagnostics[len(diagnostics)-8000:]
	}
	return map[string]any{
		"status": "needs_resolution", "action": "deploy",
		"project": project.Key, "project_type": project.Type, "task_id": taskID,
		"active_resolution": map[string]any{
			"label": item.Label, "path": repository, "branch": item.DevBranch,
			"conflict_files": conflicts, "unstaged_files": unstaged,
			"next_command": "--continue " + taskID,
		},
		"diagnostics": diagnostics, "push_started": false,
	}
}

func splitNonEmpty(s string) []string {
	var out []string
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			out = append(out, line)
		}
	}
	if out == nil {
		out = []string{}
	}
	return out
}

// beginConflict - открыть manual merge на первой conflicting репе (или quirk: clean → commit и стоп).
func beginConflict(project workspace.DeployProject, taskID string, item workspace.DeployRepo) (map[string]any, int) {
	repository, _ := filepath.Abs(item.Path)
	dev, remote := item.DevBranch, item.Remote
	checked, _ := gitcmd.Git(repository, 60*time.Second, "switch", dev)
	if !checked.Ok() {
		return map[string]any{
			"status": "failed", "phase": "begin_resolution",
			"errors": []string{fmt.Sprintf("%s: %s", item.Label, checked.ErrMsg())},
		}, 3
	}
	updated, _ := gitcmd.Git(repository, 60*time.Second, "merge", "--ff-only", fmt.Sprintf("refs/remotes/%s/%s", remote, dev))
	if !updated.Ok() {
		return map[string]any{
			"status": "failed", "phase": "begin_resolution",
			"errors": []string{fmt.Sprintf("%s: %s", item.Label, updated.ErrMsg())},
		}, 3
	}
	merged, _ := gitcmd.Git(repository, 120*time.Second, "merge", "--no-ff", "--no-commit", fmt.Sprintf("refs/remotes/%s/%s", remote, taskID))
	if !merged.Ok() && mergeActive(repository) {
		return resolutionPayload(project, taskID, item, merged.Stdout+merged.Stderr), 4
	}
	if !merged.Ok() {
		return map[string]any{
			"status": "failed", "phase": "begin_resolution",
			"errors": []string{fmt.Sprintf("%s: %s", item.Label, merged.ErrMsg())},
		}, 3
	}
	// Quirk: merge внезапно clean → commit и стоп без остальных реп/push.
	committed, _ := gitcmd.Git(repository, 60*time.Second, "commit", "-m", fmt.Sprintf("Merge branch '%s' into dev", taskID))
	if !committed.Ok() {
		return map[string]any{
			"status": "failed", "phase": "commit_merge",
			"errors": []string{fmt.Sprintf("%s: %s", item.Label, committed.ErrMsg())},
		}, 3
	}
	return map[string]any{"status": "local_merge_created"}, 0
}

// continueActiveMerge - commit после resolve; nil → fallthrough в полный deploy.
func continueActiveMerge(project workspace.DeployProject, taskID string) (map[string]any, int) {
	var active []workspace.DeployRepo
	for _, item := range project.Repositories {
		repository, _ := filepath.Abs(item.Path)
		if mergeActive(repository) {
			active = append(active, item)
		}
	}
	if len(active) == 0 {
		return nil, 0
	}
	if len(active) != 1 {
		return map[string]any{
			"status": "failed", "phase": "continue_resolution",
			"errors": []string{"больше одного активного merge"},
		}, 3
	}
	item := active[0]
	repository, _ := filepath.Abs(item.Path)
	conflicts := splitNonEmpty(mustStdout(repository, "diff", "--name-only", "--diff-filter=U"))
	unstaged := splitNonEmpty(mustStdout(repository, "diff", "--name-only"))
	if len(conflicts) > 0 || len(unstaged) > 0 {
		return resolutionPayload(project, taskID, item, ""), 4
	}
	committed, _ := gitcmd.Git(repository, 60*time.Second, "commit", "--no-edit")
	if !committed.Ok() {
		return map[string]any{
			"status": "failed", "phase": "continue_resolution",
			"errors": []string{fmt.Sprintf("%s: %s", item.Label, committed.ErrMsg())},
		}, 3
	}
	return nil, 0
}

func mustStdout(repository string, args ...string) string {
	r, _ := gitcmd.Git(repository, 30*time.Second, args...)
	return r.Stdout
}

// createLocalMerges - все remaining merges локально до любого push.
func createLocalMerges(project workspace.DeployProject, taskID string, plan []map[string]any) ([]map[string]any, []string) {
	var results []map[string]any
	var errors []string
	for i, item := range project.Repositories {
		entry := plan[i]
		repository, _ := filepath.Abs(item.Path)
		if already, _ := entry["already_deployed"].(bool); already {
			results = append(results, map[string]any{"label": item.Label, "already_deployed": true})
			continue
		}
		if prepared, _ := entry["prepared_local"].(bool); prepared {
			sha := strings.TrimSpace(mustStdout(repository, "rev-parse", "refs/heads/"+item.DevBranch))
			results = append(results, map[string]any{"label": item.Label, "local_merge_sha": sha, "created": false})
			continue
		}
		checked, _ := gitcmd.Git(repository, 60*time.Second, "switch", item.DevBranch)
		if !checked.Ok() {
			errors = append(errors, fmt.Sprintf("%s: %s", item.Label, checked.ErrMsg()))
			break
		}
		updated, _ := gitcmd.Git(repository, 60*time.Second, "merge", "--ff-only",
			fmt.Sprintf("refs/remotes/%s/%s", item.Remote, item.DevBranch))
		if !updated.Ok() {
			errors = append(errors, fmt.Sprintf("%s: %s", item.Label, updated.ErrMsg()))
			break
		}
		merged, _ := gitcmd.Git(repository, 120*time.Second, "merge", "--no-ff",
			fmt.Sprintf("refs/remotes/%s/%s", item.Remote, taskID),
			"-m", fmt.Sprintf("Merge branch '%s' into dev", taskID))
		if !merged.Ok() {
			errors = append(errors, fmt.Sprintf("%s: неожиданный сбой merge: %s", item.Label, merged.ErrMsg()))
			break
		}
		sha := strings.TrimSpace(mustStdout(repository, "rev-parse", "refs/heads/"+item.DevBranch))
		results = append(results, map[string]any{"label": item.Label, "local_merge_sha": sha, "created": true})
	}
	if errors == nil {
		errors = []string{}
	}
	return results, errors
}

// verifyRemoteUnchanged - remote race: re-ls-remote vs observed_remote_dev_sha.
func verifyRemoteUnchanged(project workspace.DeployProject, plan []map[string]any) []string {
	var errors []string
	for i, item := range project.Repositories {
		entry := plan[i]
		if already, _ := entry["already_deployed"].(bool); already {
			continue
		}
		repository, _ := filepath.Abs(item.Path)
		result, _ := gitcmd.Git(repository, 120*time.Second, "ls-remote", "--heads", item.Remote, "refs/heads/"+item.DevBranch)
		var actual any
		if result.Ok() && strings.Contains(result.Stdout, "\t") {
			actual = strings.SplitN(strings.TrimSpace(result.Stdout), "\t", 2)[0]
		}
		if actual != entry["observed_remote_dev_sha"] {
			errors = append(errors, fmt.Sprintf("%s: remote dev изменился во время интеграции", item.Label))
		}
	}
	if errors == nil {
		errors = []string{}
	}
	return errors
}

func pushSummary(results []map[string]any) map[string]any {
	ops := []string{"pushed", "failed", "not_attempted", "already_deployed"}
	out := map[string]any{}
	for _, op := range ops {
		labels := []string{}
		for _, item := range results {
			if item["operation"] == op {
				if label, ok := item["label"].(string); ok {
					labels = append(labels, label)
				}
			}
		}
		out[op] = labels
	}
	return out
}

// pushAllDev - sequential PushDevBranch; после fail → остальные not_attempted.
func pushAllDev(project workspace.DeployProject, plan []map[string]any) ([]map[string]any, map[string]any, []string) {
	var results []map[string]any
	var errors []string
	failed := false
	for i, item := range project.Repositories {
		entry := plan[i]
		repository, _ := filepath.Abs(item.Path)
		if already, _ := entry["already_deployed"].(bool); already {
			results = append(results, map[string]any{
				"label": item.Label, "operation": "already_deployed", "already_deployed": true, "pushed": false,
			})
			continue
		}
		if failed {
			results = append(results, map[string]any{
				"label": item.Label, "operation": "not_attempted", "pushed": false,
			})
			continue
		}
		pushed, err := gitcmd.PushDevBranch(repository, item.Remote, item.DevBranch)
		if err != nil {
			results = append(results, map[string]any{
				"label": item.Label, "operation": "failed", "pushed": false, "error": err.Error(),
			})
			errors = append(errors, fmt.Sprintf("%s: %s", item.Label, err.Error()))
			failed = true
			continue
		}
		result := map[string]any{
			"label": item.Label, "operation": "pushed", "pushed": pushed.Ok(),
		}
		if !pushed.Ok() {
			result["operation"] = "failed"
			result["error"] = pushed.ErrMsg()
			errors = append(errors, fmt.Sprintf("%s: %s", item.Label, pushed.ErrMsg()))
			results = append(results, result)
			failed = true
			continue
		}
		result["remote_dev_sha"] = strings.TrimSpace(mustStdout(repository, "rev-parse", "refs/heads/"+item.DevBranch))
		results = append(results, result)
	}
	if errors == nil {
		errors = []string{}
	}
	return results, pushSummary(results), errors
}
