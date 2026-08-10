// Package gitlab - HTTP-клиент GitLab API (MR notes / create / discussions).
package gitlab

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Web-URL MR: https://host/group/proj/-/merge_requests/42?...
// Группы: base (хост), project (путь), iid (номер MR).
var mrURLRe = regexp.MustCompile(
	`^(?P<base>https?://[^/]+)/(?P<project>.+?)/-/merge_requests/(?P<iid>\d+)(?:[/?#].*)?$`,
)

// Заголовок hunk в unified diff: @@ -old[,len] +new[,len] @@
// Нужен FindDiffLine, чтобы считать номера old/new строк.
var hunkRe = regexp.MustCompile(`^@@ -(?P<old>\d+)(?:,\d+)? \+(?P<new>\d+)(?:,\d+)? @@`)

// MRRef - разобранный URL merge request.
type MRRef struct {
	BaseURL     string // https://gitlab.example.com (из URL)
	ProjectPath string // group/subgroup/project
	IID         int
}

// ParseMRURL разбирает web-URL MR.
func ParseMRURL(mrURL string) (MRRef, error) {
	m := mrURLRe.FindStringSubmatch(strings.TrimSpace(mrURL))
	if m == nil {
		return MRRef{}, fmt.Errorf("unsupported MR URL format")
	}
	names := mrURLRe.SubexpNames()
	var base, project, iidStr string
	for i, name := range names {
		switch name {
		case "base":
			base = m[i]
		case "project":
			project = m[i]
		case "iid":
			iidStr = m[i]
		}
	}
	projectPath, err := url.PathUnescape(project)
	if err != nil {
		projectPath = project
	}
	iid, err := strconv.Atoi(iidStr)
	if err != nil {
		return MRRef{}, fmt.Errorf("unsupported MR URL format")
	}
	return MRRef{
		BaseURL:     strings.TrimRight(base, "/"),
		ProjectPath: projectPath,
		IID:         iid,
	}, nil
}

// Client - GitLab API с PRIVATE-TOKEN.
type Client struct {
	BaseURL string
	Token   string
	HTTP    *http.Client
}

// NewClient из map после envfile.LoadDir.
// baseURLFallback - хост из MR URL, если GITLAB_BASE_URL пуст.
func NewClient(env map[string]string, baseURLFallback string) (*Client, error) {
	base := strings.TrimRight(strings.TrimSpace(env["GITLAB_BASE_URL"]), "/")
	if base == "" {
		base = strings.TrimRight(baseURLFallback, "/")
	}
	if base == "" {
		return nil, fmt.Errorf("GITLAB_BASE_URL is empty")
	}
	return &Client{
		BaseURL: base,
		Token:   strings.TrimSpace(env["GITLAB_TOKEN"]),
		HTTP:    &http.Client{Timeout: 30 * time.Second},
	}, nil
}

func (c *Client) requireToken() error {
	if strings.TrimSpace(c.Token) == "" {
		return fmt.Errorf("GITLAB_TOKEN is empty")
	}
	return nil
}

func (c *Client) projectAPI(projectPath string) string {
	return fmt.Sprintf("%s/api/v4/projects/%s", c.BaseURL, url.PathEscape(projectPath))
}

// NoteURL - POST form note к MR.
func (c *Client) NoteURL(projectPath string, iid int) string {
	return fmt.Sprintf("%s/merge_requests/%d/notes", c.projectAPI(projectPath), iid)
}

// DiscussionURL - POST JSON discussion к MR.
func (c *Client) DiscussionURL(projectPath string, iid int) string {
	return fmt.Sprintf("%s/merge_requests/%d/discussions", c.projectAPI(projectPath), iid)
}

func (c *Client) do(method, rawURL string, body []byte, contentType string) (json.RawMessage, error) {
	if err := c.requireToken(); err != nil {
		return nil, err
	}
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequest(method, rawURL, reader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("PRIVATE-TOKEN", c.Token)
	req.Header.Set("Accept", "application/json")
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("GitLab API HTTP %d: %s", resp.StatusCode, string(data))
	}
	if len(bytes.TrimSpace(data)) == 0 {
		return json.RawMessage("{}"), nil
	}
	return json.RawMessage(data), nil
}

func (c *Client) getJSON(rawURL string) (map[string]any, error) {
	raw, err := c.do(http.MethodGet, rawURL, nil, "")
	if err != nil {
		return nil, err
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("unexpected GitLab response: %w", err)
	}
	return out, nil
}

// GetMR - GET merge_request (нужны diff_refs для line discussion).
func (c *Client) GetMR(projectPath string, iid int) (map[string]any, error) {
	return c.getJSON(fmt.Sprintf("%s/merge_requests/%d", c.projectAPI(projectPath), iid))
}

// GetMRChanges - GET changes (diff файлов).
func (c *Client) GetMRChanges(projectPath string, iid int) (map[string]any, error) {
	return c.getJSON(fmt.Sprintf("%s/merge_requests/%d/changes", c.projectAPI(projectPath), iid))
}

// PostMRNote - MR-level note (form-urlencoded, как Python).
func (c *Client) PostMRNote(projectPath string, iid int, body string) (map[string]any, error) {
	form := url.Values{"body": {body}}.Encode()
	raw, err := c.do(http.MethodPost, c.NoteURL(projectPath, iid), []byte(form), "application/x-www-form-urlencoded")
	if err != nil {
		return nil, err
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("unexpected note response: %w", err)
	}
	return out, nil
}

// PostLineDiscussion - line discussion (JSON body + position).
func (c *Client) PostLineDiscussion(projectPath string, iid int, body string, position map[string]any) (map[string]any, error) {
	payload, err := json.Marshal(map[string]any{"body": body, "position": position})
	if err != nil {
		return nil, err
	}
	raw, err := c.do(http.MethodPost, c.DiscussionURL(projectPath, iid), payload, "application/json")
	if err != nil {
		return nil, err
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("unexpected discussion response: %w", err)
	}
	return out, nil
}

// DiffLineCheck - результат проверки строки в diff.
type DiffLineCheck struct {
	Line       int    `json:"line"`
	LineType   string `json:"line_type"`
	DiffMarker string `json:"diff_marker"`
}

// FindDiffLine ищет строку в unified diff MR (как Python find_diff_line).
func FindDiffLine(changes map[string]any, filePath string, line int, lineType string) (DiffLineCheck, error) {
	var diffText string
	found := false
	rawChanges, _ := changes["changes"].([]any)
	for _, item := range rawChanges {
		ch, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if fmt.Sprint(ch["new_path"]) == filePath || fmt.Sprint(ch["old_path"]) == filePath {
			diffText = fmt.Sprint(ch["diff"])
			if diffText == "<nil>" {
				diffText = ""
			}
			found = true
			break
		}
	}
	if !found {
		return DiffLineCheck{}, fmt.Errorf("file is missing from MR diff: %s", filePath)
	}

	oldLine, newLine := 0, 0
	for _, rawLine := range strings.Split(diffText, "\n") {
		if hm := hunkRe.FindStringSubmatch(rawLine); hm != nil {
			oldLine, _ = strconv.Atoi(hm[hunkRe.SubexpIndex("old")])
			newLine, _ = strconv.Atoi(hm[hunkRe.SubexpIndex("new")])
			continue
		}
		if strings.HasPrefix(rawLine, `\`) {
			continue
		}
		if rawLine == "" {
			continue
		}
		prefix := rawLine[:1]
		switch prefix {
		case "+":
			if lineType == "new" && newLine == line {
				return DiffLineCheck{Line: line, LineType: lineType, DiffMarker: "added"}, nil
			}
			newLine++
		case "-":
			if lineType == "old" && oldLine == line {
				return DiffLineCheck{Line: line, LineType: lineType, DiffMarker: "removed"}, nil
			}
			oldLine++
		case " ":
			if lineType == "new" && newLine == line {
				return DiffLineCheck{Line: line, LineType: lineType, DiffMarker: "context"}, nil
			}
			if lineType == "old" && oldLine == line {
				return DiffLineCheck{Line: line, LineType: lineType, DiffMarker: "context"}, nil
			}
			oldLine++
			newLine++
		}
	}
	return DiffLineCheck{}, fmt.Errorf("line is missing from MR diff: %s:%d (%s)", filePath, line, lineType)
}

// DiffRefsFromMR достаёт base/start/head sha из MR.
func DiffRefsFromMR(mr map[string]any) (base, start, head string, err error) {
	refs, _ := mr["diff_refs"].(map[string]any)
	if refs == nil {
		return "", "", "", fmt.Errorf("MR diff_refs are missing; cannot create line discussion")
	}
	base = strings.TrimSpace(fmt.Sprint(refs["base_sha"]))
	start = strings.TrimSpace(fmt.Sprint(refs["start_sha"]))
	head = strings.TrimSpace(fmt.Sprint(refs["head_sha"]))
	if base == "" || base == "<nil>" || start == "" || start == "<nil>" || head == "" || head == "<nil>" {
		return "", "", "", fmt.Errorf("MR diff_refs are missing; cannot create line discussion")
	}
	return base, start, head, nil
}

// RemoteRef - host + project path из git remote URL.
type RemoteRef struct {
	Host        string
	ProjectPath string
}

// ParseRemoteURL - git@host:path.git / ssh:// / https:// → host + path без .git.
func ParseRemoteURL(remoteURL string) (RemoteRef, error) {
	raw := strings.TrimSpace(remoteURL)
	var host, path string
	switch {
	case strings.HasPrefix(raw, "git@") && strings.Contains(raw, ":"):
		rest := raw[len("git@"):]
		host, path, _ = strings.Cut(rest, ":")
	case strings.HasPrefix(raw, "ssh://"):
		u, err := url.Parse(raw)
		if err != nil {
			return RemoteRef{}, fmt.Errorf("неподдерживаемый remote URL: %s", remoteURL)
		}
		host = u.Hostname()
		path = strings.TrimPrefix(u.Path, "/")
	default:
		u, err := url.Parse(raw)
		if err != nil {
			return RemoteRef{}, fmt.Errorf("неподдерживаемый remote URL: %s", remoteURL)
		}
		host = u.Hostname()
		path = strings.TrimPrefix(u.Path, "/")
	}
	path = strings.TrimSuffix(path, ".git")
	if host == "" || path == "" {
		return RemoteRef{}, fmt.Errorf("неподдерживаемый remote URL: %s", remoteURL)
	}
	return RemoteRef{Host: host, ProjectPath: path}, nil
}

// MRSummary - iid / url / title после validate.
type MRSummary struct {
	IID   int
	URL   string
	Title any
}

// ValidateMRRecord проверяет iid и web_url path = /{project}/-/merge_requests/{iid}.
func ValidateMRRecord(record map[string]any, projectPath string) (MRSummary, error) {
	if record == nil {
		return MRSummary{}, fmt.Errorf("ответ GitLab MR должен быть объектом")
	}
	iid, ok := asPositiveInt(record["iid"])
	if !ok {
		return MRSummary{}, fmt.Errorf("в ответе GitLab MR нет валидного iid")
	}
	urlRaw, _ := record["web_url"].(string)
	urlRaw = strings.TrimSpace(urlRaw)
	if urlRaw == "" {
		return MRSummary{}, fmt.Errorf("в ответе GitLab MR нет web_url")
	}
	parsed, err := url.Parse(urlRaw)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return MRSummary{}, fmt.Errorf("GitLab MR web_url должен быть абсолютным HTTP(S) URL")
	}
	expected := fmt.Sprintf("/%s/-/merge_requests/%d", projectPath, iid)
	got, err := url.PathUnescape(strings.TrimRight(parsed.Path, "/"))
	if err != nil {
		got = strings.TrimRight(parsed.Path, "/")
	}
	if got != expected {
		return MRSummary{}, fmt.Errorf("GitLab MR web_url не совпадает с project и iid: %s", urlRaw)
	}
	return MRSummary{IID: iid, URL: urlRaw, Title: record["title"]}, nil
}

func asPositiveInt(v any) (int, bool) {
	switch n := v.(type) {
	case int:
		return n, n >= 1
	case int64:
		return int(n), n >= 1
	case float64:
		if n != float64(int(n)) || n < 1 {
			return 0, false
		}
		return int(n), true
	case json.Number:
		i, err := n.Int64()
		return int(i), err == nil && i >= 1
	default:
		return 0, false
	}
}

// FindOpenedMR - первый opened MR с source/target branch.
func (c *Client) FindOpenedMR(projectPath, sourceBranch, targetBranch string) (*MRSummary, error) {
	q := url.Values{
		"state":         {"opened"},
		"source_branch": {sourceBranch},
		"target_branch": {targetBranch},
	}
	rawURL := fmt.Sprintf("%s/merge_requests?%s", c.projectAPI(projectPath), q.Encode())
	raw, err := c.do(http.MethodGet, rawURL, nil, "")
	if err != nil {
		return nil, err
	}
	var list []map[string]any
	if err := json.Unmarshal(raw, &list); err != nil {
		return nil, fmt.Errorf("неожиданный ответ GitLab при проверке existing MR")
	}
	if len(list) == 0 {
		return nil, nil
	}
	sum, err := ValidateMRRecord(list[0], projectPath)
	if err != nil {
		return nil, err
	}
	return &sum, nil
}

// CreateMR - POST merge_request.
func (c *Client) CreateMR(projectPath, sourceBranch, targetBranch, title, description string) (MRSummary, error) {
	payload, err := json.Marshal(map[string]string{
		"source_branch": sourceBranch,
		"target_branch": targetBranch,
		"title":         title,
		"description":   description,
	})
	if err != nil {
		return MRSummary{}, err
	}
	raw, err := c.do(http.MethodPost, c.projectAPI(projectPath)+"/merge_requests", payload, "application/json")
	if err != nil {
		return MRSummary{}, err
	}
	var record map[string]any
	if err := json.Unmarshal(raw, &record); err != nil {
		return MRSummary{}, fmt.Errorf("неожиданный ответ GitLab при создании MR: %w", err)
	}
	return ValidateMRRecord(record, projectPath)
}
