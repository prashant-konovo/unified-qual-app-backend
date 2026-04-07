package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/InCrowd/unified-qual-api/internal/config"
)

// ──────────────────────────────────────────────
// Notification Service Client
// Matches: InCrowdAPI NotificationService.scala
// (SES-backed notification API Gateway)
// ──────────────────────────────────────────────

type NotificationClient struct {
	baseURL     string
	apiKey      string
	bearerToken string
	client      *http.Client
}

func newNotificationClient(cfg config.NotificationServiceConfig, c *http.Client) *NotificationClient {
	return &NotificationClient{
		baseURL: strings.TrimRight(cfg.BaseURL, "/"), apiKey: cfg.APIKey,
		bearerToken: cfg.BearerToken, client: c,
	}
}

func (n *NotificationClient) Configured() bool { return n.baseURL != "" }

type EmailMessage struct {
	To          []string `json:"to"`
	Subject     string   `json:"subject"`
	Body        string   `json:"body"`
	From        string   `json:"from,omitempty"`
	ReplyTo     string   `json:"replyTo,omitempty"`
	ContentType string   `json:"contentType,omitempty"` // "text/html" or "text/plain"
}

func (n *NotificationClient) SendEmail(ctx context.Context, msg EmailMessage) error {
	if n.baseURL == "" {
		return fmt.Errorf("notification service not configured")
	}
	payload := map[string]any{
		"to":          msg.To,
		"subject":     msg.Subject,
		"body":        msg.Body,
		"from":        msg.From,
		"replyTo":     msg.ReplyTo,
		"contentType": msg.ContentType,
	}
	return n.doPost(ctx, "/send", payload)
}

func (n *NotificationClient) SendBatch(ctx context.Context, messages []EmailMessage) error {
	if n.baseURL == "" {
		return fmt.Errorf("notification service not configured")
	}
	payload := map[string]any{"messages": messages}
	return n.doPost(ctx, "/send-batch", payload)
}

func (n *NotificationClient) doPost(ctx context.Context, path string, payload any) error {
	b, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, n.baseURL+path, bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if n.apiKey != "" {
		req.Header.Set("x-api-key", n.apiKey)
	}
	if n.bearerToken != "" {
		req.Header.Set("Authorization", "Bearer "+n.bearerToken)
	}

	resp, err := n.client.Do(req)
	if err != nil {
		return fmt.Errorf("notification HTTP call: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		slog.Warn("notification service error", "status", resp.StatusCode, "body", string(body))
		return fmt.Errorf("notification service returned %d", resp.StatusCode)
	}
	return nil
}
