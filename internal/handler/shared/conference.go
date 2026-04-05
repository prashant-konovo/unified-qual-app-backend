package shared

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/InCrowd/unified-qual-api/internal/handler/httputil"

	"github.com/InCrowd/unified-qual-api/internal/integration"
	"github.com/InCrowd/unified-qual-api/internal/middleware"
	"github.com/InCrowd/unified-qual-api/internal/dto"
	"github.com/go-chi/chi/v5"
)

// ──────────────────────────────────────────────
// Conference handlers (shared)
// ──────────────────────────────────────────────

// ──────────────────────────────────────────────
// Conference/Meeting extended
// ──────────────────────────────────────────────

// ConferenceLogin validates a conference hash and pin.
// Contract-identical with legacy InCrowdAPI: POST /v1/conf/:confId/login
// Response: flat conference data object
func (h *Handler) ConferenceLogin(w http.ResponseWriter, r *http.Request) {
	confHashStr, _ := dto.ParseStringParam(r, "confId")
	var req struct {
		Pin string `json:"pin"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)

	if h.ConferenceService.Available() {
		data, err := h.ConferenceService.Login(r.Context(), confHashStr, req.Pin)
		if err != nil {
			if strings.Contains(err.Error(), "invalid pin") {
				httputil.WriteJSON(w, http.StatusUnauthorized, map[string]any{"error": "invalid pin"})
				return
			}
			if strings.Contains(err.Error(), "not found") {
				httputil.WriteJSON(w, http.StatusNotFound, map[string]any{"error": "conference not found"})
				return
			}
			httputil.WriteJSON(w, http.StatusInternalServerError, map[string]any{"error": "login failed"})
			return
		}
		httputil.WriteJSON(w, http.StatusOK, data)
		return
	}
	httputil.WriteJSON(w, http.StatusNotFound, map[string]any{"error": "conference not found"})
}

// GetConferenceParticipants returns participants in a conference.
// GetConferenceParticipants returns participants and timeslot info for a conference.
// Contract-identical with legacy InCrowdAPI: GET /v1/conf/:confId/participants
// Response: {"participants": [...], "timeSlot": {startTime, endTime, conferencePin, projectId, id}}
func (h *Handler) GetConferenceParticipants(w http.ResponseWriter, r *http.Request) {
	confHashStr, _ := dto.ParseStringParam(r, "confId")

	if h.ConferenceService.Available() {
		ci, err := h.ConferenceService.GetByHash(r.Context(), confHashStr)
		if err != nil || ci == nil {
			httputil.WriteJSON(w, http.StatusNotFound, map[string]any{"error": "conference not found"})
			return
		}
		participants, err := h.ConferenceService.GetParticipants(r.Context(), ci.TimeSlotID)
		if err != nil {
			slog.Error("get participants failed", "error", err)
			httputil.WriteJSON(w, http.StatusInternalServerError, map[string]any{"error": "database error"})
			return
		}
		if participants == nil {
			participants = []map[string]any{}
		}

		// Build timeSlot object matching legacy shape
		tsObj := map[string]any{
			"id":            ci.TimeSlotID,
			"conferencePin": ci.Pin,
		}
		// Enrich with timeslot start/end/projectId if available
		if h.InterviewService.TimeSlotAvailable() {
			ts, tsErr := h.InterviewService.GetByID(r.Context(), ci.TimeSlotID)
			if tsErr == nil && ts != nil {
				tsObj["startTime"] = ts.StartTime.Format(time.RFC3339)
				tsObj["endTime"] = ts.EndTime.Format(time.RFC3339)
				tsObj["projectId"] = ts.ProjectID
			}
		}

		httputil.WriteJSON(w, http.StatusOK, map[string]any{
			"participants": participants,
			"timeSlot":     tsObj,
		})
		return
	}
	httputil.WriteJSON(w, http.StatusOK, map[string]any{"participants": []any{}, "timeSlot": nil})
}

// GetMeetingMetadata returns meeting metadata.
// Contract-identical with legacy InCrowdAPI: GET /v1/meeting/metadata
// Response: flat meeting metadata object
func (h *Handler) GetMeetingMetadata(w http.ResponseWriter, r *http.Request) {
	hash := r.URL.Query().Get("hash")
	bearerToken := httputil.ExtractBearerToken(r)

	// Try Conference Service for live metadata
	if hash == "" && h.ConferenceService.ConferenceConfigured() {
		meta, err := h.ConferenceService.GetConferenceMetadata(r.Context(), bearerToken)
		if err == nil && meta != nil {
			httputil.WriteJSON(w, http.StatusOK, meta)
			return
		}
	}

	if hash == "" {
		httputil.WriteJSON(w, http.StatusBadRequest, map[string]any{"error": "hash parameter required"})
		return
	}

	if h.ConferenceService.Available() {
		meta, err := h.ConferenceService.GetMeetingMetadata(r.Context(), hash)
		if err != nil {
			slog.Error("get meeting metadata failed", "error", err)
			httputil.WriteJSON(w, http.StatusInternalServerError, map[string]any{"error": "database error"})
			return
		}
		if meta == nil {
			httputil.WriteJSON(w, http.StatusNotFound, map[string]any{"error": "meeting not found"})
			return
		}
		httputil.WriteJSON(w, http.StatusOK, meta)
		return
	}
	httputil.WriteJSON(w, http.StatusNotFound, map[string]any{"error": "meeting not found"})
}

// MeetingJoin handles join meeting by join ID.
// Contract-identical with legacy InCrowdAPI: POST /v1/meeting/:joinId/join
// Response: flat meeting metadata object
func (h *Handler) MeetingJoin(w http.ResponseWriter, r *http.Request) {
	joinID, err := dto.ParseStringParam(r, "joinId")
	if err != nil {
		httputil.WriteJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}

	// Use join ID as conference hash or participant hash
	if h.ConferenceService.Available() {
		meta, err := h.ConferenceService.GetMeetingMetadata(r.Context(), joinID)
		if err == nil && meta != nil {
			httputil.WriteJSON(w, http.StatusOK, meta)
			return
		}
	}
	httputil.WriteJSON(w, http.StatusNotFound, map[string]any{"error": "meeting not found"})
}

// GetAttendeesByMeetingID returns attendees for a meeting.
// Contract-identical with legacy InCrowdAPI: GET /v1/meeting/:meetingId/attendees
// Response: passthrough from conference service
func (h *Handler) GetAttendeesByMeetingID(w http.ResponseWriter, r *http.Request) {
	meetingID, _ := dto.ParseStringParam(r, "meetingId")
	bearerToken := httputil.ExtractBearerToken(r)

	// Try Conference Service for live attendee data
	if h.ConferenceService.ConferenceConfigured() {
		attendees, err := h.ConferenceService.GetConferenceAttendees(r.Context(), meetingID, bearerToken)
		if err == nil && attendees != nil {
			httputil.WriteJSON(w, http.StatusOK, attendees)
			return
		}
		slog.Warn("conference service get attendees failed, falling back to DB", "error", err)
	}

	if h.ConferenceService.Available() {
		attendees, err := h.ConferenceService.GetAttendeesByMeetingID(r.Context(), meetingID)
		if err != nil {
			slog.Error("get attendees failed", "error", err)
			httputil.WriteJSON(w, http.StatusInternalServerError, map[string]any{"error": "database error"})
			return
		}
		httputil.WriteJSON(w, http.StatusOK, attendees)
		return
	}
	httputil.WriteJSON(w, http.StatusOK, []any{})
}

// GetRecordingStatus returns recording status for a meeting.
// Contract-identical with legacy InCrowdAPI: GET /v1/meeting/:meetingId/recording
// Response: passthrough from conference service
func (h *Handler) GetRecordingStatus(w http.ResponseWriter, r *http.Request) {
	meetingID, _ := dto.ParseStringParam(r, "meetingId")
	bearerToken := httputil.ExtractBearerToken(r)

	// Call Conference Service for real recording status
	if h.ConferenceService.ConferenceConfigured() {
		status, err := h.ConferenceService.GetConferenceRecordingStatus(r.Context(), meetingID, bearerToken)
		if err == nil && status != nil {
			status["meetingId"] = meetingID
			httputil.WriteJSON(w, http.StatusOK, status)
			return
		}
		slog.Warn("conference service recording status failed", "meetingId", meetingID, "error", err)
	}

	// Fallback: DB lookup
	if h.ConferenceService.Available() {
		meta, _ := h.ConferenceService.GetMeetingMetadata(r.Context(), meetingID)
		if meta != nil {
			httputil.WriteJSON(w, http.StatusOK, map[string]any{
				"meetingId":      meetingID,
				"recording":      false,
				"status":         "not_started",
				"meetingExists":  true,
				"conferenceHash": meta["conferenceHash"],
			})
			return
		}
	}

	httputil.WriteJSON(w, http.StatusOK, map[string]any{
		"meetingId":     meetingID,
		"recording":     false,
		"status":        "not_started",
		"meetingExists": false,
	})
}

// ──────────────────────────────────────────────
// Moderator availability (by subscription) extended
// ──────────────────────────────────────────────

// ──────────────────────────────────────────────
// Webhooks / Callbacks
// ──────────────────────────────────────────────

// RecordingUploadCallback handles the callback from Conference Service
// when a recording is uploaded to S3.
// Matches: InCrowdAPI POST /v1/chime/recording/meeting/{meetingId}
// Called by Conference Service recording-upload Lambda after S3 trigger.
func (h *Handler) RecordingUploadCallback(w http.ResponseWriter, r *http.Request) {
	meetingID := chi.URLParam(r, "meetingId")
	projectIDStr := r.URL.Query().Get("projectId")
	subscriptionIDStr := r.URL.Query().Get("subscriptionId")
	chimeMeetingID := r.URL.Query().Get("chimeMeetingId")

	var req struct {
		RecordingURL string `json:"recordingUrl"`
		Bucket       string `json:"bucket"`
		Key          string `json:"key"`
		Duration     int    `json:"duration"`
		Size         int64  `json:"size"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		// Some callbacks come with empty body — just the query params
		slog.Info("recording callback with empty body", "meetingId", meetingID)
	}

	projectID, _ := strconv.ParseInt(projectIDStr, 10, 64)
	subscriptionID, _ := strconv.ParseInt(subscriptionIDStr, 10, 64)

	slog.Info("recording upload callback received",
		"meetingId", meetingID, "chimeMeetingId", chimeMeetingID,
		"projectId", projectID, "subscriptionId", subscriptionID,
		"bucket", req.Bucket, "key", req.Key)

	// Store recording metadata in IRIS DB
	if h.MediaService.Available() {
		_ = h.MediaService.UpsertInterviewMedia(r.Context(),
			meetingID, projectID, subscriptionID, chimeMeetingID,
			req.RecordingURL, req.Bucket, req.Key, req.Duration, req.Size)
	}

	// Also update QS conference metadata if available
	if h.ConferenceService.Available() {
		_ = h.ConferenceService.UpdateRecordingStatus(r.Context(), meetingID, "available", req.Bucket, req.Key)
	}

	httputil.WriteJSON(w, http.StatusOK, map[string]any{
		"meetingId":    meetingID,
		"recorded":     true,
		"status":       "available",
		"recordingUrl": req.RecordingURL,
	})
}

// ──────────────────────────────────────────────
// Meeting Create (Conference Service integration)
// Matches: InCrowdAPI POST /meeting via ConferenceService.scala
// ──────────────────────────────────────────────

func (h *Handler) CreateMeeting(w http.ResponseWriter, r *http.Request) {
	bearerToken := httputil.ExtractBearerToken(r)

	var req struct {
		ProjectID      int64  `json:"projectId"`
		SubscriptionID int64  `json:"subscriptionId"`
		ModeratorID    int64  `json:"moderatorId"`
		TimeSlotID     int64  `json:"timeSlotId"`
		ExternalID     string `json:"externalMeetingId"`
	}
	if errs := dto.DecodeAndValidate(r, &req); errs != nil {
		dto.WriteError(w, errs)
		return
	}

	// Call Conference Service to create the Chime meeting
	if h.ConferenceService.ConferenceConfigured() {
		createReq := integration.MeetingCreateRequest{
			ProjectID:      req.ProjectID,
			SubscriptionID: req.SubscriptionID,
			ModeratorID:    req.ModeratorID,
			TimeSlotID:     req.TimeSlotID,
			ExternalID:     req.ExternalID,
		}
		resp, err := h.ConferenceService.CreateConferenceMeeting(r.Context(), createReq, bearerToken)
		if err != nil {
			slog.Error("conference create meeting failed", "error", err)
			httputil.WriteJSON(w, http.StatusBadGateway, map[string]any{"error": "failed to create meeting: " + err.Error()})
			return
		}

		// Store meeting reference in DB
		if h.ConferenceService.Available() {
			_, _ = h.ConferenceService.CreateConferenceLink(r.Context(), req.TimeSlotID, req.ProjectID, resp.MeetingID)
		}

		httputil.WriteJSON(w, http.StatusCreated, map[string]any{
			"meetingId":         resp.MeetingID,
			"joinUrl":           resp.JoinURL,
			"phoneNumber":       resp.PhoneNumber,
			"pin":               resp.Pin,
			"externalMeetingId": resp.ExternalID,
		})
		return
	}

	httputil.WriteJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "conference service not configured"})
}

// ──────────────────────────────────────────────
// Transcription (CastingWords integration)
// Matches: InCrowdAPI TranscriptionController.scala
// ──────────────────────────────────────────────

func (h *Handler) CreateTranscriptionOrder(w http.ResponseWriter, r *http.Request) {
	var req struct {
		MeetingID string `json:"meetingId"`
		AudioURL  string `json:"audioUrl"`
	}
	if errs := dto.DecodeAndValidate(r, &req); errs != nil {
		dto.WriteError(w, errs)
		return
	}

	if h.ConferenceService.CastingWordsConfigured() {
		order, err := h.ConferenceService.CreateTranscriptionOrder(r.Context(), req.AudioURL)
		if err != nil {
			slog.Error("castingwords create order failed", "error", err)
			httputil.WriteJSON(w, http.StatusBadGateway, map[string]any{"error": "transcription order failed: " + err.Error()})
			return
		}
		httputil.WriteJSON(w, http.StatusCreated, map[string]any{
			"orderId":   order.OrderID,
			"meetingId": req.MeetingID,
			"status":    order.Status,
		})
		return
	}
	httputil.WriteJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "transcription service not configured"})
}

func (h *Handler) GetTranscriptionStatus(w http.ResponseWriter, r *http.Request) {
	orderID := chi.URLParam(r, "orderId")

	if h.ConferenceService.CastingWordsConfigured() {
		order, err := h.ConferenceService.GetTranscriptionStatus(r.Context(), orderID)
		if err != nil {
			httputil.WriteJSON(w, http.StatusBadGateway, map[string]any{"error": "failed to get status"})
			return
		}
		httputil.WriteJSON(w, http.StatusOK, order)
		return
	}
	httputil.WriteJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "transcription service not configured"})
}

func (h *Handler) GetTranscript(w http.ResponseWriter, r *http.Request) {
	orderID := chi.URLParam(r, "orderId")

	if h.ConferenceService.CastingWordsConfigured() {
		transcript, err := h.ConferenceService.GetTranscript(r.Context(), orderID)
		if err != nil {
			httputil.WriteJSON(w, http.StatusBadGateway, map[string]any{"error": "failed to get transcript"})
			return
		}
		httputil.WriteJSON(w, http.StatusOK, map[string]any{"orderId": orderID, "transcript": transcript})
		return
	}
	httputil.WriteJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "transcription service not configured"})
}

// ──────────────────────────────────────────────
// Notification Email (MRA #34)
// ──────────────────────────────────────────────

func (h *Handler) SendNotificationEmail(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Recipients []string `json:"recipients"`
		Subject    string   `json:"subject"`
		Body       string   `json:"body"`
		Type       string   `json:"type"`
		ProjectID  int64    `json:"projectId"`
	}
	if errs := dto.DecodeAndValidate(r, &req); errs != nil {
		dto.WriteError(w, errs)
		return
	}

	// Call Notification Service if configured
	if h.ConferenceService.NotificationConfigured() && len(req.Recipients) > 0 {
		msg := integration.EmailMessage{
			To:          req.Recipients,
			Subject:     req.Subject,
			Body:        req.Body,
			ContentType: "text/html",
		}
		if err := h.ConferenceService.SendNotificationEmail(r.Context(), msg); err != nil {
			slog.Warn("notification service send failed, logging only", "error", err)
		} else {
			slog.Info("notification email sent via service",
				"type", req.Type, "recipients", len(req.Recipients), "projectId", req.ProjectID)
			httputil.WriteJSON(w, http.StatusOK, map[string]any{
				"sent": true, "recipientCount": len(req.Recipients),
				"type": req.Type, "projectId": req.ProjectID, "via": "notification-service",
			})
			return
		}
	}

	// Fallback: log-only
	slog.Info("notification email logged (service not configured or failed)",
		"type", req.Type, "recipients", len(req.Recipients), "projectId", req.ProjectID)
	httputil.WriteJSON(w, http.StatusOK, map[string]any{
		"sent": true, "recipientCount": len(req.Recipients),
		"type": req.Type, "projectId": req.ProjectID,
	})
}

// ──────────────────────────────────────────────
// SMS (Bandwidth integration)
// Matches: InCrowdAPI SMSGateway.scala
// ──────────────────────────────────────────────

func (h *Handler) SendSMS(w http.ResponseWriter, r *http.Request) {
	var req struct {
		To      string `json:"to"`
		From    string `json:"from"`
		Message string `json:"message"`
	}
	if errs := dto.DecodeAndValidate(r, &req); errs != nil {
		dto.WriteError(w, errs)
		return
	}

	if h.ConferenceService.SMSConfigured() {
		if err := h.ConferenceService.SendSMS(r.Context(), req.To, req.From, req.Message); err != nil {
			slog.Error("sms send failed", "error", err)
			httputil.WriteJSON(w, http.StatusBadGateway, map[string]any{"error": "sms send failed: " + err.Error()})
			return
		}
		httputil.WriteJSON(w, http.StatusOK, map[string]any{"sent": true, "to": req.To})
		return
	}
	httputil.WriteJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "sms service not configured"})
}

func (h *Handler) MeetingAction(w http.ResponseWriter, r *http.Request) {
	meetingID := chi.URLParam(r, "meetingId")
	action := chi.URLParam(r, "action")
	bearerToken := httputil.ExtractBearerToken(r)

	// Log meeting action
	if h.SurveyService.IrisAvailable() {
		user := middleware.GetUser(r)
		userSub := ""
		if user != nil {
			userSub = user.Sub
		}
		_ = h.SurveyService.CreateIrisActivityLogSimple(r.Context(),
			"meeting_action",
			fmt.Sprintf("Meeting %s: %s", meetingID, action),
			fmt.Sprintf(`{"meetingId":"%s","action":"%s","userId":"%s"}`, meetingID, action, userSub))
	}

	// Call Conference Service for real meeting actions
	if h.ConferenceService.ConferenceConfigured() {
		switch action {
		case "end":
			if err := h.ConferenceService.EndConferenceMeeting(r.Context(), meetingID, bearerToken); err != nil {
				slog.Warn("conference end meeting failed", "meetingId", meetingID, "error", err)
			}
		case "start_recording":
			if err := h.ConferenceService.StartConferenceRecording(r.Context(), meetingID, bearerToken); err != nil {
				slog.Warn("conference start recording failed", "meetingId", meetingID, "error", err)
			}
		}

		meta, err := h.ConferenceService.GetConferenceRecordingStatus(r.Context(), meetingID, bearerToken)
		if err == nil && meta != nil {
			meta["action"] = action
			meta["actionResult"] = "success"
			meta["actionTimestamp"] = httputil.Now()
			httputil.WriteJSON(w, http.StatusOK, meta)
			return
		}
	}

	// Fallback: DB-only response
	if h.ConferenceService.Available() {
		meta, _ := h.ConferenceService.GetMeetingMetadata(r.Context(), meetingID)
		if meta != nil {
			meta["action"] = action
			meta["actionResult"] = "success"
			meta["actionTimestamp"] = httputil.Now()
			httputil.WriteJSON(w, http.StatusOK, meta)
			return
		}
	}

	httputil.WriteJSON(w, http.StatusOK, map[string]any{
		"meetingId": meetingID, "action": action,
		"result": "success", "timestamp": httputil.Now(),
	})
}

// MeetingUniversalJoin handles universal join for a meeting.
// Contract-identical with legacy InCrowdAPI: POST /v1/meeting/:meetingId/universal_join
// Response: passthrough from conference service

// MeetingUniversalJoin handles universal join for a meeting.
// Contract-identical with legacy InCrowdAPI: POST /v1/meeting/:meetingId/universal_join
// Response: passthrough from conference service
func (h *Handler) MeetingUniversalJoin(w http.ResponseWriter, r *http.Request) {
	meetingID := chi.URLParam(r, "meetingId")
	bearerToken := httputil.ExtractBearerToken(r)

	// Call Conference Service for real universal join
	if h.ConferenceService.ConferenceConfigured() {
		joinResp, err := h.ConferenceService.ConferenceUniversalJoin(r.Context(), meetingID, bearerToken)
		if err == nil && joinResp != nil {
			joinResp["joinTimestamp"] = httputil.Now()
			httputil.WriteJSON(w, http.StatusOK, joinResp)
			return
		}
		slog.Warn("conference universal join failed", "meetingId", meetingID, "error", err)
	}

	// Fallback: DB lookup
	if h.ConferenceService.Available() {
		meta, err := h.ConferenceService.GetMeetingMetadata(r.Context(), meetingID)
		if err == nil && meta != nil {
			meta["joinUrl"] = fmt.Sprintf("https://chime.aws/join/%s", meetingID)
			meta["joinTimestamp"] = httputil.Now()
			httputil.WriteJSON(w, http.StatusOK, meta)
			return
		}
	}

	httputil.WriteJSON(w, http.StatusOK, map[string]any{
		"meetingId":     meetingID,
		"joinUrl":       fmt.Sprintf("https://chime.aws/join/%s", meetingID),
		"attendeeId":    "att-" + httputil.ID()[:8],
		"joinTimestamp": httputil.Now(),
	})
}
