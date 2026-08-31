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

// ScrollOptions tunes the helper's scrolling behaviour. Zero values mean "let
// the helper use its built-in defaults".
type ScrollOptions struct {
	MaxScrolls         int
	IdleRounds         int
	WaitTimeoutSeconds int
}

type result struct {
	Items []Item `json:"items"`
}

// FetchUserPosts runs the optional Node/Playwright helper. The Go server has no
// hard browser dependency: if node, the helper, or Playwright is absent this
// simply returns an error and callers can surface the original API failure.
func FetchUserPosts(ctx context.Context, helperPath, userURL string, cookies map[string]string, maxItems int, headless bool, scroll ScrollOptions) ([]Item, error) {
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
	if scroll.MaxScrolls > 0 {
		cmd.Env = append(cmd.Env, "DOUYIN_BROWSER_MAX_SCROLLS="+strconv.Itoa(scroll.MaxScrolls))
	}
	if scroll.IdleRounds > 0 {
		cmd.Env = append(cmd.Env, "DOUYIN_BROWSER_IDLE_ROUNDS="+strconv.Itoa(scroll.IdleRounds))
	}
	if scroll.WaitTimeoutSeconds > 0 {
		cmd.Env = append(cmd.Env, "DOUYIN_BROWSER_WAIT_TIMEOUT_SECONDS="+strconv.Itoa(scroll.WaitTimeoutSeconds))
	}
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
