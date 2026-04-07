package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/InCrowd/unified-qual-api/internal/config"
)

// ──────────────────────────────────────────────
// Event Log Client
// Matches: QS-Tool event logging POST calls
// ──────────────────────────────────────────────

type EventLogClient struct {
	baseURL string
	apiKey  string
	client  *http.Client
}

func newEventLogClient(cfg config.EventLogConfig, c *http.Client) *EventLogClient {
	return &EventLogClient{baseURL: strings.TrimRight(cfg.BaseURL, "/"), apiKey: cfg.APIKey, client: c}
}

func (e *EventLogClient) Configured() bool { return e.baseURL != "" }

func (e *EventLogClient) LogEvent(ctx context.Context, eventType, description string, metadata map[string]any) error {
	if !e.Configured() {
		return nil // silently skip if not configured
	}
	payload := map[string]any{
		"eventType":   eventType,
		"description": description,
		"metadata":    metadata,
		"timestamp":   time.Now().UTC().Format(time.RFC3339),
	}
	b, _ := json.Marshal(payload)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, e.baseURL+"/events", bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if e.apiKey != "" {
		req.Header.Set("x-api-key", e.apiKey)
	}

	resp, err := e.client.Do(req)
	if err != nil {
		slog.Warn("event log call failed", "error", err)
		return nil // non-critical, don't bubble up
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		slog.Warn("event log error", "status", resp.StatusCode)
	}
	return nil
}
