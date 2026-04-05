package mra

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"


	"github.com/InCrowd/unified-qual-api/internal/dto"
	"github.com/InCrowd/unified-qual-api/internal/httpkit"
)

// ──────────────────────────────────────────────
// MRA Integration handlers
// ──────────────────────────────────────────────

// ──────────────────────────────────────────────
// MRA #61 — ThirdPartyIntegrateMRA
// POST /v1/third-party-integrate (MRA route)
// Legacy: test/mock endpoint that inserts a respondent with hardcoded values.
// ──────────────────────────────────────────────

func (h *Handler) ThirdPartyIntegrateMRA(w http.ResponseWriter, r *http.Request) {
	if !h.ParticipantService.Available() {
		httpkit.WriteJSON(w, http.StatusInternalServerError, map[string]any{
			"error":        "repository not available",
			"errorMessage": "An error occured in third party integration",
		})
		return
	}

	// Parse body (legacy parses but ignores values)
	var body map[string]any
	_ = json.NewDecoder(r.Body).Decode(&body)

	// Legacy uses hardcoded test values
	insertID, err := h.ParticipantService.CreateRespondentMRA(
		r.Context(),
		"Mohammed",    // firstName
		"Abadi",       // lastName
		"Mr",          // title
		"testId",      // externalResponderId
		"sesskeymock", // sessKey
		"",            // timeZone
		"",            // timeZoneAbbr
		"",            // languageCountry
	)
	if err != nil {
		slog.Error("third party integrate mra failed", "error", err)
		httpkit.WriteJSON(w, http.StatusInternalServerError, map[string]any{
			"error":        err.Error(),
			"errorMessage": "An error occured in third party integration",
		})
		return
	}

	httpkit.WriteJSON(w, http.StatusOK, map[string]any{
		"responder":    insertID,
		"QContactr6":   fmt.Sprintf("walid.samaha.01+%d@gmail.com", insertID),
		"QContactr7":   "81702668",
		"identifier":   "testId",
		"QContactr2":   "Abadi",
		"QContactr1":   "Mohammed",
		"honorarium":   "testHonarrium",
		"CurrencyText": "USD",
		"sessKey":      "sesskeymock",
	})
}

// ──────────────────────────────────────────────
// MRA #62 — LogFrontEndEventMRA
// POST /v1/log-front-end-event (MRA route)
// Legacy: logs event to CloudWatch and returns 200 with no body.
// ──────────────────────────────────────────────

func (h *Handler) LogFrontEndEventMRA(w http.ResponseWriter, r *http.Request) {
	var body dto.MraLogFrontEndEventRequest
	if errs := httpkit.DecodeAndValidate(r, &body); errs != nil {
		httpkit.WriteError(w, errs)
		return
	}

	slog.Info("front-end event",
		"eventName", body.EventName,
		"error", body.Error,
		"statusCode", body.StatusCode,
		"surveyId", body.SurveyID,
		"decipherSurveyId", body.DecipherSurveyID,
		"projectId", body.ProjectID,
		"shgHash", body.ShgHash,
		"qsPath", body.QsPath,
		"respondentIdentifer", body.RespondentIdentifer,
	)

	// Legacy returns 200 with no body
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Brand", "unified")
	w.WriteHeader(http.StatusOK)
}

// ──────────────────────────────────────────────
// MRA #63 — QualEligibilityMRA
// POST /v1/qual/eligibility (MRA route)
// Legacy: bulk eligibility status update with upsert.
// ──────────────────────────────────────────────

func (h *Handler) QualEligibilityMRA(w http.ResponseWriter, r *http.Request) {
	if !h.ParticipantService.TimeSlotAvailable() {
		httpkit.WriteJSON(w, http.StatusInternalServerError, map[string]any{
			"error":        "repository not available",
			"errorMessage": "repository not available",
		})
		return
	}

	var body dto.MraQualEligibilityRequest
	if errs := httpkit.DecodeAndValidate(r, &body); errs != nil {
		httpkit.WriteError(w, errs)
		return
	}

	// Validation
	var validationErrors []string
	if len(body.ParticipantIDs) == 0 {
		validationErrors = append(validationErrors, "participant_ids must be a non-empty array")
	}
	if body.UpdatedBy == "" {
		validationErrors = append(validationErrors, "updated_by is required")
	}
	if body.Reason == "" {
		validationErrors = append(validationErrors, "reason is required")
	}
	statusUpper := strings.ToUpper(body.Status)
	if statusUpper != "ELIGIBLE" && statusUpper != "INELIGIBLE" {
		validationErrors = append(validationErrors, "status must be ELIGIBLE or INELIGIBLE")
	}
	if len(validationErrors) > 0 {
		httpkit.WriteJSON(w, http.StatusBadRequest, map[string]any{
			"error": strings.Join(validationErrors, ", "),
		})
		return
	}

	isEligible := statusUpper == "ELIGIBLE"
	ctx := r.Context()

	var processedIDs []any
	var failedIDs []any

	for _, pid := range body.ParticipantIDs {
		// Convert participant_id to string (may arrive as number or string)
		pidStr := fmt.Sprintf("%v", pid)

		if err := h.ParticipantService.UpsertEligibilityStatusMRA(ctx, pidStr, isEligible, body.Reason, body.UpdatedBy); err != nil {
			slog.Error("upsert eligibility status failed", "participantId", pidStr, "error", err)
			failedIDs = append(failedIDs, pid)
			continue
		}

		// Reset ineligible mail flag when marking ELIGIBLE
		if isEligible {
			if err := h.ParticipantService.ResetIneligibleMailSentMRA(ctx, pidStr); err != nil {
				slog.Error("reset ineligible mail sent failed", "participantId", pidStr, "error", err)
			}
		}

		// PARTIAL: Legacy publishes EventBridge events for INELIGIBLE participants
		if !isEligible {
			slog.Info("QualEligibilityMRA: EventBridge publish skipped (PARTIAL)", "participantId", pidStr, "status", statusUpper)
		}

		processedIDs = append(processedIDs, pid)
	}

	if processedIDs == nil {
		processedIDs = []any{}
	}
	if failedIDs == nil {
		failedIDs = []any{}
	}

	httpkit.WriteJSON(w, http.StatusOK, map[string]any{
		"success":       len(failedIDs) == 0,
		"processed_ids": processedIDs,
		"failed_ids":    failedIDs,
	})
}
