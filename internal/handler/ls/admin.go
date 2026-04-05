package ls

import (
	"net/http"


	"github.com/InCrowd/unified-qual-api/internal/dto"
	"github.com/InCrowd/unified-qual-api/internal/httpkit"
)

// ──────────────────────────────────────────────
// LS Admin handlers
// ──────────────────────────────────────────────

// ══════════════════════════════════════════════════════
// Phase 7 — Implement all NOT IMPLEMENTED, PARTIAL, STUB APIs
// Brand Separation: httpkit.ResolveSource(r) → "iris" (LS) or "qs" (MRA)
// ══════════════════════════════════════════════════════

// ──────────────────────────────────────────────
// User Role Management (MRA #6, #7)
// ──────────────────────────────────────────────

func (h *Handler) AddUserRoles(w http.ResponseWriter, r *http.Request) {
	var req dto.LsAddUserRolesRequest
	if errs := httpkit.DecodeAndValidate(r, &req); errs != nil {
		httpkit.WriteError(w, errs)
		return
	}

	if !h.AdminService.QsAvailable() {
		httpkit.WriteJSON(w, http.StatusInternalServerError, map[string]any{
			"error":        "no database available",
			"errorMessage": "An error occured while adding user roles",
		})
		return
	}

	// Legacy flow: look up user by email (including deleted users)
	user, _ := h.AdminService.GetByEmailIncludeDeleted(r.Context(), req.Email)

	if user == nil {
		// User doesn't exist → create user + role + client + comm prefs
		userID, err := h.AdminService.CreateQsUser(r.Context(), req.FirstName, req.LastName, req.Email, "", []int{req.RoleID})
		if err != nil {
			httpkit.WriteJSON(w, http.StatusInternalServerError, map[string]any{
				"error":        err.Error(),
				"errorMessage": "An error occured while adding user roles",
			})
			return
		}
		_ = h.AdminService.AddUserClient(r.Context(), userID, req.ClientID)
		_ = h.AdminService.CreateUserCommPrefs(r.Context(), userID, req.Email, req.CognitoUserID)
		httpkit.WriteJSON(w, http.StatusOK, map[string]any{
			"insertId":               userID,
			"numberOfRecordsUpdated": 1,
		})
		return
	}

	if user.Deleted == 1 {
		// User exists but deleted → restore + role + client
		if err := h.AdminService.RestoreByEmail(r.Context(), req.Email); err != nil {
			httpkit.WriteJSON(w, http.StatusInternalServerError, map[string]any{
				"error":        err.Error(),
				"errorMessage": "An error occured while adding user roles",
			})
			return
		}
		_ = h.AdminService.AddRoles(r.Context(), user.ID, []int{req.RoleID})
		_ = h.AdminService.AddUserClient(r.Context(), user.ID, req.ClientID)
		httpkit.WriteJSON(w, http.StatusOK, map[string]any{
			"numberOfRecordsUpdated": 1,
		})
		return
	}

	// User exists and active → just add the role
	if err := h.AdminService.AddRoles(r.Context(), user.ID, []int{req.RoleID}); err != nil {
		httpkit.WriteJSON(w, http.StatusInternalServerError, map[string]any{
			"error":        err.Error(),
			"errorMessage": "An error occured while adding user roles",
		})
		return
	}
	httpkit.WriteJSON(w, http.StatusOK, map[string]any{
		"numberOfRecordsUpdated": 1,
	})
}

func (h *Handler) DeleteUserRoles(w http.ResponseWriter, r *http.Request) {
	var req dto.LsDeleteUserRolesRequest
	if errs := httpkit.DecodeAndValidate(r, &req); errs != nil {
		httpkit.WriteError(w, errs)
		return
	}

	if !h.AdminService.QsAvailable() {
		httpkit.WriteJSON(w, http.StatusInternalServerError, map[string]any{
			"error":        "no database available",
			"errorMessage": "An error occured while removing user roles",
		})
		return
	}

	// Legacy flow: look up user by email, delete role, then soft-delete user
	user, err := h.AdminService.GetByEmail(r.Context(), req.Email)
	if err != nil {
		httpkit.WriteJSON(w, http.StatusInternalServerError, map[string]any{
			"error":        err.Error(),
			"errorMessage": "An error occured while removing user roles",
		})
		return
	}

	// Delete the role
	if err := h.AdminService.DeleteRoles(r.Context(), user.ID, []int{req.RoleID}); err != nil {
		httpkit.WriteJSON(w, http.StatusInternalServerError, map[string]any{
			"error":        err.Error(),
			"errorMessage": "An error occured while removing user roles",
		})
		return
	}
	// Soft-delete the user (matching legacy deleteUserRoleAndDeleteUser transaction)
	if err := h.AdminService.SoftDelete(r.Context(), user.ID); err != nil {
		httpkit.WriteJSON(w, http.StatusInternalServerError, map[string]any{
			"error":        err.Error(),
			"errorMessage": "An error occured while removing user roles",
		})
		return
	}

	httpkit.WriteJSON(w, http.StatusOK, map[string]any{
		"userId": user.ID,
	})
}

// ──────────────────────────────────────────────
// Get User Email by Cognito ID (MRA #8)
// ──────────────────────────────────────────────
