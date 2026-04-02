package integration

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/sfn"

	"github.com/InCrowd/unified-qual-api/internal/config"
)

// ══════════════════════════════════════════════════════════════════
// ServiceClients — aggregates all external integration HTTP clients.
// Matches the external dependencies of InCrowdAPI (LS) and QS-Tool (MRA).
// ══════════════════════════════════════════════════════════════════

type ServiceClients struct {
	Conference   *ConferenceClient
	Notification *NotificationClient
	GoogleCal    *GoogleCalendarClient
	Decipher     *DecipherClient
	CastingWords *CastingWordsClient
	Stripe       *StripeClient
	Tango        *TangoClient
	PayPal       *PayPalClient
	SMS          *SMSClient
	EventLog     *EventLogClient
	GoogleSheets *GoogleSheetsClient
	S3           *S3Client
	Lambda       *LambdaClient
	StepFn       *StepFunctionsClient
	ICAuth       *InCrowdAPIAuthClient
}

func NewServiceClients(cfg *config.Config) *ServiceClients {
	httpClient := &http.Client{Timeout: 30 * time.Second}

	return &ServiceClients{
		Conference:   newConferenceClient(cfg.ConferenceService, httpClient),
		Notification: newNotificationClient(cfg.NotificationService, httpClient),
		GoogleCal:    newGoogleCalendarClient(cfg.GoogleCalendar, httpClient),
		Decipher:     newDecipherClient(cfg.Decipher, httpClient),
		CastingWords: newCastingWordsClient(cfg.CastingWords, httpClient),
		Stripe:       newStripeClient(cfg.Stripe, httpClient),
		Tango:        newTangoClient(cfg.Tango, httpClient),
		PayPal:       newPayPalClient(cfg.PayPal, httpClient),
		SMS:          newSMSClient(cfg.SMS, httpClient),
		EventLog:     newEventLogClient(cfg.EventLog, httpClient),
		GoogleSheets: newGoogleSheetsClient(cfg.GoogleSheets, cfg.GoogleCalendar.ServiceAccountKeyPath, httpClient),
		S3:           newS3Client(cfg.S3),
		Lambda:       newLambdaClient(cfg.AWS),
		StepFn:       newStepFunctionsClient(cfg.AWS),
		ICAuth:       newInCrowdAPIAuthClient(cfg.AWS.ICApiURL),
	}
}

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
	MeetingID    string `json:"meetingId"`
	JoinURL      string `json:"joinUrl"`
	PhoneNumber  string `json:"phoneNumber"`
	Pin          string `json:"pin"`
	ExternalID   string `json:"externalMeetingId"`
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
	ID          string    `json:"id,omitempty"`
	Summary     string    `json:"summary"`
	Description string    `json:"description,omitempty"`
	Location    string    `json:"location,omitempty"`
	Start       EventTime `json:"start"`
	End         EventTime `json:"end"`
	Attendees   []Attendee `json:"attendees,omitempty"`
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
	OrderID   string `json:"orderId"`
	AudioURL  string `json:"audioUrl"`
	Status    string `json:"status"`
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
		"accountIdentifier":   t.platformName,
		"amount":              amount,
		"utid":                utid,
		"sendEmail":           true,
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

// ──────────────────────────────────────────────
// Google Sheets Client
// Matches: QS-Tool update-google-sheet-first-date.js
// (google-spreadsheet library for moderator availability)
// ──────────────────────────────────────────────

type GoogleSheetsClient struct {
	spreadsheetID         string
	serviceAccountKeyPath string
	client                *http.Client
}

func newGoogleSheetsClient(cfg config.GoogleSheetsConfig, saKeyPath string, c *http.Client) *GoogleSheetsClient {
	return &GoogleSheetsClient{
		spreadsheetID: cfg.SpreadsheetID, serviceAccountKeyPath: saKeyPath, client: c,
	}
}

func (gs *GoogleSheetsClient) Configured() bool {
	return gs.spreadsheetID != "" && gs.serviceAccountKeyPath != ""
}

func (gs *GoogleSheetsClient) UpdateFirstDate(ctx context.Context, sheetName, cellRange, value string) error {
	if !gs.Configured() {
		return fmt.Errorf("google sheets not configured")
	}
	apiURL := fmt.Sprintf(
		"https://sheets.googleapis.com/v4/spreadsheets/%s/values/%s!%s?valueInputOption=USER_ENTERED",
		url.PathEscape(gs.spreadsheetID),
		url.PathEscape(sheetName),
		url.PathEscape(cellRange),
	)

	payload := map[string]any{
		"values": [][]string{{value}},
	}
	b, _ := json.Marshal(payload)

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, apiURL, bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	// Service account auth would be applied via OAuth2 token
	slog.Info("google sheets update first date", "spreadsheet", gs.spreadsheetID, "sheet", sheetName, "value", value)

	resp, err := gs.client.Do(req)
	if err != nil {
		return fmt.Errorf("google sheets API call: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("google sheets returned %d: %s", resp.StatusCode, string(body))
	}
	return nil
}

// ──────────────────────────────────────────────
// S3 Client (AWS SDK v2)
// ──────────────────────────────────────────────

// S3Client wraps AWS SDK v2 S3 operations.
type S3Client struct {
	region        string
	inquiryBucket string
	exportBucket  string
}

func newS3Client(cfg config.S3Config) *S3Client {
	return &S3Client{region: cfg.Region, inquiryBucket: cfg.InquiryBucket, exportBucket: cfg.ExportBucket}
}

func (sc *S3Client) Configured() bool { return sc.inquiryBucket != "" }

func (sc *S3Client) InquiryBucket() string { return sc.inquiryBucket }

func (sc *S3Client) ExportBucket() string { return sc.exportBucket }

func (sc *S3Client) UploadFile(ctx context.Context, bucket, key string, body io.Reader, contentType string) (string, error) {
	awsCfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(sc.region))
	if err != nil {
		return "", fmt.Errorf("load AWS config: %w", err)
	}
	client := s3.NewFromConfig(awsCfg)
	_, err = client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      &bucket,
		Key:         &key,
		Body:        body,
		ContentType: &contentType,
	})
	if err != nil {
		return "", fmt.Errorf("s3 upload: %w", err)
	}
	publicURL := fmt.Sprintf("https://%s.s3.%s.amazonaws.com/%s", bucket, sc.region, key)
	return publicURL, nil
}

// GetPresignedURL returns a presigned GET URL for an S3 object.
func (sc *S3Client) GetPresignedURL(ctx context.Context, bucket, key string, expires time.Duration) (string, error) {
	awsCfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(sc.region))
	if err != nil {
		return "", fmt.Errorf("load AWS config: %w", err)
	}
	client := s3.NewFromConfig(awsCfg)
	presignClient := s3.NewPresignClient(client)
	req, err := presignClient.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: &bucket,
		Key:    &key,
	}, func(opts *s3.PresignOptions) {
		opts.Expires = expires
	})
	if err != nil {
		return "", fmt.Errorf("presign: %w", err)
	}
	return req.URL, nil
}

// GetObject downloads an object from S3 and returns the body reader and content length.
func (sc *S3Client) GetObject(ctx context.Context, bucket, key string) (io.ReadCloser, int64, error) {
	awsCfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(sc.region))
	if err != nil {
		return nil, 0, fmt.Errorf("load AWS config: %w", err)
	}
	client := s3.NewFromConfig(awsCfg)
	out, err := client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: &bucket,
		Key:    &key,
	})
	if err != nil {
		return nil, 0, fmt.Errorf("s3 get object: %w", err)
	}
	var length int64
	if out.ContentLength != nil {
		length = *out.ContentLength
	}
	return out.Body, length, nil
}

// DeleteObject deletes an object from S3.
func (sc *S3Client) DeleteObject(ctx context.Context, bucket, key string) error {
	awsCfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(sc.region))
	if err != nil {
		return fmt.Errorf("load AWS config: %w", err)
	}
	client := s3.NewFromConfig(awsCfg)
	_, err = client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: &bucket,
		Key:    &key,
	})
	if err != nil {
		return fmt.Errorf("s3 delete object: %w", err)
	}
	return nil
}

// ──────────────────────────────────────────────
// Lambda Client (AWS SDK v2)
// ──────────────────────────────────────────────

// LambdaClient wraps AWS Lambda invocations.
type LambdaClient struct {
region      string
environment string
client      *lambda.Client
}

func newLambdaClient(cfg config.AWSConfig) *LambdaClient {
lc := &LambdaClient{region: cfg.Region, environment: cfg.Environment}
if cfg.Region != "" {
awsCfg, err := awsconfig.LoadDefaultConfig(context.Background(), awsconfig.WithRegion(cfg.Region))
if err == nil {
lc.client = lambda.NewFromConfig(awsCfg)
} else {
slog.Warn("lambda client init failed", "error", err)
}
}
return lc
}

func (lc *LambdaClient) Configured() bool {
return lc.client != nil && lc.environment != ""
}

// Invoke calls a Lambda function by name with a JSON payload and returns the response payload.
func (lc *LambdaClient) Invoke(ctx context.Context, functionName string, payload []byte) ([]byte, int32, error) {
if !lc.Configured() {
return nil, 0, fmt.Errorf("lambda client not configured")
}

out, err := lc.client.Invoke(ctx, &lambda.InvokeInput{
FunctionName: &functionName,
Payload:      payload,
})
if err != nil {
return nil, 0, fmt.Errorf("lambda invoke %s: %w", functionName, err)
}

return out.Payload, out.StatusCode, nil
}

// FullLambdaName returns the environment-prefixed Lambda function name.
// Legacy mapping: "prd" → "production", "data-qa" → "qual-qa", else as-is.
func (lc *LambdaClient) FullLambdaName(baseName string) string {
prefix := lc.environment
switch prefix {
case "prd":
prefix = "production"
case "data-qa":
prefix = "qual-qa"
}
return prefix + "-" + baseName
}

// ──────────────────────────────────────────────
// Step Functions Client (AWS SDK v2)
// ──────────────────────────────────────────────

// StepFunctionsClient wraps AWS Step Functions invocations.
type StepFunctionsClient struct {
region      string
account     string
environment string
client      *sfn.Client
}

func newStepFunctionsClient(cfg config.AWSConfig) *StepFunctionsClient {
sc := &StepFunctionsClient{
region: cfg.Region, account: cfg.Account, environment: cfg.Environment,
}
if cfg.Region != "" {
awsCfg, err := awsconfig.LoadDefaultConfig(context.Background(), awsconfig.WithRegion(cfg.Region))
if err == nil {
sc.client = sfn.NewFromConfig(awsCfg)
} else {
slog.Warn("step functions client init failed", "error", err)
}
}
return sc
}

func (sc *StepFunctionsClient) Configured() bool {
return sc.client != nil && sc.account != "" && sc.environment != ""
}

// StartExecution starts a Step Function execution with the given state machine name and input payload.
func (sc *StepFunctionsClient) StartExecution(ctx context.Context, stateMachineName string, input map[string]any) error {
if !sc.Configured() {
return fmt.Errorf("step functions client not configured")
}

arn := fmt.Sprintf("arn:aws:states:%s:%s:stateMachine:%s-%s",
sc.region, sc.account, sc.environment, stateMachineName)

payload := map[string]any{"payload": input}
inputJSON, err := json.Marshal(payload)
if err != nil {
return fmt.Errorf("marshal step function input: %w", err)
}
inputStr := string(inputJSON)

_, err = sc.client.StartExecution(ctx, &sfn.StartExecutionInput{
StateMachineArn: &arn,
Input:           &inputStr,
})
if err != nil {
return fmt.Errorf("start step function %s: %w", stateMachineName, err)
}

slog.Info("step function started", "arn", arn)
return nil
}

// ──────────────────────────────────────────────
// Google Sheets: BatchUpdateFirstDate
// ──────────────────────────────────────────────

// UpdateFirstDateCalendar implements the legacy 30-day rolling calendar update on Google Sheets.
// It writes weekday names in row 1 and NOW()+i formulas in row 2 for columns F through AL (indices 5..37).
func (gs *GoogleSheetsClient) UpdateFirstDateCalendar(ctx context.Context) error {
if !gs.Configured() {
return fmt.Errorf("google sheets not configured")
}

// Build the batch update: row 1 = weekday names, row 2 = NOW()+i formulas
weekdays := []string{"Sunday", "Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday"}
now := time.Now()

var row1Values []any // weekday names
var row2Values []any // formulas

// Column F (index 5) = today, then 32 more days
for i := 0; i <= 32; i++ {
d := now.AddDate(0, 0, i)
dayName := weekdays[int(d.Weekday())]
row1Values = append(row1Values, dayName)
if i == 0 {
row2Values = append(row2Values, "=NOW()")
} else {
row2Values = append(row2Values, fmt.Sprintf("=NOW()+%d", i))
}
}

// Update row 1 (weekday names) — F1:AL1
if err := gs.batchUpdateRange(ctx, "Sheet1!F1:AL1", [][]any{row1Values}); err != nil {
return fmt.Errorf("update row 1: %w", err)
}

// Update row 2 (formulas) — F2:AL2
if err := gs.batchUpdateRange(ctx, "Sheet1!F2:AL2", [][]any{row2Values}); err != nil {
return fmt.Errorf("update row 2: %w", err)
}

return nil
}

func (gs *GoogleSheetsClient) batchUpdateRange(ctx context.Context, rangeStr string, values [][]any) error {
apiURL := fmt.Sprintf(
"https://sheets.googleapis.com/v4/spreadsheets/%s/values/%s?valueInputOption=USER_ENTERED",
url.PathEscape(gs.spreadsheetID),
url.PathEscape(rangeStr),
)

payload := map[string]any{
"range":  rangeStr,
"values": values,
}
b, _ := json.Marshal(payload)

req, err := http.NewRequestWithContext(ctx, http.MethodPut, apiURL, bytes.NewReader(b))
if err != nil {
return err
}
req.Header.Set("Content-Type", "application/json")

resp, err := gs.client.Do(req)
if err != nil {
return fmt.Errorf("sheets API call: %w", err)
}
defer resp.Body.Close()

if resp.StatusCode >= 400 {
body, _ := io.ReadAll(resp.Body)
return fmt.Errorf("sheets returned %d: %s", resp.StatusCode, string(body))
}
return nil
}

// ──────────────────────────────────────────────────────────────────────────────
// InCrowdAPI Auth Client — proxies AuthN/AuthZ to InCrowdAPI
// ──────────────────────────────────────────────────────────────────────────────

// InCrowdAPIAuthClient proxies authentication requests to InCrowdAPI.
type InCrowdAPIAuthClient struct {
baseURL string
client  *http.Client
}

func newInCrowdAPIAuthClient(baseURL string) *InCrowdAPIAuthClient {
return &InCrowdAPIAuthClient{
baseURL: strings.TrimRight(baseURL, "/"),
client:  &http.Client{Timeout: 15 * time.Second},
}
}

func (ic *InCrowdAPIAuthClient) Configured() bool { return ic.baseURL != "" }

// ICLoginResponse holds the relevant fields from InCrowdAPI POST /v1/user/login.
type ICLoginResponse struct {
ID            int64    `json:"id"`
Email         string   `json:"email"`
FirstName     string   `json:"firstName"`
LastName      string   `json:"lastName"`
CognitoToken  string   `json:"cognitoToken"`
AccessToken   string   `json:"accessToken"`
AcceptedTerms bool     `json:"acceptedTerms"`
Roles         []string `json:"roles"`
MustChangePwd bool     `json:"mustChangePassword"`
}

// Login calls InCrowdAPI POST /v1/user/login and returns the response.
func (ic *InCrowdAPIAuthClient) Login(ctx context.Context, email, password string, brandID int64) (*ICLoginResponse, int, error) {
if !ic.Configured() {
return nil, 0, fmt.Errorf("incrowd api auth client not configured")
}

payload, _ := json.Marshal(map[string]any{
"email":    email,
"password": password,
"brandId":  brandID,
})

req, err := http.NewRequestWithContext(ctx, http.MethodPost, ic.baseURL+"/v1/user/login", bytes.NewReader(payload))
if err != nil {
return nil, 0, fmt.Errorf("create login request: %w", err)
}
req.Header.Set("Content-Type", "application/json")

resp, err := ic.client.Do(req)
if err != nil {
return nil, 0, fmt.Errorf("incrowd api login call: %w", err)
}
defer resp.Body.Close()

body, _ := io.ReadAll(resp.Body)

if resp.StatusCode != http.StatusOK {
// Return the status code and raw body so caller can forward the error
return nil, resp.StatusCode, fmt.Errorf("%s", string(body))
}

var loginResp ICLoginResponse
if err := json.Unmarshal(body, &loginResp); err != nil {
return nil, resp.StatusCode, fmt.Errorf("parse login response: %w", err)
}

return &loginResp, resp.StatusCode, nil
}

// RefreshToken calls InCrowdAPI GET /v1/refresh-token/:userId to get a new idToken.
func (ic *InCrowdAPIAuthClient) RefreshToken(ctx context.Context, userID int64, icAuthToken string) (string, error) {
if !ic.Configured() {
return "", fmt.Errorf("incrowd api auth client not configured")
}

url := fmt.Sprintf("%s/v1/refresh-token/%d", ic.baseURL, userID)
req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
if err != nil {
return "", fmt.Errorf("create refresh request: %w", err)
}
req.Header.Set("IC-Auth", fmt.Sprintf("%d:%s", userID, icAuthToken))

resp, err := ic.client.Do(req)
if err != nil {
return "", fmt.Errorf("incrowd api refresh call: %w", err)
}
defer resp.Body.Close()

body, _ := io.ReadAll(resp.Body)
if resp.StatusCode != http.StatusOK {
return "", fmt.Errorf("refresh failed (%d): %s", resp.StatusCode, string(body))
}

var result struct {
IDToken string `json:"id_token"`
}
if err := json.Unmarshal(body, &result); err != nil {
return "", fmt.Errorf("parse refresh response: %w", err)
}

return result.IDToken, nil
}

// AcceptTerms calls InCrowdAPI POST /v1/user/:userId/accept_terms.
func (ic *InCrowdAPIAuthClient) AcceptTerms(ctx context.Context, userID int64) error {
if !ic.Configured() {
return fmt.Errorf("incrowd api auth client not configured")
}

url := fmt.Sprintf("%s/v1/user/%d/accept_terms", ic.baseURL, userID)
payload, _ := json.Marshal(map[string]any{})

req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
if err != nil {
return fmt.Errorf("create accept terms request: %w", err)
}
req.Header.Set("Content-Type", "application/json")

resp, err := ic.client.Do(req)
if err != nil {
return fmt.Errorf("incrowd api accept terms call: %w", err)
}
defer resp.Body.Close()

if resp.StatusCode != http.StatusOK {
body, _ := io.ReadAll(resp.Body)
return fmt.Errorf("accept terms failed (%d): %s", resp.StatusCode, string(body))
}

return nil
}

// Logout calls InCrowdAPI GET /v1/user/logout/:userId to revoke the session.
func (ic *InCrowdAPIAuthClient) Logout(ctx context.Context, userID int64, icAuthToken string) error {
if !ic.Configured() {
return fmt.Errorf("incrowd api auth client not configured")
}

logoutURL := fmt.Sprintf("%s/v1/user/logout/%d", ic.baseURL, userID)
req, err := http.NewRequestWithContext(ctx, http.MethodGet, logoutURL, nil)
if err != nil {
return fmt.Errorf("create logout request: %w", err)
}
if icAuthToken != "" {
req.Header.Set("IC-Auth", fmt.Sprintf("%d:%s", userID, icAuthToken))
}

resp, err := ic.client.Do(req)
if err != nil {
return fmt.Errorf("incrowd api logout call: %w", err)
}
defer resp.Body.Close()

if resp.StatusCode != http.StatusOK {
body, _ := io.ReadAll(resp.Body)
return fmt.Errorf("logout failed (%d): %s", resp.StatusCode, string(body))
}

return nil
}

// ChangePassword calls InCrowdAPI to update the user's password.
func (ic *InCrowdAPIAuthClient) ChangePassword(ctx context.Context, userID int64, icAuthToken, oldPassword, newPassword string) error {
if !ic.Configured() {
return fmt.Errorf("incrowd api auth client not configured")
}

changePwdURL := fmt.Sprintf("%s/v1/user/%d/change_password", ic.baseURL, userID)
payload, _ := json.Marshal(map[string]any{
"oldPassword": oldPassword,
"newPassword": newPassword,
})

req, err := http.NewRequestWithContext(ctx, http.MethodPost, changePwdURL, bytes.NewReader(payload))
if err != nil {
return fmt.Errorf("create change password request: %w", err)
}
req.Header.Set("Content-Type", "application/json")
if icAuthToken != "" {
req.Header.Set("IC-Auth", fmt.Sprintf("%d:%s", userID, icAuthToken))
}

resp, err := ic.client.Do(req)
if err != nil {
return fmt.Errorf("incrowd api change password call: %w", err)
}
defer resp.Body.Close()

if resp.StatusCode != http.StatusOK {
body, _ := io.ReadAll(resp.Body)
return fmt.Errorf("change password failed (%d): %s", resp.StatusCode, string(body))
}

return nil
}
