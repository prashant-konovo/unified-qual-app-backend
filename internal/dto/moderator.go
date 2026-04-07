package dto

// CreateModeratorRequest is the request body for creating a moderator.
type CreateModeratorRequest struct {
	FirstName string `json:"firstName"`
	LastName  string `json:"lastName"`
	Email     string `json:"email"    validate:"required,email"`
	TimeZone  string `json:"timeZone"`
}

// UpdateModeratorRequest is the request body for updating a moderator.
type UpdateModeratorRequest struct {
	FirstName string `json:"firstName"`
	LastName  string `json:"lastName"`
	TimeZone  string `json:"timezone"`
	Source    string `json:"source"`
}

// PostModeratorAvailabilityRequest is the request body for creating moderator availability.
type PostModeratorAvailabilityRequest struct {
	ClientID  int64  `json:"clientId"`
	StartTime string `json:"startTime"`
	EndTime   string `json:"endTime"`
}

// MraPostModeratorAvailabilityRequest is the request body for posting moderator availability (MRA).
type MraPostModeratorAvailabilityRequest struct {
	ModeratorID int64  `json:"moderatorId"`
	ClientID    int64  `json:"clientId"`
	StartTime   string `json:"startTime" validate:"required"`
	EndTime     string `json:"endTime" validate:"required"`
}

// MraUpdateModeratorAvailabilityRequest is the request body for updating moderator availability (MRA).
type MraUpdateModeratorAvailabilityRequest struct {
	StartTime string `json:"startTime"`
	EndTime   string `json:"endTime"`
}

// MraGetModeratorTimeslotsRequest is the request body for filtering moderator timeslots (MRA).
type MraGetModeratorTimeslotsRequest struct {
	ProjectsToFilter []int64 `json:"projectsToFilter"`
}

// MraGetModeratorInterviewsRequest is the request body for fetching moderator interviews (MRA).
type MraGetModeratorInterviewsRequest struct {
	ProjectsIDs       string `json:"projectsIds"`
	PaymentStatusCode string `json:"paymentStatusCode"`
}

// MraUpdateModeratorBufferRequest is the request body for updating the moderator buffer (MRA).
type MraUpdateModeratorBufferRequest struct {
	ModeratorBuffer      int   `json:"moderatorBuffer"`
	UpdateAvailabilities *bool `json:"updateAvailabilities,omitempty"`
}

// MraStartModeratorImportRequest is the request body for starting a moderator calendar import (MRA).
type MraStartModeratorImportRequest struct {
	ExternalCalendarInput    string `json:"externalCalendarInput"`
	ExternalCalendarKeyInput string `json:"externalCalendarKeyInput"`
	ForceUpdate              bool   `json:"forceUpdate"`
	UserID                   any    `json:"userId"`
}

// MraUnlinkImportedModeratorRequest is the request body for unlinking an imported moderator (MRA).
type MraUnlinkImportedModeratorRequest struct {
	ClientID int64 `json:"clientId"`
}

// MraUpsertModeratorTimeRangeRequest is the request body for upserting a moderator time range (MRA).
type MraUpsertModeratorTimeRangeRequest struct {
	StartTime string `json:"startTime"`
	EndTime   string `json:"endTime"`
	Timezone  string `json:"timezone"`
}

// MraModTimeRange holds a moderator's time-range window (MRA).
type MraModTimeRange struct {
	StartTime string
	EndTime   string
	Timezone  string
}

// LsModeratorAvailabilityTimeRequest is the request body for creating/updating moderator availability by time range (LS brand).
type LsModeratorAvailabilityTimeRequest struct {
	StartTime string `json:"startTime" validate:"required"`
	EndTime   string `json:"endTime"   validate:"required"`
}

// LsUpdateGoogleSheetRequest is the request body for updating a Google Sheets cell (LS brand).
type LsUpdateGoogleSheetRequest struct {
	SheetName string `json:"sheetName"`
	CellRange string `json:"cellRange"`
	Value     string `json:"value"`
}
