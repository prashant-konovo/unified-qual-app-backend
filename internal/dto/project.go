package dto

import (
	"database/sql"
	"time"

	"github.com/InCrowd/unified-qual-api/internal/repository/iris"
	qs "github.com/InCrowd/unified-qual-api/internal/repository/qs"
)

// ──────────────────────────────────────────────
// Project list item mappers (dual-source → map[string]any for exact JSON compat)
// ──────────────────────────────────────────────

// ProjectFromIRISList converts an IRIS project list row to a response map.
func ProjectFromIRISList(p iris.ProjectListRow) map[string]any {
	return map[string]any{
		"id":                  p.ID,
		"name":                p.Name,
		"description":         nullStr(p.Description),
		"subscriptionId":      p.SubscriptionID,
		"subscriptionCompany": nullStr(p.SubscriptionCompany),
		"statusId":            p.ProjectStatusID,
		"status":              p.ProjectStatusName,
		"projectTypeId":       p.ProjectTypeID,
		"salesforceProjectId": nullStr(p.SalesforceProjectID),
		"isArchived":          p.IsArchived,
		"createdAt":           p.CreatedOn.Format(time.RFC3339),
		"modifiedAt":          nullTimeStr(p.ModifiedOn),
		"source":              "iris",
		"serviceCategory":     "LS",
	}
}

// ProjectFromQSList converts a QS project list row to a response map.
func ProjectFromQSList(p qs.ProjectListRow) map[string]any {
	return map[string]any{
		"id":                  p.ID,
		"name":                p.Name,
		"salesforceJobNumber": nullStr(p.SalesforceJobNumber),
		"clientId":            nullInt64(p.ClientID),
		"clientCompany":       nullStr(p.ClientCompany),
		"sampleSize":          nullInt64(p.SampleSize),
		"interviewLength":     nullInt64(p.InterviewLength),
		"statusId":            p.ProjectStatusID,
		"status":              p.ProjectStatusName,
		"scheduledCount":      p.ScheduledCount,
		"completedCount":      p.CompletedCount,
		"createdAt":           p.CreatedOn.Format(time.RFC3339),
		"modifiedAt":          nullTimeStr(p.ModifiedOn),
		"source":              "qs",
		"serviceCategory":     "MRA",
	}
}

// ProjectDetailFromIRIS builds the GetProject response for an IRIS project.
func ProjectDetailFromIRIS(p *iris.Project, statusName string) map[string]any {
	return map[string]any{
		"id":                  p.ID,
		"name":                p.Name,
		"description":         nullStr(p.Description),
		"subscriptionId":      p.SubscriptionID,
		"statusId":            p.ProjectStatusID,
		"status":              statusName,
		"projectTypeId":       p.ProjectTypeID,
		"salesforceProjectId": nullStr(p.SalesforceProjectID),
		"isPrivate":           p.IsPrivate,
		"isArchived":          p.IsArchived,
		"createdAt":           p.CreatedOn.Format(time.RFC3339),
		"modifiedAt":          nullTimeStr(p.ModifiedOn),
		"source":              "iris",
		"serviceCategory":     "LS",
	}
}

// ProjectDetailFromQS builds the GetProject response for a QS project.
func ProjectDetailFromQS(p *qs.Project, scheduled, completed int, topicNames []string) map[string]any {
	return map[string]any{
		"id":                 p.ID,
		"name":               p.Name,
		"externalSurveyId":   nullStr(p.ExternalSurveyID),
		"salesforceJobNumber": nullStr(p.SalesforceJobNumber),
		"clientId":           nullInt64(p.ClientID),
		"sampleSize":         nullInt64(p.SampleSize),
		"interviewLength":    nullInt64(p.InterviewLength),
		"statusId":           p.ProjectStatusID,
		"schedulerGenerated": p.SchedulerGenerated,
		"postScreeninBuffer": nullStr(p.PostScreeninBuffer),
		"moderatorBuffer":    nullStr(p.ModeratorBuffer),
		"topics":             topicNames,
		"scheduledCount":     scheduled,
		"completedCount":     completed,
		"createdAt":          p.CreatedOn.Format(time.RFC3339),
		"modifiedAt":         nullTimeStr(p.ModifiedOn),
		"source":             "qs",
		"serviceCategory":    "MRA",
	}
}

// ──────────────────────────────────────────────
// Null helpers (package-private, same as handler/helpers.go)
// ──────────────────────────────────────────────

func nullStr(s sql.NullString) *string {
	if !s.Valid {
		return nil
	}
	return &s.String
}

func nullInt64(n sql.NullInt64) *int64 {
	if !n.Valid {
		return nil
	}
	return &n.Int64
}

func nullTimeStr(t sql.NullTime) *string {
	if !t.Valid {
		return nil
	}
	s := t.Time.Format(time.RFC3339)
	return &s
}

// CreateProjectRequest is the request body for creating a project.
type CreateProjectRequest struct {
	Name                string  `json:"name"                validate:"required"`
	Description         string  `json:"description"`
	SubscriptionID      int64   `json:"subscriptionId"`
	SalesforceProjectID string  `json:"salesforceProjectId"`
	SalesforceJobNumber string  `json:"salesforceJobNumber"`
	SampleSize          int64   `json:"sampleSize"`
	InterviewLength     int64   `json:"interviewLength"`
	ClientID            int64   `json:"clientId"`
	PostScreeninBuffer  float64 `json:"postScreeninBuffer"`
	ModeratorBuffer     float64 `json:"moderatorBuffer"`
	Source              string  `json:"source"` // "iris" or "qs"
}

// UpdateProjectRequest is the request body for updating a project.
type UpdateProjectRequest struct {
	Name                string `json:"name"`
	Description         string `json:"description"`
	StatusID            *int   `json:"statusId"`
	SalesforceProjectID string `json:"salesforceProjectId"`
	SampleSize          *int64 `json:"sampleSize"`
	InterviewLength     *int64 `json:"interviewLength"`
	IsArchived          *bool  `json:"isArchived"`
	Source              string `json:"source"`
}
