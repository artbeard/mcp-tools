// CLI: JSON-контекст GitLab MR для ревьюера (metadata, diffs, commits, discussions/notes).
// Не делает суждений - только факты из API.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/artbeard/mcp-tools/internal/envfile"
	"github.com/artbeard/mcp-tools/internal/gitlab"
	"github.com/artbeard/mcp-tools/internal/repopath"
)

func main() {
	mrURL := flag.String("mr-url", "", "URL merge request")
	envDir := flag.String("env-dir", "", "каталог с .env / .env.local (по умолчанию корень mcp-tools)")
	authorFilter := flag.String("author-filter", "", "CSV username/name/email/id; пусто = все notes (или GITLAB_MR_AUTHOR_FILTER)")
	parseOnly := flag.Bool("parse-only", false, "только разобрать URL, без API")
	flag.Parse()

	if err := run(*mrURL, *envDir, *authorFilter, *parseOnly); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
		os.Exit(1)
	}
}

func run(mrURL, envDir, authorFilter string, parseOnly bool) error {
	if strings.TrimSpace(mrURL) == "" {
		return fmt.Errorf("--mr-url is required")
	}
	ref, err := gitlab.ParseMRURL(mrURL)
	if err != nil {
		return err
	}
	if parseOnly {
		return printJSON(map[string]any{
			"status":       "PARSED",
			"base_url":     ref.BaseURL,
			"project_path": ref.ProjectPath,
			"iid":          ref.IID,
			"mr_url":       strings.TrimSpace(mrURL),
		})
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
	client, err := gitlab.NewClient(env, ref.BaseURL)
	if err != nil {
		return err
	}

	filterCSV := strings.TrimSpace(authorFilter)
	if filterCSV == "" {
		filterCSV = strings.TrimSpace(env["GITLAB_MR_AUTHOR_FILTER"])
	}
	authors := gitlab.ParseAuthorFilterCSV(filterCSV)

	mr, err := client.GetMR(ref.ProjectPath, ref.IID)
	if err != nil {
		return fmt.Errorf("get MR: %w", err)
	}
	changes, err := client.GetMRChanges(ref.ProjectPath, ref.IID)
	if err != nil {
		return fmt.Errorf("get changes: %w", err)
	}
	commits, err := client.ListMRCommits(ref.ProjectPath, ref.IID)
	if err != nil {
		return fmt.Errorf("list commits: %w", err)
	}
	discussions, err := client.ListMRDiscussions(ref.ProjectPath, ref.IID)
	if err != nil {
		return fmt.Errorf("list discussions: %w", err)
	}
	notes, err := client.ListMRNotes(ref.ProjectPath, ref.IID)
	if err != nil {
		return fmt.Errorf("list notes: %w", err)
	}
	filteredNotes := gitlab.FilterNotesByAuthors(notes, authors)

	return printJSON(map[string]any{
		"status": "OK",
		"ref": map[string]any{
			"base_url":     ref.BaseURL,
			"project_path": ref.ProjectPath,
			"iid":          ref.IID,
			"mr_url":       strings.TrimSpace(mrURL),
		},
		"merge_request": mr,
		"changes":       changes,
		"commits":       commits,
		"discussions":   discussions,
		"notes":         filteredNotes,
		"notes_meta": map[string]any{
			"total":           len(notes),
			"returned":        len(filteredNotes),
			"author_filter":   authors,
			"filter_applied":  len(authors) > 0,
		},
	})
}

func printJSON(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}
