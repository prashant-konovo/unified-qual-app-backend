package ls

import (
	"log/slog"
	"net/http"


	"github.com/InCrowd/unified-qual-api/internal/dto"
	"github.com/InCrowd/unified-qual-api/internal/utilities"
)

// ──────────────────────────────────────────────
// LS Integration handlers
// ──────────────────────────────────────────────

// ──────────────────────────────────────────────
// Third-Party Integration (MRA #61)
// ──────────────────────────────────────────────

func (h *Handler) ThirdPartyIntegrate(w http.ResponseWriter, r *http.Request) {
	var req map[string]any
	if errs := utilities.DecodeAndValidate(r, &req); errs != nil {
		utilities.WriteError(w, errs)
		return
	}

	// If the request includes surveyId, try Decipher integration
	if surveyID, ok := req["surveyId"].(string); ok && surveyID != "" && h.SurveyService.DecipherConfigured() {
		data, err := h.SurveyService.GetDecipherRespondentData(r.Context(), surveyID)
		if err != nil {
			slog.Warn("decipher respondent data failed", "surveyId", surveyID, "error", err)
		} else {
			slog.Info("decipher data retrieved", "surveyId", surveyID, "records", len(data))
			utilities.WriteJSON(w, http.StatusOK, map[string]any{
				"accepted":    true,
				"source":      "decipher",
				"surveyId":    surveyID,
				"recordCount": len(data),
				"timestamp":   utilities.Now(),
			})
			return
		}
	}

	// Log event if event logging is configured
	if h.SurveyService.EventLogConfigured() {
		_ = h.SurveyService.LogEvent(r.Context(), "third_party_integrate", "Third-party integration request", req)
	}

	slog.Info("third-party integration received", "payload_keys", len(req))
	utilities.WriteJSON(w, http.StatusOK, map[string]any{"accepted": true, "timestamp": utilities.Now()})
}

// ──────────────────────────────────────────────
// Qual Eligibility (MRA #63)
// ──────────────────────────────────────────────

func (h *Handler) CheckQualEligibility(w http.ResponseWriter, r *http.Request) {
	var req dto.LsCheckQualEligibilityRequest
	if errs := utilities.DecodeAndValidate(r, &req); errs != nil {
		utilities.WriteError(w, errs)
		return
	}

	if h.TranslationService.AnswerRepoAvailable() {
		result, err := h.TranslationService.GetParticipantEligibility(r.Context(), req.ResponderID, req.ProjectID)
		if err != nil {
			slog.Error("eligibility check failed", "error", err)
			utilities.WriteJSON(w, http.StatusInternalServerError, map[string]any{"error": "check failed"})
			return
		}
		if result == nil {
			utilities.WriteJSON(w, http.StatusOK, map[string]any{
				"eligible": true, "responderId": req.ResponderID,
				"projectId": req.ProjectID, "source": "qs",
			})
			return
		}
		result["source"] = "qs"
		utilities.WriteJSON(w, http.StatusOK, result)
		return
	}

	utilities.WriteJSON(w, http.StatusOK, map[string]any{"eligible": true, "responderId": req.ResponderID, "projectId": req.ProjectID})
}
