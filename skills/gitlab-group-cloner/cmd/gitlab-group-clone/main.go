package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"github.com/artbeard/mcp-tools/internal/envfile"
	"github.com/artbeard/mcp-tools/internal/repopath"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type project struct {
	ID                int    `json:"id"`
	PathWithNamespace string `json:"path_with_namespace"`
	HTTPURLToRepo     string `json:"http_url_to_repo"`
	SSHURLToRepo      string `json:"ssh_url_to_repo"`
}

func main() {
	mode := flag.String("mode", "list", "list или clone")
	protocol := flag.String("protocol", "https", "https или ssh")
	group := flag.String("group", "", "путь группы, например etp/replicas")
	output := flag.String("output-dir", "", "каталог клонов")
	envDir := flag.String("env-dir", "", "каталог с .env/.env.local")
	flag.Parse()
	if err := run(*mode, *protocol, *group, *output, *envDir); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
		os.Exit(1)
	}
}

func run(mode, protocol, group, output, envDir string) error {
	if mode != "list" && mode != "clone" {
		return fmt.Errorf("--mode must be list or clone")
	}
	if protocol != "https" && protocol != "ssh" {
		return fmt.Errorf("--protocol must be https or ssh")
	}
	group = strings.Trim(group, "/ ")
	if group == "" {
		return fmt.Errorf("--group is required")
	}
	var err error
	if envDir == "" {
		envDir, err = repopath.EnvDir()
		if err != nil {
			return err
		}
	}
	env, err := envfile.LoadDir(envDir)
	if err != nil {
		return err
	}
	base := strings.TrimRight(strings.TrimSpace(env["GITLAB_BASE_URL"]), "/")
	user := strings.TrimSpace(env["GITLAB_USERNAME"])
	token := strings.TrimSpace(env["GITLAB_TOKEN"])
	if base == "" || token == "" {
		return fmt.Errorf("GITLAB_BASE_URL и GITLAB_TOKEN должны быть заполнены")
	}
	if protocol == "https" && user == "" {
		return fmt.Errorf("GITLAB_USERNAME должен быть заполнен для HTTPS")
	}
	projects, err := listProjects(base, token, group)
	if err != nil {
		return err
	}
	if mode == "list" {
		return printJSON(map[string]any{"status": "LISTED", "group": group, "count": len(projects), "projects": projects})
	}
	if output == "" {
		return fmt.Errorf("--output-dir is required for clone")
	}
	if err := os.MkdirAll(output, 0755); err != nil {
		return err
	}
	results := make([]map[string]any, 0, len(projects))
	for _, p := range projects {
		dir := filepath.Join(output, filepath.FromSlash(p.PathWithNamespace))
		result := map[string]any{"project": p.PathWithNamespace, "path": dir}
		if info, e := os.Stat(filepath.Join(dir, ".git")); e == nil && info.IsDir() {
			result["status"] = "SKIPPED_EXISTS"
			results = append(results, result)
			continue
		}
		if _, e := os.Stat(dir); e == nil {
			result["status"] = "SKIPPED_NON_GIT_DIRECTORY"
			results = append(results, result)
			continue
		}
		repo := p.HTTPURLToRepo
		if protocol == "ssh" {
			repo = p.SSHURLToRepo
		}
		if repo == "" {
			result["status"] = "ERROR"
			result["error"] = fmt.Sprintf("GitLab не вернул %s URL репозитория", strings.ToUpper(protocol))
		} else if e := clone(repo, protocol, user, token, dir); e != nil {
			result["status"] = "ERROR"
			result["error"] = e.Error()
		} else {
			result["status"] = "CLONED"
		}
		results = append(results, result)
	}
	return printJSON(map[string]any{"status": "DONE", "group": group, "protocol": protocol, "output_dir": output, "count": len(results), "projects": results})
}

func listProjects(base, token, group string) ([]project, error) {
	u := fmt.Sprintf("%s/api/v4/groups/%s/projects?include_subgroups=true&with_shared=false&per_page=100", base, url.PathEscape(group))
	var all []project
	for page := 1; ; page++ {
		req, e := http.NewRequest(http.MethodGet, u+fmt.Sprintf("&page=%d", page), nil)
		if e != nil {
			return nil, e
		}
		req.Header.Set("PRIVATE-TOKEN", token)
		resp, e := http.DefaultClient.Do(req)
		if e != nil {
			return nil, e
		}
		body, re := io.ReadAll(resp.Body)
		resp.Body.Close()
		if re != nil {
			return nil, re
		}
		if resp.StatusCode >= 400 {
			return nil, fmt.Errorf("GitLab API HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
		}
		var batch []project
		if e = json.Unmarshal(body, &batch); e != nil {
			return nil, e
		}
		all = append(all, batch...)
		if len(batch) == 0 || resp.Header.Get("X-Next-Page") == "" {
			break
		}
	}
	return all, nil
}

func clone(repo, protocol, username, token, destination string) error {
	if protocol == "ssh" {
		cmd := exec.Command("git", "clone", repo, destination)
		out, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("git clone: %s", strings.TrimSpace(string(out)))
		}
		return nil
	}
	f, e := os.CreateTemp("", "mcp-gitlab-askpass-*")
	if e != nil {
		return e
	}
	path := f.Name()
	defer os.Remove(path)
	if _, e = f.WriteString("#!/bin/sh\ncase \"$1\" in\n*Username*) printf '%s' \"$GITLAB_USERNAME\";;\n*) printf '%s' \"$GITLAB_TOKEN\";;\nesac\n"); e != nil {
		f.Close()
		return e
	}
	if e = f.Close(); e != nil {
		return e
	}
	if e = os.Chmod(path, 0700); e != nil {
		return e
	}
	cmd := exec.Command("git", "clone", repo, destination)
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0", "GIT_ASKPASS="+path, "GITLAB_USERNAME="+username, "GITLAB_TOKEN="+token)
	out, e := cmd.CombinedOutput()
	if e != nil {
		return fmt.Errorf("git clone: %s", strings.TrimSpace(string(out)))
	}
	return nil
}
func printJSON(v any) error {
	e := json.NewEncoder(os.Stdout)
	e.SetEscapeHTML(false)
	e.SetIndent("", "  ")
	return e.Encode(v)
}
