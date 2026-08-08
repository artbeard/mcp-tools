package tracker

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
)

// Очередь Tracker: IT, PROJ_1 и т.п.
var queueRe = regexp.MustCompile(`^[A-Z][A-Z0-9_]*$`)

// ValidQueue - формат ключа очереди.
func ValidQueue(queue string) bool {
	return queueRe.MatchString(queue)
}

// IssuesURL - POST /v2/issues
func (c *Client) IssuesURL() string {
	return c.BaseURL + "/v2/issues"
}

// ParseUserSelector - assignee и т.п.: login:… / id:…; без префикса всегда login
// (в отличие от ParseSummonee, где голые цифры → id).
func ParseUserSelector(value string) (any, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, fmt.Errorf("selector value is empty")
	}
	if kind, id, ok := strings.Cut(value, ":"); ok {
		kind = strings.TrimSpace(strings.ToLower(kind))
		id = strings.TrimSpace(id)
		if id == "" {
			return nil, fmt.Errorf("selector identifier is empty")
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
				return nil, fmt.Errorf("selector uid: %w", err)
			}
			return map[string]any{"uid": n}, nil
		case "passport":
			n, err := strconv.Atoi(id)
			if err != nil {
				return nil, fmt.Errorf("selector passport: %w", err)
			}
			return map[string]any{"passportUid": n}, nil
		case "tracker":
			n, err := strconv.Atoi(id)
			if err != nil {
				return nil, fmt.Errorf("selector tracker: %w", err)
			}
			return map[string]any{"trackerUid": n}, nil
		default:
			return nil, fmt.Errorf("unsupported selector kind: %s", kind)
		}
	}
	return map[string]any{"login": value}, nil
}

// ParseTags - повторяемые --tag и значения через запятую, без дублей.
func ParseTags(values []string) []string {
	var tags []string
	seen := map[string]struct{}{}
	for _, raw := range values {
		for _, item := range strings.Split(raw, ",") {
			tag := strings.TrimSpace(item)
			if tag == "" {
				continue
			}
			if _, ok := seen[tag]; ok {
				continue
			}
			seen[tag] = struct{}{}
			tags = append(tags, tag)
		}
	}
	return tags
}

// ParseExtraField - --field KEY=VALUE; VALUE может быть JSON.
func ParseExtraField(raw string) (key string, value any, err error) {
	key, val, ok := strings.Cut(raw, "=")
	if !ok {
		return "", nil, fmt.Errorf("--field must be KEY=VALUE")
	}
	key = strings.TrimSpace(key)
	if key == "" {
		return "", nil, fmt.Errorf("--field key is empty")
	}
	val = strings.TrimSpace(val)
	if val == "" {
		return key, val, nil
	}
	var parsed any
	if err := json.Unmarshal([]byte(val), &parsed); err == nil {
		return key, parsed, nil
	}
	return key, val, nil
}

// CreateIssueFields - опции сборки payload создания задачи.
type CreateIssueFields struct {
	Queue         string
	Summary       string
	Description   string
	Project       string
	ProjectField  string // пусто → из env или "project"
	Tags          []string
	Period        string
	PeriodField   string
	SP            *float64
	SPField       string
	Assignee      string
	ExtraFields   []string // KEY=VALUE
	Env           map[string]string
}

func fieldKey(explicit, envKey, fallback string, env map[string]string) string {
	if strings.TrimSpace(explicit) != "" {
		return strings.TrimSpace(explicit)
	}
	if env != nil {
		if v := strings.TrimSpace(env[envKey]); v != "" {
			return v
		}
	}
	return fallback
}

func setOptional(payload map[string]any, key, value string) {
	if strings.TrimSpace(key) == "" || value == "" {
		return
	}
	payload[key] = value
}

// BuildCreateIssuePayload собирает тело POST /v2/issues (как Python build_payload).
func BuildCreateIssuePayload(f CreateIssueFields) (map[string]any, error) {
	queue := strings.ToUpper(strings.TrimSpace(f.Queue))
	if !ValidQueue(queue) {
		return nil, fmt.Errorf("unsupported queue format")
	}
	summary := strings.TrimSpace(f.Summary)
	if summary == "" {
		return nil, fmt.Errorf("summary is empty")
	}
	if strings.TrimSpace(f.Description) == "" {
		return nil, fmt.Errorf("description is empty")
	}

	payload := map[string]any{
		"queue":       queue,
		"summary":     summary,
		"description": f.Description,
	}

	if tags := ParseTags(f.Tags); len(tags) > 0 {
		payload["tags"] = tags
	}
	if strings.TrimSpace(f.Assignee) != "" {
		assignee, err := ParseUserSelector(f.Assignee)
		if err != nil {
			return nil, err
		}
		payload["assignee"] = assignee
	}

	projectField := fieldKey(f.ProjectField, "TRACKER_PROJECT_FIELD", "project", f.Env)
	periodField := fieldKey(f.PeriodField, "TRACKER_PERIOD_FIELD", "period", f.Env)
	spField := fieldKey(f.SPField, "TRACKER_SP_FIELD", "storyPoints", f.Env)

	setOptional(payload, projectField, f.Project)
	setOptional(payload, periodField, f.Period)
	if f.SP != nil {
		if strings.TrimSpace(spField) != "" {
			payload[spField] = *f.SP
		}
	}

	for _, raw := range f.ExtraFields {
		key, value, err := ParseExtraField(raw)
		if err != nil {
			return nil, err
		}
		payload[key] = value
	}
	return payload, nil
}

// CreateIssue - POST /v2/issues.
func (c *Client) CreateIssue(payload map[string]any) (map[string]any, error) {
	raw, err := c.do(http.MethodPost, c.IssuesURL(), payload)
	if err != nil {
		return nil, err
	}
	var result map[string]any
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("unexpected create issue response: %w", err)
	}
	return result, nil
}
