package shared

import (
	"github.com/InCrowd/unified-qual-api/internal/dto"
	"github.com/InCrowd/unified-qual-api/internal/httpkit"
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"


	"github.com/InCrowd/unified-qual-api/internal/integration"
)

// ──────────────────────────────────────────────
// Notification handlers (shared)
// ──────────────────────────────────────────────

// ──────────────────────────────────────────────
// Notifications (LLD endpoints)
// ──────────────────────────────────────────────

func (h *Handler) GetEmailTemplate(w http.ResponseWriter, r *http.Request) {
	projectIDStr := r.URL.Query().Get("projectId")
	templateType := r.URL.Query().Get("type")
	source := httpkit.ResolveSource(r)

	if projectIDStr != "" {
		projectID, _ := strconv.ParseInt(projectIDStr, 10, 64)
		if source == "iris" && h.NotificationService.IrisAvailable() {
			tpl, err := h.NotificationService.GetEmailTemplateForProject(r.Context(), projectID)
			if err != nil {
				slog.Error("get email template failed", "error", err)
			}
			if tpl != nil {
				tpl["source"] = "iris"
				tpl["templateType"] = templateType
				httpkit.WriteJSON(w, http.StatusOK, tpl)
				return
			}
		}
	}

	if h.NotificationService.QsAvailable() {
		name := templateType
		if name == "" {
			name = "reschedule"
		}
		tpl, _ := h.NotificationService.GetCommunicationTemplate(r.Context(), name)
		if tpl != nil {
			httpkit.WriteJSON(w, http.StatusOK, map[string]any{
				"subject": tpl.Subject, "body": tpl.Body,
				"templateType": name, "source": "qs",
			})
			return
		}
	}

	httpkit.WriteJSON(w, http.StatusOK, map[string]any{
		"subject":      "Your Interview Has Been Rescheduled",
		"body":         "<html><body><p>Dear {{.Name}}, your interview has been rescheduled.</p></body></html>",
		"templateType": templateType,
		"source":       "default",
	})
}

func (h *Handler) SendReminder(w http.ResponseWriter, r *http.Request) {
	var req dto.SendReminderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		// If no body, treat as simple reminder
		httpkit.WriteJSON(w, http.StatusOK, map[string]any{"sent": true, "recipientCount": 0})
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
			slog.Warn("notification service send reminder failed", "error", err)
			// Don't fail the request — log and continue with DB fallback
		} else {
			slog.Info("reminder sent via notification service",
				"type", req.Type, "recipients", len(req.Recipients), "projectId", req.ProjectID)
		}
	} else {
		slog.Info("reminder logged (notification service not configured)",
			"type", req.Type, "recipients", len(req.Recipients), "projectId", req.ProjectID)
	}

	httpkit.WriteJSON(w, http.StatusOK, map[string]any{
		"sent": true, "recipientCount": len(req.Recipients),
		"type": req.Type, "projectId": req.ProjectID,
	})
}
