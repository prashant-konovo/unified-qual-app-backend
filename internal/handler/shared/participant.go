package shared

import (
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/InCrowd/unified-qual-api/internal/handler/core"

	"github.com/InCrowd/unified-qual-api/internal/repository/qs"
	"github.com/InCrowd/unified-qual-api/internal/validate"
)

// ──────────────────────────────────────────────
// Participant handlers (shared)
// ──────────────────────────────────────────────

// ──────────────────────────────────────────────
// Participants
// ──────────────────────────────────────────────

func (h *Handler) ListParticipants(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if h.QsRespondentRepo == nil {
		core.WriteJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "QS database unavailable"})
		return
	}

	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("pageSize"))
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	search := strings.TrimSpace(r.URL.Query().Get("search"))

	respondents, total, err := h.QsRespondentRepo.List(ctx, page, pageSize, search)
	if err != nil {
		slog.ErrorContext(ctx, "list participants failed", "error", err)
		core.WriteJSON(w, http.StatusInternalServerError, map[string]any{"error": "failed to list participants"})
		return
	}

	result := make([]map[string]any, 0, len(respondents))
	for _, r := range respondents {
		item := map[string]any{
			"id":              r.ID,
			"firstName":       r.FirstName,
			"lastName":        r.LastName,
			"name":            strings.TrimSpace(r.FirstName + " " + r.LastName),
			"source":          "qs",
			"serviceCategory": "MRA",
			"modifiedOn":      r.ModifiedOn.Format(time.RFC3339),
		}
		if r.Title.Valid {
			item["title"] = r.Title.String
		}
		if r.ExternalResponderID.Valid {
			item["externalResponderId"] = r.ExternalResponderID.String
		}
		if r.TimeZone.Valid {
			item["timeZone"] = r.TimeZone.String
		}
		if r.Email.Valid {
			item["email"] = r.Email.String
		}
		if r.Phone.Valid {
			item["phone"] = r.Phone.String
		}
		result = append(result, item)
	}

	core.WriteJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"data":    result,
		"meta": map[string]any{
			"page":       page,
			"pageSize":   pageSize,
			"totalCount": total,
		},
	})
}

func (h *Handler) CreateParticipant(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if h.QsRespondentRepo == nil {
		core.WriteJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "QS database unavailable"})
		return
	}

	var body struct {
		FirstName           string `json:"firstName"           validate:"required"`
		LastName            string `json:"lastName"            validate:"required"`
		Title               string `json:"title"`
		Email               string `json:"email"`
		Phone               string `json:"phone"`
		ExternalResponderID string `json:"externalResponderId"`
		TimeZone            string `json:"timeZone"`
	}
	if errs := validate.DecodeAndValidate(r, &body); errs != nil {
		validate.WriteError(w, errs)
		return
	}

	resp := &qs.Respondent{
		FirstName:           body.FirstName,
		LastName:            body.LastName,
		Title:               core.ToNullStr(body.Title),
		ExternalResponderID: core.ToNullStr(body.ExternalResponderID),
		TimeZone:            core.ToNullStr(body.TimeZone),
	}

	respID, err := h.QsRespondentRepo.Create(ctx, resp)
	if err != nil {
		slog.ErrorContext(ctx, "create participant failed", "error", err)
		core.WriteJSON(w, http.StatusInternalServerError, map[string]any{"error": "failed to create participant"})
		return
	}

	// Add email as communication address (transport_type_id=1)
	if body.Email != "" {
		if _, err := h.QsRespondentRepo.CreateCommunicationAddress(ctx, respID, 1, body.Email); err != nil {
			slog.ErrorContext(ctx, "create participant email failed", "error", err)
		}
	}
	// Add phone as communication address (transport_type_id=2)
	if body.Phone != "" {
		if _, err := h.QsRespondentRepo.CreateCommunicationAddress(ctx, respID, 2, body.Phone); err != nil {
			slog.ErrorContext(ctx, "create participant phone failed", "error", err)
		}
	}

	core.WriteJSON(w, http.StatusCreated, map[string]any{"id": respID, "source": "qs"})
}

func (h *Handler) GetParticipant(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if h.QsRespondentRepo == nil {
		core.WriteJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "QS database unavailable"})
		return
	}

	respID, err := validate.ParseIDParam(r, "id")
	if err != nil {
		core.WriteJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}

	resp, err := h.QsRespondentRepo.GetByID(ctx, respID)
	if err != nil {
		slog.ErrorContext(ctx, "get participant failed", "error", err, "id", respID)
		core.WriteJSON(w, http.StatusInternalServerError, map[string]any{"error": "database error"})
		return
	}
	if resp == nil {
		core.WriteJSON(w, http.StatusNotFound, map[string]any{"error": "participant not found"})
		return
	}

	result := map[string]any{
		"id":              resp.ID,
		"firstName":       resp.FirstName,
		"lastName":        resp.LastName,
		"name":            strings.TrimSpace(resp.FirstName + " " + resp.LastName),
		"source":          "qs",
		"serviceCategory": "MRA",
		"modifiedOn":      resp.ModifiedOn.Format(time.RFC3339),
	}
	if resp.Title.Valid {
		result["title"] = resp.Title.String
	}
	if resp.ExternalResponderID.Valid {
		result["externalResponderId"] = resp.ExternalResponderID.String
	}
	if resp.TimeZone.Valid {
		result["timeZone"] = resp.TimeZone.String
	}
	if resp.LanguageCountry.Valid {
		result["languageCountry"] = resp.LanguageCountry.String
	}

	// Get communication addresses
	addrs, err := h.QsRespondentRepo.GetCommunicationAddresses(ctx, respID)
	if err != nil {
		slog.ErrorContext(ctx, "get participant addresses failed", "error", err)
	}
	if addrs != nil {
		contacts := make([]map[string]any, 0, len(addrs))
		for _, a := range addrs {
			transport := "other"
			switch a.TransportTypeID {
			case 1:
				transport = "email"
			case 2:
				transport = "sms"
			}
			contacts = append(contacts, map[string]any{
				"type":        transport,
				"address":     a.Address,
				"contactable": a.Contactable,
				"optedOut":    a.OptedOut,
			})
		}
		result["contacts"] = contacts
	}

	core.WriteJSON(w, http.StatusOK, result)
}
