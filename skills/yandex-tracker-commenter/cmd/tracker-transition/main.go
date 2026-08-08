// CLI: смена статуса задачи в Yandex Tracker (бывший transition_tracker_issue.py).
// По умолчанию dry-run: только показывает какой transition выбран.
// Реальная смена: --send + --confirm-send (или CONFIRM_SEND=YES).
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/artbeard/mcp-tools/internal/envfile"
	"github.com/artbeard/mcp-tools/internal/repopath"
	"github.com/artbeard/mcp-tools/internal/tracker"
)

func main() {
	issueKey := flag.String("issue-key", "", "ключ задачи (например IT-123)")
	target := flag.String("target", "Открыто", "целевой статус: display / key / id")
	send := flag.Bool("send", false, "реально выполнить transition (без флага - dry-run)")
	confirmSend := flag.Bool("confirm-send", false, "подтверждение отправки (вместе с --send)")
	envDir := flag.String("env-dir", "", "каталог с .env / .env.local (по умолчанию корень mcp-tools)")
	flag.Parse()

	if err := run(*issueKey, *target, *send, *confirmSend, *envDir); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
		os.Exit(1)
	}
}

func run(issueKey, target string, send, confirmSend bool, envDir string) error {
	if issueKey == "" {
		return fmt.Errorf("--issue-key is required")
	}

	// .env обязателен, .env.local поверх (если есть)
	dir := envDir
	if dir == "" {
		var err error
		dir, err = repopath.EnvDir()
		if err != nil {
			return err
		}
	}

	env, err := envfile.LoadDir(dir)
	if err != nil {
		return err
	}
	client, err := tracker.NewClient(env)
	if err != nil {
		return err
	}

	// Читаем из API:
	//задачу (текущий статус)
	
	issue, err := client.GetIssue(issueKey)
	if err != nil {
		return err
	}
	//доступные transitions из этого статуса
	transitions, err := client.ListTransitions(issueKey)
	if err != nil {
		return err
	}

	compact := tracker.CompactTransitions(transitions)
	currentStatus := tracker.StatusSummary(issue)
	// Ищем transition, у которого id/key/display/to совпал с --target (с алиасами)
	match := tracker.MatchTransition(transitions, target)
	if match == nil {
		// Ничего не подходит - отдаём список, exit 2 (как в питоне)
		payload := map[string]any{
			"status":         "NO_MATCH",
			"issue_key":      issueKey,
			"current_status": currentStatus,
			"target":         target,
			"transitions":    compact,
		}
		return printJSON(payload, 2)
	}

	// id в JSON иногда строка, иногда число - приводим к строке для URL
	transitionID, _ := match["id"].(string)
	if transitionID == "" {
		if id := match["id"]; id != nil {
			transitionID = fmt.Sprint(id)
		}
	}
	if transitionID == "" || transitionID == "<nil>" {
		return fmt.Errorf("transition has no id: %#v", match)
	}

	result := map[string]any{
		"status":         "DRY_RUN",
		"issue_key":      issueKey,
		"current_status": currentStatus,
		"target":         target,
		"transition": map[string]any{
			"id":      match["id"],
			"key":     match["key"],
			"display": match["display"],
			"to":      match["to"],
		},
		"all_transitions": compact,
	}

	// Без --send только показываем выбор и выходим
	if !send {
		return printJSON(result, 0)
	}

	// Защита от случайной записи: нужен второй флаг или env
	if !confirmSend && os.Getenv("CONFIRM_SEND") != "YES" {
		return fmt.Errorf("--send requires --confirm-send or CONFIRM_SEND=YES")
	}

	sent, err := client.ExecuteTransition(issueKey, transitionID)
	if err != nil {
		return err
	}
	var sentObj any
	if err := json.Unmarshal(sent, &sentObj); err != nil {
		sentObj = json.RawMessage(sent)
	}
	result["status"] = "POSTED"
	result["result"] = sentObj
	return printJSON(result, 0)
}

// printJSON пишет ответ агенту/человеку в stdout. exitCode≠0 - сразу os.Exit.
func printJSON(payload any, exitCode int) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(payload); err != nil {
		return err
	}
	if exitCode != 0 {
		os.Exit(exitCode)
	}
	return nil
}
