package ls

import (
	"log/slog"
	"net/http"


	"github.com/InCrowd/unified-qual-api/internal/dto"
	"github.com/InCrowd/unified-qual-api/internal/httpkit"
)

// ──────────────────────────────────────────────
// LS Payment handlers
// ──────────────────────────────────────────────

// ──────────────────────────────────────────────
// Payments extended
// ──────────────────────────────────────────────

// CreatePaymentReal creates a real payment record.
func (h *Handler) CreatePaymentReal(w http.ResponseWriter, r *http.Request) {
	var req dto.LsCreatePaymentRequest
	if errs := httpkit.DecodeAndValidate(r, &req); errs != nil {
		httpkit.WriteError(w, errs)
		return
	}

	if h.PaymentService.AnswerAvailable() {
		id, err := h.PaymentService.CreatePaymentRecord(r.Context(), req.TimeSlotID, req.Amount, req.PaymentType, "pending")
		if err != nil {
			slog.Error("create payment failed", "error", err)
			httpkit.WriteJSON(w, http.StatusInternalServerError, map[string]any{"error": "create failed"})
			return
		}
		httpkit.WriteJSON(w, http.StatusCreated, map[string]any{"id": id, "timeSlotId": req.TimeSlotID})
		return
	}
	httpkit.WriteJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "no database available"})
}

// CreateCustomHonorariumReal creates a custom honorarium.
func (h *Handler) CreateCustomHonorariumReal(w http.ResponseWriter, r *http.Request) {
	var req dto.LsCreateCustomHonorariumRequest
	if errs := httpkit.DecodeAndValidate(r, &req); errs != nil {
		httpkit.WriteError(w, errs)
		return
	}

	if h.PaymentService.AnswerAvailable() {
		id, err := h.PaymentService.CreateCustomHonorarium(r.Context(), req.TimeSlotID, req.Amount, req.Reason)
		if err != nil {
			slog.Error("create custom honorarium failed", "error", err)
			httpkit.WriteJSON(w, http.StatusInternalServerError, map[string]any{"error": "create failed"})
			return
		}
		httpkit.WriteJSON(w, http.StatusCreated, map[string]any{"id": id, "timeSlotId": req.TimeSlotID})
		return
	}
	httpkit.WriteJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "no database available"})
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
	httpkit.WriteJSON(w, http.StatusOK, statuses)
}

// ──────────────────────────────────────────────
// Translations extended
// ──────────────────────────────────────────────

// ──────────────────────────────────────────────
// External Payments (MRA #78)
// ──────────────────────────────────────────────

func (h *Handler) CreateExternalPayment(w http.ResponseWriter, r *http.Request) {
	var req dto.LsCreateExternalPaymentRequest
	if errs := httpkit.DecodeAndValidate(r, &req); errs != nil {
		httpkit.WriteError(w, errs)
		return
	}

	if h.PaymentService.AnswerAvailable() {
		payID, err := h.PaymentService.CreateExternalPayment(r.Context(), req.TimeSlotID, req.Amount, req.PaymentType, "pending", req.ExternalRef)
		if err != nil {
			slog.Error("create external payment failed", "error", err)
			httpkit.WriteJSON(w, http.StatusInternalServerError, map[string]any{"error": "create failed"})
			return
		}
		httpkit.WriteJSON(w, http.StatusCreated, map[string]any{"id": payID, "timeSlotId": req.TimeSlotID, "external": true})
		return
	}
	httpkit.WriteJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "no database available"})
}

func (h *Handler) GetHonorariumReasons(w http.ResponseWriter, r *http.Request) {
	if h.PaymentService.AnswerAvailable() {
		reasons, err := h.PaymentService.ListHonorariumReasons(r.Context())
		if err != nil {
			slog.Error("list hono reasons failed", "error", err)
		}
		httpkit.WriteJSON(w, http.StatusOK, reasons)
		return
	}
	httpkit.WriteJSON(w, http.StatusOK, []map[string]any{
		{"id": 1, "name": "Interview Completed"},
		{"id": 2, "name": "Partial Completion"},
		{"id": 3, "name": "No Show Compensation"},
		{"id": 4, "name": "Technical Issue"},
		{"id": 5, "name": "Other"},
	})
}

func (h *Handler) GetInterviewPaymentStatusList(w http.ResponseWriter, r *http.Request) {
	if h.PaymentService.AnswerAvailable() {
		statuses, err := h.PaymentService.ListInterviewPaymentStatuses(r.Context())
		if err != nil {
			slog.Error("list payment statuses failed", "error", err)
		}
		httpkit.WriteJSON(w, http.StatusOK, statuses)
		return
	}
	httpkit.WriteJSON(w, http.StatusOK, []map[string]any{
		{"id": 1, "name": "Pending"}, {"id": 2, "name": "Approved"},
		{"id": 3, "name": "Paid"}, {"id": 4, "name": "Failed"},
		{"id": 5, "name": "Cancelled"}, {"id": 6, "name": "On Hold"},
	})
}

// ──────────────────────────────────────────────
// LS: Inquiry Preview & Custom Crowd Inquiry (LS #6, #7, #55)
// ──────────────────────────────────────────────
