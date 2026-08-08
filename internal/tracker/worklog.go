package tracker

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/url"
)

// SecondsToHuman - "1 ч 5 мин" / "3 ч" / "12 мин".
func SecondsToHuman(seconds int) string {
	minutes := int(math.Round(float64(seconds) / 60.0))
	if minutes < 0 {
		minutes = 0
	}
	hours, rem := minutes/60, minutes%60
	if hours > 0 && rem > 0 {
		return fmt.Sprintf("%d ч %d мин", hours, rem)
	}
	if hours > 0 {
		return fmt.Sprintf("%d ч", hours)
	}
	return fmt.Sprintf("%d мин", rem)
}

// SecondsToISODuration - PT1H5M для worklog (минимум 1 минута).
func SecondsToISODuration(seconds int) string {
	minutes := int(math.Round(float64(seconds) / 60.0))
	if minutes < 1 {
		minutes = 1
	}
	hours, rem := minutes/60, minutes%60
	if hours > 0 && rem > 0 {
		return fmt.Sprintf("PT%dH%dM", hours, rem)
	}
	if hours > 0 {
		return fmt.Sprintf("PT%dH", hours)
	}
	return fmt.Sprintf("PT%dM", rem)
}

// RoundSeconds округляет длительность до шага roundToMinutes (как Python).
func RoundSeconds(seconds, roundToMinutes int) int {
	if roundToMinutes <= 1 {
		if seconds < 60 {
			return 60
		}
		return seconds
	}
	step := roundToMinutes * 60
	rounded := int(math.Round(float64(seconds)/float64(step))) * step
	if rounded < step {
		return step
	}
	return rounded
}

// PostWorklog - POST /v2/issues/{key}/worklog.
func (c *Client) PostWorklog(issueKey string, payload map[string]any) (map[string]any, error) {
	u := fmt.Sprintf("%s/v2/issues/%s/worklog", c.BaseURL, url.PathEscape(issueKey))
	raw, err := c.do(http.MethodPost, u, payload)
	if err != nil {
		return nil, err
	}
	var result map[string]any
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("unexpected worklog response: %w", err)
	}
	return result, nil
}
