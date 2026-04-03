package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"

	"github.com/InCrowd/unified-qual-api/internal/config"
)

// ──────────────────────────────────────────────
// Stripe Client
// Matches: InCrowdAPI StripeGateway.scala
// ──────────────────────────────────────────────

type StripeClient struct {
	secretKey string
	client    *http.Client
}

func newStripeClient(cfg config.StripeConfig, c *http.Client) *StripeClient {
	return &StripeClient{secretKey: cfg.SecretKey, client: c}
}

func (s *StripeClient) Configured() bool { return s.secretKey != "" }

type PaymentResult struct {
	ID       string `json:"id"`
	Status   string `json:"status"`
	Amount   int    `json:"amount"`
	Currency string `json:"currency"`
}

func (s *StripeClient) Charge(ctx context.Context, amount int, currency, token, description string) (*PaymentResult, error) {
	if !s.Configured() {
		return nil, fmt.Errorf("stripe not configured")
	}
	form := url.Values{}
	form.Set("amount", fmt.Sprintf("%d", amount))
	form.Set("currency", currency)
	form.Set("source", token)
	form.Set("description", description)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		"https://api.stripe.com/v1/charges", strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(s.secretKey, "")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("stripe charge: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		slog.Warn("stripe error", "status", resp.StatusCode, "body", string(body))
		return nil, fmt.Errorf("stripe returned %d", resp.StatusCode)
	}

	var result PaymentResult
	_ = json.Unmarshal(body, &result)
	return &result, nil
}
