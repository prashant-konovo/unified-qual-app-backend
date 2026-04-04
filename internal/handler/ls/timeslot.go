package ls

import (
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/InCrowd/unified-qual-api/internal/handler/core"
	"github.com/InCrowd/unified-qual-api/internal/integration"
	"github.com/InCrowd/unified-qual-api/internal/repository/iris"
	"github.com/InCrowd/unified-qual-api/internal/validate"
)

// ──────────────────────────────────────────────
// LS Timeslot handlers
// ──────────────────────────────────────────────

// ──────────────────────────────────────────────
// Timeslot Sub-resources (moderators, observers)
// ──────────────────────────────────────────────

// GetTimeslotModerators returns moderators assigned to a timeslot.
// Contract-identical with legacy InCrowdAPI: GET /v1/time_slot/:tsId/moderators
// Response: flat array of moderator objects
func (h *Handler) GetTimeslotModerators(w http.ResponseWriter, r *http.Request) {
	tsID, err := validate.ParseIDParam(r, "tsId")
	if err != nil {
		core.WriteJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	source := core.ResolveSource(r)

	if source == "iris" && h.IrisSurveyRepo != nil {
		mods, err := h.IrisSurveyRepo.GetModeratorsForTimeSlot(r.Context(), tsID)
		if err != nil {
			slog.Error("get ts mods failed", "error", err)
		}
		core.WriteJSON(w, http.StatusOK, mods)
		return
	}
	if h.QsTimeSlotRepo != nil {
		mods, err := h.QsTimeSlotRepo.GetModerators(r.Context(), tsID)
		if err != nil {
			slog.Error("get qs ts mods failed", "error", err)
		}
		core.WriteJSON(w, http.StatusOK, mods)
		return
	}
	core.WriteJSON(w, http.StatusOK, []any{})
}

// GetTimeslotModeratorOptionsExt returns possible moderators for a timeslot (extended).
// Contract-identical with legacy InCrowdAPI: GET /v1/time_slot/:tsId/moderator_options
// Response: flat array of moderator option objects
func (h *Handler) GetTimeslotModeratorOptionsExt(w http.ResponseWriter, r *http.Request) {
	tsID, err := validate.ParseIDParam(r, "tsId")
	if err != nil {
		core.WriteJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	source := core.ResolveSource(r)

	if source == "iris" && h.IrisSurveyRepo != nil {
		mods, err := h.IrisSurveyRepo.GetPossibleModeratorsForTimeSlot(r.Context(), tsID)
		if err != nil {
			slog.Error("get possible mods failed", "error", err)
		}
		core.WriteJSON(w, http.StatusOK, mods)
		return
	}
	if h.QsUserRepo != nil {
		allMods, err := h.QsUserRepo.GetModerators(r.Context())
		if err != nil {
			slog.Error("get qs moderators failed", "error", err)
		}
		assigned := map[int64]bool{}
		if h.QsTimeSlotRepo != nil {
			tsMods, _ := h.QsTimeSlotRepo.GetModerators(r.Context(), tsID)
			for _, m := range tsMods {
				assigned[m.ModeratorID] = true
			}
		}
		result := make([]map[string]any, 0)
		for _, m := range allMods {
			result = append(result, map[string]any{
				"id": m.ID, "firstName": m.FirstName.String, "lastName": m.LastName.String,
				"isAssigned": assigned[m.ID],
			})
		}
		core.WriteJSON(w, http.StatusOK, result)
		return
	}
	core.WriteJSON(w, http.StatusOK, []any{})
}

// AssignTimeslotModerator assigns a moderator to a timeslot.
// Contract-identical with legacy InCrowdAPI: POST /v1/time_slot/:tsId/moderators
// Response: flat array of moderator objects (200, not 201)
func (h *Handler) AssignTimeslotModerator(w http.ResponseWriter, r *http.Request) {
	tsID, err := validate.ParseIDParam(r, "tsId")
	if err != nil {
		core.WriteJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	var req struct {
		ModeratorID int64 `json:"moderatorId"`
		IsHost      bool  `json:"isHost"`
	}
	if errs := validate.DecodeAndValidate(r, &req); errs != nil {
		validate.WriteError(w, errs)
		return
	}
	source := core.ResolveSource(r)

	if source == "iris" && h.IrisSurveyRepo != nil {
		_, err := h.IrisSurveyRepo.AssignModeratorToTimeSlot(r.Context(), tsID, req.ModeratorID, req.IsHost)
		if err != nil {
			slog.Error("assign mod failed", "error", err)
			core.WriteJSON(w, http.StatusInternalServerError, map[string]any{"error": "assign failed"})
			return
		}
		// Return updated moderators list (legacy returns array)
		mods, err := h.IrisSurveyRepo.GetModeratorsForTimeSlot(r.Context(), tsID)
		if err != nil {
			slog.Error("get mods after assign failed", "error", err)
		}
		core.WriteJSON(w, http.StatusOK, mods)
		return
	}
	core.WriteJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "not available"})
}

// UnassignTimeslotModerator removes a moderator from a timeslot.
// Contract-identical with legacy InCrowdAPI: DELETE /v1/time_slot/:tsId/moderators/:modId
// Response: flat array of remaining moderator objects
func (h *Handler) UnassignTimeslotModerator(w http.ResponseWriter, r *http.Request) {
	tsID, _ := validate.ParseIDParam(r, "tsId")
	modID, _ := validate.ParseIDParam(r, "modId")
	source := core.ResolveSource(r)

	if source == "iris" && h.IrisSurveyRepo != nil {
		// Hook: ModeratorTimeSlot.beforeDeleteHooks — cleanup calendar + invitations
		events, err := h.IrisSurveyRepo.GetTimeSlotEvents(r.Context(), tsID, &modID, nil, iris.RoleModerator)
		if err != nil {
			slog.Warn("hook: get time_slot_events failed (non-fatal)", "tsId", tsID, "error", err)
		}
		for _, evt := range events {
			if h.Services.GoogleCal.Configured() {
				if err := h.Services.GoogleCal.DeleteEvent(r.Context(), "", evt.GCalEventID); err != nil {
					slog.Warn("hook: delete gcal event failed (non-fatal)", "eventId", evt.GCalEventID, "error", err)
				}
			}
			if err := h.IrisSurveyRepo.DeleteTimeSlotEvent(r.Context(), evt.ID); err != nil {
				slog.Warn("hook: delete time_slot_event failed (non-fatal)", "id", evt.ID, "error", err)
			}
		}
		if err := h.IrisSurveyRepo.DeleteConferenceInvitations(r.Context(), tsID, &modID, nil, iris.RoleModerator); err != nil {
			slog.Warn("hook: delete conference_invitations failed (non-fatal)", "tsId", tsID, "error", err)
		}

		if err := h.IrisSurveyRepo.RemoveModeratorFromTimeSlot(r.Context(), tsID, modID); err != nil {
			slog.Error("unassign mod failed", "error", err)
			core.WriteJSON(w, http.StatusInternalServerError, map[string]any{"error": "unassign failed"})
			return
		}
		// Return remaining moderators (legacy returns array)
		mods, err := h.IrisSurveyRepo.GetModeratorsForTimeSlot(r.Context(), tsID)
		if err != nil {
			slog.Error("get mods after unassign failed", "error", err)
		}
		core.WriteJSON(w, http.StatusOK, mods)
		return
	}
	core.WriteJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "not available"})
}

// GetTimeslotObservers returns observers for a timeslot.
// Contract-identical with legacy InCrowdAPI: GET /v1/time_slot/:tsId/observers
// Response: {"observers": [...]}
func (h *Handler) GetTimeslotObservers(w http.ResponseWriter, r *http.Request) {
	tsID, err := validate.ParseIDParam(r, "tsId")
	if err != nil {
		core.WriteJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}

	if h.IrisSurveyRepo != nil {
		observers, err := h.IrisSurveyRepo.ListObserversForTimeSlot(r.Context(), tsID)
		if err != nil {
			slog.Error("get ts observers failed", "error", err)
		}
		result := make([]map[string]any, 0, len(observers))
		for _, o := range observers {
			result = append(result, map[string]any{
				"id": o.ID, "email": o.Email, "timeSlotId": core.NullInt64(o.TimeSlotID),
			})
		}
		core.WriteJSON(w, http.StatusOK, map[string]any{"observers": result})
		return
	}
	core.WriteJSON(w, http.StatusOK, map[string]any{"observers": []any{}})
}

// UpdateTimeslotObservers adds/removes observers for a timeslot.
// Contract-identical with legacy InCrowdAPI: PUT /v1/time_slot/:tsId/observers
// Response: {"observers": [...]}
func (h *Handler) UpdateTimeslotObservers(w http.ResponseWriter, r *http.Request) {
	tsID, err := validate.ParseIDParam(r, "tsId")
	if err != nil {
		core.WriteJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	var req struct {
		ProjectID int64    `json:"projectId"`
		ToAdd     []string `json:"toAdd"`
		ToDelete  []string `json:"toDelete"`
	}
	if errs := validate.DecodeAndValidate(r, &req); errs != nil {
		validate.WriteError(w, errs)
		return
	}

	if h.IrisSurveyRepo != nil {
		ctx := r.Context()

		// Hook: Observer.beforeDeleteHooks — cleanup calendar + TimeSlotEvent for each deleted observer
		for _, email := range req.ToDelete {
			obs, err := h.IrisSurveyRepo.GetObserverByEmail(ctx, req.ProjectID, tsID, email)
			if err != nil {
				slog.Warn("hook: get observer by email failed (non-fatal)", "email", email, "error", err)
				continue
			}
			if obs != nil {
				events, _ := h.IrisSurveyRepo.GetTimeSlotEvents(ctx, tsID, nil, &obs.ID, iris.RoleObserver)
				for _, evt := range events {
					if h.Services.GoogleCal.Configured() {
						if err := h.Services.GoogleCal.DeleteEvent(ctx, "", evt.GCalEventID); err != nil {
							slog.Warn("hook: delete observer gcal event failed (non-fatal)", "eventId", evt.GCalEventID, "error", err)
						}
					}
					_ = h.IrisSurveyRepo.DeleteTimeSlotEvent(ctx, evt.ID)
				}
			}
		}

		// Core DB operation: add/remove observers
		if err := h.IrisSurveyRepo.PutObserversForTimeSlot(ctx, req.ProjectID, tsID, req.ToAdd, req.ToDelete); err != nil {
			slog.Error("update observers failed", "error", err)
			core.WriteJSON(w, http.StatusInternalServerError, map[string]any{"error": "update failed"})
			return
		}

		// Hook: Observer.afterCreateHooks — create ConferenceInvitation + calendar event for each added observer
		for _, email := range req.ToAdd {
			obs, err := h.IrisSurveyRepo.GetObserverByEmail(ctx, req.ProjectID, tsID, email)
			if err != nil || obs == nil {
				slog.Warn("hook: get new observer failed (non-fatal)", "email", email, "error", err)
				continue
			}
			if err := h.IrisSurveyRepo.CreateConferenceInvitation(ctx, tsID, obs.ID); err != nil {
				slog.Warn("hook: create conference_invitation failed (non-fatal)", "observerId", obs.ID, "error", err)
			}
			// Best-effort Google Calendar event creation
			if h.Services.GoogleCal.Configured() {
				start, end, err := h.IrisSurveyRepo.GetTimeSlotTimes(ctx, tsID)
				if err == nil {
					evt := &integration.CalendarEvent{
						Summary:   fmt.Sprintf("Interview Observer - %s", email),
						Start:     integration.EventTime{DateTime: start.Format(time.RFC3339), TimeZone: "America/New_York"},
						End:       integration.EventTime{DateTime: end.Format(time.RFC3339), TimeZone: "America/New_York"},
						Attendees: []integration.Attendee{{Email: email}},
					}
					created, err := h.Services.GoogleCal.CreateEvent(ctx, "", evt)
					if err != nil {
						slog.Warn("hook: create observer gcal event failed (non-fatal)", "email", email, "error", err)
					} else if created != nil && created.ID != "" {
						_ = h.IrisSurveyRepo.CreateTimeSlotEvent(ctx, tsID, created.ID, iris.RoleObserver, nil, &obs.ID)
					}
				}
			}
		}

		// Return updated observers list (legacy returns {"observers": [...]})
		observers, err := h.IrisSurveyRepo.ListObserversForTimeSlot(ctx, tsID)
		if err != nil {
			slog.Error("get observers after update failed", "error", err)
		}
		result := make([]map[string]any, 0, len(observers))
		for _, o := range observers {
			result = append(result, map[string]any{
				"id": o.ID, "email": o.Email, "timeSlotId": core.NullInt64(o.TimeSlotID),
			})
		}
		core.WriteJSON(w, http.StatusOK, map[string]any{"observers": result})
		return
	}
	core.WriteJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "not available"})
}

// ──────────────────────────────────────────────
// Conference/Meeting extended
// ──────────────────────────────────────────────
