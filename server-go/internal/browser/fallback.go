package browser

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strconv"
)

// Item is the minimal work metadata returned by the Playwright helper.
type Item struct {
	AwemeID string `json:"aweme_id"`
	Type    string `json:"type"`
	URL     string `json:"url"`
}

type result struct {
	Items []Item `json:"items"`
}

// FetchUserPosts runs the optional Node/Playwright helper. The Go server has no
// hard browser dependency: if node, the helper, or Playwright is absent this
// simply returns an error and callers can surface the original API failure.
func FetchUserPosts(ctx context.Context, helperPath, userURL string, cookies map[string]string, maxItems int, headless bool) ([]Item, error) {
	if maxItems <= 0 {
		maxItems = 50
	}
	if _, err := os.Stat(helperPath); err != nil {
		return nil, fmt.Errorf("browser helper unavailable: %w", err)
	}
	node, err := exec.LookPath("node")
	if err != nil {
		return nil, fmt.Errorf("node not found: %w", err)
	}
	cookieJSON, _ := json.Marshal(cookies)
	cmd := exec.CommandContext(ctx, node, helperPath, userURL, strconv.Itoa(maxItems), strconv.FormatBool(headless))
	cmd.Env = append(os.Environ(), "DOUYIN_BROWSER_COOKIES="+string(cookieJSON))
	out, err := cmd.Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("playwright helper failed: %s", string(ee.Stderr))
		}
		return nil, err
	}
	var parsed result
	if err := json.Unmarshal(out, &parsed); err != nil {
		return nil, fmt.Errorf("parse playwright result: %w", err)
	}
	return parsed.Items, nil
}
