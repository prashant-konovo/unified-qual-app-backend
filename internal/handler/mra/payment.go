package mra

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"


	"github.com/InCrowd/unified-qual-api/internal/repository/qs"
	"github.com/InCrowd/unified-qual-api/internal/dto"
)

// ──────────────────────────────────────────────
// MRA Payment handlers
// ──────────────────────────────────────────────

// ──────────────────────────────────────────────────────────────────────────────
// MRA #75: POST /add-honorarium-amount
// Legacy: AddHonorariumAmountFactory — validates fields, inserts into honorarium_amount
// ──────────────────────────────────────────────────────────────────────────────

func (h *Handler) AddHonorariumAmountMRA(w http.ResponseWriter, r *http.Request) {
	var body dto.MraAddHonorariumAmountRequest
	if errs := dto.DecodeAndValidate(r, &body); errs != nil {
		dto.WriteError(w, errs)
		return
	}

	// Validate required fields (legacy validation)
	projectID, _ := body.ProjectID.Int64()
	honorarium, _ := body.Honorarium.Int64()

	if projectID == 0 {
		dto.WriteJSON(w, http.StatusInternalServerError, map[string]any{"message": "missing projectId param"})
		return
	}
	if honorarium == 0 {
		dto.WriteJSON(w, http.StatusInternalServerError, map[string]any{"message": "missing honorarium param"})
		return
	}
	if body.Currency == "" {
		dto.WriteJSON(w, http.StatusInternalServerError, map[string]any{"message": "missing currency param"})
		return
	}
	if body.SessKey == "" {
		dto.WriteJSON(w, http.StatusInternalServerError, map[string]any{"message": "missing sessKey param"})
		return
	}

	if err := h.PaymentService.AddHonorariumAmountMRA(r.Context(), projectID, honorarium, body.Currency, body.SessKey,
		body.ExternalProjectID, body.ExternalUserSurveyID, body.ExternalUserID,
		body.ExternalCreditOrderID, body.ExternalCountryID); err != nil {
		slog.Error("add honorarium amount mra", "error", err)
		dto.WriteJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error(), "errorMessage": err.Error()})
		return
	}

	dto.WriteJSON(w, http.StatusOK, map[string]any{"message": "Successfully Added honorarium amount"})
}

// ──────────────────────────────────────────────────────────────────────────────
// MRA #76: GET /hono-value-update-reason-list
// Legacy: getHonoValueUpdateReasonListFactory — returns flat array from hono_value_update_reason
// ──────────────────────────────────────────────────────────────────────────────

func (h *Handler) GetHonoValueUpdateReasonListMRA(w http.ResponseWriter, r *http.Request) {
	result, err := h.PaymentService.GetHonoValueUpdateReasonListMRA(r.Context())
	if err != nil {
		slog.Error("get hono value update reason list mra", "error", err)
		dto.WriteJSON(w, http.StatusInternalServerError, map[string]any{
			"error":           err.Error(),
			"errorMessage":    "an error occurred while getting data",
			"customErrorCode": err.Error(),
		})
		return
	}

	dto.WriteJSON(w, http.StatusOK, result)
}

// ──────────────────────────────────────────────────────────────────────────────
// MRA #77: POST /time-slot-payments
// Legacy: addTimeSlotPaymentsFactory — bulk payment creation with user lookup + pending processing
// ──────────────────────────────────────────────────────────────────────────────

func (h *Handler) AddTimeSlotPaymentsMRA(w http.ResponseWriter, r *http.Request) {
	var body dto.MraAddTimeSlotPaymentsRequest
	if errs := dto.DecodeAndValidate(r, &body); errs != nil {
		dto.WriteError(w, errs)
		return
	}

	if len(body.TimeSlotIDs) == 0 {
		dto.WriteJSON(w, http.StatusBadRequest, map[string]any{
			"error":        "Bad Request",
			"errorMessage": "timeSlotIds is not an array of int",
		})
		return
	}

	// Get payment info for all time slot IDs
	paymentInfo, err := h.PaymentService.GetPaymentInfoByTimeSlotIdsMRA(r.Context(), body.TimeSlotIDs)
	if err != nil {
		slog.Error("add time slot payments mra: get payment info", "error", err)
		dto.WriteJSON(w, http.StatusInternalServerError, map[string]any{
			"error":           "Internal Error",
			"errorMessage":    "Internal Error",
			"customErrorCode": "Internal Error",
		})
		return
	}

	// Get user ID from email in auth token (best-effort)
	var userID int64
	email := r.Header.Get("X-User-Email")
	if email != "" {
		userID, _ = h.PaymentService.GetUserByEmailMRA(r.Context(), email)
	}

	// Get payment type list for resolving INTERVIEW type code
	paymentTypes, err := h.PaymentService.GetTimeSlotPaymentTypeListMRA(r.Context())
	if err != nil {
		slog.Error("add time slot payments mra: get payment types", "error", err)
		dto.WriteJSON(w, http.StatusInternalServerError, map[string]any{
			"error":           "Internal Error",
			"errorMessage":    "Internal Error",
			"customErrorCode": "Internal Error",
		})
		return
	}

	// Find INTERVIEW payment type ID
	var interviewTypeID int64
	for _, pt := range paymentTypes {
		if code, ok := pt["code"].(string); ok && code == "INTERVIEW" {
			interviewTypeID, _ = pt["id"].(int64)
			break
		}
	}

	// Build payment records
	var payments []map[string]any
	for _, info := range paymentInfo {
		amount := info["customHonorariumAmount"]
		if amount == nil || amount == "" {
			amount = info["defaultHonorariumAmount"]
		}
		if amount == nil || amount == "" {
			continue
		}
		payments = append(payments, map[string]any{
			"timeSlotId":            info["timeSlotId"],
			"amount":                amount,
			"currency":              info["defaultHonorariumCurrency"],
			"source":                "QS",
			"paymentDate":           time.Now().UTC().Format("2006-01-02 15:04:05"),
			"paymentUserId":         userID,
			"paymentTypeId":         interviewTypeID,
			"paymentStatus":         "PENDING",
			"externalCreditOrderId": info["externalCreditOrderId"],
			"externalUserId":        info["externalUserId"],
			"externalCountryId":     info["externalCountryId"],
			"externalUserSurveyId":  info["externalUserSurveyId"],
			"externalProjectId":     info["externalProjectId"],
		})
	}

	if len(payments) > 0 {
		if err := h.PaymentService.AddQSTimeSlotPaymentsMRA(r.Context(), payments); err != nil {
			slog.Error("add time slot payments mra: insert", "error", err)
			dto.WriteJSON(w, http.StatusInternalServerError, map[string]any{
				"error":           "Internal Error",
				"errorMessage":    "Internal Error",
				"customErrorCode": "Internal Error",
			})
			return
		}
		// Process pending payments: call Credit-Rewards Lambda
		if h.PaymentService.LambdaConfigured() {
			h.processPendingPaymentsForTimeslotIdsMRA(r.Context(), payments)
		} else {
			slog.Info("AddTimeSlotPaymentsMRA: Lambda client not configured, skipping Credit-Rewards call")
		}
	}

	dto.WriteJSON(w, http.StatusCreated, map[string]any{"message": "Success"})
}

// processPendingPaymentsForTimeslotIdsMRA implements the legacy processPendingPaymentsForTimeslotIds flow:
// 1. Begin transaction
// 2. Get pending payment records (SELECT … FOR UPDATE)
// 3. Deduplicate per timeSlotId (first record wins)
// 4. Mark selected records as COMPLETED, remainder as CANCELED
// 5. Invoke Credit-Rewards-CDKV2 Lambda
// 6. On failure: rollback + mark FAILED
func (h *Handler) processPendingPaymentsForTimeslotIdsMRA(ctx context.Context, payments []map[string]any) {
	timeSlotIDs := make([]int64, 0, len(payments))
	for _, p := range payments {
		if tsID, ok := p["timeSlotId"].(int64); ok {
			timeSlotIDs = append(timeSlotIDs, tsID)
		}
	}
	if len(timeSlotIDs) == 0 {
		return
	}

	tx, err := h.PaymentService.BeginQSTx(ctx)
	if err != nil {
		slog.Error("processPendingPayments: begin tx", "error", err)
		return
	}

	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback()
			slog.Error("processPendingPayments: panic recovered", "panic", p)
		}
	}()

	pendingRecords, err := h.PaymentService.GetPendingPaymentsMRA(ctx, tx, timeSlotIDs)
	if err != nil {
		slog.Error("processPendingPayments: get pending", "error", err)
		_ = tx.Rollback()
		return
	}

	if len(pendingRecords) == 0 {
		slog.Info("processPendingPayments: no pending payments to process")
		_ = tx.Commit()
		return
	}

	// Deduplicate: first record per timeSlotId
	seen := map[int64]bool{}
	var uniqueRecords []qs.PendingPaymentRecord
	for _, rec := range pendingRecords {
		if !seen[rec.TimeSlotID] {
			seen[rec.TimeSlotID] = true
			uniqueRecords = append(uniqueRecords, rec)
		}
	}

	// Build Lambda payload from unique records matched to payments
	paymentsByTS := map[int64]map[string]any{}
	for _, p := range payments {
		if tsID, ok := p["timeSlotId"].(int64); ok {
			paymentsByTS[tsID] = p
		}
	}

	var lambdaData []map[string]any
	var toProcessTS []int64
	var toProcessIDs []int64
	for _, rec := range uniqueRecords {
		info := paymentsByTS[rec.TimeSlotID]
		if info == nil {
			continue
		}
		lambdaData = append(lambdaData, map[string]any{
			"creditOrderId":  info["externalCreditOrderId"],
			"userId":         info["externalUserId"],
			"countryId":      info["externalCountryId"],
			"userSurveyId":   info["externalUserSurveyId"],
			"projectId":      info["externalProjectId"],
			"amount":         info["amount"],
			"allowDuplicate": true,
			"transferNote":   "Payment for QS Interview",
			"source":         "QS",
		})
		toProcessTS = append(toProcessTS, rec.TimeSlotID)
		toProcessIDs = append(toProcessIDs, rec.ID)
	}

	// Mark COMPLETED
	if err := h.PaymentService.UpdateCompletedPaymentHistoryMRA(ctx, tx, toProcessTS, toProcessIDs); err != nil {
		slog.Error("processPendingPayments: update completed", "error", err)
		_ = tx.Rollback()
		_ = h.PaymentService.UpdateFailedPaymentHistoryMRA(ctx, timeSlotIDs)
		return
	}

	// Mark remaining PENDING as CANCELED
	if err := h.PaymentService.UpdateCanceledPaymentHistoryMRA(ctx, tx, toProcessTS, toProcessIDs); err != nil {
		slog.Error("processPendingPayments: update canceled", "error", err)
		_ = tx.Rollback()
		_ = h.PaymentService.UpdateFailedPaymentHistoryMRA(ctx, timeSlotIDs)
		return
	}

	// Invoke Lambda
	payloadJSON, err := json.Marshal(lambdaData)
	if err != nil {
		slog.Error("processPendingPayments: marshal lambda payload", "error", err)
		_ = tx.Rollback()
		_ = h.PaymentService.UpdateFailedPaymentHistoryMRA(ctx, timeSlotIDs)
		return
	}

	fullName := h.PaymentService.FullLambdaName("Credit-Rewards-CDKV2")
	respPayload, statusCode, err := h.PaymentService.InvokeLambda(ctx, fullName, payloadJSON)
	if err != nil {
		slog.Error("processPendingPayments: lambda invoke failed", "error", err, "function", fullName)
		_ = tx.Rollback()
		_ = h.PaymentService.UpdateFailedPaymentHistoryMRA(ctx, timeSlotIDs)
		return
	}

	if statusCode != 200 {
		slog.Error("processPendingPayments: lambda returned non-200", "statusCode", statusCode, "response", string(respPayload))
		_ = tx.Rollback()
		_ = h.PaymentService.UpdateFailedPaymentHistoryMRA(ctx, timeSlotIDs)
		return
	}

	if err := tx.Commit(); err != nil {
		slog.Error("processPendingPayments: commit tx", "error", err)
		_ = h.PaymentService.UpdateFailedPaymentHistoryMRA(ctx, timeSlotIDs)
		return
	}

	slog.Info("processPendingPayments: completed successfully", "timeslots", toProcessTS)
}

// ──────────────────────────────────────────────────────────────────────────────
// MRA #78: POST /time-slot-payments-external
// Legacy: addExternalTimeSlotPaymentsExternalFactory — bulk external payment creation
// ──────────────────────────────────────────────────────────────────────────────

func (h *Handler) AddExternalTimeSlotPaymentsMRA(w http.ResponseWriter, r *http.Request) {
	var body []dto.MraExternalTimeSlotPaymentItem
	if errs := dto.DecodeAndValidate(r, &body); errs != nil {
		dto.WriteError(w, errs)
		return
	}

	// Get payment type list for resolving type codes
	paymentTypes, err := h.PaymentService.GetTimeSlotPaymentTypeListMRA(r.Context())
	if err != nil {
		slog.Error("add external time slot payments mra: get payment types", "error", err)
		dto.WriteJSON(w, http.StatusInternalServerError, map[string]any{
			"error":           err.Error(),
			"errorMessage":    "an error occurred while getting data",
			"customErrorCode": err.Error(),
		})
		return
	}

	// Build type code → ID map
	typeCodeMap := make(map[string]int64)
	for _, pt := range paymentTypes {
		code, _ := pt["code"].(string)
		id, _ := pt["id"].(int64)
		typeCodeMap[code] = id
	}

	var payments []map[string]any
	for _, p := range body {
		typeID, ok := typeCodeMap[p.PaymentTypeCode]
		if !ok {
			dto.WriteJSON(w, http.StatusInternalServerError, map[string]any{
				"error":           fmt.Sprintf("Invalid payment type code: %s", p.PaymentTypeCode),
				"errorMessage":    "an error occurred while getting data",
				"customErrorCode": fmt.Sprintf("Invalid payment type code: %s", p.PaymentTypeCode),
			})
			return
		}

		paymentDate := p.PaymentDate
		if paymentDate == "" {
			paymentDate = time.Now().UTC().Format("2006-01-02 15:04:05")
		}

		source := p.Source
		if source == "" {
			source = "IRIS"
		}

		payments = append(payments, map[string]any{
			"amount":                p.Amount.String(),
			"currency":              p.Currency,
			"source":                source,
			"paymentDate":           paymentDate,
			"paymentUserId":         p.PaymentUserID.String(),
			"externalUserSurveyId":  p.ExternalUserSurveyID,
			"externalCreditOrderId": p.ExternalCreditOrderID,
			"paymentTypeId":         typeID,
		})
	}

	if len(payments) > 0 {
		if err := h.PaymentService.AddExternalTimeSlotPaymentsMRA(r.Context(), payments); err != nil {
			slog.Error("add external time slot payments mra", "error", err)
			dto.WriteJSON(w, http.StatusInternalServerError, map[string]any{
				"error":           err.Error(),
				"errorMessage":    "an error occurred while getting data",
				"customErrorCode": err.Error(),
			})
			return
		}
	}

	dto.WriteJSON(w, http.StatusCreated, map[string]any{"message": "Success"})
}

// ──────────────────────────────────────────────────────────────────────────────
// MRA #79: POST /time-slot-custom-hono
// Legacy: addTimeSlotCustomHonorariumFactory — upsert custom honorarium + Step Function call (PARTIAL)
// ──────────────────────────────────────────────────────────────────────────────

func (h *Handler) AddTimeSlotCustomHonorariumMRA(w http.ResponseWriter, r *http.Request) {
	var body dto.MraAddTimeSlotCustomHonorariumRequest
	if errs := dto.DecodeAndValidate(r, &body); errs != nil {
		dto.WriteError(w, errs)
		return
	}

	// Get user from email header
	var userID int64
	email := r.Header.Get("X-User-Email")
	if email != "" {
		userID, _ = h.PaymentService.GetUserByEmailMRA(r.Context(), email)
	}

	// Resolve reason code to ID
	reasons, err := h.PaymentService.GetHonoValueUpdateReasonListMRA(r.Context())
	if err != nil {
		slog.Error("add time slot custom honorarium mra: get reasons", "error", err)
		dto.WriteJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error(), "errorMessage": err.Error()})
		return
	}

	var reasonID int64
	for _, reason := range reasons {
		if code, ok := reason["code"].(string); ok && code == body.ReasonCode {
			reasonID, _ = reason["id"].(int64)
			break
		}
	}
	if reasonID == 0 {
		dto.WriteJSON(w, http.StatusInternalServerError, map[string]any{
			"error":        fmt.Sprintf("Invalid reason code: %s", body.ReasonCode),
			"errorMessage": fmt.Sprintf("Invalid reason code: %s", body.ReasonCode),
		})
		return
	}

	if err := h.PaymentService.AddTimeSlotCustomHonorariumMRA(r.Context(), body.TimeSlotID, body.OldValue, body.NewValue, reasonID, userID); err != nil {
		slog.Error("add time slot custom honorarium mra", "error", err)
		dto.WriteJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error(), "errorMessage": err.Error()})
		return
	}

	// Call Step Function to update IRIS honorarium (legacy: external-calls-StateMachine)
	if h.PaymentService.StepFnConfigured() {
		go h.callStepFunctionForCustomHonoMRA(r.Context(), body.TimeSlotID, body.OldValue, body.NewValue, body.ReasonCode, userID)
	} else {
		slog.Info("AddTimeSlotCustomHonorariumMRA: Step Function client not configured, skipping IRIS update")
	}

	dto.WriteJSON(w, http.StatusOK, map[string]any{"message": "Successfully Added honorarium amount"})
}

// callStepFunctionForCustomHonoMRA invokes the external-calls-StateMachine Step Function
// to sync custom honorarium updates to IRIS. Runs in a goroutine (fire-and-forget with logging).
func (h *Handler) callStepFunctionForCustomHonoMRA(ctx context.Context, timeSlotID int64, oldValue, newValue float64, reasonCode string, userID int64) {
	// Look up externalUserSurveyId for this timeslot
	externalSurveyID, err := h.PaymentService.GetExternalSurveyIdByTimeSlotIdMRA(ctx, timeSlotID)
	if err != nil {
		slog.Error("callStepFunctionForCustomHono: get external survey id", "error", err, "timeSlotId", timeSlotID)
		return
	}
	if externalSurveyID == "" {
		slog.Warn("callStepFunctionForCustomHono: no external survey id, skipping", "timeSlotId", timeSlotID)
		return
	}

	// Build env prefix for token secret name (legacy: prd→production, data-qa→qual-qa)
	envPrefix := h.PaymentService.AWSEnvironment()
	switch envPrefix {
	case "prod":
		envPrefix = "production"
	case "data-qa":
		envPrefix = "qual-qa"
	}

	apiPayload := map[string]any{
		"apiPayload": map[string]any{
			"source": "QS",
			"url":    h.PaymentService.ICApiURL() + "/v1/qual_hono_update",
			"method": "POST",
			"data": map[string]any{
				"userSurveyId":   externalSurveyID,
				"oldHono":        oldValue,
				"updatedHono":    newValue,
				"reason":         reasonCode,
				"externalUserId": userID,
			},
			"headers": map[string]any{
				"Content-Type": "application/json",
			},
			"authType":        "EXTERNAL_API_TOKEN",
			"tokenSecretName": "external_api_token_" + envPrefix,
			"tokenKey":        "qual-hono-update",
			"authHeaderKey":   "Authorization",
			"authHeaderType":  "BEARER",
		},
	}

	if err := h.PaymentService.StartStepFunctionExecution(ctx, "external-calls-StateMachine", apiPayload); err != nil {
		slog.Error("callStepFunctionForCustomHono: step function failed", "error", err, "timeSlotId", timeSlotID)
		return
	}

	slog.Info("callStepFunctionForCustomHono: step function started successfully", "timeSlotId", timeSlotID)
}

// ──────────────────────────────────────────────────────────────────────────────
// MRA #80: GET /interview-payment-status-list
// Legacy: getInterviewPaymentStatusListFactory — returns flat array from time_slot_payment_status
// ──────────────────────────────────────────────────────────────────────────────

func (h *Handler) GetInterviewPaymentStatusListMRA(w http.ResponseWriter, r *http.Request) {
	result, err := h.PaymentService.GetTimeSlotPaymentStatusListMRA(r.Context())
	if err != nil {
		slog.Error("get interview payment status list mra", "error", err)
		dto.WriteJSON(w, http.StatusInternalServerError, map[string]any{
			"error":           err.Error(),
			"errorMessage":    "an error occurred while getting data",
			"customErrorCode": err.Error(),
		})
		return
	}

	dto.WriteJSON(w, http.StatusOK, result)
}
