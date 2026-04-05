package ls

import (
	"net/http"

	qualapi "github.com/InCrowd/unified-qual-api"

	"github.com/InCrowd/unified-qual-api/internal/validate"
)

// ──────────────────────────────────────────────
// LS Admin handlers
// ──────────────────────────────────────────────

// ══════════════════════════════════════════════════════
// Phase 7 — Implement all NOT IMPLEMENTED, PARTIAL, STUB APIs
// Brand Separation: qualapi.ResolveSource(r) → "iris" (LS) or "qs" (MRA)
// ══════════════════════════════════════════════════════

// ──────────────────────────────────────────────
// User Role Management (MRA #6, #7)
// ──────────────────────────────────────────────

func (h *Handler) AddUserRoles(w http.ResponseWriter, r *http.Request) {
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

	if h.QsUserRepo == nil {
		qualapi.WriteJSON(w, http.StatusInternalServerError, map[string]any{
			"error":        "no database available",
			"errorMessage": "An error occured while adding user roles",
		})
		return
	}

	// Legacy flow: look up user by email (including deleted users)
	user, _ := h.QsUserRepo.GetByEmailIncludeDeleted(r.Context(), req.Email)

	if user == nil {
		// User doesn't exist → create user + role + client + comm prefs
		userID, err := h.QsUserRepo.Create(r.Context(), req.FirstName, req.LastName, req.Email, "", []int{req.RoleID})
		if err != nil {
			qualapi.WriteJSON(w, http.StatusInternalServerError, map[string]any{
				"error":        err.Error(),
				"errorMessage": "An error occured while adding user roles",
			})
			return
		}
		_ = h.QsUserRepo.AddUserClient(r.Context(), userID, req.ClientID)
		_ = h.QsUserRepo.CreateUserCommPrefs(r.Context(), userID, req.Email, req.CognitoUserID)
		qualapi.WriteJSON(w, http.StatusOK, map[string]any{
			"insertId":               userID,
			"numberOfRecordsUpdated": 1,
		})
		return
	}

	if user.Deleted == 1 {
		// User exists but deleted → restore + role + client
		if err := h.QsUserRepo.RestoreByEmail(r.Context(), req.Email); err != nil {
			qualapi.WriteJSON(w, http.StatusInternalServerError, map[string]any{
				"error":        err.Error(),
				"errorMessage": "An error occured while adding user roles",
			})
			return
		}
		_ = h.QsUserRepo.AddRoles(r.Context(), user.ID, []int{req.RoleID})
		_ = h.QsUserRepo.AddUserClient(r.Context(), user.ID, req.ClientID)
		qualapi.WriteJSON(w, http.StatusOK, map[string]any{
			"numberOfRecordsUpdated": 1,
		})
		return
	}

	// User exists and active → just add the role
	if err := h.QsUserRepo.AddRoles(r.Context(), user.ID, []int{req.RoleID}); err != nil {
		qualapi.WriteJSON(w, http.StatusInternalServerError, map[string]any{
			"error":        err.Error(),
			"errorMessage": "An error occured while adding user roles",
		})
		return
	}
	qualapi.WriteJSON(w, http.StatusOK, map[string]any{
		"numberOfRecordsUpdated": 1,
	})
}

func (h *Handler) DeleteUserRoles(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email  string `json:"email"`
		RoleID int    `json:"roleId"`
	}
	if errs := validate.DecodeAndValidate(r, &req); errs != nil {
		validate.WriteError(w, errs)
		return
	}

	if h.QsUserRepo == nil {
		qualapi.WriteJSON(w, http.StatusInternalServerError, map[string]any{
			"error":        "no database available",
			"errorMessage": "An error occured while removing user roles",
		})
		return
	}

	// Legacy flow: look up user by email, delete role, then soft-delete user
	user, err := h.QsUserRepo.GetByEmail(r.Context(), req.Email)
	if err != nil {
		qualapi.WriteJSON(w, http.StatusInternalServerError, map[string]any{
			"error":        err.Error(),
			"errorMessage": "An error occured while removing user roles",
		})
		return
	}

	// Delete the role
	if err := h.QsUserRepo.DeleteRoles(r.Context(), user.ID, []int{req.RoleID}); err != nil {
		qualapi.WriteJSON(w, http.StatusInternalServerError, map[string]any{
			"error":        err.Error(),
			"errorMessage": "An error occured while removing user roles",
		})
		return
	}
	// Soft-delete the user (matching legacy deleteUserRoleAndDeleteUser transaction)
	if err := h.QsUserRepo.SoftDelete(r.Context(), user.ID); err != nil {
		qualapi.WriteJSON(w, http.StatusInternalServerError, map[string]any{
			"error":        err.Error(),
			"errorMessage": "An error occured while removing user roles",
		})
		return
	}

	qualapi.WriteJSON(w, http.StatusOK, map[string]any{
		"userId": user.ID,
	})
}

// ──────────────────────────────────────────────
// Get User Email by Cognito ID (MRA #8)
// ──────────────────────────────────────────────
