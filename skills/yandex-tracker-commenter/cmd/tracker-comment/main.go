// CLI: комментарий к задаче Yandex Tracker (бывший post_tracker_comment.py).
// По умолчанию dry-run. Реальная отправка: --send + --confirm-send (или CONFIRM_SEND=YES).
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
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
	issueKey := flag.String("issue-key", "", "ключ задачи (например IT-123)")
	bodyFile := flag.String("body-file", "", "UTF-8 файл с текстом комментария")
	markupType := flag.String("markup-type", "", "markupType Tracker (только md)")
	envDir := flag.String("env-dir", "", "каталог с .env / .env.local (по умолчанию корень mcp-tools)")
	replyTo := flag.String("reply-to-comment", "", "numeric comment id или 24-char longId")
	send := flag.Bool("send", false, "реально отправить (без флага — dry-run)")
	confirmSend := flag.Bool("confirm-send", false, "подтверждение отправки (вместе с --send)")
	var summonees stringList
	flag.Var(&summonees, "summonee", "кого призвать (повторяемый); login:… / id:… / …")
	flag.Parse()

	if err := run(*issueKey, *bodyFile, *markupType, *envDir, *replyTo, summonees, *send, *confirmSend); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
		os.Exit(1)
	}
}

func run(
	issueKey, bodyFile, markupType, envDir, replyTo string,
	summoneeArgs []string,
	send, confirmSend bool,
) error {
	issueKey = strings.ToUpper(strings.TrimSpace(issueKey))
	if issueKey == "" {
		return fmt.Errorf("--issue-key is required")
	}
	if !tracker.ValidIssueKey(issueKey) {
		return fmt.Errorf("unsupported issue key format")
	}
	if bodyFile == "" {
		return fmt.Errorf("--body-file is required")
	}
	rawBody, err := os.ReadFile(bodyFile)
	if err != nil {
		return fmt.Errorf("body-file: %w", err)
	}
	body := string(rawBody)
	if strings.TrimSpace(body) == "" {
		return fmt.Errorf("comment body is empty")
	}
	if markupType != "" && markupType != "md" {
		return fmt.Errorf("--markup-type supports only md")
	}

	dir := envDir
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

	parsed := make([]any, 0, len(summoneeArgs))
	for _, s := range summoneeArgs {
		item, err := tracker.ParseSummonee(s)
		if err != nil {
			return err
		}
		parsed = append(parsed, item)
	}

	var replyLongID string
	if strings.TrimSpace(replyTo) != "" {
		replyLongID, err = client.ResolveReplyTo(issueKey, replyTo)
		if err != nil {
			return err
		}
	}

	orgName, _ := client.OrgHeader()
	endpoint := client.CommentsURL(issueKey)

	if !send {
		return printJSON(map[string]any{
			"status":      "DRY_RUN",
			"base_url":    client.BaseURL,
			"issue_key":   issueKey,
			"body_chars":  len(body),
			"markup_type": nilIfEmpty(markupType),
			"summonees":   parsed,
			"reply_to":    nilIfEmpty(replyLongID),
			"endpoint":    endpoint,
			"org_header":  orgName,
		}, 0)
	}

	if !confirmSend && os.Getenv("CONFIRM_SEND") != "YES" {
		return fmt.Errorf("--send requires --confirm-send or CONFIRM_SEND=YES")
	}

	payload := map[string]any{"text": body}
	if markupType != "" {
		payload["markupType"] = markupType
	}
	if len(parsed) > 0 {
		payload["summonees"] = parsed
	}
	if replyLongID != "" {
		payload["replyTo"] = []string{replyLongID}
	}

	result, err := client.PostComment(issueKey, payload)
	if err != nil {
		return err
	}
	commentID := result["id"]
	if commentID == nil {
		commentID = result["self"]
	}
	created := result["createdAt"]
	if created == nil {
		created = result["created_at"]
	}
	return printJSON(map[string]any{
		"status":     "POSTED",
		"issue_key":  issueKey,
		"id":         commentID,
		"self":       result["self"],
		"reply_to":   nilIfEmpty(replyLongID),
		"summonees":  result["summonees"],
		"created_at": created,
	}, 0)
}

func nilIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
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
