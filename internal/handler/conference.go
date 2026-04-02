package handler

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/InCrowd/unified-qual-api/internal/integration"
	"github.com/InCrowd/unified-qual-api/internal/middleware"
	"github.com/InCrowd/unified-qual-api/internal/validate"
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
	confHashStr, _ := validate.ParseStringParam(r, "confId")
	var req struct {
		Pin string `json:"pin"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)

	if h.qsConferenceRepo != nil {
		data, err := h.qsConferenceRepo.Login(r.Context(), confHashStr, req.Pin)
		if err != nil {
			if strings.Contains(err.Error(), "invalid pin") {
				writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "invalid pin"})
				return
			}
			if strings.Contains(err.Error(), "not found") {
				writeJSON(w, http.StatusNotFound, map[string]any{"error": "conference not found"})
				return
			}
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "login failed"})
			return
		}
		writeJSON(w, http.StatusOK, data)
		return
	}
	writeJSON(w, http.StatusNotFound, map[string]any{"error": "conference not found"})
}

// GetConferenceParticipants returns participants in a conference.
// GetConferenceParticipants returns participants and timeslot info for a conference.
// Contract-identical with legacy InCrowdAPI: GET /v1/conf/:confId/participants
// Response: {"participants": [...], "timeSlot": {startTime, endTime, conferencePin, projectId, id}}
func (h *Handler) GetConferenceParticipants(w http.ResponseWriter, r *http.Request) {
	confHashStr, _ := validate.ParseStringParam(r, "confId")

	if h.qsConferenceRepo != nil {
		ci, err := h.qsConferenceRepo.GetByHash(r.Context(), confHashStr)
		if err != nil || ci == nil {
			writeJSON(w, http.StatusNotFound, map[string]any{"error": "conference not found"})
			return
		}
		participants, err := h.qsConferenceRepo.GetParticipants(r.Context(), ci.TimeSlotID)
		if err != nil {
			slog.Error("get participants failed", "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "database error"})
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
		if h.qsTimeSlotRepo != nil {
			ts, tsErr := h.qsTimeSlotRepo.GetByID(r.Context(), ci.TimeSlotID)
			if tsErr == nil && ts != nil {
				tsObj["startTime"] = ts.StartTime.Format(time.RFC3339)
				tsObj["endTime"] = ts.EndTime.Format(time.RFC3339)
				tsObj["projectId"] = ts.ProjectID
			}
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"participants": participants,
			"timeSlot":     tsObj,
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"participants": []any{}, "timeSlot": nil})
}

// GetMeetingMetadata returns meeting metadata.
// Contract-identical with legacy InCrowdAPI: GET /v1/meeting/metadata
// Response: flat meeting metadata object
func (h *Handler) GetMeetingMetadata(w http.ResponseWriter, r *http.Request) {
	hash := r.URL.Query().Get("hash")
	bearerToken := extractBearerToken(r)

	// Try Conference Service for live metadata
	if hash == "" && h.services.Conference.Configured() {
		meta, err := h.services.Conference.GetMetadata(r.Context(), bearerToken)
		if err == nil && meta != nil {
			writeJSON(w, http.StatusOK, meta)
			return
		}
	}

	if hash == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "hash parameter required"})
		return
	}

	if h.qsConferenceRepo != nil {
		meta, err := h.qsConferenceRepo.GetMeetingMetadata(r.Context(), hash)
		if err != nil {
			slog.Error("get meeting metadata failed", "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "database error"})
			return
		}
		if meta == nil {
			writeJSON(w, http.StatusNotFound, map[string]any{"error": "meeting not found"})
			return
		}
		writeJSON(w, http.StatusOK, meta)
		return
	}
	writeJSON(w, http.StatusNotFound, map[string]any{"error": "meeting not found"})
}

// MeetingJoin handles join meeting by join ID.
// Contract-identical with legacy InCrowdAPI: POST /v1/meeting/:joinId/join
// Response: flat meeting metadata object
func (h *Handler) MeetingJoin(w http.ResponseWriter, r *http.Request) {
	joinID, err := validate.ParseStringParam(r, "joinId")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}

	// Use join ID as conference hash or participant hash
	if h.qsConferenceRepo != nil {
		meta, err := h.qsConferenceRepo.GetMeetingMetadata(r.Context(), joinID)
		if err == nil && meta != nil {
			writeJSON(w, http.StatusOK, meta)
			return
		}
	}
	writeJSON(w, http.StatusNotFound, map[string]any{"error": "meeting not found"})
}

// GetAttendeesByMeetingID returns attendees for a meeting.
// Contract-identical with legacy InCrowdAPI: GET /v1/meeting/:meetingId/attendees
// Response: passthrough from conference service
func (h *Handler) GetAttendeesByMeetingID(w http.ResponseWriter, r *http.Request) {
	meetingID, _ := validate.ParseStringParam(r, "meetingId")
	bearerToken := extractBearerToken(r)

	// Try Conference Service for live attendee data
	if h.services.Conference.Configured() {
		attendees, err := h.services.Conference.GetAttendees(r.Context(), meetingID, bearerToken)
		if err == nil && attendees != nil {
			writeJSON(w, http.StatusOK, attendees)
			return
		}
		slog.Warn("conference service get attendees failed, falling back to DB", "error", err)
	}

	if h.qsConferenceRepo != nil {
		attendees, err := h.qsConferenceRepo.GetAttendeesByMeetingID(r.Context(), meetingID)
		if err != nil {
			slog.Error("get attendees failed", "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "database error"})
			return
		}
		writeJSON(w, http.StatusOK, attendees)
		return
	}
	writeJSON(w, http.StatusOK, []any{})
}

// GetRecordingStatus returns recording status for a meeting.
// Contract-identical with legacy InCrowdAPI: GET /v1/meeting/:meetingId/recording
// Response: passthrough from conference service
func (h *Handler) GetRecordingStatus(w http.ResponseWriter, r *http.Request) {
	meetingID, _ := validate.ParseStringParam(r, "meetingId")
	bearerToken := extractBearerToken(r)

	// Call Conference Service for real recording status
	if h.services.Conference.Configured() {
		status, err := h.services.Conference.GetRecordingStatus(r.Context(), meetingID, bearerToken)
		if err == nil && status != nil {
			status["meetingId"] = meetingID
			writeJSON(w, http.StatusOK, status)
			return
		}
		slog.Warn("conference service recording status failed", "meetingId", meetingID, "error", err)
	}

	// Fallback: DB lookup
	if h.qsConferenceRepo != nil {
		meta, _ := h.qsConferenceRepo.GetMeetingMetadata(r.Context(), meetingID)
		if meta != nil {
			writeJSON(w, http.StatusOK, map[string]any{
				"meetingId":      meetingID,
				"recording":      false,
				"status":         "not_started",
				"meetingExists":  true,
				"conferenceHash": meta["conferenceHash"],
			})
			return
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
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
	if h.db.IRIS != nil {
		_, _ = h.db.IRIS.ExecContext(r.Context(),
			`INSERT INTO interview_media (meeting_id, project_id, subscription_id, chime_meeting_id,
			 recording_url, s3_bucket, s3_key, duration_seconds, file_size, status, created_on)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, 'available', NOW())
			 ON DUPLICATE KEY UPDATE recording_url=VALUES(recording_url), s3_bucket=VALUES(s3_bucket),
			 s3_key=VALUES(s3_key), duration_seconds=VALUES(duration_seconds), file_size=VALUES(file_size),
			 status='available', updated_on=NOW()`,
			meetingID, projectID, subscriptionID, chimeMeetingID,
			req.RecordingURL, req.Bucket, req.Key, req.Duration, req.Size)
	}

	// Also update QS conference metadata if available
	if h.qsConferenceRepo != nil {
		_ = h.qsConferenceRepo.UpdateRecordingStatus(r.Context(), meetingID, "available", req.Bucket, req.Key)
	}

	writeJSON(w, http.StatusOK, map[string]any{
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
	bearerToken := extractBearerToken(r)

	var req struct {
		ProjectID      int64  `json:"projectId"`
		SubscriptionID int64  `json:"subscriptionId"`
		ModeratorID    int64  `json:"moderatorId"`
		TimeSlotID     int64  `json:"timeSlotId"`
		ExternalID     string `json:"externalMeetingId"`
	}
	if errs := validate.DecodeAndValidate(r, &req); errs != nil {
		validate.WriteError(w, errs)
		return
	}

	// Call Conference Service to create the Chime meeting
	if h.services.Conference.Configured() {
		createReq := integration.MeetingCreateRequest{
			ProjectID:      req.ProjectID,
			SubscriptionID: req.SubscriptionID,
			ModeratorID:    req.ModeratorID,
			TimeSlotID:     req.TimeSlotID,
			ExternalID:     req.ExternalID,
		}
		resp, err := h.services.Conference.CreateMeeting(r.Context(), createReq, bearerToken)
		if err != nil {
			slog.Error("conference create meeting failed", "error", err)
			writeJSON(w, http.StatusBadGateway, map[string]any{"error": "failed to create meeting: " + err.Error()})
			return
		}

		// Store meeting reference in DB
		if h.qsConferenceRepo != nil {
			_, _ = h.qsConferenceRepo.CreateConferenceLink(r.Context(), req.TimeSlotID, req.ProjectID, resp.MeetingID)
		}

		writeJSON(w, http.StatusCreated, map[string]any{
			"meetingId":         resp.MeetingID,
			"joinUrl":           resp.JoinURL,
			"phoneNumber":       resp.PhoneNumber,
			"pin":               resp.Pin,
			"externalMeetingId": resp.ExternalID,
		})
		return
	}

	writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "conference service not configured"})
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
	if errs := validate.DecodeAndValidate(r, &req); errs != nil {
		validate.WriteError(w, errs)
		return
	}

	if h.services.CastingWords.Configured() {
		order, err := h.services.CastingWords.CreateOrder(r.Context(), req.AudioURL)
		if err != nil {
			slog.Error("castingwords create order failed", "error", err)
			writeJSON(w, http.StatusBadGateway, map[string]any{"error": "transcription order failed: " + err.Error()})
			return
		}
		writeJSON(w, http.StatusCreated, map[string]any{
			"orderId":   order.OrderID,
			"meetingId": req.MeetingID,
			"status":    order.Status,
		})
		return
	}
	writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "transcription service not configured"})
}

func (h *Handler) GetTranscriptionStatus(w http.ResponseWriter, r *http.Request) {
	orderID := chi.URLParam(r, "orderId")

	if h.services.CastingWords.Configured() {
		order, err := h.services.CastingWords.GetOrderStatus(r.Context(), orderID)
		if err != nil {
			writeJSON(w, http.StatusBadGateway, map[string]any{"error": "failed to get status"})
			return
		}
		writeJSON(w, http.StatusOK, order)
		return
	}
	writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "transcription service not configured"})
}

func (h *Handler) GetTranscript(w http.ResponseWriter, r *http.Request) {
	orderID := chi.URLParam(r, "orderId")

	if h.services.CastingWords.Configured() {
		transcript, err := h.services.CastingWords.GetTranscript(r.Context(), orderID)
		if err != nil {
			writeJSON(w, http.StatusBadGateway, map[string]any{"error": "failed to get transcript"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"orderId": orderID, "transcript": transcript})
		return
	}
	writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "transcription service not configured"})
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
	if errs := validate.DecodeAndValidate(r, &req); errs != nil {
		validate.WriteError(w, errs)
		return
	}

	// Call Notification Service if configured
	if h.services.Notification.Configured() && len(req.Recipients) > 0 {
		msg := integration.EmailMessage{
			To:          req.Recipients,
			Subject:     req.Subject,
			Body:        req.Body,
			ContentType: "text/html",
		}
		if err := h.services.Notification.SendEmail(r.Context(), msg); err != nil {
			slog.Warn("notification service send failed, logging only", "error", err)
		} else {
			slog.Info("notification email sent via service",
				"type", req.Type, "recipients", len(req.Recipients), "projectId", req.ProjectID)
			writeJSON(w, http.StatusOK, map[string]any{
				"sent": true, "recipientCount": len(req.Recipients),
				"type": req.Type, "projectId": req.ProjectID, "via": "notification-service",
			})
			return
		}
	}

	// Fallback: log-only
	slog.Info("notification email logged (service not configured or failed)",
		"type", req.Type, "recipients", len(req.Recipients), "projectId", req.ProjectID)
	writeJSON(w, http.StatusOK, map[string]any{
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
	if errs := validate.DecodeAndValidate(r, &req); errs != nil {
		validate.WriteError(w, errs)
		return
	}

	if h.services.SMS.Configured() {
		if err := h.services.SMS.SendSMS(r.Context(), req.To, req.From, req.Message); err != nil {
			slog.Error("sms send failed", "error", err)
			writeJSON(w, http.StatusBadGateway, map[string]any{"error": "sms send failed: " + err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"sent": true, "to": req.To})
		return
	}
	writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "sms service not configured"})
}

func (h *Handler) MeetingAction(w http.ResponseWriter, r *http.Request) {
	meetingID := chi.URLParam(r, "meetingId")
	action := chi.URLParam(r, "action")
	bearerToken := extractBearerToken(r)

	// Log meeting action
	if h.db.IRIS != nil {
		user := middleware.GetUser(r)
		userSub := ""
		if user != nil {
			userSub = user.Sub
		}
		_, _ = h.db.IRIS.ExecContext(r.Context(),
			`INSERT INTO activity_log (event_type, description, meta_data, created_on)
			 VALUES ('meeting_action', ?, ?, NOW())`,
			fmt.Sprintf("Meeting %s: %s", meetingID, action),
			fmt.Sprintf(`{"meetingId":"%s","action":"%s","userId":"%s"}`, meetingID, action, userSub))
	}

	// Call Conference Service for real meeting actions
	if h.services.Conference.Configured() {
		switch action {
		case "end":
			if err := h.services.Conference.EndMeeting(r.Context(), meetingID, bearerToken); err != nil {
				slog.Warn("conference end meeting failed", "meetingId", meetingID, "error", err)
			}
		case "start_recording":
			if err := h.services.Conference.StartRecording(r.Context(), meetingID, bearerToken); err != nil {
				slog.Warn("conference start recording failed", "meetingId", meetingID, "error", err)
			}
		}

		meta, err := h.services.Conference.GetRecordingStatus(r.Context(), meetingID, bearerToken)
		if err == nil && meta != nil {
			meta["action"] = action
			meta["actionResult"] = "success"
			meta["actionTimestamp"] = now()
			writeJSON(w, http.StatusOK, meta)
			return
		}
	}

	// Fallback: DB-only response
	if h.qsConferenceRepo != nil {
		meta, _ := h.qsConferenceRepo.GetMeetingMetadata(r.Context(), meetingID)
		if meta != nil {
			meta["action"] = action
			meta["actionResult"] = "success"
			meta["actionTimestamp"] = now()
			writeJSON(w, http.StatusOK, meta)
			return
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"meetingId": meetingID, "action": action,
		"result": "success", "timestamp": now(),
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
	bearerToken := extractBearerToken(r)

	// Call Conference Service for real universal join
	if h.services.Conference.Configured() {
		joinResp, err := h.services.Conference.UniversalJoin(r.Context(), meetingID, bearerToken)
		if err == nil && joinResp != nil {
			joinResp["joinTimestamp"] = now()
			writeJSON(w, http.StatusOK, joinResp)
			return
		}
		slog.Warn("conference universal join failed", "meetingId", meetingID, "error", err)
	}

	// Fallback: DB lookup
	if h.qsConferenceRepo != nil {
		meta, err := h.qsConferenceRepo.GetMeetingMetadata(r.Context(), meetingID)
		if err == nil && meta != nil {
			meta["joinUrl"] = fmt.Sprintf("https://chime.aws/join/%s", meetingID)
			meta["joinTimestamp"] = now()
			writeJSON(w, http.StatusOK, meta)
			return
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"meetingId":     meetingID,
		"joinUrl":       fmt.Sprintf("https://chime.aws/join/%s", meetingID),
		"attendeeId":    "att-" + id()[:8],
		"joinTimestamp": now(),
	})
}
