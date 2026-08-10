// CLI: lifecycle sandbox compose/tmux (бывший start_sandbox.py).
// Режимы: --list | --check|status|start|stop|diagnose [PROJECT]
//
// Exit codes:
//
//	0 - ok (list/check/status/diagnose; start/stop без ошибок)
//	1 - start/stop: есть lifecycle errors у компонентов
//	2 - setup: манифест, project, validate
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/artbeard/mcp-tools/internal/sandbox"
	"github.com/artbeard/mcp-tools/internal/workspace"
)

func main() {
	// Bool-флаги вместо String: PROJECT идёт позиционным argv после --start/--stop/...
	// (как Python nargs='?' у argparse). Пример: start-sandbox --start myproj
	list := flag.Bool("list", false, "показать манифест")
	check := flag.Bool("check", false, "validate компонентов")
	status := flag.Bool("status", false, "статус компонентов")
	start := flag.Bool("start", false, "запустить компоненты")
	stop := flag.Bool("stop", false, "остановить компоненты")
	diagnose := flag.Bool("diagnose", false, "статус + diagnostics")
	cwd := flag.String("cwd", "", "старт discovery")
	manifest := flag.String("manifest", "", "явный путь к .workspace.yaml")
	flag.Parse()

	// Ровно один action-флаг.
	modes := 0
	action := ""
	if *list {
		modes++
		action = "list"
	}
	if *check {
		modes++
		action = "check"
	}
	if *status {
		modes++
		action = "status"
	}
	if *start {
		modes++
		action = "start"
	}
	if *stop {
		modes++
		action = "stop"
	}
	if *diagnose {
		modes++
		action = "diagnose"
	}
	if modes != 1 {
		fmt.Fprintf(os.Stderr, "использование: start-sandbox (--list | --check|status|start|stop|diagnose [PROJECT]) [--cwd DIR] [--manifest PATH]\n")
		os.Exit(2)
	}

	// Остаток argv после флагов = опциональный project (name/alias).
	projectArg := ""
	if args := flag.Args(); len(args) > 0 {
		projectArg = args[0]
	}

	os.Exit(run(action, projectArg, *cwd, *manifest))
}

// run - discover → (list | validate → check/status/diagnose/start/stop).
// Lifecycle (start/stop) делегируется internal/sandbox; здесь только оркестрация и JSON.
func run(action, projectArg, cwd, manifestPath string) int {
	m, err := workspace.Discover(cwd, manifestPath)
	if err != nil {
		return emit(map[string]any{"status": "failed", "errors": []string{err.Error()}}, 2)
	}
	if action == "list" {
		// list не требует sandbox.components и не зовёт Validate.
		return emit(workspace.ListPayload(m), 0)
	}

	// Пустой projectArg → берём name манифеста; иначе name/alias должен совпасть.
	if err := workspace.AssertProjectName(m, projectArg); err != nil {
		return emit(map[string]any{"status": "failed", "errors": []string{err.Error()}}, 2)
	}
	project := projectArg
	if project == "" {
		project = m.Name
	}
	_, items := workspace.AsSandbox(m)
	if len(items) == 0 {
		return emit(map[string]any{
			"status": "failed",
			"errors": []string{fmt.Sprintf("в манифесте %s нет sandbox.components", m.ManifestPath)},
		}, 2)
	}

	// Preflight до любой мутации: dirs, LookPath, tmux sessions, docker images.
	if errs := sandbox.Validate(items); len(errs) > 0 {
		return emit(map[string]any{
			"status": "failed", "action": "check", "project": project, "errors": errs,
		}, 2)
	}

	switch action {
	case "check":
		ids := make([]string, 0, len(items))
		for _, item := range items {
			ids = append(ids, item.Component)
		}
		return emit(map[string]any{
			"status": "ok", "action": "check", "project": project, "components": ids,
		}, 0)
	case "status":
		components := make([]sandbox.Status, 0, len(items))
		for _, item := range items {
			components = append(components, sandbox.ComponentStatus(item))
		}
		return emit(map[string]any{
			"status": "ok", "action": "status", "project": project, "components": components,
		}, 0)
	case "diagnose":
		// Diagnostics (compose logs / tmux pane) только для !ready - не шумим по здоровым.
		components := make([]sandbox.Status, 0, len(items))
		for _, item := range items {
			st := sandbox.ComponentStatus(item)
			if ready, _ := st["ready"].(bool); !ready {
				st["diagnostics"] = sandbox.Diagnostics(item)
			}
			components = append(components, st)
		}
		return emit(map[string]any{
			"status": "ok", "action": "diagnose", "project": project, "components": components,
		}, 0)
	}

	// --- start / stop ---
	// start: порядок как в манифесте, fail-fast (break на первой ошибке).
	// stop: reverse order, собираем все ошибки (не break).
	opItems := items
	if action == "stop" {
		opItems = reverseCopy(items)
	}
	results := make([]sandbox.Status, 0, len(opItems))
	var lifecycleErrors []string
	for _, item := range opItems {
		var st sandbox.Status
		var errMsg string
		if action == "start" {
			// ownership guards + wait_until_ready внутри StartComponent.
			st, errMsg = sandbox.StartComponent(item)
		} else {
			// refuse unmanaged tmux; volumes не трогаем.
			st, errMsg = sandbox.StopComponent(item)
		}
		if errMsg != "" {
			st["error"] = errMsg
			st["diagnostics"] = sandbox.Diagnostics(item)
			lifecycleErrors = append(lifecycleErrors, item.Component+": "+errMsg)
		}
		results = append(results, st)
		if errMsg != "" && action == "start" {
			break
		}
	}
	if lifecycleErrors == nil {
		lifecycleErrors = []string{}
	}
	status := "ok"
	exit := 0
	if len(lifecycleErrors) > 0 {
		status = "failed"
		exit = 1
	}
	return emit(map[string]any{
		"status": status, "action": action, "project": project,
		"components": results, "errors": lifecycleErrors,
	}, exit)
}

// reverseCopy - порядок stop: последние компоненты манифеста гасим первыми.
func reverseCopy(items []workspace.SandboxComponent) []workspace.SandboxComponent {
	out := make([]workspace.SandboxComponent, len(items))
	for i := range items {
		out[len(items)-1-i] = items[i]
	}
	return out
}

// emit - JSON в stdout + вернуть exit code (для os.Exit).
func emit(payload map[string]any, code int) int {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	_ = enc.Encode(payload)
	return code
}
