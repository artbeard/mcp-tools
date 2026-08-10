package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/artbeard/mcp-tools/internal/timerstate"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func registerTools(s *server.MCPServer) {
	// Health-check самого MCP-сервера (не скилл).
	s.AddTool(mcp.NewTool("ping",
		mcp.WithDescription("Проверка живости MCP-сервера mcp-tools."),
	), handlePing)

	// skills/start-project-sandbox — lifecycle docker/tmux песочницы.
	s.AddTool(mcp.NewTool("start_project_sandbox",
		mcp.WithDescription("Управление песочницей проекта (list/check/status/start/stop/diagnose) через skills/start-project-sandbox. Ищет .workspace.yaml от cwd вверх. project опционален и при указании должен совпасть с name/aliases манифеста."),
		mcp.WithString("action", mcp.Required(), mcp.Description("list|check|status|start|stop|diagnose")),
		mcp.WithString("project", mcp.Description("имя или alias манифеста")),
		mcp.WithString("cwd", mcp.Description("каталог, откуда начинать поиск манифеста")),
		mcp.WithString("manifest", mcp.Description("явный путь к .workspace.yaml")),
	), handleStartSandbox)

	// skills/prepare-project-task — локальные task-ветки без push.
	s.AddTool(mcp.NewTool("prepare_project_task",
		mcp.WithDescription("Готовит локальные task-ветки во всех репах из .workspace.yaml. Никогда не делает push. Режимы: list, check, prepare. Манифест ищется от cwd."),
		mcp.WithString("mode", mcp.Required(), mcp.Description("list|check|prepare")),
		mcp.WithString("task_id", mcp.Description("обязателен для check/prepare")),
		mcp.WithString("project", mcp.Description("опциональный фильтр проекта")),
		mcp.WithString("cwd", mcp.Description("каталог, откуда начинать поиск манифеста")),
		mcp.WithString("manifest", mcp.Description("явный путь к .workspace.yaml")),
	), handlePrepareTask)

	// skills/prepare-project-merge-requests — push task + create/reuse GitLab MR.
	s.AddTool(mcp.NewTool("prepare_project_merge_requests",
		mcp.WithDescription("Публикует локальные task-ветки и создаёт/переиспользует GitLab MR по .workspace.yaml. Режимы: list, check, create."),
		mcp.WithString("mode", mcp.Required(), mcp.Description("list|check|create")),
		mcp.WithString("task_id", mcp.Description("обязателен для check/create")),
		mcp.WithString("project", mcp.Description("опциональный фильтр проекта")),
		mcp.WithString("cwd", mcp.Description("каталог, откуда начинать поиск манифеста")),
		mcp.WithString("manifest", mcp.Description("явный путь к .workspace.yaml")),
		mcp.WithString("title", mcp.Description("переопределение заголовка MR")),
		mcp.WithString("description", mcp.Description("переопределение описания MR")),
	), handlePrepareMR)

	// skills/deploy-project-task-to-dev — merge task → origin/dev + push только dev.
	s.AddTool(mcp.NewTool("deploy_project_task_to_dev",
		mcp.WithDescription("Мержит удалённые task-ветки в origin/dev по .workspace.yaml и пушит только origin/dev. Режимы: list, check, deploy, continue."),
		mcp.WithString("mode", mcp.Required(), mcp.Description("list|check|deploy|continue")),
		mcp.WithString("task_id", mcp.Description("обязателен для check/deploy/continue")),
		mcp.WithString("project", mcp.Description("опциональный фильтр проекта")),
		mcp.WithString("cwd", mcp.Description("каталог, откуда начинать поиск манифеста")),
		mcp.WithString("manifest", mcp.Description("явный путь к .workspace.yaml")),
	), handleDeployTask)

	// skills/yandex-tracker-timekeeper — локальный таймер + worklog.
	s.AddTool(mcp.NewTool("yandex_tracker_timekeeper",
		mcp.WithDescription("Локальный таймер Yandex Tracker: start/stop/status/write/clear. write по умолчанию dry-run; dry_run=false — отправить worklog."),
		mcp.WithString("action", mcp.Required(), mcp.Description("start|status|stop|write|clear")),
		mcp.WithString("issue_key", mcp.Description("ключ задачи Tracker")),
		mcp.WithString("comment", mcp.Description("комментарий к таймеру/worklog")),
		mcp.WithNumber("round_to", mcp.Description("округление минут для write"), mcp.DefaultNumber(1)),
		mcp.WithBoolean("ignore_active", mcp.Description("write даже при активном таймере"), mcp.DefaultBool(false)),
		mcp.WithBoolean("dry_run", mcp.Description("write без отправки (по умолчанию true)"), mcp.DefaultBool(true)),
		mcp.WithBoolean("clear_active", mcp.Description("сбросить активный таймер"), mcp.DefaultBool(false)),
		mcp.WithBoolean("clear_sessions", mcp.Description("сбросить ожидающие сессии"), mcp.DefaultBool(false)),
		mcp.WithString("env_dir", mcp.Description("каталог с .env / .env.local")),
		mcp.WithString("state_file", mcp.Description("путь к JSON state таймера")),
	), handleTrackerTime)

	// skills/yandex-tracker-issue-creator — создание задачи Tracker.
	s.AddTool(mcp.NewTool("yandex_tracker_issue_creator",
		mcp.WithDescription("Создаёт задачу в Yandex Tracker. По умолчанию dry-run; dry_run=false только после явного подтверждения точного payload."),
		mcp.WithString("queue", mcp.Required(), mcp.Description("ключ очереди Tracker")),
		mcp.WithString("summary", mcp.Required(), mcp.Description("заголовок задачи")),
		mcp.WithString("description", mcp.Required(), mcp.Description("тело описания")),
		mcp.WithBoolean("dry_run", mcp.Description("по умолчанию true"), mcp.DefaultBool(true)),
		mcp.WithString("project", mcp.Description("значение поля проекта")),
		mcp.WithString("project_field", mcp.Description("id кастомного поля проекта")),
		mcp.WithArray("tags", mcp.Description("теги задачи"), mcp.WithStringItems()),
		mcp.WithString("period", mcp.Description("значение поля периода")),
		mcp.WithString("period_field", mcp.Description("id кастомного поля периода")),
		mcp.WithNumber("sp", mcp.Description("story points")),
		mcp.WithString("sp_field", mcp.Description("id кастомного поля SP")),
		mcp.WithString("assignee", mcp.Description("логин/id исполнителя")),
		mcp.WithArray("fields", mcp.Description("доп. пары field=value"), mcp.WithStringItems()),
		mcp.WithString("env_dir", mcp.Description("каталог с .env / .env.local")),
	), handleTrackerCreateIssue)

	// skills/yandex-tracker-commenter — комментарий к задаче Tracker.
	s.AddTool(mcp.NewTool("yandex_tracker_comment",
		mcp.WithDescription("Публикует комментарий к задаче Yandex Tracker. По умолчанию dry-run; dry_run=false только после явного подтверждения ключа задачи и полного текста."),
		mcp.WithString("issue_key", mcp.Required(), mcp.Description("ключ задачи Tracker")),
		mcp.WithString("body", mcp.Required(), mcp.Description("текст комментария")),
		mcp.WithBoolean("dry_run", mcp.Description("по умолчанию true"), mcp.DefaultBool(true)),
		mcp.WithString("markup_type", mcp.Description("тип разметки")),
		mcp.WithArray("summonees", mcp.Description("кого призвать"), mcp.WithStringItems()),
		mcp.WithString("reply_to_comment", mcp.Description("id комментария для ответа")),
		mcp.WithString("env_dir", mcp.Description("каталог с .env / .env.local")),
	), handleTrackerComment)

	// skills/yandex-tracker-commenter — смена статуса задачи Tracker.
	s.AddTool(mcp.NewTool("yandex_tracker_transition",
		mcp.WithDescription("Меняет статус задачи Yandex Tracker. По умолчанию dry-run; dry_run=false только после явного подтверждения."),
		mcp.WithString("issue_key", mcp.Required(), mcp.Description("ключ задачи Tracker")),
		mcp.WithString("target", mcp.Description("целевой статус (display name)"), mcp.DefaultString("Открыто")),
		mcp.WithBoolean("dry_run", mcp.Description("по умолчанию true"), mcp.DefaultBool(true)),
	), handleTrackerTransition)

	// skills/gitlab-mr-commenter — note / line discussion в GitLab MR.
	s.AddTool(mcp.NewTool("gitlab_mr_comment",
		mcp.WithDescription("Публикует note или line discussion в GitLab MR. По умолчанию dry-run; dry_run=false только после явного подтверждения URL MR, места и полного текста."),
		mcp.WithString("mr_url", mcp.Required(), mcp.Description("URL merge request в GitLab")),
		mcp.WithString("body", mcp.Required(), mcp.Description("текст комментария")),
		mcp.WithBoolean("dry_run", mcp.Description("по умолчанию true"), mcp.DefaultBool(true)),
		mcp.WithBoolean("line_comment", mcp.Description("отправить как line discussion"), mcp.DefaultBool(false)),
		mcp.WithString("file_path", mcp.Description("путь к файлу для line discussion")),
		mcp.WithNumber("line", mcp.Description("номер строки")),
		mcp.WithString("line_type", mcp.Description("new|old"), mcp.DefaultString("new")),
		mcp.WithString("env_dir", mcp.Description("каталог с .env / .env.local")),
	), handleGitLabMRComment)

	// skills/gitlab-mr-reviewer — только факты MR (без суждений ревью).
	s.AddTool(mcp.NewTool("gitlab_mr_context",
		mcp.WithDescription("Читает контекст GitLab MR: metadata, changes/diffs, commits, discussions/notes. Не делает ревью - только JSON для skill gitlab-mr-reviewer."),
		mcp.WithString("mr_url", mcp.Required(), mcp.Description("URL merge request в GitLab")),
		mcp.WithString("author_filter", mcp.Description("CSV username/name/email/id для фильтра notes; пусто = все (или GITLAB_MR_AUTHOR_FILTER)")),
		mcp.WithBoolean("parse_only", mcp.Description("только разобрать URL без API"), mcp.DefaultBool(false)),
		mcp.WithString("env_dir", mcp.Description("каталог с .env / .env.local")),
	), handleGitLabMRContext)

	// skills/codestyle — поиск релевантных правил по описанию задачи.
	s.AddTool(mcp.NewTool("codestyle_find_rules",
		mcp.WithDescription("Ищет правила кодстайла (PHP/Symfony + Vue/TS) по описанию задачи. Вернёт URI + priority + summary; затем читай critical через codestyle_read_rule. Не пиши код «по памяти»."),
		mcp.WithString("task", mcp.Required(), mcp.Description("описание задачи ru/en, минимум 3 символа")),
		mcp.WithNumber("max_results", mcp.Description("макс. результатов 1..15"), mcp.DefaultNumber(8)),
	), handleCodestyleFindRules)

	// skills/codestyle — чтение полного текста правила по URI.
	s.AddTool(mcp.NewTool("codestyle_read_rule",
		mcp.WithDescription("Читает полный markdown правила по URI из codestyle_find_rules (например codestyle://backend/dto)."),
		mcp.WithString("uri", mcp.Required(), mcp.Description("codestyle://category/file")),
	), handleCodestyleReadRule)
}

func handlePing(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return mcp.NewToolResultText("pong"), nil
}

func handleStartSandbox(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	action := strings.ToLower(strings.TrimSpace(req.GetString("action", "")))
	allowed := map[string]bool{"list": true, "check": true, "status": true, "start": true, "stop": true, "diagnose": true}
	if !allowed[action] {
		return textResult("ОШИБКА: action должен быть одним из: check, diagnose, list, start, status, stop"), nil
	}
	project := req.GetString("project", "")
	cwd := req.GetString("cwd", "")
	manifest := req.GetString("manifest", "")

	args := []string{"--" + action}
	if action != "list" && project != "" {
		args = append(args, project)
	}
	args = append(args, optFlag("--cwd", cwd)...)
	args = append(args, optFlag("--manifest", manifest)...)
	return textResult(runCLI(resolveCmd("start-sandbox", "./skills/start-project-sandbox/cmd/start-sandbox"), args, cwd)), nil
}

func handlePrepareTask(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	mode := strings.ToLower(strings.TrimSpace(req.GetString("mode", "")))
	taskID := req.GetString("task_id", "")
	project := req.GetString("project", "")
	cwd := req.GetString("cwd", "")
	manifest := req.GetString("manifest", "")

	var args []string
	switch mode {
	case "list":
		args = append(args, "--list")
	case "check", "prepare":
		if taskID == "" {
			return textResult("ОШИБКА: для check/prepare нужен task_id"), nil
		}
		args = append(args, "--"+mode, taskID)
	default:
		return textResult("ОШИБКА: mode должен быть list|check|prepare"), nil
	}
	args = append(args, optFlag("--project", project)...)
	args = append(args, optFlag("--cwd", cwd)...)
	args = append(args, optFlag("--manifest", manifest)...)
	return textResult(runCLI(resolveCmd("prepare-task", "./skills/prepare-project-task/cmd/prepare-task"), args, cwd)), nil
}

func handlePrepareMR(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	mode := strings.ToLower(strings.TrimSpace(req.GetString("mode", "")))
	taskID := req.GetString("task_id", "")
	project := req.GetString("project", "")
	cwd := req.GetString("cwd", "")
	manifest := req.GetString("manifest", "")
	title := req.GetString("title", "")

	var args []string
	switch mode {
	case "list":
		args = append(args, "--list")
	case "check", "create":
		if taskID == "" {
			return textResult("ОШИБКА: для check/create нужен task_id"), nil
		}
		args = append(args, "--"+mode, taskID)
	default:
		return textResult("ОШИБКА: mode должен быть list|check|create"), nil
	}
	args = append(args, optFlag("--project", project)...)
	args = append(args, optFlag("--cwd", cwd)...)
	args = append(args, optFlag("--manifest", manifest)...)
	args = append(args, optFlag("--title", title)...)

	raw := req.GetArguments()
	if hasArg(raw, "description") {
		desc := req.GetString("description", "")
		path, err := writeTemp(desc, ".md")
		if err != nil {
			return textResult(fmt.Sprintf("ОШИБКА: временный файл: %v", err)), nil
		}
		defer os.Remove(path)
		args = append(args, "--description-file", path)
	}
	return textResult(runCLI(resolveCmd("prepare-mr", "./skills/prepare-project-merge-requests/cmd/prepare-mr"), args, cwd)), nil
}

func handleDeployTask(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	mode := strings.ToLower(strings.TrimSpace(req.GetString("mode", "")))
	taskID := req.GetString("task_id", "")
	project := req.GetString("project", "")
	cwd := req.GetString("cwd", "")
	manifest := req.GetString("manifest", "")

	var args []string
	switch mode {
	case "list":
		args = append(args, "--list")
	case "check", "deploy":
		if taskID == "" {
			return textResult("ОШИБКА: нужен task_id"), nil
		}
		args = append(args, "--"+mode, taskID)
	case "continue":
		if taskID == "" {
			return textResult("ОШИБКА: нужен task_id"), nil
		}
		args = append(args, "--continue", taskID)
	default:
		return textResult("ОШИБКА: mode должен быть list|check|deploy|continue"), nil
	}
	args = append(args, optFlag("--project", project)...)
	args = append(args, optFlag("--cwd", cwd)...)
	args = append(args, optFlag("--manifest", manifest)...)
	return textResult(runCLI(resolveCmd("deploy-task", "./skills/deploy-project-task-to-dev/cmd/deploy-task"), args, cwd)), nil
}

func handleTrackerTime(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	action := strings.ToLower(strings.TrimSpace(req.GetString("action", "")))
	issueKey := req.GetString("issue_key", "")
	comment := req.GetString("comment", "")
	roundTo := req.GetInt("round_to", 1)
	ignoreActive := req.GetBool("ignore_active", false)
	dryRun := req.GetBool("dry_run", true)
	clearActive := req.GetBool("clear_active", false)
	clearSessions := req.GetBool("clear_sessions", false)
	envDir := req.GetString("env_dir", "")
	stateFile := req.GetString("state_file", "")

	if stateFile == "" {
		path, err := timerstate.DefaultPath()
		if err != nil {
			return textResult(fmt.Sprintf("ОШИБКА: путь к state: %v", err)), nil
		}
		stateFile = path
	}

	args := optFlag("--state-file", stateFile)
	args = append(args, optFlag("--env-dir", envDir)...)

	switch action {
	case "start":
		if issueKey == "" {
			return textResult("ОШИБКА: для start нужен issue_key"), nil
		}
		args = append(args, "start", issueKey)
		args = append(args, optFlag("--comment", comment)...)
	case "status":
		args = append(args, "status")
	case "stop":
		args = append(args, "stop")
		args = append(args, optFlag("--comment", comment)...)
	case "write":
		args = append(args, "write")
		args = append(args, optFlag("--issue-key", issueKey)...)
		args = append(args, optFlag("--comment", comment)...)
		args = append(args, "--round-to", fmt.Sprintf("%d", roundTo))
		if ignoreActive {
			args = append(args, "--ignore-active")
		}
		if !dryRun {
			args = append(args, "--send", "--confirm-send")
		}
	case "clear":
		args = append(args, "clear")
		if clearActive {
			args = append(args, "--active")
		}
		if clearSessions {
			args = append(args, "--sessions")
		}
		args = append(args, optFlag("--issue-key", issueKey)...)
	default:
		return textResult("ОШИБКА: action должен быть start|status|stop|write|clear"), nil
	}
	return textResult(runCLI(resolveCmd("tracker-time", "./skills/yandex-tracker-timekeeper/cmd/tracker-time"), args, "")), nil
}

func handleTrackerCreateIssue(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	queue := req.GetString("queue", "")
	summary := req.GetString("summary", "")
	description := req.GetString("description", "")
	dryRun := req.GetBool("dry_run", true)
	project := req.GetString("project", "")
	projectField := req.GetString("project_field", "")
	tags := req.GetStringSlice("tags", nil)
	period := req.GetString("period", "")
	periodField := req.GetString("period_field", "")
	spField := req.GetString("sp_field", "")
	assignee := req.GetString("assignee", "")
	fields := req.GetStringSlice("fields", nil)
	envDir := req.GetString("env_dir", "")

	descPath, err := writeTemp(description, ".md")
	if err != nil {
		return textResult(fmt.Sprintf("ОШИБКА: временный файл: %v", err)), nil
	}
	defer os.Remove(descPath)

	args := []string{
		"--queue", queue,
		"--summary", summary,
		"--description-file", descPath,
	}
	args = append(args, optFlag("--env-dir", envDir)...)
	args = append(args, optFlag("--project", project)...)
	args = append(args, optFlag("--project-field", projectField)...)
	for _, tag := range tags {
		args = append(args, "--tag", tag)
	}
	args = append(args, optFlag("--period", period)...)
	args = append(args, optFlag("--period-field", periodField)...)
	if hasArg(req.GetArguments(), "sp") {
		args = append(args, "--sp", fmt.Sprintf("%v", req.GetFloat("sp", 0)))
	}
	args = append(args, optFlag("--sp-field", spField)...)
	args = append(args, optFlag("--assignee", assignee)...)
	for _, field := range fields {
		args = append(args, "--field", field)
	}
	if !dryRun {
		args = append(args, "--send", "--confirm-send")
	}
	return textResult(runCLI(resolveCmd("tracker-create-issue", "./skills/yandex-tracker-issue-creator/cmd/tracker-create-issue"), args, "")), nil
}

func handleTrackerComment(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	issueKey := req.GetString("issue_key", "")
	body := req.GetString("body", "")
	dryRun := req.GetBool("dry_run", true)
	markupType := req.GetString("markup_type", "")
	summonees := req.GetStringSlice("summonees", nil)
	replyTo := req.GetString("reply_to_comment", "")
	envDir := req.GetString("env_dir", "")

	bodyPath, err := writeTemp(body, ".md")
	if err != nil {
		return textResult(fmt.Sprintf("ОШИБКА: временный файл: %v", err)), nil
	}
	defer os.Remove(bodyPath)

	args := []string{
		"--issue-key", issueKey,
		"--body-file", bodyPath,
	}
	args = append(args, optFlag("--env-dir", envDir)...)
	args = append(args, optFlag("--markup-type", markupType)...)
	for _, s := range summonees {
		args = append(args, "--summonee", s)
	}
	args = append(args, optFlag("--reply-to-comment", replyTo)...)
	if !dryRun {
		args = append(args, "--send", "--confirm-send")
	}
	return textResult(runCLI(resolveCmd("tracker-comment", "./skills/yandex-tracker-commenter/cmd/tracker-comment"), args, "")), nil
}

func handleTrackerTransition(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	issueKey := req.GetString("issue_key", "")
	target := req.GetString("target", "Открыто")
	dryRun := req.GetBool("dry_run", true)

	args := []string{"--issue-key", issueKey, "--target", target}
	if !dryRun {
		args = append(args, "--send", "--confirm-send")
	}
	return textResult(runCLI(resolveCmd("tracker-transition", "./skills/yandex-tracker-commenter/cmd/tracker-transition"), args, "")), nil
}

func handleGitLabMRComment(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	mrURL := req.GetString("mr_url", "")
	body := req.GetString("body", "")
	dryRun := req.GetBool("dry_run", true)
	lineComment := req.GetBool("line_comment", false)
	filePath := req.GetString("file_path", "")
	lineType := req.GetString("line_type", "new")
	envDir := req.GetString("env_dir", "")

	bodyPath, err := writeTemp(body, ".md")
	if err != nil {
		return textResult(fmt.Sprintf("ОШИБКА: временный файл: %v", err)), nil
	}
	defer os.Remove(bodyPath)

	args := []string{
		"--mr-url", mrURL,
		"--body-file", bodyPath,
		"--line-type", lineType,
	}
	args = append(args, optFlag("--env-dir", envDir)...)
	if lineComment {
		args = append(args, "--line-comment")
		args = append(args, optFlag("--file-path", filePath)...)
		if hasArg(req.GetArguments(), "line") {
			args = append(args, "--line", fmt.Sprintf("%d", req.GetInt("line", 0)))
		}
	}
	if !dryRun {
		args = append(args, "--send", "--confirm-send")
	}
	return textResult(runCLI(resolveCmd("gitlab-mr-comment", "./skills/gitlab-mr-commenter/cmd/gitlab-mr-comment"), args, "")), nil
}

func handleGitLabMRContext(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	mrURL := req.GetString("mr_url", "")
	authorFilter := req.GetString("author_filter", "")
	parseOnly := req.GetBool("parse_only", false)
	envDir := req.GetString("env_dir", "")

	args := []string{"--mr-url", mrURL}
	args = append(args, optFlag("--author-filter", authorFilter)...)
	args = append(args, optFlag("--env-dir", envDir)...)
	if parseOnly {
		args = append(args, "--parse-only")
	}
	return textResult(runCLI(resolveCmd("gitlab-mr-context", "./skills/gitlab-mr-reviewer/cmd/gitlab-mr-context"), args, "")), nil
}

func handleCodestyleFindRules(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	task := strings.TrimSpace(req.GetString("task", ""))
	maxResults := req.GetInt("max_results", 8)
	args := []string{"find-rules", "--task", task, "--max-results", fmt.Sprintf("%d", maxResults)}
	return textResult(runCLI(resolveCmd("codestyle", "./skills/codestyle/cmd/codestyle"), args, "")), nil
}

func handleCodestyleReadRule(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	uri := strings.TrimSpace(req.GetString("uri", ""))
	if uri == "" {
		return textResult("ОШИБКА: нужен uri"), nil
	}
	args := []string{"read-rule", "--uri", uri}
	return textResult(runCLI(resolveCmd("codestyle", "./skills/codestyle/cmd/codestyle"), args, "")), nil
}

func textResult(s string) *mcp.CallToolResult {
	return mcp.NewToolResultText(s)
}
