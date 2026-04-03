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
// Conference Service Client
// Matches: InCrowdAPI ConferenceService.scala
// (API Gateway + Lambda → AWS Chime SDK)
// ──────────────────────────────────────────────

type ConferenceClient struct {
	baseURL string
	apiKey  string
	client  *http.Client
}

func newConferenceClient(cfg config.ConferenceServiceConfig, c *http.Client) *ConferenceClient {
	return &ConferenceClient{baseURL: strings.TrimRight(cfg.BaseURL, "/"), apiKey: cfg.APIKey, client: c}
}

func (c *ConferenceClient) Configured() bool { return c.baseURL != "" }

type MeetingCreateRequest struct {
	ProjectID      int64  `json:"projectId"`
	SubscriptionID int64  `json:"subscriptionId"`
	ModeratorID    int64  `json:"moderatorId"`
	TimeSlotID     int64  `json:"timeSlotId"`
	ExternalID     string `json:"externalMeetingId,omitempty"`
}

type MeetingCreateResponse struct {
	MeetingID   string `json:"meetingId"`
	JoinURL     string `json:"joinUrl"`
	PhoneNumber string `json:"phoneNumber"`
	Pin         string `json:"pin"`
	ExternalID  string `json:"externalMeetingId"`
}

func (c *ConferenceClient) CreateMeeting(ctx context.Context, req MeetingCreateRequest, bearerToken string) (*MeetingCreateResponse, error) {
	body, err := c.doJSON(ctx, http.MethodPost, "/meeting", req, bearerToken)
	if err != nil {
		return nil, fmt.Errorf("conference create meeting: %w", err)
	}
	var resp MeetingCreateResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("conference parse create response: %w", err)
	}
	return &resp, nil
}

type MeetingJoinResponse struct {
	AttendeeID string `json:"attendeeId"`
	JoinURL    string `json:"joinUrl"`
	JoinToken  string `json:"joinToken"`
}

func (c *ConferenceClient) JoinMeeting(ctx context.Context, meetingID string, userID string, role string, bearerToken string) (*MeetingJoinResponse, error) {
	payload := map[string]string{"userId": userID, "role": role}
	body, err := c.doJSON(ctx, http.MethodPut, "/join/"+meetingID, payload, bearerToken)
	if err != nil {
		return nil, fmt.Errorf("conference join: %w", err)
	}
	var resp MeetingJoinResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("conference parse join response: %w", err)
	}
	return &resp, nil
}

func (c *ConferenceClient) EndMeeting(ctx context.Context, meetingID string, bearerToken string) error {
	_, err := c.doJSON(ctx, http.MethodPost, "/meeting/"+meetingID+"/end", nil, bearerToken)
	return err
}

func (c *ConferenceClient) AddUsersToMeeting(ctx context.Context, meetingID string, userIDs []string, bearerToken string) error {
	payload := map[string]any{"userIds": userIDs}
	_, err := c.doJSON(ctx, http.MethodPost, "/meeting/"+meetingID+"/add_users", payload, bearerToken)
	return err
}

func (c *ConferenceClient) UniversalJoin(ctx context.Context, meetingID string, bearerToken string) (map[string]any, error) {
	body, err := c.doJSON(ctx, http.MethodPost, "/meeting/"+meetingID+"/universal", nil, bearerToken)
	if err != nil {
		return nil, err
	}
	var resp map[string]any
	_ = json.Unmarshal(body, &resp)
	return resp, nil
}

func (c *ConferenceClient) GetRecordingStatus(ctx context.Context, meetingID string, bearerToken string) (map[string]any, error) {
	body, err := c.doJSON(ctx, http.MethodGet, "/meeting/recording_status/"+meetingID, nil, bearerToken)
	if err != nil {
		return nil, err
	}
	var resp map[string]any
	_ = json.Unmarshal(body, &resp)
	return resp, nil
}

func (c *ConferenceClient) GetAttendees(ctx context.Context, meetingID string, bearerToken string) ([]map[string]any, error) {
	body, err := c.doJSON(ctx, http.MethodGet, "/meeting/get_attendees_by_meeting_id/"+meetingID, nil, bearerToken)
	if err != nil {
		return nil, err
	}
	var resp []map[string]any
	_ = json.Unmarshal(body, &resp)
	return resp, nil
}

func (c *ConferenceClient) GetMetadata(ctx context.Context, bearerToken string) (map[string]any, error) {
	body, err := c.doJSON(ctx, http.MethodGet, "/metadata", nil, bearerToken)
	if err != nil {
		return nil, err
	}
	var resp map[string]any
	_ = json.Unmarshal(body, &resp)
	return resp, nil
}

func (c *ConferenceClient) ValidateToken(ctx context.Context, token string) (map[string]any, error) {
	payload := map[string]string{"token": token}
	body, err := c.doJSON(ctx, http.MethodPost, "/validate", payload, token)
	if err != nil {
		return nil, err
	}
	var resp map[string]any
	_ = json.Unmarshal(body, &resp)
	return resp, nil
}

func (c *ConferenceClient) StartRecording(ctx context.Context, meetingID string, bearerToken string) error {
	payload := map[string]string{"meetingId": meetingID}
	_, err := c.doJSON(ctx, http.MethodPost, "/recording", payload, bearerToken)
	return err
}

func (c *ConferenceClient) doJSON(ctx context.Context, method, path string, payload any, bearerToken string) ([]byte, error) {
	if c.baseURL == "" {
		return nil, fmt.Errorf("conference service not configured")
	}

	var bodyReader io.Reader
	if payload != nil {
		b, err := json.Marshal(payload)
		if err != nil {
			return nil, err
		}
		bodyReader = bytes.NewReader(b)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bodyReader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		req.Header.Set("x-api-key", c.apiKey)
	}
	if bearerToken != "" {
		req.Header.Set("Authorization", "Bearer "+bearerToken)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("conference HTTP call %s %s: %w", method, path, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read conference response: %w", err)
	}
	if resp.StatusCode >= 400 {
		slog.Warn("conference service error", "status", resp.StatusCode, "path", path, "body", string(body))
		return body, fmt.Errorf("conference service returned %d", resp.StatusCode)
	}
	return body, nil
}
