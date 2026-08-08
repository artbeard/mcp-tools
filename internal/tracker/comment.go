package tracker

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

var (
	// Ключ задачи вида IT-123
	issueKeyRe = regexp.MustCompile(`^[A-Z][A-Z0-9]+-\d+$`)
	// longId комментария Tracker: 24 hex-символа
	objectIDRe = regexp.MustCompile(`^[0-9a-f]{24}$`)
)

// ValidIssueKey - формат QUEUE-123.
func ValidIssueKey(key string) bool {
	return issueKeyRe.MatchString(key)
}

// OrgHeader - имя и значение org-заголовка для dry-run JSON.
func (c *Client) OrgHeader() (name, value string) {
	if c.CloudOrgID != "" {
		return "X-Cloud-Org-ID", c.CloudOrgID
	}
	return "X-Org-ID", c.OrgID
}

// CommentsURL - POST endpoint комментариев к задаче.
func (c *Client) CommentsURL(issueKey string) string {
	return fmt.Sprintf("%s/v2/issues/%s/comments", c.BaseURL, url.PathEscape(issueKey))
}

// ParseSummonee разбирает --summonee: login:… / id:… / plain login или digits→id.
func ParseSummonee(value string) (any, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, fmt.Errorf("summonee value is empty")
	}
	if kind, id, ok := strings.Cut(value, ":"); ok {
		kind = strings.TrimSpace(strings.ToLower(kind))
		id = strings.TrimSpace(id)
		if id == "" {
			return nil, fmt.Errorf("summonee identifier is empty")
		}
		switch kind {
		case "login":
			return map[string]any{"login": id}, nil
		case "id":
			return map[string]any{"id": id}, nil
		case "cloud":
			return map[string]any{"cloudUid": id}, nil
		case "uid":
			n, err := strconv.Atoi(id)
			if err != nil {
				return nil, fmt.Errorf("summonee uid: %w", err)
			}
			return map[string]any{"uid": n}, nil
		case "passport":
			n, err := strconv.Atoi(id)
			if err != nil {
				return nil, fmt.Errorf("summonee passport: %w", err)
			}
			return map[string]any{"passportUid": n}, nil
		case "tracker":
			n, err := strconv.Atoi(id)
			if err != nil {
				return nil, fmt.Errorf("summonee tracker: %w", err)
			}
			return map[string]any{"trackerUid": n}, nil
		default:
			return nil, fmt.Errorf("unsupported summonee selector: %s", kind)
		}
	}
	if isAllDigits(value) {
		return map[string]any{"id": value}, nil
	}
	return map[string]any{"login": value}, nil
}

func isAllDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// ResolveReplyTo: 24-char longId как есть; numeric id - GET comment → longId.
func (c *Client) ResolveReplyTo(issueKey, replyTo string) (string, error) {
	replyTo = strings.TrimSpace(replyTo)
	if replyTo == "" {
		return "", fmt.Errorf("reply-to-comment is empty")
	}
	if objectIDRe.MatchString(replyTo) {
		return replyTo, nil
	}
	if !isAllDigits(replyTo) {
		return "", fmt.Errorf("reply-to-comment must be a numeric comment id or 24-char longId")
	}
	raw, err := c.do(
		http.MethodGet,
		fmt.Sprintf("%s/v2/issues/%s/comments/%s", c.BaseURL, url.PathEscape(issueKey), url.PathEscape(replyTo)),
		nil,
	)
	if err != nil {
		return "", err
	}
	var comment map[string]any
	if err := json.Unmarshal(raw, &comment); err != nil {
		return "", fmt.Errorf("unexpected comment response: %w", err)
	}
	longID, _ := comment["longId"].(string)
	if longID == "" {
		longID, _ = comment["long_id"].(string)
	}
	if !objectIDRe.MatchString(longID) {
		return "", fmt.Errorf("Tracker comment longId is missing; cannot create reply")
	}
	return longID, nil
}

// PostComment - POST /v2/issues/{key}/comments.
func (c *Client) PostComment(issueKey string, payload map[string]any) (map[string]any, error) {
	raw, err := c.do(http.MethodPost, c.CommentsURL(issueKey), payload)
	if err != nil {
		return nil, err
	}
	var result map[string]any
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("unexpected comment create response: %w", err)
	}
	return result, nil
}
