// CLI поиска/чтения правил кодстайла (skills/codestyle/content).
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/artbeard/mcp-tools/internal/codestyle"
	"github.com/artbeard/mcp-tools/internal/repopath"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	switch os.Args[1] {
	case "find-rules":
		os.Exit(runFind(os.Args[2:]))
	case "read-rule":
		os.Exit(runRead(os.Args[2:]))
	case "list":
		os.Exit(runList(os.Args[2:]))
	case "-h", "--help", "help":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "ОШИБКА: неизвестная команда %q\n", os.Args[1])
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Fprintf(os.Stderr, `usage:
  codestyle find-rules --task "..." [--max-results N] [--content-dir DIR]
  codestyle read-rule  --uri codestyle://backend/dto [--content-dir DIR]
  codestyle list [--content-dir DIR]
`)
}

func contentDir(flagDir string) (codestyle.ContentDir, error) {
	if flagDir != "" {
		return codestyle.ContentDir(flagDir), nil
	}
	root, err := repopath.Root()
	if err != nil {
		return "", err
	}
	return codestyle.DefaultContentDir(root), nil
}

func runFind(args []string) int {
	fs := flag.NewFlagSet("find-rules", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	task := fs.String("task", "", "описание задачи")
	max := fs.Int("max-results", 8, "макс. результатов 1..15")
	dir := fs.String("content-dir", "", "каталог content")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	c, err := contentDir(*dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ОШИБКА: %v\n", err)
		return 1
	}
	meta, err := c.LoadMeta()
	if err != nil {
		fmt.Fprintf(os.Stderr, "ОШИБКА: %v\n", err)
		return 1
	}
	out, err := codestyle.FindRules(meta, codestyle.FindRulesInput{Task: *task, MaxResults: *max})
	if err != nil {
		fmt.Fprintf(os.Stderr, "ОШИБКА: %v\n", err)
		return 1
	}
	return writeJSON(out)
}

func runRead(args []string) int {
	fs := flag.NewFlagSet("read-rule", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	uri := fs.String("uri", "", "codestyle://...")
	dir := fs.String("content-dir", "", "каталог content")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	c, err := contentDir(*dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ОШИБКА: %v\n", err)
		return 1
	}
	out, err := c.ReadRule(*uri)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ОШИБКА: %v\n", err)
		return 1
	}
	return writeJSON(out)
}

func runList(args []string) int {
	fs := flag.NewFlagSet("list", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	dir := fs.String("content-dir", "", "каталог content")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	c, err := contentDir(*dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ОШИБКА: %v\n", err)
		return 1
	}
	meta, err := c.LoadMeta()
	if err != nil {
		fmt.Fprintf(os.Stderr, "ОШИБКА: %v\n", err)
		return 1
	}
	return writeJSON(codestyle.ListEntries(meta))
}

func writeJSON(v any) int {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		fmt.Fprintf(os.Stderr, "ОШИБКА: json: %v\n", err)
		return 1
	}
	return 0
}
