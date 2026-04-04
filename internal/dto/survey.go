package dto

import (
	"encoding/json"
	"fmt"
	"time"

	qs "github.com/InCrowd/unified-qual-api/internal/repository/qs"
)

// ──────────────────────────────────────────────
// Survey response mappers
// ──────────────────────────────────────────────

// SurveyFromRow converts a QS SurveyRow to a response map.
// This matches the existing surveyToMap() shape for backward compatibility.
func SurveyFromRow(s *qs.SurveyRow) map[string]any {
	var questions []any
	var rules []any
	_ = json.Unmarshal([]byte(s.Questions), &questions)
	_ = json.Unmarshal([]byte(s.Rules), &rules)
	if questions == nil {
		questions = []any{}
	}
	if rules == nil {
		rules = []any{}
	}
	m := map[string]any{
		"id":        fmt.Sprintf("%d", s.ID),
		"title":     s.Title,
		"status":    s.Status,
		"questions": questions,
		"rules":     rules,
		"crowdId":   "",
		"crowdName": "",
		"createdAt": s.CreatedOn.Format(time.RFC3339),
		"updatedAt": s.ModifiedOn.Format(time.RFC3339),
	}
	if s.ProjectID.Valid {
		m["projectId"] = fmt.Sprintf("%d", s.ProjectID.Int64)
	} else {
		m["projectId"] = ""
	}
	if s.ProjectName.Valid {
		m["projectName"] = s.ProjectName.String
	} else {
		m["projectName"] = ""
	}
	return m
}

// CreateSurveyRequest is the request body for creating a survey.
type CreateSurveyRequest struct {
	Title     string          `json:"title"`
	ProjectID *int64          `json:"projectId,omitempty"`
	Status    string          `json:"status"`
	Questions json.RawMessage `json:"questions"`
	Rules     json.RawMessage `json:"rules"`
}
