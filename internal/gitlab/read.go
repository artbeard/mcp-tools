package gitlab

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// doRaw - HTTP с телом и заголовками (нужны X-Next-Page / X-Total-Pages).
func (c *Client) doRaw(method, rawURL string, body []byte, contentType string) ([]byte, http.Header, error) {
	if err := c.requireToken(); err != nil {
		return nil, nil, err
	}
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequest(method, rawURL, reader)
	if err != nil {
		return nil, nil, err
	}
	req.Header.Set("PRIVATE-TOKEN", c.Token)
	req.Header.Set("Accept", "application/json")
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, err
	}
	if resp.StatusCode >= 400 {
		return nil, resp.Header, fmt.Errorf("GitLab API HTTP %d: %s", resp.StatusCode, string(data))
	}
	return data, resp.Header, nil
}

// withPage добавляет/заменяет page и per_page в URL.
func withPage(rawURL string, page, perPage int) (string, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", err
	}
	q := u.Query()
	q.Set("page", strconv.Itoa(page))
	q.Set("per_page", strconv.Itoa(perPage))
	u.RawQuery = q.Encode()
	return u.String(), nil
}

// hasNextPage - GitLab X-Next-Page или X-Page < X-Total-Pages.
func hasNextPage(h http.Header, currentPage int) bool {
	if next := strings.TrimSpace(h.Get("X-Next-Page")); next != "" {
		return true
	}
	total := strings.TrimSpace(h.Get("X-Total-Pages"))
	if total == "" {
		return false
	}
	n, err := strconv.Atoi(total)
	if err != nil {
		return false
	}
	return currentPage < n
}

// listAllPages GET массив JSON с пагинацией (до maxPages).
func (c *Client) listAllPages(baseURL string, perPage, maxPages int) ([]map[string]any, error) {
	if perPage <= 0 {
		perPage = 100
	}
	if maxPages <= 0 {
		maxPages = 50
	}
	var all []map[string]any
	for page := 1; page <= maxPages; page++ {
		pageURL, err := withPage(baseURL, page, perPage)
		if err != nil {
			return nil, err
		}
		data, hdr, err := c.doRaw(http.MethodGet, pageURL, nil, "")
		if err != nil {
			return nil, err
		}
		trimmed := bytes.TrimSpace(data)
		if len(trimmed) == 0 || string(trimmed) == "null" {
			break
		}
		var batch []map[string]any
		if err := json.Unmarshal(trimmed, &batch); err != nil {
			return nil, fmt.Errorf("unexpected GitLab list response: %w", err)
		}
		if len(batch) == 0 {
			break
		}
		all = append(all, batch...)
		if !hasNextPage(hdr, page) {
			break
		}
	}
	return all, nil
}

// ListMRDiscussions - GET /merge_requests/:iid/discussions (все страницы).
func (c *Client) ListMRDiscussions(projectPath string, iid int) ([]map[string]any, error) {
	u := fmt.Sprintf("%s/merge_requests/%d/discussions", c.projectAPI(projectPath), iid)
	return c.listAllPages(u, 100, 50)
}

// ListMRNotes - GET /merge_requests/:iid/notes (все страницы).
func (c *Client) ListMRNotes(projectPath string, iid int) ([]map[string]any, error) {
	u := fmt.Sprintf("%s/merge_requests/%d/notes", c.projectAPI(projectPath), iid)
	return c.listAllPages(u, 100, 50)
}

// ListMRCommits - GET /merge_requests/:iid/commits (все страницы).
func (c *Client) ListMRCommits(projectPath string, iid int) ([]map[string]any, error) {
	u := fmt.Sprintf("%s/merge_requests/%d/commits", c.projectAPI(projectPath), iid)
	return c.listAllPages(u, 100, 50)
}

// FilterNotesByAuthors оставляет notes, у которых username/name/email/id совпали с фильтром.
// Пустой filter → копия без фильтрации.
func FilterNotesByAuthors(notes []map[string]any, filter []string) []map[string]any {
	if len(filter) == 0 {
		out := make([]map[string]any, len(notes))
		copy(out, notes)
		return out
	}
	want := map[string]struct{}{}
	for _, f := range filter {
		f = strings.ToLower(strings.TrimSpace(f))
		if f != "" {
			want[f] = struct{}{}
		}
	}
	if len(want) == 0 {
		out := make([]map[string]any, len(notes))
		copy(out, notes)
		return out
	}
	var out []map[string]any
	for _, n := range notes {
		if noteMatchesAuthorFilter(n, want) {
			out = append(out, n)
		}
	}
	return out
}

func noteMatchesAuthorFilter(note map[string]any, want map[string]struct{}) bool {
	author, _ := note["author"].(map[string]any)
	if author == nil {
		return false
	}
	candidates := []string{
		fmt.Sprint(author["username"]),
		fmt.Sprint(author["name"]),
		fmt.Sprint(author["email"]),
		fmt.Sprint(author["id"]),
	}
	for _, c := range candidates {
		c = strings.ToLower(strings.TrimSpace(c))
		if c == "" || c == "<nil>" {
			continue
		}
		if _, ok := want[c]; ok {
			return true
		}
	}
	return false
}

// ParseAuthorFilterCSV - "alice,bob,5" → []string (trim, drop empty).
func ParseAuthorFilterCSV(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	var out []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
