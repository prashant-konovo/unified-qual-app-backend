package dto

import (
	"github.com/InCrowd/unified-qual-api/internal/httpkit"
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
		"description":         httpkit.NullStrPtr(p.Description),
		"subscriptionId":      p.SubscriptionID,
		"subscriptionCompany": httpkit.NullStrPtr(p.SubscriptionCompany),
		"statusId":            p.ProjectStatusID,
		"status":              p.ProjectStatusName,
		"projectTypeId":       p.ProjectTypeID,
		"salesforceProjectId": httpkit.NullStrPtr(p.SalesforceProjectID),
		"isArchived":          p.IsArchived,
		"createdAt":           p.CreatedOn.Format(time.RFC3339),
		"modifiedAt":          httpkit.NullTimeStr(p.ModifiedOn),
		"source":              "iris",
		"serviceCategory":     "LS",
	}
}

// ProjectFromQSList converts a QS project list row to a response map.
func ProjectFromQSList(p qs.ProjectListRow) map[string]any {
	return map[string]any{
		"id":                  p.ID,
		"name":                p.Name,
		"salesforceJobNumber": httpkit.NullStrPtr(p.SalesforceJobNumber),
		"clientId":            httpkit.NullInt64Ptr(p.ClientID),
		"clientCompany":       httpkit.NullStrPtr(p.ClientCompany),
		"sampleSize":          httpkit.NullInt64Ptr(p.SampleSize),
		"interviewLength":     httpkit.NullInt64Ptr(p.InterviewLength),
		"statusId":            p.ProjectStatusID,
		"status":              p.ProjectStatusName,
		"scheduledCount":      p.ScheduledCount,
		"completedCount":      p.CompletedCount,
		"createdAt":           p.CreatedOn.Format(time.RFC3339),
		"modifiedAt":          httpkit.NullTimeStr(p.ModifiedOn),
		"source":              "qs",
		"serviceCategory":     "MRA",
	}
}

// ProjectDetailFromIRIS builds the GetProject response for an IRIS project.
func ProjectDetailFromIRIS(p *iris.Project, statusName string) map[string]any {
	return map[string]any{
		"id":                  p.ID,
		"name":                p.Name,
		"description":         httpkit.NullStrPtr(p.Description),
		"subscriptionId":      p.SubscriptionID,
		"statusId":            p.ProjectStatusID,
		"status":              statusName,
		"projectTypeId":       p.ProjectTypeID,
		"salesforceProjectId": httpkit.NullStrPtr(p.SalesforceProjectID),
		"isPrivate":           p.IsPrivate,
		"isArchived":          p.IsArchived,
		"createdAt":           p.CreatedOn.Format(time.RFC3339),
		"modifiedAt":          httpkit.NullTimeStr(p.ModifiedOn),
		"source":              "iris",
		"serviceCategory":     "LS",
	}
}

// ProjectDetailFromQS builds the GetProject response for a QS project.
func ProjectDetailFromQS(p *qs.Project, scheduled, completed int, topicNames []string) map[string]any {
	return map[string]any{
		"id":                 p.ID,
		"name":               p.Name,
		"externalSurveyId":   httpkit.NullStrPtr(p.ExternalSurveyID),
		"salesforceJobNumber": httpkit.NullStrPtr(p.SalesforceJobNumber),
		"clientId":           httpkit.NullInt64Ptr(p.ClientID),
		"sampleSize":         httpkit.NullInt64Ptr(p.SampleSize),
		"interviewLength":    httpkit.NullInt64Ptr(p.InterviewLength),
		"statusId":           p.ProjectStatusID,
		"schedulerGenerated": p.SchedulerGenerated,
		"postScreeninBuffer": httpkit.NullStrPtr(p.PostScreeninBuffer),
		"moderatorBuffer":    httpkit.NullStrPtr(p.ModeratorBuffer),
		"topics":             topicNames,
		"scheduledCount":     scheduled,
		"completedCount":     completed,
		"createdAt":          p.CreatedOn.Format(time.RFC3339),
		"modifiedAt":         httpkit.NullTimeStr(p.ModifiedOn),
		"source":             "qs",
		"serviceCategory":    "MRA",
	}
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

// MraUpdateExternalSurveyIDRequest is the request body for updating the external survey ID (MRA).
type MraUpdateExternalSurveyIDRequest struct {
	ExternalSurveyID string `json:"externalSurveyId"`
}

// MraResetProjectModeratorsRequest is the request body for resetting project moderators (MRA).
type MraResetProjectModeratorsRequest struct {
	ModeratorIDs []int64 `json:"moderatorIds"`
}

// MraHandleProjectExportRequest is the request body for project export (MRA).
type MraHandleProjectExportRequest struct {
	PMTimeZone     string `json:"pmTimeZone"`
	PMTimeZoneAbbr string `json:"pmTimeZoneAbbr"`
}

// MraUpdateSampleSizeRequest is the request body for updating a project's sample size (MRA).
type MraUpdateSampleSizeRequest struct {
	SampleSize int64 `json:"sampleSize"`
}
