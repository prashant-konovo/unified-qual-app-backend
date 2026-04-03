package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/InCrowd/unified-qual-api/internal/config"
)

// ──────────────────────────────────────────────
// Tango Card Client
// Matches: InCrowdAPI TangoGateway.scala
// ──────────────────────────────────────────────

type TangoClient struct {
	baseURL      string
	platformName string
	platformKey  string
	client       *http.Client
}

func newTangoClient(cfg config.TangoConfig, c *http.Client) *TangoClient {
	return &TangoClient{
		baseURL: strings.TrimRight(cfg.BaseURL, "/"), platformName: cfg.PlatformName,
		platformKey: cfg.PlatformKey, client: c,
	}
}

func (t *TangoClient) Configured() bool { return t.baseURL != "" && t.platformName != "" }

func (t *TangoClient) CreateOrder(ctx context.Context, recipientEmail, recipientName string, amount int, utid string) (*PaymentResult, error) {
	if !t.Configured() {
		return nil, fmt.Errorf("tango not configured")
	}
	payload := map[string]any{
		"accountIdentifier": t.platformName,
		"amount":            amount,
		"utid":              utid,
		"sendEmail":         true,
		"recipient": map[string]string{
			"email":     recipientEmail,
			"firstName": recipientName,
		},
	}
	b, _ := json.Marshal(payload)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, t.baseURL+"/orders", bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.SetBasicAuth(t.platformName, t.platformKey)

	resp, err := t.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("tango order: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("tango returned %d: %s", resp.StatusCode, string(body))
	}

	var result PaymentResult
	_ = json.Unmarshal(body, &result)
	return &result, nil
}
