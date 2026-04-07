package shared

import (
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/InCrowd/unified-qual-api/internal/dto"
	"github.com/InCrowd/unified-qual-api/internal/utilities"
)

// ──────────────────────────────────────────────
// Admin handlers (shared)
// ──────────────────────────────────────────────

// ──────────────────────────────────────────────
// Admin (LLD endpoints)
// ──────────────────────────────────────────────

func (h *Handler) ListAdminUsers(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	pg := utilities.ParsePagination(r, 20, 100)
	page, pageSize := pg.Page, pg.PageSize
	search := r.URL.Query().Get("search")
	source := r.URL.Query().Get("source") // "qs", "iris", or "" (both)

	var allUsers []map[string]any

	// QS users
	if (source == "" || source == "qs") && h.AdminService.QsAvailable() {
		users, total, err := h.AdminService.ListQsUsers(ctx, page, pageSize, nil, search)
		if err != nil {
			slog.ErrorContext(ctx, "list QS users failed", "error", err)
		} else {
			for _, u := range users {
				roles := []string{}
				for _, rid := range dto.ParseRoleCSV(u.RoleIDs) {
					roles = append(roles, dto.QsRoleName(rid))
				}
				allUsers = append(allUsers, map[string]any{
					"id":              u.ID,
					"firstName":       u.FirstName.String,
					"lastName":        u.LastName.String,
					"email":           u.Email.String,
					"roles":           roles,
					"source":          "qs",
					"serviceCategory": "MRA",
					"updatedAt":       u.ModifiedOn.Format(time.RFC3339),
				})
			}
			_ = total // used when source-specific pagination implemented
		}
	}

	// IRIS users
	if (source == "" || source == "iris") && h.AdminService.IrisAvailable() {
		users, total, err := h.AdminService.ListIrisUsers(ctx, page, pageSize, nil, search)
		if err != nil {
			slog.ErrorContext(ctx, "list IRIS users failed", "error", err)
		} else {
			for _, u := range users {
				lastLogin := ""
				if u.LastLogin.Valid {
					lastLogin = u.LastLogin.Time.Format(time.RFC3339)
				}
				allUsers = append(allUsers, map[string]any{
					"id":              u.ID,
					"firstName":       u.FirstName,
					"lastName":        u.LastName,
					"email":           u.Email.String,
					"roles":           strings.Split(u.RoleNames, ","),
					"source":          "iris",
					"serviceCategory": "LS",
					"lastLogin":       lastLogin,
					"registeredAt":    u.RegistrationDate.Format(time.RFC3339),
				})
			}
			_ = total
		}
	}

	utilities.WriteJSON(w, http.StatusOK, map[string]any{"users": allUsers})
}

func (h *Handler) CreateAdminUser(w http.ResponseWriter, r *http.Request) {
	var req dto.CreateAdminUserRequest
	if errs := utilities.DecodeAndValidate(r, &req); errs != nil {
		utilities.WriteJSON(w, http.StatusInternalServerError, map[string]any{
			"error":        errs,
			"errorMessage": "An error occurred while creating a new user",
		})
		return
	}
	if req.FirstName == "" {
		utilities.WriteJSON(w, http.StatusInternalServerError, map[string]any{
			"error":        "firstName is required",
			"errorMessage": "An error occurred while creating a new user",
		})
		return
	}
	if len(req.RoleIDs) == 0 {
		req.RoleIDs = []int{3} // default: admin
	}

	source := utilities.ResolveSource(r)
	if (source == "" || source == "qs") && h.AdminService.QsAvailable() {
		uid, err := h.AdminService.CreateQsUser(r.Context(), req.FirstName, req.LastName, req.Email, req.TimeZone, req.RoleIDs)
		if err != nil {
			slog.Error("create admin user failed", "error", err)
			utilities.WriteJSON(w, http.StatusInternalServerError, map[string]any{
				"error":        err.Error(),
				"errorMessage": "An error occurred while creating a new user",
			})
			return
		}
		// Legacy side effect: create user_communication_preferences row
		_ = h.AdminService.CreateUserCommPrefsAdmin(r.Context(), uid, req.Email)
		// Legacy returns created user row with HTTP 200
		utilities.WriteJSON(w, http.StatusOK, map[string]any{
			"id": uid, "first_name": req.FirstName, "last_name": req.LastName,
			"email": req.Email,
		})
		return
	}
	utilities.WriteJSON(w, http.StatusInternalServerError, map[string]any{
		"error":        "no database available",
		"errorMessage": "An error occurred while creating a new user",
	})
}
