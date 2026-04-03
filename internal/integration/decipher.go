package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/InCrowd/unified-qual-api/internal/config"
)

// ──────────────────────────────────────────────
// Decipher Client
// Matches: QS-Tool helpers.js Decipher API calls
// (survey.opinionsite.com — responder data retrieval)
// ──────────────────────────────────────────────

type DecipherClient struct {
	baseURL string
	apiKey  string
	client  *http.Client
}

func newDecipherClient(cfg config.DecipherConfig, c *http.Client) *DecipherClient {
	return &DecipherClient{baseURL: strings.TrimRight(cfg.BaseURL, "/"), apiKey: cfg.APIKey, client: c}
}

func (d *DecipherClient) Configured() bool { return d.baseURL != "" && d.apiKey != "" }

func (d *DecipherClient) GetRespondentData(ctx context.Context, surveyID string) ([]map[string]any, error) {
	if !d.Configured() {
		return nil, fmt.Errorf("decipher not configured")
	}
	apiURL := fmt.Sprintf("%s/surveys/selfserve/20dc/%s/data", d.baseURL, url.PathEscape(surveyID))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("x-api-key", d.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := d.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("decipher API call: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("decipher returned %d: %s", resp.StatusCode, string(body))
	}

	var result []map[string]any
	_ = json.Unmarshal(body, &result)
	return result, nil
}
