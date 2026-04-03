package integration

import (
	"bytes"
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
// CastingWords Client
// Matches: InCrowdAPI TranscriptionController.scala
// (CastingWords API4 — transcription order/status/download)
// ──────────────────────────────────────────────

type CastingWordsClient struct {
	baseURL string
	apiKey  string
	client  *http.Client
}

func newCastingWordsClient(cfg config.CastingWordsConfig, c *http.Client) *CastingWordsClient {
	return &CastingWordsClient{baseURL: strings.TrimRight(cfg.BaseURL, "/"), apiKey: cfg.APIKey, client: c}
}

func (cw *CastingWordsClient) Configured() bool { return cw.baseURL != "" && cw.apiKey != "" }

type TranscriptionOrder struct {
	OrderID    string `json:"orderId"`
	AudioURL   string `json:"audioUrl"`
	Status     string `json:"status"`
	Transcript string `json:"transcript,omitempty"`
}

func (cw *CastingWordsClient) CreateOrder(ctx context.Context, audioURL string) (*TranscriptionOrder, error) {
	if !cw.Configured() {
		return nil, fmt.Errorf("castingwords not configured")
	}
	payload := map[string]string{"url": audioURL}
	b, _ := json.Marshal(payload)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, cw.baseURL+"/order_url", bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Token "+cw.apiKey)

	resp, err := cw.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("castingwords create order: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("castingwords returned %d: %s", resp.StatusCode, string(body))
	}

	var order TranscriptionOrder
	_ = json.Unmarshal(body, &order)
	return &order, nil
}

func (cw *CastingWordsClient) GetOrderStatus(ctx context.Context, orderID string) (*TranscriptionOrder, error) {
	if !cw.Configured() {
		return nil, fmt.Errorf("castingwords not configured")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		cw.baseURL+"/audiofile/"+url.PathEscape(orderID), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Token "+cw.apiKey)

	resp, err := cw.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var order TranscriptionOrder
	_ = json.Unmarshal(body, &order)
	return &order, nil
}

func (cw *CastingWordsClient) GetTranscript(ctx context.Context, orderID string) (string, error) {
	if !cw.Configured() {
		return "", fmt.Errorf("castingwords not configured")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		cw.baseURL+"/transcript/"+url.PathEscape(orderID), nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Token "+cw.apiKey)

	resp, err := cw.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("castingwords transcript returned %d", resp.StatusCode)
	}
	return string(body), nil
}
