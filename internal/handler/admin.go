package handler

import (
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/InCrowd/unified-qual-api/internal/validate"
)

// ──────────────────────────────────────────────
// Admin handlers (shared)
// ──────────────────────────────────────────────

// ──────────────────────────────────────────────
// Admin (LLD endpoints)
// ──────────────────────────────────────────────

func (h *Handler) ListAdminUsers(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	page, pageSize := parsePagination(r)
	search := r.URL.Query().Get("search")
	source := r.URL.Query().Get("source") // "qs", "iris", or "" (both)

	var allUsers []map[string]any

	// QS users
	if (source == "" || source == "qs") && h.qsUserRepo != nil {
		users, total, err := h.qsUserRepo.List(ctx, page, pageSize, nil, search)
		if err != nil {
			slog.ErrorContext(ctx, "list QS users failed", "error", err)
		} else {
			for _, u := range users {
				roles := []string{}
				for _, rid := range parseRoleCSV(u.RoleIDs) {
					roles = append(roles, qsRoleName(rid))
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
	if (source == "" || source == "iris") && h.irisUserRepo != nil {
		users, total, err := h.irisUserRepo.List(ctx, page, pageSize, nil, search)
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

	writeJSON(w, http.StatusOK, map[string]any{"users": allUsers})
}

func (h *Handler) CreateAdminUser(w http.ResponseWriter, r *http.Request) {
	var req struct {
		FirstName string `json:"firstName"`
		LastName  string `json:"lastName"`
		Email     string `json:"email"     validate:"required,email"`
		TimeZone  string `json:"timeZone"`
		RoleIDs   []int  `json:"roleIds"`
	}
	if errs := validate.DecodeAndValidate(r, &req); errs != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"error":        errs,
			"errorMessage": "An error occured while creating a new user",
		})
		return
	}
	if req.FirstName == "" {
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"error":        "firstName is required",
			"errorMessage": "An error occured while creating a new user",
		})
		return
	}
	if len(req.RoleIDs) == 0 {
		req.RoleIDs = []int{3} // default: admin
	}

	source := resolveSource(r)
	if (source == "" || source == "qs") && h.qsUserRepo != nil {
		uid, err := h.qsUserRepo.Create(r.Context(), req.FirstName, req.LastName, req.Email, req.TimeZone, req.RoleIDs)
		if err != nil {
			slog.Error("create admin user failed", "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]any{
				"error":        err.Error(),
				"errorMessage": "An error occured while creating a new user",
			})
			return
		}
		// Legacy side effect: create user_communication_preferences row
		if h.db.QS != nil {
			_, _ = h.db.QS.ExecContext(r.Context(),
				`INSERT INTO user_communication_preferences (user_id, email, allow_contact_by_email, modified_by, created_by) VALUES (?, ?, 0, ?, ?)`,
				uid, req.Email, uid, uid)
		}
		// Legacy returns created user row with HTTP 200
		writeJSON(w, http.StatusOK, map[string]any{
			"id": uid, "first_name": req.FirstName, "last_name": req.LastName,
			"email": req.Email,
		})
		return
	}
	writeJSON(w, http.StatusInternalServerError, map[string]any{
		"error":        "no database available",
		"errorMessage": "An error occured while creating a new user",
	})
}
