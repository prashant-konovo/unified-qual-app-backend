package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"time"

	"github.com/InCrowd/unified-qual-api/internal/config"
)

// ──────────────────────────────────────────────
// Google Calendar Client
// Matches: InCrowdAPI GoogleCalendar.scala
// (Google Calendar API v3 with service account)
// ──────────────────────────────────────────────

type GoogleCalendarClient struct {
	serviceAccountKeyPath string
	defaultCalendarID     string
	client                *http.Client
}

func newGoogleCalendarClient(cfg config.GoogleCalendarConfig, c *http.Client) *GoogleCalendarClient {
	return &GoogleCalendarClient{
		serviceAccountKeyPath: cfg.ServiceAccountKeyPath,
		defaultCalendarID:     cfg.DefaultCalendarID,
		client:                c,
	}
}

func (g *GoogleCalendarClient) Configured() bool {
	return g.serviceAccountKeyPath != "" && g.defaultCalendarID != ""
}

type CalendarEvent struct {
	ID             string         `json:"id,omitempty"`
	Summary        string         `json:"summary"`
	Description    string         `json:"description,omitempty"`
	Location       string         `json:"location,omitempty"`
	Start          EventTime      `json:"start"`
	End            EventTime      `json:"end"`
	Attendees      []Attendee     `json:"attendees,omitempty"`
	ConferenceData map[string]any `json:"conferenceData,omitempty"`
}

type EventTime struct {
	DateTime string `json:"dateTime"`
	TimeZone string `json:"timeZone,omitempty"`
}

type Attendee struct {
	Email string `json:"email"`
}

func (g *GoogleCalendarClient) CreateEvent(ctx context.Context, calendarID string, event *CalendarEvent) (*CalendarEvent, error) {
	if !g.Configured() {
		return nil, fmt.Errorf("google calendar not configured")
	}
	if calendarID == "" {
		calendarID = g.defaultCalendarID
	}
	apiURL := fmt.Sprintf("https://www.googleapis.com/calendar/v3/calendars/%s/events", url.PathEscape(calendarID))

	b, err := json.Marshal(event)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	// Service account auth would be set up via OAuth2 token here.
	// In production, use google.DefaultTokenSource or JWT signing.
	slog.Info("google calendar create event", "calendar", calendarID, "summary", event.Summary)

	resp, err := g.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("google calendar API call: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		slog.Warn("google calendar error", "status", resp.StatusCode, "body", string(body))
		return nil, fmt.Errorf("google calendar returned %d", resp.StatusCode)
	}

	var created CalendarEvent
	_ = json.Unmarshal(body, &created)
	return &created, nil
}

func (g *GoogleCalendarClient) UpdateEvent(ctx context.Context, calendarID, eventID string, event *CalendarEvent) error {
	if !g.Configured() {
		return fmt.Errorf("google calendar not configured")
	}
	if calendarID == "" {
		calendarID = g.defaultCalendarID
	}
	apiURL := fmt.Sprintf("https://www.googleapis.com/calendar/v3/calendars/%s/events/%s",
		url.PathEscape(calendarID), url.PathEscape(eventID))

	b, err := json.Marshal(event)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, apiURL, bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := g.client.Do(req)
	if err != nil {
		return fmt.Errorf("google calendar API call: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("google calendar update returned %d: %s", resp.StatusCode, string(body))
	}
	return nil
}

func (g *GoogleCalendarClient) DeleteEvent(ctx context.Context, calendarID, eventID string) error {
	if !g.Configured() {
		return fmt.Errorf("google calendar not configured")
	}
	if calendarID == "" {
		calendarID = g.defaultCalendarID
	}
	apiURL := fmt.Sprintf("https://www.googleapis.com/calendar/v3/calendars/%s/events/%s",
		url.PathEscape(calendarID), url.PathEscape(eventID))

	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, apiURL, nil)
	if err != nil {
		return err
	}
	resp, err := g.client.Do(req)
	if err != nil {
		return fmt.Errorf("google calendar API call: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("google calendar delete returned %d", resp.StatusCode)
	}
	return nil
}

func (g *GoogleCalendarClient) ListEvents(ctx context.Context, calendarID string, timeMin, timeMax time.Time) ([]CalendarEvent, error) {
	if !g.Configured() {
		return nil, fmt.Errorf("google calendar not configured")
	}
	if calendarID == "" {
		calendarID = g.defaultCalendarID
	}
	apiURL := fmt.Sprintf("https://www.googleapis.com/calendar/v3/calendars/%s/events?timeMin=%s&timeMax=%s&singleEvents=true&orderBy=startTime",
		url.PathEscape(calendarID),
		url.QueryEscape(timeMin.Format(time.RFC3339)),
		url.QueryEscape(timeMax.Format(time.RFC3339)))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := g.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("google calendar list: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("google calendar list returned %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Items []CalendarEvent `json:"items"`
	}
	_ = json.Unmarshal(body, &result)
	return result.Items, nil
}
