// CLI: комментарий в GitLab MR (бывший post_gitlab_mr_comment.py).
// MR-level note или line discussion.
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
	"github.com/artbeard/mcp-tools/internal/gitlab"
	"github.com/artbeard/mcp-tools/internal/repopath"
)

func main() {
	mrURL := flag.String("mr-url", "", "URL merge request")
	bodyFile := flag.String("body-file", "", "UTF-8 файл с текстом комментария")
	envDir := flag.String("env-dir", "", "каталог с .env / .env.local (по умолчанию корень mcp-tools)")
	lineComment := flag.Bool("line-comment", false, "discussion на строке diff вместо MR note")
	filePath := flag.String("file-path", "", "путь файла в diff (для --line-comment)")
	line := 0
	lineSet := false
	flag.Func("line", "номер строки (для --line-comment)", func(s string) error {
		lineSet = true
		n, err := strconv.Atoi(s)
		if err != nil {
			return fmt.Errorf("--line: %w", err)
		}
		line = n
		return nil
	})
	lineType := flag.String("line-type", "new", "new или old")
	send := flag.Bool("send", false, "реально отправить (без флага - dry-run)")
	confirmSend := flag.Bool("confirm-send", false, "подтверждение вместе с --send")
	flag.Parse()

	if err := run(runArgs{
		mrURL: *mrURL, bodyFile: *bodyFile, envDir: *envDir,
		lineComment: *lineComment, filePath: *filePath,
		line: line, lineSet: lineSet, lineType: *lineType,
		send: *send, confirmSend: *confirmSend,
	}); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
		os.Exit(1)
	}
}

type runArgs struct {
	mrURL, bodyFile, envDir          string
	lineComment                      bool
	filePath                         string
	line                             int
	lineSet                          bool
	lineType                         string
	send, confirmSend                bool
}

func run(a runArgs) error {
	if strings.TrimSpace(a.mrURL) == "" {
		return fmt.Errorf("--mr-url is required")
	}
	if a.bodyFile == "" {
		return fmt.Errorf("--body-file is required")
	}
	rawBody, err := os.ReadFile(a.bodyFile)
	if err != nil {
		return fmt.Errorf("body-file: %w", err)
	}
	body := string(rawBody)
	if strings.TrimSpace(body) == "" {
		return fmt.Errorf("comment body is empty")
	}
	if a.lineType != "new" && a.lineType != "old" {
		return fmt.Errorf("--line-type must be new or old")
	}
	if a.lineComment && (a.filePath == "" || !a.lineSet) {
		return fmt.Errorf("--line-comment requires --file-path and --line")
	}
	if !a.lineComment && (a.filePath != "" || a.lineSet) {
		return fmt.Errorf("--file-path/--line require --line-comment")
	}

	ref, err := gitlab.ParseMRURL(a.mrURL)
	if err != nil {
		return err
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
	client, err := gitlab.NewClient(env, ref.BaseURL)
	if err != nil {
		return err
	}

	if a.lineComment {
		return runLineComment(client, ref, body, a)
	}
	return runMRNote(client, ref, body, a)
}

// Обычный комментарий к MR (notes API).
func runMRNote(client *gitlab.Client, ref gitlab.MRRef, body string, a runArgs) error {
	noteURL := client.NoteURL(ref.ProjectPath, ref.IID)
	if !a.send {
		return printJSON(map[string]any{
			"status":             "DRY_RUN",
			"comment_type":       "mr_note",
			"base_url":           client.BaseURL,
			"project_path":       ref.ProjectPath,
			"merge_request_iid":  ref.IID,
			"body_chars":         len(body),
			"endpoint":           noteURL,
		})
	}
	if !a.confirmSend && os.Getenv("CONFIRM_SEND") != "YES" {
		return fmt.Errorf("--send requires --confirm-send or CONFIRM_SEND=YES")
	}
	result, err := client.PostMRNote(ref.ProjectPath, ref.IID, body)
	if err != nil {
		return err
	}
	return printJSON(map[string]any{
		"status":       "POSTED",
		"comment_type": "mr_note",
		"id":           result["id"],
		"url":          result["url"],
		"created_at":   result["created_at"],
	})
}

// Line discussion: dry-run тоже ходит в API (diff_refs + changes), нужен токен.
func runLineComment(client *gitlab.Client, ref gitlab.MRRef, body string, a runArgs) error {
	mr, err := client.GetMR(ref.ProjectPath, ref.IID)
	if err != nil {
		return err
	}
	baseSHA, startSHA, headSHA, err := gitlab.DiffRefsFromMR(mr)
	if err != nil {
		return err
	}
	changes, err := client.GetMRChanges(ref.ProjectPath, ref.IID)
	if err != nil {
		return err
	}
	lineCheck, err := gitlab.FindDiffLine(changes, a.filePath, a.line, a.lineType)
	if err != nil {
		return err
	}
	position := map[string]any{
		"position_type": "text",
		"base_sha":      baseSHA,
		"start_sha":     startSHA,
		"head_sha":      headSHA,
		"old_path":      a.filePath,
		"new_path":      a.filePath,
	}
	if a.lineType == "new" {
		position["new_line"] = a.line
	} else {
		position["old_line"] = a.line
	}

	discussionURL := client.DiscussionURL(ref.ProjectPath, ref.IID)
	if !a.send {
		return printJSON(map[string]any{
			"status":            "DRY_RUN",
			"comment_type":      "line_discussion",
			"base_url":          client.BaseURL,
			"project_path":      ref.ProjectPath,
			"merge_request_iid": ref.IID,
			"file_path":         a.filePath,
			"line":              a.line,
			"line_type":         a.lineType,
			"body_chars":        len(body),
			"endpoint":          discussionURL,
			"diff_refs_present": true,
			"line_check":        lineCheck,
			"position":          position,
		})
	}
	if !a.confirmSend && os.Getenv("CONFIRM_SEND") != "YES" {
		return fmt.Errorf("--send requires --confirm-send or CONFIRM_SEND=YES")
	}
	result, err := client.PostLineDiscussion(ref.ProjectPath, ref.IID, body, position)
	if err != nil {
		return err
	}
	var firstNote map[string]any
	if notes, ok := result["notes"].([]any); ok && len(notes) > 0 {
		firstNote, _ = notes[0].(map[string]any)
	}
	if firstNote == nil {
		firstNote = map[string]any{}
	}
	return printJSON(map[string]any{
		"status":         "POSTED",
		"comment_type":   "line_discussion",
		"discussion_id":  result["id"],
		"id":             firstNote["id"],
		"url":            firstNote["url"],
		"created_at":     firstNote["created_at"],
	})
}

func printJSON(payload any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	return enc.Encode(payload)
}
