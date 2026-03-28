package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/InCrowd/unified-qual-api/internal/config"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type Handler struct {
	cfg *config.Config
	db  *config.DBPair
}

func New(cfg *config.Config, db *config.DBPair) *Handler {
	return &Handler{cfg: cfg, db: db}
}

// ──────────────────────────────────────────────
// Helpers
// ──────────────────────────────────────────────

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Brand", "unified")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func success(w http.ResponseWriter, data any) {
	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"data":    data,
	})
}

func created(w http.ResponseWriter, data any) {
	writeJSON(w, http.StatusCreated, map[string]any{
		"success": true,
		"data":    data,
	})
}

func successList(w http.ResponseWriter, data any, total int) {
	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"data":    data,
		"meta": map[string]any{
			"page":       1,
			"pageSize":   20,
			"totalCount": total,
			"totalPages": 1,
		},
	})
}

func id() string { return uuid.New().String() }

func now() string { return time.Now().UTC().Format(time.RFC3339) }

// ──────────────────────────────────────────────
// Health
// ──────────────────────────────────────────────

func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()

	dbChecks := h.db.HealthCheck(ctx)

	status := "healthy"
	httpCode := http.StatusOK
	for _, v := range dbChecks {
		if v != "ok" && v != "not_configured" {
			status = "degraded"
			httpCode = http.StatusServiceUnavailable
			break
		}
	}

	writeJSON(w, httpCode, map[string]any{
		"status":      status,
		"version":     "1.1.0",
		"environment": h.cfg.Environment,
		"checks":      dbChecks,
	})
}

// ──────────────────────────────────────────────
// Auth
// ──────────────────────────────────────────────

func (h *Handler) AuthLogin(w http.ResponseWriter, r *http.Request) {
	success(w, map[string]any{
		"user": map[string]any{
			"id":        12345,
			"email":     "demo@konovo.com",
			"firstName": "Demo",
			"lastName":  "User",
			"roles":     []string{"admin", "manager"},
			"timezone":  "America/New_York",
			"brand":     "unified",
		},
		"token":        "dummy-jwt-token-" + id(),
		"cognitoToken": "dummy-cognito-token-" + id(),
		"apiKey":       "dummy-api-key",
	})
}

func (h *Handler) AuthMagicLink(w http.ResponseWriter, r *http.Request) {
	success(w, map[string]any{
		"user": map[string]any{
			"id": 12345, "email": "magic@konovo.com",
			"firstName": "Magic", "lastName": "User",
			"roles": []string{"moderator"}, "brand": "unified",
		},
		"token": "dummy-magic-jwt-" + id(),
	})
}

func (h *Handler) AuthRefresh(w http.ResponseWriter, r *http.Request) {
	success(w, map[string]any{
		"token":        "dummy-refreshed-jwt-" + id(),
		"cognitoToken": "dummy-refreshed-cognito-" + id(),
	})
}

func (h *Handler) AuthPassword(w http.ResponseWriter, r *http.Request) {
	success(w, map[string]any{"message": "password updated successfully"})
}

// ──────────────────────────────────────────────
// Projects
// ──────────────────────────────────────────────

func dummyProject(pid string) map[string]any {
	return map[string]any{
		"id":              pid,
		"name":            "Cardiology Study Q1",
		"interviewLength": 30,
		"sampleSize":      25,
		"salesforceProject": map[string]any{
			"id": 500, "jobNumber": "SF-2026-001",
		},
		"status": "active",
		"moderators": []map[string]any{
			{"id": "mod-201", "firstName": "Jane", "lastName": "Smith"},
		},
		"scheduledCount": 15,
		"completedCount": 8,
		"createdAt":      "2026-03-01T10:00:00Z",
		"brand":          "unified",
	}
}

func (h *Handler) ListProjects(w http.ResponseWriter, r *http.Request) {
	projects := []map[string]any{
		dummyProject("proj-101"),
		dummyProject("proj-102"),
		dummyProject("proj-103"),
	}
	projects[1]["name"] = "Oncology Research Wave 3"
	projects[2]["name"] = "Neurology Follow-up Study"
	writeJSON(w, http.StatusOK, projects)
}

func (h *Handler) CreateProject(w http.ResponseWriter, r *http.Request) {
	p := dummyProject("proj-" + id()[:8])
	p["status"] = "draft"
	p["createdAt"] = now()
	created(w, p)
}

func (h *Handler) GetProject(w http.ResponseWriter, r *http.Request) {
	pid := chi.URLParam(r, "id")
	p := dummyProject(pid)
	p["conferenceLink"] = map[string]any{
		"link": "https://meet.example.com/abc", "meetingInfo": "Password: 1234",
	}
	p["scheduling"] = map[string]any{
		"totalSlots": 25, "scheduled": 15, "completed": 8, "canceled": 2, "remaining": 10,
	}
	writeJSON(w, http.StatusOK, p)
}

func (h *Handler) UpdateProject(w http.ResponseWriter, r *http.Request) {
	pid := chi.URLParam(r, "id")
	p := dummyProject(pid)
	p["updatedAt"] = now()
	writeJSON(w, http.StatusOK, p)
}

func (h *Handler) DeleteProject(w http.ResponseWriter, r *http.Request) {
	success(w, map[string]any{"deleted": true})
}

// ──────────────────────────────────────────────
// Surveys
// ──────────────────────────────────────────────

func dummySurvey(sid string) map[string]any {
	return map[string]any{
		"id":          sid,
		"title":       "Screening Survey - Cardiology",
		"description": "Pre-screening questionnaire for cardiology study",
		"status":      "active",
		"questions":   []map[string]any{},
		"rules":       []map[string]any{},
		"createdAt":   "2026-03-01T10:00:00Z",
	}
}

func (h *Handler) ListSurveys(w http.ResponseWriter, r *http.Request) {
	surveys := []map[string]any{
		dummySurvey("surv-1"),
		dummySurvey("surv-2"),
	}
	surveys[1]["title"] = "Screening Survey - Oncology"
	writeJSON(w, http.StatusOK, surveys)
}

func (h *Handler) CreateSurvey(w http.ResponseWriter, r *http.Request) {
	s := dummySurvey("surv-" + id()[:8])
	s["createdAt"] = now()
	created(w, s)
}

func (h *Handler) UpdateSurvey(w http.ResponseWriter, r *http.Request) {
	sid := chi.URLParam(r, "id")
	s := dummySurvey(sid)
	s["updatedAt"] = now()
	writeJSON(w, http.StatusOK, s)
}

func (h *Handler) DeleteSurvey(w http.ResponseWriter, r *http.Request) {
	success(w, map[string]any{"deleted": true})
}

func (h *Handler) GetPublicSurvey(w http.ResponseWriter, r *http.Request) {
	sid := chi.URLParam(r, "surveyId")
	writeJSON(w, http.StatusOK, map[string]any{
		"id":          sid,
		"projectId":   "proj-101",
		"projectName": "Cardiology Study Q1",
		"questions": []map[string]any{
			{"id": "q1", "type": "single_choice", "text": "What is your specialty?",
				"choices": []string{"Cardiology", "Oncology", "Neurology", "Other"}},
			{"id": "q2", "type": "text", "text": "Years of experience?"},
		},
		"rules": []map[string]any{},
	})
}

// ──────────────────────────────────────────────
// Survey Responses
// ──────────────────────────────────────────────

func (h *Handler) GetSurveyResponses(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, []map[string]any{
		{
			"id": "resp-1", "userId": chi.URLParam(r, "userId"),
			"projectId": "proj-101", "status": "qualified",
			"submittedAt": "2026-03-15T10:00:00Z",
		},
	})
}

func (h *Handler) SubmitSurveyResponse(w http.ResponseWriter, r *http.Request) {
	created(w, map[string]any{
		"id": "resp-" + id()[:8], "status": "qualified", "submittedAt": now(),
	})
}

func (h *Handler) SubmitParticipantSurvey(w http.ResponseWriter, r *http.Request) {
	created(w, map[string]any{
		"id": "resp-" + id()[:8], "status": "qualified", "submittedAt": now(),
	})
}

func (h *Handler) GetParticipantSurveyResponse(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, []map[string]any{
		{"id": "resp-1", "userId": chi.URLParam(r, "userId"),
			"projectId": "proj-101", "status": "qualified"},
	})
}

// ──────────────────────────────────────────────
// Timeslots
// ──────────────────────────────────────────────

func dummyTimeslot(tid string) map[string]any {
	return map[string]any{
		"id":            tid,
		"moderatorId":   "mod-201",
		"moderatorName": "Jane Smith",
		"start":         "2026-04-01T09:00:00Z",
		"end":           "2026-04-01T09:30:00Z",
		"type":          "availability",
		"projectId":     "proj-101",
		"timezone":      "America/New_York",
		"createdAt":     "2026-03-20T10:00:00Z",
	}
}

func (h *Handler) ListTimeslots(w http.ResponseWriter, r *http.Request) {
	slots := []map[string]any{
		dummyTimeslot("ts-1"),
		dummyTimeslot("ts-2"),
		dummyTimeslot("ts-3"),
	}
	slots[1]["start"] = "2026-04-01T10:00:00Z"
	slots[1]["end"] = "2026-04-01T10:30:00Z"
	slots[1]["type"] = "interview"
	slots[1]["participant"] = "Alice Johnson"
	slots[2]["start"] = "2026-04-02T09:00:00Z"
	slots[2]["end"] = "2026-04-02T17:00:00Z"
	writeJSON(w, http.StatusOK, slots)
}

func (h *Handler) CreateTimeslot(w http.ResponseWriter, r *http.Request) {
	ts := dummyTimeslot("ts-" + id()[:8])
	ts["createdAt"] = now()
	created(w, ts)
}

func (h *Handler) GetTimeslot(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, dummyTimeslot(chi.URLParam(r, "id")))
}

func (h *Handler) UpdateTimeslot(w http.ResponseWriter, r *http.Request) {
	ts := dummyTimeslot(chi.URLParam(r, "id"))
	ts["updatedAt"] = now()
	writeJSON(w, http.StatusOK, ts)
}

func (h *Handler) DeleteTimeslot(w http.ResponseWriter, r *http.Request) {
	success(w, map[string]any{"deleted": true})
}

// ──────────────────────────────────────────────
// Interview Slots
// ──────────────────────────────────────────────

func (h *Handler) GenerateSlots(w http.ResponseWriter, r *http.Request) {
	slots := []map[string]any{
		{"id": "slot-1", "projectId": "proj-101", "moderatorId": "mod-201",
			"start": "2026-04-01T09:00:00Z", "end": "2026-04-01T09:30:00Z", "capacity": 1},
		{"id": "slot-2", "projectId": "proj-101", "moderatorId": "mod-201",
			"start": "2026-04-01T09:30:00Z", "end": "2026-04-01T10:00:00Z", "capacity": 1},
		{"id": "slot-3", "projectId": "proj-101", "moderatorId": "mod-201",
			"start": "2026-04-01T10:00:00Z", "end": "2026-04-01T10:30:00Z", "capacity": 1},
	}
	writeJSON(w, http.StatusOK, slots)
}

func (h *Handler) GetAvailableSlots(w http.ResponseWriter, r *http.Request) {
	slots := []map[string]any{
		{"id": "slot-1", "projectId": "proj-101", "moderatorId": "mod-201", "moderatorName": "Jane Smith",
			"start": "2026-04-01T09:00:00Z", "end": "2026-04-01T09:30:00Z", "capacity": 1},
		{"id": "slot-2", "projectId": "proj-101", "moderatorId": "mod-201", "moderatorName": "Jane Smith",
			"start": "2026-04-01T10:00:00Z", "end": "2026-04-01T10:30:00Z", "capacity": 1},
	}
	writeJSON(w, http.StatusOK, slots)
}

func (h *Handler) GetAISuggestions(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"projectId": r.URL.Query().Get("projectId"),
		"suggestions": []map[string]any{
			{"suggestedStart": "2026-04-01T09:00:00Z", "suggestedEnd": "2026-04-01T09:30:00Z", "participantCount": 3},
			{"suggestedStart": "2026-04-01T14:00:00Z", "suggestedEnd": "2026-04-01T14:30:00Z", "participantCount": 5},
		},
	})
}

// ──────────────────────────────────────────────
// Moderators
// ──────────────────────────────────────────────

func dummyModerator(mid string) map[string]any {
	return map[string]any{
		"id":        mid,
		"name":      "Jane Smith",
		"email":     "jane.smith@konovo.com",
		"phone":     "+15551234567",
		"role":      "moderator",
		"status":    "active",
		"createdAt": "2025-06-01T10:00:00Z",
	}
}

func (h *Handler) ListModerators(w http.ResponseWriter, r *http.Request) {
	mods := []map[string]any{
		dummyModerator("mod-201"),
		dummyModerator("mod-202"),
		dummyModerator("mod-203"),
	}
	mods[1]["name"] = "Bob Wilson"
	mods[1]["email"] = "bob.wilson@konovo.com"
	mods[2]["name"] = "Carol Davis"
	mods[2]["email"] = "carol.davis@konovo.com"
	writeJSON(w, http.StatusOK, mods)
}

func (h *Handler) CreateModerator(w http.ResponseWriter, r *http.Request) {
	m := dummyModerator("mod-" + id()[:8])
	m["createdAt"] = now()
	created(w, m)
}

func (h *Handler) GetModerator(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, dummyModerator(chi.URLParam(r, "id")))
}

func (h *Handler) UpdateModerator(w http.ResponseWriter, r *http.Request) {
	m := dummyModerator(chi.URLParam(r, "id"))
	m["updatedAt"] = now()
	writeJSON(w, http.StatusOK, m)
}

func (h *Handler) DeleteModerator(w http.ResponseWriter, r *http.Request) {
	success(w, map[string]any{"deleted": true})
}

func (h *Handler) BulkUploadModerators(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"created": 5, "failed": 0, "errors": []any{},
	})
}

func (h *Handler) GetModeratorTimeslots(w http.ResponseWriter, r *http.Request) {
	modID := chi.URLParam(r, "moderatorId")
	slots := []map[string]any{
		dummyTimeslot("ts-m-1"),
		dummyTimeslot("ts-m-2"),
	}
	slots[0]["moderatorId"] = modID
	slots[1]["moderatorId"] = modID
	slots[1]["start"] = "2026-04-02T09:00:00Z"
	slots[1]["end"] = "2026-04-02T17:00:00Z"
	writeJSON(w, http.StatusOK, slots)
}

// ──────────────────────────────────────────────
// Participants
// ──────────────────────────────────────────────

func dummyParticipant(pid string) map[string]any {
	return map[string]any{
		"id":        pid,
		"name":      "Alice Johnson",
		"email":     "alice@hospital.org",
		"phone":     "+15559876543",
		"role":      "participant",
		"status":    "active",
		"createdAt": "2026-03-10T10:00:00Z",
	}
}

func (h *Handler) ListParticipants(w http.ResponseWriter, r *http.Request) {
	parts := []map[string]any{
		dummyParticipant("par-301"),
		dummyParticipant("par-302"),
		dummyParticipant("par-303"),
	}
	parts[1]["name"] = "Bob Patient"
	parts[1]["email"] = "bob@clinic.org"
	parts[2]["name"] = "Carol Respondent"
	parts[2]["email"] = "carol@lab.org"
	writeJSON(w, http.StatusOK, parts)
}

func (h *Handler) CreateParticipant(w http.ResponseWriter, r *http.Request) {
	p := dummyParticipant("par-" + id()[:8])
	p["createdAt"] = now()
	created(w, p)
}

func (h *Handler) GetParticipant(w http.ResponseWriter, r *http.Request) {
	pid := chi.URLParam(r, "id")
	p := dummyParticipant(pid)
	p["surveyResponse"] = map[string]any{
		"id": "resp-1", "projectId": "proj-101", "status": "qualified",
		"submittedAt": "2026-03-15T10:00:00Z",
		"answers":     map[string]any{"q1": "Cardiology", "q2": "10"},
	}
	p["booking"] = map[string]any{
		"id": "bk-1", "projectId": "proj-101", "slotId": "ts-1",
		"moderatorId": "mod-201", "moderatorName": "Jane Smith",
		"slotStart": "2026-04-01T09:00:00Z", "slotEnd": "2026-04-01T09:30:00Z",
		"status": "scheduled",
	}
	writeJSON(w, http.StatusOK, p)
}

// ──────────────────────────────────────────────
// Bookings
// ──────────────────────────────────────────────

func dummyBooking(bid string) map[string]any {
	return map[string]any{
		"id":              bid,
		"userId":          "par-301",
		"projectId":       "proj-101",
		"projectName":     "Cardiology Study Q1",
		"slotId":          "ts-1",
		"moderatorId":     "mod-201",
		"moderatorName":   "Jane Smith",
		"participantName": "Alice Johnson",
		"slotStart":       "2026-04-01T09:00:00Z",
		"slotEnd":         "2026-04-01T09:30:00Z",
		"meetingLink":     "https://meet.example.com/abc",
		"status":          "scheduled",
		"rewardPoints":    150,
		"rewardStatus":    "not_credited",
		"createdAt":       "2026-03-20T10:00:00Z",
	}
}

func (h *Handler) ListBookings(w http.ResponseWriter, r *http.Request) {
	bookings := []map[string]any{
		dummyBooking("bk-1"),
		dummyBooking("bk-2"),
		dummyBooking("bk-3"),
	}
	bookings[1]["participantName"] = "Bob Patient"
	bookings[1]["status"] = "completed"
	bookings[1]["rewardStatus"] = "credited"
	bookings[2]["participantName"] = "Carol Respondent"
	bookings[2]["status"] = "cancelled"
	writeJSON(w, http.StatusOK, bookings)
}

func (h *Handler) CreateBooking(w http.ResponseWriter, r *http.Request) {
	b := dummyBooking("bk-" + id()[:8])
	b["createdAt"] = now()
	created(w, b)
}

func (h *Handler) GetBookingsByUser(w http.ResponseWriter, r *http.Request) {
	b := dummyBooking("bk-1")
	b["userId"] = chi.URLParam(r, "userId")
	writeJSON(w, http.StatusOK, []map[string]any{b})
}

func (h *Handler) UpdateBooking(w http.ResponseWriter, r *http.Request) {
	b := dummyBooking(chi.URLParam(r, "id"))
	b["updatedAt"] = now()
	writeJSON(w, http.StatusOK, b)
}

func (h *Handler) UpdateBookingReward(w http.ResponseWriter, r *http.Request) {
	success(w, map[string]any{"updated": true})
}

// ──────────────────────────────────────────────
// Subscriptions
// ──────────────────────────────────────────────

func dummySubscription(sid string) map[string]any {
	return map[string]any{
		"id":                  sid,
		"company":             "Konovo Health",
		"plan":                "enterprise",
		"shortCode":           "KH",
		"serviceType":         "full-service",
		"businessType":        "pharmaceutical",
		"salesforceAccount":   "SF-ACC-001",
		"salesContact":        "sales@konovo.com",
		"pmContact":           "pm@konovo.com",
		"csUser":              "cs@konovo.com",
		"currency":            "USD",
		"markets":             "US,EU",
		"panels":              "HCP",
		"phone":               "+15551234567",
		"aeConsent":           true,
		"aeReporting":         "quarterly",
		"skipSfValidation":    false,
		"createdAt":           "2025-01-01T00:00:00Z",
	}
}

func (h *Handler) ListSubscriptions(w http.ResponseWriter, r *http.Request) {
	subs := []map[string]any{
		dummySubscription("sub-1"),
		dummySubscription("sub-2"),
	}
	subs[1]["company"] = "Apollo Research"
	subs[1]["shortCode"] = "AR"
	writeJSON(w, http.StatusOK, subs)
}

func (h *Handler) CreateSubscription(w http.ResponseWriter, r *http.Request) {
	s := dummySubscription("sub-" + id()[:8])
	s["createdAt"] = now()
	created(w, s)
}

func (h *Handler) GetSubscription(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, dummySubscription(chi.URLParam(r, "id")))
}

func (h *Handler) UpdateSubscription(w http.ResponseWriter, r *http.Request) {
	s := dummySubscription(chi.URLParam(r, "id"))
	s["updatedAt"] = now()
	writeJSON(w, http.StatusOK, s)
}

func (h *Handler) DeleteSubscription(w http.ResponseWriter, r *http.Request) {
	success(w, map[string]any{"deleted": true})
}

// ──────────────────────────────────────────────
// Waiting Queue
// ──────────────────────────────────────────────

func (h *Handler) GetWaitingQueue(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, []map[string]any{
		{
			"id": "wq-1", "userId": "par-301", "projectId": "proj-101",
			"participantName": "Alice Johnson", "participantEmail": "alice@hospital.org",
			"projectName": "Cardiology Study Q1",
			"preferredStart": "09:00", "preferredEnd": "17:00",
			"preferredDays": []string{"Monday", "Wednesday", "Friday"},
			"timezone": "America/New_York", "status": "waiting",
			"waitingSince": "2026-03-18T10:00:00Z",
		},
		{
			"id": "wq-2", "userId": "par-302", "projectId": "proj-101",
			"participantName": "Bob Patient", "participantEmail": "bob@clinic.org",
			"projectName": "Cardiology Study Q1",
			"preferredStart": "10:00", "preferredEnd": "14:00",
			"preferredDays": []string{"Tuesday", "Thursday"},
			"timezone": "America/Chicago", "status": "waiting",
			"waitingSince": "2026-03-19T10:00:00Z",
		},
	})
}

func (h *Handler) AddToWaitingQueue(w http.ResponseWriter, r *http.Request) {
	created(w, map[string]any{
		"id": "wq-" + id()[:8], "status": "waiting", "waitingSince": now(),
	})
}

func (h *Handler) RemoveFromWaitingQueue(w http.ResponseWriter, r *http.Request) {
	success(w, map[string]any{"deleted": true})
}

func (h *Handler) TriggerMatching(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"assigned": 2, "invited": 3, "message": "Matching complete. 2 assigned, 3 invited.",
	})
}

// ──────────────────────────────────────────────
// Scheduler (LLD endpoints)
// ──────────────────────────────────────────────

func (h *Handler) ScheduleInterview(w http.ResponseWriter, r *http.Request) {
	created(w, map[string]any{
		"interviewId":    "int-" + id()[:8],
		"timeSlotId":     301,
		"conferenceLink": "https://chime.aws/meeting/abc123",
		"startTime":      "2026-04-01T14:00:00Z",
		"endTime":        "2026-04-01T14:30:00Z",
		"status":         "scheduled",
	})
}

func (h *Handler) CancelInterview(w http.ResponseWriter, r *http.Request) {
	success(w, map[string]any{
		"interviewId": chi.URLParam(r, "id"), "status": "canceled",
	})
}

func (h *Handler) RescheduleInterview(w http.ResponseWriter, r *http.Request) {
	success(w, map[string]any{
		"interviewId": chi.URLParam(r, "id"), "status": "rescheduled",
	})
}

// ──────────────────────────────────────────────
// Moderator Availability (LLD endpoints)
// ──────────────────────────────────────────────

func (h *Handler) GetModeratorAvailability(w http.ResponseWriter, r *http.Request) {
	success(w, map[string]any{
		"availabilities": []map[string]any{
			{"id": 801, "moderatorId": chi.URLParam(r, "id"),
				"startTime": "2026-04-01T09:00:00Z", "endTime": "2026-04-01T17:00:00Z",
				"isImported": false},
			{"id": 802, "moderatorId": chi.URLParam(r, "id"),
				"startTime": "2026-04-02T09:00:00Z", "endTime": "2026-04-02T12:00:00Z",
				"isImported": true, "projectId": "proj-101"},
		},
	})
}

func (h *Handler) PostModeratorAvailability(w http.ResponseWriter, r *http.Request) {
	created(w, map[string]any{
		"id": 803, "moderatorId": chi.URLParam(r, "id"),
		"startTime": "2026-04-05T09:00:00Z", "endTime": "2026-04-05T17:00:00Z",
	})
}

func (h *Handler) GetTimeslotModeratorOptions(w http.ResponseWriter, r *http.Request) {
	success(w, map[string]any{
		"availableModerators": []map[string]any{
			{"id": "mod-201", "firstName": "Jane", "lastName": "Smith",
				"hasConflict": false, "availabilityId": 801},
			{"id": "mod-202", "firstName": "Bob", "lastName": "Wilson",
				"hasConflict": true, "conflictReason": "Overlapping interview at 14:00-14:30"},
		},
	})
}

// ──────────────────────────────────────────────
// Conference (LLD endpoints)
// ──────────────────────────────────────────────

func (h *Handler) MeetingAction(w http.ResponseWriter, r *http.Request) {
	success(w, map[string]any{
		"meetingId": chi.URLParam(r, "meetingId"),
		"action":    chi.URLParam(r, "action"),
		"result":    "success",
	})
}

func (h *Handler) MeetingUniversalJoin(w http.ResponseWriter, r *http.Request) {
	success(w, map[string]any{
		"joinUrl":    "https://chime.aws/join/" + chi.URLParam(r, "meetingId"),
		"attendeeId": "att-" + id()[:8],
	})
}

// ──────────────────────────────────────────────
// Payments (LLD endpoints)
// ──────────────────────────────────────────────

func (h *Handler) CreatePayment(w http.ResponseWriter, r *http.Request) {
	created(w, map[string]any{"paymentId": 7001, "status": "PENDING"})
}

func (h *Handler) CreateCustomHonorarium(w http.ResponseWriter, r *http.Request) {
	success(w, map[string]any{"timeSlotId": 301, "honorarium": 200.00, "reasonId": 2})
}

func (h *Handler) GetPaymentStatusList(w http.ResponseWriter, r *http.Request) {
	success(w, map[string]any{
		"payments": []map[string]any{
			{"timeSlotId": 301, "respondentName": "Alice Johnson", "amount": 150.00,
				"currency": "USD", "status": "PENDING", "source": "QS",
				"updatedAt": "2026-03-20T12:00:00Z"},
		},
	})
}

// ──────────────────────────────────────────────
// Translations (LLD endpoints)
// ──────────────────────────────────────────────

func (h *Handler) GetLocales(w http.ResponseWriter, r *http.Request) {
	success(w, map[string]any{
		"locales": []map[string]string{
			{"code": "en_us", "name": "English (US)"},
			{"code": "es_es", "name": "Spanish"},
			{"code": "fr_fr", "name": "French"},
			{"code": "fr_ca", "name": "French (Canada)"},
			{"code": "de_de", "name": "German"},
			{"code": "it_it", "name": "Italian"},
			{"code": "pt_pt", "name": "Portuguese"},
		},
	})
}

func (h *Handler) UpdateTopicTranslations(w http.ResponseWriter, r *http.Request) {
	success(w, map[string]any{
		"projectId":         chi.URLParam(r, "projectId"),
		"translationsCount": 2,
	})
}

// ──────────────────────────────────────────────
// Notifications (LLD endpoints)
// ──────────────────────────────────────────────

func (h *Handler) GetEmailTemplate(w http.ResponseWriter, r *http.Request) {
	success(w, map[string]any{
		"subject":      "Your Interview Has Been Rescheduled",
		"body":         "<html><body><p>Dear {{.Name}}, your interview has been rescheduled.</p></body></html>",
		"templateType": r.URL.Query().Get("type"),
		"language":     r.URL.Query().Get("language"),
	})
}

func (h *Handler) SendReminder(w http.ResponseWriter, r *http.Request) {
	success(w, map[string]any{"sent": true, "recipientCount": 2})
}

// ──────────────────────────────────────────────
// Admin (LLD endpoints)
// ──────────────────────────────────────────────

func (h *Handler) ListAdminUsers(w http.ResponseWriter, r *http.Request) {
	successList(w, map[string]any{
		"users": []map[string]any{
			{"id": 12345, "email": "john@konovo.com", "firstName": "John", "lastName": "Doe",
				"roles": []string{"manager"}, "createdAt": "2025-01-15T10:00:00Z",
				"lastLogin": "2026-03-20T09:30:00Z"},
			{"id": 12346, "email": "jane@konovo.com", "firstName": "Jane", "lastName": "Smith",
				"roles": []string{"admin"}, "createdAt": "2025-02-01T10:00:00Z",
				"lastLogin": "2026-03-21T14:00:00Z"},
		},
	}, 2)
}

func (h *Handler) CreateAdminUser(w http.ResponseWriter, r *http.Request) {
	created(w, map[string]any{
		"id": 12347, "email": "newuser@konovo.com",
	})
}
