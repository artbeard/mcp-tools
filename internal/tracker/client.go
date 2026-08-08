// Package tracker — тонкий HTTP-клиент Yandex Tracker API (issues / transitions).
// Общий код для будущих Go-тулов: comment, create-issue, timekeeper.
package tracker

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Client держит креды и базовый URL API.
type Client struct {
	BaseURL    string
	Token      string
	OrgID      string
	CloudOrgID string
	HTTP       *http.Client
}

// NewClient собирает клиента из map env (как после envfile.LoadDir).
// Токен может быть пустым для dry-run без API; do() потребует его при запросе.
func NewClient(env map[string]string) (*Client, error) {
	token := strings.TrimSpace(env["TRACKER_OAUTH_TOKEN"])
	orgID := strings.TrimSpace(env["TRACKER_ORG_ID"])
	cloudOrgID := strings.TrimSpace(env["TRACKER_CLOUD_ORG_ID"])
	if orgID == "" && cloudOrgID == "" {
		return nil, fmt.Errorf("TRACKER_ORG_ID or TRACKER_CLOUD_ORG_ID is required")
	}
	base := strings.TrimRight(env["TRACKER_BASE_URL"], "/")
	if base == "" {
		base = "https://api.tracker.yandex.net"
	}
	return &Client{
		BaseURL:    base,
		Token:      token,
		OrgID:      orgID,
		CloudOrgID: cloudOrgID,
		HTTP:       &http.Client{Timeout: 30 * time.Second},
	}, nil
}

// do — общий GET/POST с OAuth и Org-заголовками. Тело ответа — сырой JSON.
func (c *Client) do(method, url string, payload any) (json.RawMessage, error) {
	if strings.TrimSpace(c.Token) == "" {
		return nil, fmt.Errorf("TRACKER_OAUTH_TOKEN is empty")
	}
	var body io.Reader
	if payload != nil {
		raw, err := json.Marshal(payload)
		if err != nil {
			return nil, err
		}
		body = bytes.NewReader(raw)
	}
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "OAuth "+c.Token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	// Cloud-org приоритетнее (как в Python-клиенте), иначе обычный org
	if c.CloudOrgID != "" {
		req.Header.Set("X-Cloud-Org-ID", c.CloudOrgID)
	} else {
		req.Header.Set("X-Org-ID", c.OrgID)
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
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(data))
	}
	if len(bytes.TrimSpace(data)) == 0 {
		return json.RawMessage("{}"), nil
	}
	return json.RawMessage(data), nil
}

// GetIssue — GET /v2/issues/{key}
func (c *Client) GetIssue(issueKey string) (map[string]any, error) {
	raw, err := c.do(http.MethodGet, fmt.Sprintf("%s/v2/issues/%s", c.BaseURL, issueKey), nil)
	if err != nil {
		return nil, err
	}
	var issue map[string]any
	if err := json.Unmarshal(raw, &issue); err != nil {
		return nil, fmt.Errorf("unexpected issue response: %w", err)
	}
	return issue, nil
}

// ListTransitions — доступные переходы из текущего статуса задачи
func (c *Client) ListTransitions(issueKey string) ([]map[string]any, error) {
	raw, err := c.do(http.MethodGet, fmt.Sprintf("%s/v2/issues/%s/transitions", c.BaseURL, issueKey), nil)
	if err != nil {
		return nil, err
	}
	var transitions []map[string]any
	if err := json.Unmarshal(raw, &transitions); err != nil {
		return nil, fmt.Errorf("unexpected transitions response: %w", err)
	}
	return transitions, nil
}

// ExecuteTransition — POST …/transitions/{id}/_execute (реальная смена статуса)
func (c *Client) ExecuteTransition(issueKey, transitionID string) (json.RawMessage, error) {
	url := fmt.Sprintf("%s/v2/issues/%s/transitions/%s/_execute", c.BaseURL, issueKey, transitionID)
	return c.do(http.MethodPost, url, map[string]any{})
}

// StatusSummary достаёт из issue короткий объект статуса для ответа CLI.
func StatusSummary(issue map[string]any) any {
	status, ok := issue["status"]
	if !ok {
		return nil
	}
	obj, ok := status.(map[string]any)
	if !ok {
		return status
	}
	return map[string]any{
		"id":      obj["id"],
		"key":     obj["key"],
		"display": obj["display"],
	}
}

// CompactTransitions — урезанный список для JSON-ответа (без лишних полей API).
func CompactTransitions(transitions []map[string]any) []map[string]any {
	out := make([]map[string]any, 0, len(transitions))
	for _, item := range transitions {
		out = append(out, map[string]any{
			"id":      item["id"],
			"key":     item["key"],
			"display": item["display"],
			"to":      item["to"],
		})
	}
	return out
}

// normalizedValues — все «имена» transition в нижнем регистре для сравнения с --target.
func normalizedValues(item map[string]any) map[string]struct{} {
	values := map[string]struct{}{}
	add := func(v any) {
		if v == nil {
			return
		}
		s := strings.TrimSpace(strings.ToLower(fmt.Sprint(v)))
		if s != "" && s != "<nil>" {
			values[s] = struct{}{}
		}
	}
	add(item["id"])
	add(item["key"])
	add(item["display"])
	add(item["name"])
	if to, ok := item["to"].(map[string]any); ok {
		add(to["id"])
		add(to["key"])
		add(to["display"])
	}
	return values
}

// TargetAliases — синонимы статуса («открыто» ≈ open, «в работе» ≈ inProgress).
func TargetAliases(target string) map[string]struct{} {
	target = strings.TrimSpace(strings.ToLower(target))
	aliases := map[string]struct{}{target: {}}
	switch target {
	case "открыто", "открыт", "open", "opened":
		for _, a := range []string{"открыто", "открыт", "open", "opened"} {
			aliases[a] = struct{}{}
		}
	case "в работе", "inprogress", "in_progress", "in progress":
		for _, a := range []string{"в работе", "inprogress", "in_progress", "in progress"} {
			aliases[a] = struct{}{}
		}
	}
	return aliases
}

// MatchTransition ищет первый transition, пересекающийся с алиасами --target.
func MatchTransition(transitions []map[string]any, target string) map[string]any {
	aliases := TargetAliases(target)
	for _, item := range transitions {
		for value := range normalizedValues(item) {
			if _, ok := aliases[value]; ok {
				return item
			}
		}
	}
	return nil
}
