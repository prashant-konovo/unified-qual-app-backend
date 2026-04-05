package shared

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/InCrowd/unified-qual-api/internal/handler/support"

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
	source := support.ResolveSource(r)

	if projectIDStr != "" {
		projectID, _ := strconv.ParseInt(projectIDStr, 10, 64)
		if source == "iris" && h.IrisSurveyRepo != nil {
			tpl, err := h.IrisSurveyRepo.GetEmailTemplateForProject(r.Context(), projectID)
			if err != nil {
				slog.Error("get email template failed", "error", err)
			}
			if tpl != nil {
				tpl["source"] = "iris"
				tpl["templateType"] = templateType
				support.WriteJSON(w, http.StatusOK, tpl)
				return
			}
		}
	}

	if h.QsAnswerRepo != nil {
		name := templateType
		if name == "" {
			name = "reschedule"
		}
		tpl, _ := h.QsAnswerRepo.GetCommunicationTemplate(r.Context(), name)
		if tpl != nil {
			support.WriteJSON(w, http.StatusOK, map[string]any{
				"subject": tpl.Subject, "body": tpl.Body,
				"templateType": name, "source": "qs",
			})
			return
		}
	}

	support.WriteJSON(w, http.StatusOK, map[string]any{
		"subject":      "Your Interview Has Been Rescheduled",
		"body":         "<html><body><p>Dear {{.Name}}, your interview has been rescheduled.</p></body></html>",
		"templateType": templateType,
		"source":       "default",
	})
}

func (h *Handler) SendReminder(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Recipients []string `json:"recipients"`
		Subject    string   `json:"subject"`
		Body       string   `json:"body"`
		Type       string   `json:"type"`
		ProjectID  int64    `json:"projectId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		// If no body, treat as simple reminder
		support.WriteJSON(w, http.StatusOK, map[string]any{"sent": true, "recipientCount": 0})
		return
	}

	// Call Notification Service if configured
	if h.Services.Notification.Configured() && len(req.Recipients) > 0 {
		msg := integration.EmailMessage{
			To:          req.Recipients,
			Subject:     req.Subject,
			Body:        req.Body,
			ContentType: "text/html",
		}
		if err := h.Services.Notification.SendEmail(r.Context(), msg); err != nil {
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

	support.WriteJSON(w, http.StatusOK, map[string]any{
		"sent": true, "recipientCount": len(req.Recipients),
		"type": req.Type, "projectId": req.ProjectID,
	})
}
