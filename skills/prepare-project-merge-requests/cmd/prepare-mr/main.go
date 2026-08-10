// CLI: публикация task-веток и create/reuse GitLab MR (бывший prepare_merge_requests.py).
// Режимы: --list / --check TASK / --create TASK. Push только через PushTaskBranch.
//
// Exit codes:
//
//	0 - ok (list / check без ошибок / create полный success)
//	1 - preflight errors (action=check), в т.ч. если звали --create
//	2 - setup: манифест, env, args
//	3 - mid-create fail (phase: publish / create MR / validate URLs)
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/artbeard/mcp-tools/internal/envfile"
	"github.com/artbeard/mcp-tools/internal/gitcmd"
	"github.com/artbeard/mcp-tools/internal/gitlab"
	"github.com/artbeard/mcp-tools/internal/repopath"
	"github.com/artbeard/mcp-tools/internal/workspace"
)

func main() {
	list := flag.Bool("list", false, "показать манифест")
	check := flag.String("check", "", "только preflight для TASK_ID")
	create := flag.String("create", "", "push + create/reuse MR для TASK_ID")
	project := flag.String("project", "", "имя/alias проекта из манифеста")
	cwd := flag.String("cwd", "", "старт discovery (по умолчанию getwd)")
	manifest := flag.String("manifest", "", "явный путь к .workspace.yaml")
	title := flag.String("title", "", "общий title для MR")
	descFile := flag.String("description-file", "", "UTF-8 файл с description MR")
	flag.Parse()

	// Ровно один режим: list | check | create.
	modes := 0
	if *list {
		modes++
	}
	if *check != "" {
		modes++
	}
	if *create != "" {
		modes++
	}
	if modes != 1 {
		fmt.Fprintf(os.Stderr, "использование: prepare-mr (--list | --check TASK | --create TASK) [--project NAME] [--cwd DIR] [--manifest PATH] [--title T] [--description-file PATH]\n")
		os.Exit(2)
	}

	code := run(*list, *check, *create, *project, *cwd, *manifest, *title, *descFile)
	os.Exit(code)
}

// run - оркестрация: discover → (list | inspect всех реп → check-ответ | push → create MR).
// Инвариант: полный preflight всех реп до первого push; все push до первого CreateMR.
func run(list bool, checkTask, createTask, project, cwd, manifestPath, title, descFile string) int {
	m, err := workspace.Discover(cwd, manifestPath)
	if err != nil {
		return emit(map[string]any{"status": "failed", "errors": []string{err.Error()}}, 2)
	}
	if err := workspace.AssertProjectName(m, project); err != nil {
		return emit(map[string]any{"status": "failed", "errors": []string{err.Error()}}, 2)
	}
	if list {
		return emit(workspace.ListPayload(m), 0)
	}

	proj := workspace.AsMR(m)
	taskID := checkTask
	if createTask != "" {
		taskID = createTask
	}

	// GitLab креды только из корневого .env / .env.local (не config/gitlab.env).
	envDir, err := repopath.EnvDir()
	if err != nil {
		return emit(map[string]any{"status": "failed", "errors": []string{err.Error()}}, 2)
	}
	env, err := envfile.LoadDir(envDir)
	if err != nil {
		return emit(map[string]any{"status": "failed", "errors": []string{err.Error()}}, 2)
	}
	client, err := gitlab.NewClient(env, "")
	if err != nil {
		return emit(map[string]any{"status": "failed", "errors": []string{
			fmt.Sprintf("нужны GITLAB_TOKEN и GITLAB_BASE_URL в %s/.env (или .env.local): %v", envDir, err),
		}}, 2)
	}
	if strings.TrimSpace(client.Token) == "" {
		return emit(map[string]any{"status": "failed", "errors": []string{
			fmt.Sprintf("GITLAB_TOKEN пуст в %s/.env (или .env.local)", envDir),
		}}, 2)
	}

	description := ""
	if strings.TrimSpace(descFile) != "" {
		raw, err := os.ReadFile(descFile)
		if err != nil {
			return emit(map[string]any{"status": "failed", "errors": []string{err.Error()}}, 2)
		}
		description = string(raw)
	}

	var titlePtr *string
	if strings.TrimSpace(title) != "" {
		t := title
		titlePtr = &t
	}

	// Preflight по всем репам (даже при --create): dirty / history / FF / existing MR.
	checks := make([]map[string]any, 0, len(proj.Repositories))
	for _, item := range proj.Repositories {
		checks = append(checks, inspectRepository(item, taskID, titlePtr, m.ProtectedBranches, client))
	}

	errors := flattenRepoErrors(checks)
	// Любые ошибки ИЛИ режим --check → только check-payload, без push/MR.
	// При ошибках на --create тоже action=check, exit 1 (ещё ничего не меняли).
	if len(errors) > 0 || checkTask != "" {
		status := "ok"
		exit := 0
		if len(errors) > 0 {
			status = "failed"
			exit = 1
		}
		checkURLs := []string{}
		for _, item := range checks {
			if em, ok := item["existing_mr"].(map[string]any); ok {
				if u, ok := em["url"].(string); ok && u != "" {
					checkURLs = append(checkURLs, u)
				}
			}
		}
		return emit(map[string]any{
			"status":       status,
			"action":       "check",
			"project":      proj.Key,
			"project_type": proj.Type,
			"task_id":      taskID,
			"repositories": checks,
			"mr_urls":      checkURLs,
			"errors":       errors,
		}, exit)
	}

	// --- phase: publish_task_branches ---
	// Push только needs_mr; no_changes пропускаем. Fail → exit 3, partial в repositories.
	pushResults := make([]map[string]any, 0, len(proj.Repositories))
	for i, item := range proj.Repositories {
		check := checks[i]
		localSHA, _ := check["local_sha"].(string)
		needs, _ := check["needs_mr"].(bool)
		if !needs {
			pushResults = append(pushResults, map[string]any{
				"label": item.Label, "published": false, "operation": "no_changes", "sha": localSHA,
			})
			continue
		}
		pushed, err := gitcmd.PushTaskBranch(item.Path, item.Remote, taskID, m.ProtectedBranches)
		if err != nil {
			entry := map[string]any{"label": item.Label, "published": false, "sha": localSHA, "error": err.Error()}
			pushResults = append(pushResults, entry)
			return emit(map[string]any{
				"status":       "failed",
				"action":       "create",
				"phase":        "publish_task_branches",
				"project":      proj.Key,
				"task_id":      taskID,
				"repositories": pushResults,
				"mr_urls":      []string{},
				"errors":       []string{fmt.Sprintf("%s: %s", item.Label, err.Error())},
			}, 3)
		}
		entry := map[string]any{"label": item.Label, "published": pushed.Ok(), "sha": localSHA}
		if !pushed.Ok() {
			entry["error"] = pushed.ErrMsg()
			pushResults = append(pushResults, entry)
			return emit(map[string]any{
				"status":       "failed",
				"action":       "create",
				"phase":        "publish_task_branches",
				"project":      proj.Key,
				"task_id":      taskID,
				"repositories": pushResults,
				"mr_urls":      []string{},
				"errors":       []string{fmt.Sprintf("%s: %s", item.Label, pushed.ErrMsg())},
			}, 3)
		}
		pushResults = append(pushResults, entry)
	}

	// --- phase: create_merge_requests ---
	// reuse existing_mr; иначе POST; race (уже создали параллельно) → FindOpenedMR → already_exists.
	mrResults := make([]map[string]any, 0, len(proj.Repositories))
	for i, item := range proj.Repositories {
		check := checks[i]
		needs, _ := check["needs_mr"].(bool)
		if !needs {
			mrResults = append(mrResults, map[string]any{"label": item.Label, "operation": "no_changes", "url": nil})
			continue
		}
		if em, ok := check["existing_mr"].(map[string]any); ok {
			row := map[string]any{"label": item.Label, "operation": "already_exists"}
			for k, v := range em {
				row[k] = v
			}
			mrResults = append(mrResults, row)
			continue
		}
		projectPath, _ := check["gitlab_project"].(string)
		mrTitle, _ := check["title"].(string)
		created, createErr := client.CreateMR(projectPath, taskID, item.TargetBranch, mrTitle, description)
		if createErr != nil {
			// Race: MR могли создать между check и create - ищем opened и считаем already_exists.
			existing, findErr := client.FindOpenedMR(projectPath, taskID, item.TargetBranch)
			if findErr == nil && existing != nil {
				mrResults = append(mrResults, map[string]any{
					"label": item.Label, "operation": "already_exists",
					"iid": existing.IID, "url": existing.URL, "title": existing.Title,
				})
				continue
			}
			mrResults = append(mrResults, map[string]any{
				"label": item.Label, "operation": "failed", "error": createErr.Error(),
			})
			return emit(map[string]any{
				"status":             "failed",
				"action":             "create",
				"phase":              "create_merge_requests",
				"project":            proj.Key,
				"task_id":            taskID,
				"published_branches": pushResults,
				"merge_requests":     mrResults,
				"mr_urls":            collectMRURLs(mrResults),
				"errors":             []string{fmt.Sprintf("%s: %s", item.Label, createErr.Error())},
			}, 3)
		}
		mrResults = append(mrResults, map[string]any{
			"label": item.Label, "operation": "created",
			"iid": created.IID, "url": created.URL, "title": created.Title,
		})
	}

	// --- phase: validate_merge_request_urls ---
	// У каждого created/already_exists должен быть валидный url (no_changes в mr_urls не входят).
	urls := collectMRURLs(mrResults)
	expected := 0
	for _, item := range mrResults {
		op, _ := item["operation"].(string)
		if op == "created" || op == "already_exists" {
			expected++
		}
	}
	if len(urls) != expected {
		return emit(map[string]any{
			"status":             "failed",
			"action":             "create",
			"phase":              "validate_merge_request_urls",
			"project":            proj.Key,
			"task_id":            taskID,
			"published_branches": pushResults,
			"merge_requests":     mrResults,
			"mr_urls":            urls,
			"errors":             []string{"не у каждого created/already_exists MR есть валидный URL"},
		}, 3)
	}
	return emit(map[string]any{
		"status":             "ok",
		"action":             "create",
		"project":            proj.Key,
		"project_type":       proj.Type,
		"task_id":            taskID,
		"published_branches": pushResults,
		"merge_requests":     mrResults,
		"mr_urls":            urls,
		"errors":             []string{},
	}, 0)
}

// emit - JSON в stdout + вернуть exit code (для os.Exit).
func emit(payload map[string]any, code int) int {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	_ = enc.Encode(payload)
	return code
}

// flattenRepoErrors - "label: msg" из per-repo errors[].
func flattenRepoErrors(checks []map[string]any) []string {
	var out []string
	for _, item := range checks {
		label, _ := item["label"].(string)
		errs, _ := item["errors"].([]string)
		for _, e := range errs {
			out = append(out, fmt.Sprintf("%s: %s", label, e))
		}
	}
	if out == nil {
		out = []string{}
	}
	return out
}

// collectMRURLs - url только у created / already_exists (для top-level mr_urls).
func collectMRURLs(records []map[string]any) []string {
	var out []string
	for _, item := range records {
		op, _ := item["operation"].(string)
		if op != "created" && op != "already_exists" {
			continue
		}
		if u, ok := item["url"].(string); ok && u != "" {
			out = append(out, u)
		}
	}
	if out == nil {
		out = []string{}
	}
	return out
}

// inspectRepository - preflight одной репы (без push/create).
// Готово к publish, если: clean, локальная task есть, remote target есть,
// общая история с target без merge-коммитов вне target, remote task (если есть) - предок local (FF).
// needs_mr=false когда commits_ahead_of_target==0 → skip push/MR (no_changes).
func inspectRepository(item workspace.MRRepo, taskID string, titleOverride *string, protected []string, client *gitlab.Client) map[string]any {
	repository, _ := filepath.Abs(item.Path)
	result := map[string]any{
		"label":  item.Label,
		"path":   repository,
		"ready":  false,
		"errors": []string{},
	}
	errors := []string{}
	// На любом return проставляем итоговые errors/ready.
	defer func() {
		result["errors"] = errors
		result["ready"] = len(errors) == 0
	}()

	if _, ok := gitcmd.ForbiddenTaskBranches[taskID]; ok {
		errors = append(errors, fmt.Sprintf("task-ветка защищена: %s", taskID))
		return result
	}
	for _, p := range protected {
		if taskID == p {
			errors = append(errors, fmt.Sprintf("task-ветка защищена: %s", taskID))
			return result
		}
	}
	valid := gitcmd.Run("", 30*time.Second, "git", "check-ref-format", "--branch", taskID)
	if !valid.Ok() {
		errors = append(errors, fmt.Sprintf("некорректная task-ветка: %s", taskID))
		return result
	}

	inside, err := gitcmd.Git(repository, 30*time.Second, "rev-parse", "--is-inside-work-tree")
	if err != nil || !inside.Ok() {
		errors = append(errors, "это не Git work tree")
		return result
	}
	// Dirty / in-progress не стопают сразу: копим ошибки, дальше всё равно смотрим ветку (как Python).
	status, _ := gitcmd.Git(repository, 30*time.Second, "status", "--porcelain=v1", "--untracked-files=all")
	statusOut := strings.TrimSpace(status.Stdout)
	if !status.Ok() || statusOut != "" {
		if statusOut != "" {
			result["changes"] = strings.Split(statusOut, "\n")
		}
		errors = append(errors, "репозиторий не чистый")
	}
	markers := gitcmd.OperationMarkers(repository)
	if len(markers) > 0 {
		result["in_progress"] = markers
		errors = append(errors, "идёт Git-операция")
	}

	// В отличие от prepare-task: локальная task-ветка уже должна существовать.
	localTask, _ := gitcmd.Git(repository, 30*time.Second, "show-ref", "--verify", "--hash", "refs/heads/"+taskID)
	if !localTask.Ok() {
		errors = append(errors, fmt.Sprintf("локальной task-ветки нет: %s", taskID))
		return result
	}
	localSHA := strings.TrimSpace(localTask.Stdout)
	result["local_sha"] = localSHA

	remoteURLRes, _ := gitcmd.Git(repository, 30*time.Second, "remote", "get-url", item.Remote)
	if !remoteURLRes.Ok() {
		errors = append(errors, fmt.Sprintf("remote не существует: %s", item.Remote))
		return result
	}
	remoteRef, err := gitlab.ParseRemoteURL(strings.TrimSpace(remoteURLRes.Stdout))
	if err != nil {
		errors = append(errors, err.Error())
		return result
	}
	result["gitlab_project"] = remoteRef.ProjectPath

	// ls-remote: есть ли remote target и (опционально) remote task; sha для FF-check.
	targetRef := "refs/heads/" + item.TargetBranch
	taskRef := "refs/heads/" + taskID
	lsRemote, err := gitcmd.Git(repository, 120*time.Second, "ls-remote", "--heads", item.Remote, targetRef, taskRef)
	if err != nil || !lsRemote.Ok() {
		msg := "не удалось проверить remote"
		if err != nil {
			msg = err.Error()
		} else {
			msg = lsRemote.ErrMsg()
		}
		errors = append(errors, fmt.Sprintf("не удалось проверить remote: %s", msg))
		return result
	}
	refs := map[string]string{} // ref → sha
	for _, line := range strings.Split(lsRemote.Stdout, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || !strings.Contains(line, "\t") {
			continue
		}
		parts := strings.SplitN(line, "\t", 2)
		refs[parts[1]] = parts[0]
	}
	if _, ok := refs[targetRef]; !ok {
		errors = append(errors, fmt.Sprintf("remote target-ветки нет: %s", item.TargetBranch))
		return result
	}

	// Fetch только target → tracking ref, чтобы merge-base/rev-list смотрели на актуальный remote.
	targetTracking := fmt.Sprintf("refs/remotes/%s/%s", item.Remote, item.TargetBranch)
	fetched, _ := gitcmd.Git(repository, 120*time.Second, "fetch", "--no-tags", item.Remote, targetRef+":"+targetTracking)
	if !fetched.Ok() {
		errors = append(errors, fmt.Sprintf("не удалось fetch remote target: %s", fetched.ErrMsg()))
		return result
	}

	history, historyErrors := taskHistoryGuard(repository, targetTracking, localSHA)
	result["history_guard"] = history
	errors = append(errors, historyErrors...)
	if len(historyErrors) > 0 {
		return result
	}

	// Сколько коммитов task впереди target: 0 → no_changes (пустой MR не создаём).
	unique, _ := gitcmd.Git(repository, 60*time.Second, "rev-list", "--count", targetTracking+".."+localSHA)
	if !unique.Ok() {
		errors = append(errors, fmt.Sprintf("не удалось сравнить task с target: %s", unique.ErrMsg()))
		return result
	}
	commitsAhead, _ := strconv.Atoi(strings.TrimSpace(unique.Stdout))
	result["commits_ahead_of_target"] = commitsAhead
	needsMR := commitsAhead > 0
	result["needs_mr"] = needsMR

	remoteTaskSHA := refs[taskRef]
	if remoteTaskSHA == "" {
		result["remote_task_sha"] = nil
	} else {
		result["remote_task_sha"] = remoteTaskSHA
	}
	// Remote task уже есть и отличается от local → только FF-publish: remote должен быть предком local.
	if needsMR && remoteTaskSHA != "" && remoteTaskSHA != localSHA {
		fetchedTask, _ := gitcmd.Git(repository, 120*time.Second, "fetch", "--no-tags", item.Remote,
			fmt.Sprintf("%s:refs/remotes/%s/%s", taskRef, item.Remote, taskID))
		if !fetchedTask.Ok() {
			errors = append(errors, fmt.Sprintf("не удалось fetch remote task: %s", fetchedTask.ErrMsg()))
		} else {
			ancestor, _ := gitcmd.Git(repository, 60*time.Second, "merge-base", "--is-ancestor", remoteTaskSHA, localSHA)
			if !ancestor.Ok() {
				errors = append(errors, "remote task-ветка не предок локальной task-ветки")
			}
		}
	}

	// Title: --title, иначе subject head-коммита (с префиксом taskID если его ещё нет).
	message, _ := gitcmd.Git(repository, 30*time.Second, "log", "-1", "--format=%s", "refs/heads/"+taskID)
	commitSubject := taskID
	if message.Ok() {
		commitSubject = strings.TrimSpace(message.Stdout)
	}
	if titleOverride != nil {
		result["title"] = *titleOverride
	} else if strings.HasPrefix(commitSubject, taskID) {
		result["title"] = commitSubject
	} else {
		result["title"] = fmt.Sprintf("%s | %s", taskID, commitSubject)
	}

	if needsMR {
		existing, err := client.FindOpenedMR(remoteRef.ProjectPath, taskID, item.TargetBranch)
		if err != nil {
			errors = append(errors, err.Error())
		} else if existing != nil {
			result["existing_mr"] = map[string]any{
				"iid": existing.IID, "url": existing.URL, "title": existing.Title,
			}
		}
	}
	result["target_branch"] = item.TargetBranch
	return result
}

// taskHistoryGuard - защита от контаминации (обычно merge/история с dev).
// 1) merge-base target..task должен существовать (общая история).
// 2) rev-list --merges target..task должен быть пуст: линейная ветка от master/main.
func taskHistoryGuard(repository, targetRef, taskRef string) (map[string]any, []string) {
	result := map[string]any{"target_ref": targetRef, "task_ref": taskRef}
	var errors []string
	mergeBase, _ := gitcmd.Git(repository, 60*time.Second, "merge-base", targetRef, taskRef)
	if !mergeBase.Ok() || strings.TrimSpace(mergeBase.Stdout) == "" {
		errors = append(errors, "task-ветка не имеет общей истории с remote master/main target")
		return result, errors
	}
	result["branch_point"] = strings.TrimSpace(mergeBase.Stdout)
	merges, _ := gitcmd.Git(repository, 60*time.Second, "rev-list", "--merges", targetRef+".."+taskRef)
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
			"в task-ветке есть merge-коммиты вне remote master/main; возможно попадание коммитов из dev, пересобери ветку от текущего target")
	}
	return result, errors
}
