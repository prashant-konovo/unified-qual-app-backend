package shared

import (
	"github.com/InCrowd/unified-qual-api/internal/dto"
	"net/http"

)

// ──────────────────────────────────────────────
// Payment handlers (shared)
// ──────────────────────────────────────────────

// ──────────────────────────────────────────────
// Payments (LLD endpoints)
// ──────────────────────────────────────────────

func (h *Handler) CreatePayment(w http.ResponseWriter, r *http.Request) {
	dto.WriteJSON(w, http.StatusCreated, map[string]any{"paymentId": 7001, "status": "PENDING"})
}

func (h *Handler) CreateCustomHonorarium(w http.ResponseWriter, r *http.Request) {
	dto.WriteJSON(w, http.StatusOK, map[string]any{"timeSlotId": 301, "honorarium": 200.00, "reasonId": 2})
}

func (h *Handler) GetPaymentStatusList(w http.ResponseWriter, r *http.Request) {
	dto.WriteJSON(w, http.StatusOK, map[string]any{
		"payments": []map[string]any{
			{"timeSlotId": 301, "respondentName": "Alice Johnson", "amount": 150.00,
				"currency": "USD", "status": "PENDING", "source": "QS",
				"updatedAt": "2026-03-20T12:00:00Z"},
		},
	})
}
