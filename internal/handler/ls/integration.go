package ls

import (
	"log/slog"
	"net/http"

	"github.com/InCrowd/unified-qual-api/internal/handler/core"

	"github.com/InCrowd/unified-qual-api/internal/validate"
)

// ──────────────────────────────────────────────
// LS Integration handlers
// ──────────────────────────────────────────────

// ──────────────────────────────────────────────
// Third-Party Integration (MRA #61)
// ──────────────────────────────────────────────

func (h *Handler) ThirdPartyIntegrate(w http.ResponseWriter, r *http.Request) {
	var req map[string]any
	if errs := validate.DecodeAndValidate(r, &req); errs != nil {
		validate.WriteError(w, errs)
		return
	}

	// If the request includes surveyId, try Decipher integration
	if surveyID, ok := req["surveyId"].(string); ok && surveyID != "" && h.Services.Decipher.Configured() {
		data, err := h.Services.Decipher.GetRespondentData(r.Context(), surveyID)
		if err != nil {
			slog.Warn("decipher respondent data failed", "surveyId", surveyID, "error", err)
		} else {
			slog.Info("decipher data retrieved", "surveyId", surveyID, "records", len(data))
			core.WriteJSON(w, http.StatusOK, map[string]any{
				"accepted":    true,
				"source":      "decipher",
				"surveyId":    surveyID,
				"recordCount": len(data),
				"timestamp":   core.Now(),
			})
			return
		}
	}

	// Log event if event logging is configured
	if h.Services.EventLog.Configured() {
		_ = h.Services.EventLog.LogEvent(r.Context(), "third_party_integrate", "Third-party integration request", req)
	}

	slog.Info("third-party integration received", "payload_keys", len(req))
	core.WriteJSON(w, http.StatusOK, map[string]any{"accepted": true, "timestamp": core.Now()})
}

// ──────────────────────────────────────────────
// Qual Eligibility (MRA #63)
// ──────────────────────────────────────────────

func (h *Handler) CheckQualEligibility(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ResponderID int64 `json:"responderId"`
		ProjectID   int64 `json:"projectId"`
	}
	if errs := validate.DecodeAndValidate(r, &req); errs != nil {
		validate.WriteError(w, errs)
		return
	}

	if h.QsAnswerRepo != nil {
		result, err := h.QsAnswerRepo.GetParticipantEligibility(r.Context(), req.ResponderID, req.ProjectID)
		if err != nil {
			slog.Error("eligibility check failed", "error", err)
			core.WriteJSON(w, http.StatusInternalServerError, map[string]any{"error": "check failed"})
			return
		}
		if result == nil {
			core.WriteJSON(w, http.StatusOK, map[string]any{
				"eligible": true, "responderId": req.ResponderID,
				"projectId": req.ProjectID, "source": "qs",
			})
			return
		}
		result["source"] = "qs"
		core.WriteJSON(w, http.StatusOK, result)
		return
	}

	core.WriteJSON(w, http.StatusOK, map[string]any{"eligible": true, "responderId": req.ResponderID, "projectId": req.ProjectID})
}
