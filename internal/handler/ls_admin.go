package handler

import (
	"net/http"

	"github.com/InCrowd/unified-qual-api/internal/validate"
)

// ──────────────────────────────────────────────
// LS Admin handlers
// ──────────────────────────────────────────────

// ══════════════════════════════════════════════════════
// Phase 7 — Implement all NOT IMPLEMENTED, PARTIAL, STUB APIs
// Brand Separation: resolveSource(r) → "iris" (LS) or "qs" (MRA)
// ══════════════════════════════════════════════════════

// ──────────────────────────────────────────────
// User Role Management (MRA #6, #7)
// ──────────────────────────────────────────────

func (h *LSHandler) AddUserRoles(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email         string `json:"email"`
		RoleID        int    `json:"roleId"`
		ClientID      int64  `json:"clientId"`
		FirstName     string `json:"firstName"`
		LastName      string `json:"lastName"`
		CognitoUserID string `json:"cognitoUserId"`
	}
	if errs := validate.DecodeAndValidate(r, &req); errs != nil {
		validate.WriteError(w, errs)
		return
	}

	if h.qsUserRepo == nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"error":        "no database available",
			"errorMessage": "An error occured while adding user roles",
		})
		return
	}

	// Legacy flow: look up user by email (including deleted users)
	user, _ := h.qsUserRepo.GetByEmailIncludeDeleted(r.Context(), req.Email)

	if user == nil {
		// User doesn't exist → create user + role + client + comm prefs
		userID, err := h.qsUserRepo.Create(r.Context(), req.FirstName, req.LastName, req.Email, "", []int{req.RoleID})
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{
				"error":        err.Error(),
				"errorMessage": "An error occured while adding user roles",
			})
			return
		}
		_ = h.qsUserRepo.AddUserClient(r.Context(), userID, req.ClientID)
		_ = h.qsUserRepo.CreateUserCommPrefs(r.Context(), userID, req.Email, req.CognitoUserID)
		writeJSON(w, http.StatusOK, map[string]any{
			"insertId":               userID,
			"numberOfRecordsUpdated": 1,
		})
		return
	}

	if user.Deleted == 1 {
		// User exists but deleted → restore + role + client
		if err := h.qsUserRepo.RestoreByEmail(r.Context(), req.Email); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{
				"error":        err.Error(),
				"errorMessage": "An error occured while adding user roles",
			})
			return
		}
		_ = h.qsUserRepo.AddRoles(r.Context(), user.ID, []int{req.RoleID})
		_ = h.qsUserRepo.AddUserClient(r.Context(), user.ID, req.ClientID)
		writeJSON(w, http.StatusOK, map[string]any{
			"numberOfRecordsUpdated": 1,
		})
		return
	}

	// User exists and active → just add the role
	if err := h.qsUserRepo.AddRoles(r.Context(), user.ID, []int{req.RoleID}); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"error":        err.Error(),
			"errorMessage": "An error occured while adding user roles",
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"numberOfRecordsUpdated": 1,
	})
}

func (h *LSHandler) DeleteUserRoles(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email  string `json:"email"`
		RoleID int    `json:"roleId"`
	}
	if errs := validate.DecodeAndValidate(r, &req); errs != nil {
		validate.WriteError(w, errs)
		return
	}

	if h.qsUserRepo == nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"error":        "no database available",
			"errorMessage": "An error occured while removing user roles",
		})
		return
	}

	// Legacy flow: look up user by email, delete role, then soft-delete user
	user, err := h.qsUserRepo.GetByEmail(r.Context(), req.Email)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"error":        err.Error(),
			"errorMessage": "An error occured while removing user roles",
		})
		return
	}

	// Delete the role
	if err := h.qsUserRepo.DeleteRoles(r.Context(), user.ID, []int{req.RoleID}); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"error":        err.Error(),
			"errorMessage": "An error occured while removing user roles",
		})
		return
	}
	// Soft-delete the user (matching legacy deleteUserRoleAndDeleteUser transaction)
	if err := h.qsUserRepo.SoftDelete(r.Context(), user.ID); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"error":        err.Error(),
			"errorMessage": "An error occured while removing user roles",
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"userId": user.ID,
	})
}

// ──────────────────────────────────────────────
// Get User Email by Cognito ID (MRA #8)
// ──────────────────────────────────────────────
