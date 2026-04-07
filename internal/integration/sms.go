package integration

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/InCrowd/unified-qual-api/internal/config"
)

// ──────────────────────────────────────────────
// SMS Client (Bandwidth)
// Matches: InCrowdAPI SMSGateway.scala
// ──────────────────────────────────────────────

type SMSClient struct {
	baseURL       string
	apiToken      string
	accountID     string
	applicationID string
	client        *http.Client
}

func newSMSClient(cfg config.SMSConfig, c *http.Client) *SMSClient {
	return &SMSClient{
		baseURL: strings.TrimRight(cfg.BaseURL, "/"), apiToken: cfg.APIToken,
		accountID: cfg.AccountID, applicationID: cfg.ApplicationID, client: c,
	}
}

func (s *SMSClient) Configured() bool { return s.baseURL != "" && s.accountID != "" }

func (s *SMSClient) SendSMS(ctx context.Context, to, from, message string) error {
	if !s.Configured() {
		return fmt.Errorf("sms (bandwidth) not configured")
	}
	payload := map[string]string{
		"to":            to,
		"from":          from,
		"text":          message,
		"applicationId": s.applicationID,
	}
	b, _ := json.Marshal(payload)

	apiURL := fmt.Sprintf("%s/users/%s/messages", s.baseURL, url.PathEscape(s.accountID))
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(s.apiToken)))

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("bandwidth SMS: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("bandwidth returned %d: %s", resp.StatusCode, string(body))
	}
	return nil
}
