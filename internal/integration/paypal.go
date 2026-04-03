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
	"time"

	"github.com/InCrowd/unified-qual-api/internal/config"
)

// ──────────────────────────────────────────────
// PayPal Client
// Matches: InCrowdAPI PayPalRestClient.scala
// ──────────────────────────────────────────────

type PayPalClient struct {
	baseURL      string
	clientID     string
	clientSecret string
	client       *http.Client
}

func newPayPalClient(cfg config.PayPalConfig, c *http.Client) *PayPalClient {
	return &PayPalClient{
		baseURL: strings.TrimRight(cfg.BaseURL, "/"), clientID: cfg.ClientID,
		clientSecret: cfg.ClientSecret, client: c,
	}
}

func (p *PayPalClient) Configured() bool { return p.baseURL != "" && p.clientID != "" }

func (p *PayPalClient) getAccessToken(ctx context.Context) (string, error) {
	form := url.Values{}
	form.Set("grant_type", "client_credentials")

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		p.baseURL+"/v1/oauth2/token", strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(p.clientID, p.clientSecret)

	resp, err := p.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var tokenResp struct {
		AccessToken string `json:"access_token"`
	}
	_ = json.Unmarshal(body, &tokenResp)
	return tokenResp.AccessToken, nil
}

func (p *PayPalClient) Payout(ctx context.Context, recipientEmail string, amount float64, currency string) (*PaymentResult, error) {
	if !p.Configured() {
		return nil, fmt.Errorf("paypal not configured")
	}
	token, err := p.getAccessToken(ctx)
	if err != nil {
		return nil, fmt.Errorf("paypal auth: %w", err)
	}

	payload := map[string]any{
		"sender_batch_header": map[string]string{
			"sender_batch_id": fmt.Sprintf("batch_%d", time.Now().UnixMilli()),
			"email_subject":   "Payment Received",
		},
		"items": []map[string]any{
			{
				"recipient_type": "EMAIL",
				"amount":         map[string]any{"value": fmt.Sprintf("%.2f", amount), "currency": currency},
				"receiver":       recipientEmail,
			},
		},
	}
	b, _ := json.Marshal(payload)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		p.baseURL+"/v1/payments/payouts", bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("paypal payout: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("paypal returned %d: %s", resp.StatusCode, string(respBody))
	}

	var result PaymentResult
	_ = json.Unmarshal(respBody, &result)
	return &result, nil
}
