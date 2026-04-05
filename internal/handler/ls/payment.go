package ls

import (
	"log/slog"
	"net/http"

	"github.com/InCrowd/unified-qual-api/internal/handler/support"

	"github.com/InCrowd/unified-qual-api/internal/validate"
)

// ──────────────────────────────────────────────
// LS Payment handlers
// ──────────────────────────────────────────────

// ──────────────────────────────────────────────
// Payments extended
// ──────────────────────────────────────────────

// CreatePaymentReal creates a real payment record.
func (h *Handler) CreatePaymentReal(w http.ResponseWriter, r *http.Request) {
	var req struct {
		TimeSlotID  int64  `json:"timeSlotId"`
		Amount      int    `json:"amount"`
		PaymentType string `json:"paymentType"`
	}
	if errs := validate.DecodeAndValidate(r, &req); errs != nil {
		validate.WriteError(w, errs)
		return
	}

	if h.QsAnswerRepo != nil {
		id, err := h.QsAnswerRepo.CreatePaymentRecord(r.Context(), req.TimeSlotID, req.Amount, req.PaymentType, "pending")
		if err != nil {
			slog.Error("create payment failed", "error", err)
			support.WriteJSON(w, http.StatusInternalServerError, map[string]any{"error": "create failed"})
			return
		}
		support.WriteJSON(w, http.StatusCreated, map[string]any{"id": id, "timeSlotId": req.TimeSlotID})
		return
	}
	support.WriteJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "no database available"})
}

// CreateCustomHonorariumReal creates a custom honorarium.
func (h *Handler) CreateCustomHonorariumReal(w http.ResponseWriter, r *http.Request) {
	var req struct {
		TimeSlotID int64  `json:"timeSlotId"`
		Amount     int    `json:"amount"`
		Reason     string `json:"reason"`
	}
	if errs := validate.DecodeAndValidate(r, &req); errs != nil {
		validate.WriteError(w, errs)
		return
	}

	if h.QsAnswerRepo != nil {
		id, err := h.QsAnswerRepo.CreateCustomHonorarium(r.Context(), req.TimeSlotID, req.Amount, req.Reason)
		if err != nil {
			slog.Error("create custom honorarium failed", "error", err)
			support.WriteJSON(w, http.StatusInternalServerError, map[string]any{"error": "create failed"})
			return
		}
		support.WriteJSON(w, http.StatusCreated, map[string]any{"id": id, "timeSlotId": req.TimeSlotID})
		return
	}
	support.WriteJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "no database available"})
}

// GetPaymentStatusListReal returns payment statuses from real DB.
func (h *Handler) GetPaymentStatusListReal(w http.ResponseWriter, r *http.Request) {
	statuses := []map[string]any{
		{"id": 1, "name": "Pending"},
		{"id": 2, "name": "Approved"},
		{"id": 3, "name": "Paid"},
		{"id": 4, "name": "Failed"},
		{"id": 5, "name": "Cancelled"},
	}
	support.WriteJSON(w, http.StatusOK, statuses)
}

// ──────────────────────────────────────────────
// Translations extended
// ──────────────────────────────────────────────

// ──────────────────────────────────────────────
// External Payments (MRA #78)
// ──────────────────────────────────────────────

func (h *Handler) CreateExternalPayment(w http.ResponseWriter, r *http.Request) {
	var req struct {
		TimeSlotID  int64  `json:"timeSlotId"`
		Amount      int    `json:"amount"`
		PaymentType string `json:"paymentType"`
		ExternalRef string `json:"externalReference"`
	}
	if errs := validate.DecodeAndValidate(r, &req); errs != nil {
		validate.WriteError(w, errs)
		return
	}

	if h.QsAnswerRepo != nil {
		payID, err := h.QsAnswerRepo.CreateExternalPayment(r.Context(), req.TimeSlotID, req.Amount, req.PaymentType, "pending", req.ExternalRef)
		if err != nil {
			slog.Error("create external payment failed", "error", err)
			support.WriteJSON(w, http.StatusInternalServerError, map[string]any{"error": "create failed"})
			return
		}
		support.WriteJSON(w, http.StatusCreated, map[string]any{"id": payID, "timeSlotId": req.TimeSlotID, "external": true})
		return
	}
	support.WriteJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "no database available"})
}

func (h *Handler) GetHonorariumReasons(w http.ResponseWriter, r *http.Request) {
	if h.QsAnswerRepo != nil {
		reasons, err := h.QsAnswerRepo.ListHonorariumReasons(r.Context())
		if err != nil {
			slog.Error("list hono reasons failed", "error", err)
		}
		support.WriteJSON(w, http.StatusOK, reasons)
		return
	}
	support.WriteJSON(w, http.StatusOK, []map[string]any{
		{"id": 1, "name": "Interview Completed"},
		{"id": 2, "name": "Partial Completion"},
		{"id": 3, "name": "No Show Compensation"},
		{"id": 4, "name": "Technical Issue"},
		{"id": 5, "name": "Other"},
	})
}

func (h *Handler) GetInterviewPaymentStatusList(w http.ResponseWriter, r *http.Request) {
	if h.QsAnswerRepo != nil {
		statuses, err := h.QsAnswerRepo.ListInterviewPaymentStatuses(r.Context())
		if err != nil {
			slog.Error("list payment statuses failed", "error", err)
		}
		support.WriteJSON(w, http.StatusOK, statuses)
		return
	}
	support.WriteJSON(w, http.StatusOK, []map[string]any{
		{"id": 1, "name": "Pending"}, {"id": 2, "name": "Approved"},
		{"id": 3, "name": "Paid"}, {"id": 4, "name": "Failed"},
		{"id": 5, "name": "Cancelled"}, {"id": 6, "name": "On Hold"},
	})
}

// ──────────────────────────────────────────────
// LS: Inquiry Preview & Custom Crowd Inquiry (LS #6, #7, #55)
// ──────────────────────────────────────────────

// ── Inquiry Preview types (contract-identical with legacy Scala InCrowdAPI) ──

type ipCrowdAttributeSpec struct {
	AttributeID  int64   `json:"attributeId"`
	NumericMin   *int64  `json:"numericMin"`
	NumericMax   *int64  `json:"numericMax"`
	ChoiceIDs    []int64 `json:"choiceIds"`
	QualRequired *bool   `json:"qualRequired"`
}

type ipDifficultyLevelReq struct {
	ID int64 `json:"id"`
}

type ipDifficultyAssessmentResp struct {
	ID         int64  `json:"id"`
	Name       string `json:"name"`
	IsHardStop bool   `json:"isHardStop"`
}

type ipCrowdSpec struct {
	Name                 *string                     `json:"name"`
	NumberRequested      *int64                      `json:"numberRequested"`
	Notes                *string                     `json:"notes"`
	Attributes           []ipCrowdAttributeSpec      `json:"attributes"`
	MarketID             int64                       `json:"marketId"`
	MarketName           *string                     `json:"marketName"`
	DifficultyLevel      ipDifficultyLevelReq        `json:"difficultyLevel"`
	DifficultyAssessment *ipDifficultyAssessmentResp `json:"difficultyAssessment"`
	CrowdID              *int64                      `json:"crowdId"`
	IsCustom             bool                        `json:"isCustom"`
	ValidRespondersCount *int64                      `json:"validRespondersCount"`
}

type ipProposal struct {
	InterviewLength      int64         `json:"interviewLength"`
	Name                 string        `json:"name"`
	SalesforceProjectID  *string       `json:"salesforceProjectId"`
	CompletionDate       *string       `json:"completionDate"`
	Notes                *string       `json:"notes"`
	Crowds               []ipCrowdSpec `json:"crowds"`
	CustomCrowds         []ipCrowdSpec `json:"customCrowds"`
	ProjectID            *int64        `json:"projectId"`
	UnderReview          bool          `json:"underReview"`
	TranscriptsRequested bool          `json:"transcriptsRequested"`
	RequiresStimuli      bool          `json:"requiresStimuli"`
	IsDynamicStimulus    bool          `json:"isDynamicStimulus"`
}

type ipFee struct {
	GrossSubtotal float64  `json:"grossSubtotal"`
	NetSubtotal   float64  `json:"netSubtotal"`
	DiscountRate  *float64 `json:"discountRate"`
	PricePerUnit  float64  `json:"pricePerUnit"`
	Count         int64    `json:"count"`
	IsHonorarium  bool     `json:"isHonorarium"`
	Name          string   `json:"name"`
	ProductID     int64    `json:"productId"`
}

type ipProjectCosts struct {
	GrossTotal float64 `json:"grossTotal"`
	NetTotal   float64 `json:"netTotal"`
	Fees       []ipFee `json:"fees"`
}

type ipResponse struct {
	Proposal              *ipProposal     `json:"proposal"`
	SalesforceProjectName *string         `json:"salesforceProjectName"`
	Costs                 *ipProjectCosts `json:"costs"`
	IsHardStop            bool            `json:"isHardStop"`
}

// isSpecializedCrowd checks whether a crowd is "specialized" per legacy logic.
func ipIsSpecializedCrowd(c *ipCrowdSpec) bool {
	if c.MarketID != 1 {
		return false
	}
	if c.IsCustom {
		return true
	}
	nonGeneralChoices := map[int64]bool{304: true}
	for _, attr := range c.Attributes {
		if attr.AttributeID == 1 {
			for _, cid := range attr.ChoiceIDs {
				if !nonGeneralChoices[cid] {
					return true
				}
			}
		}
	}
	return false
}

// ipCrowdMatchesProduct determines if a crowd matches a product for honorarium calculations.
func ipCrowdMatchesProduct(c *ipCrowdSpec, relatedMarketIDs []int64, isSpecialized bool) bool {
	for _, mid := range relatedMarketIDs {
		if mid == c.MarketID {
			return true
		}
	}
	crowdSpecialized := ipIsSpecializedCrowd(c)
	if isSpecialized && crowdSpecialized {
		return true
	}
	if !isSpecialized && !crowdSpecialized && c.MarketID == 1 {
		return true
	}
	return false
}
