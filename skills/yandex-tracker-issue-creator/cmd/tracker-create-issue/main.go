// CLI: создание задачи в Yandex Tracker (бывший create_tracker_issue.py).
// По умолчанию dry-run. Реально: --send + --confirm-send (или CONFIRM_SEND=YES).
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/artbeard/mcp-tools/internal/envfile"
	"github.com/artbeard/mcp-tools/internal/repopath"
	"github.com/artbeard/mcp-tools/internal/tracker"
)

type stringList []string

func (s *stringList) String() string { return strings.Join(*s, ",") }
func (s *stringList) Set(v string) error {
	*s = append(*s, v)
	return nil
}

func main() {
	queue := flag.String("queue", "", "ключ очереди (например IT)")
	summary := flag.String("summary", "", "заголовок задачи")
	descFile := flag.String("description-file", "", "UTF-8 файл с описанием")
	envDir := flag.String("env-dir", "", "каталог с .env / .env.local (по умолчанию корень mcp-tools)")
	project := flag.String("project", "", "значение проекта")
	projectField := flag.String("project-field", "", "ключ поля проекта")
	period := flag.String("period", "", "период")
	periodField := flag.String("period-field", "", "ключ поля периода")
	spRaw := flag.String("sp", "", "story points (число)")
	spField := flag.String("sp-field", "", "ключ поля story points")
	assignee := flag.String("assignee", "", "исполнитель: login / login:… / id:…")
	send := flag.Bool("send", false, "реально создать (без флага - dry-run)")
	confirmSend := flag.Bool("confirm-send", false, "подтверждение создания (вместе с --send)")
	var tags, fields stringList
	flag.Var(&tags, "tag", "тег (повторяемый; можно через запятую)")
	flag.Var(&fields, "field", "лишнее поле KEY=VALUE (VALUE может быть JSON)")
	flag.Parse()

	if err := run(runArgs{
		queue: *queue, summary: *summary, descFile: *descFile, envDir: *envDir,
		project: *project, projectField: *projectField,
		period: *period, periodField: *periodField,
		spRaw: *spRaw, spField: *spField, assignee: *assignee,
		tags: tags, fields: fields,
		send: *send, confirmSend: *confirmSend,
	}); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
		os.Exit(1)
	}
}

type runArgs struct {
	queue, summary, descFile, envDir       string
	project, projectField                  string
	period, periodField                    string
	spRaw, spField, assignee               string
	tags, fields                           []string
	send, confirmSend                      bool
}

func run(a runArgs) error {
	if strings.TrimSpace(a.queue) == "" {
		return fmt.Errorf("--queue is required")
	}
	if strings.TrimSpace(a.summary) == "" {
		return fmt.Errorf("--summary is required")
	}
	if a.descFile == "" {
		return fmt.Errorf("--description-file is required")
	}
	rawDesc, err := os.ReadFile(a.descFile)
	if err != nil {
		return fmt.Errorf("description-file: %w", err)
	}

	var sp *float64
	if strings.TrimSpace(a.spRaw) != "" {
		v, err := strconv.ParseFloat(strings.TrimSpace(a.spRaw), 64)
		if err != nil {
			return fmt.Errorf("--sp: %w", err)
		}
		sp = &v
	}

	dir := a.envDir
	if dir == "" {
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

	payload, err := tracker.BuildCreateIssuePayload(tracker.CreateIssueFields{
		Queue:        a.queue,
		Summary:      a.summary,
		Description:  string(rawDesc),
		Project:      a.project,
		ProjectField: a.projectField,
		Tags:         a.tags,
		Period:       a.period,
		PeriodField:  a.periodField,
		SP:           sp,
		SPField:      a.spField,
		Assignee:     a.assignee,
		ExtraFields:  a.fields,
		Env:          env,
	})
	if err != nil {
		return err
	}

	orgName, _ := client.OrgHeader()
	if !a.send {
		return printJSON(map[string]any{
			"status":     "DRY_RUN",
			"base_url":   client.BaseURL,
			"endpoint":   client.IssuesURL(),
			"org_header": orgName,
			"payload":    payload,
		}, 0)
	}

	if !a.confirmSend && os.Getenv("CONFIRM_SEND") != "YES" {
		return fmt.Errorf("--send requires --confirm-send or CONFIRM_SEND=YES")
	}

	result, err := client.CreateIssue(payload)
	if err != nil {
		return err
	}
	key := result["key"]
	if key == nil {
		key = result["id"]
	}
	return printJSON(map[string]any{
		"status":  "CREATED",
		"key":     key,
		"self":    result["self"],
		"display": result["display"],
	}, 0)
}

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
