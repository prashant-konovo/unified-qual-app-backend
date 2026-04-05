package dto

// CreateAdminUserRequest is the request body for creating an admin user.
type CreateAdminUserRequest struct {
	FirstName string `json:"firstName"`
	LastName  string `json:"lastName"`
	Email     string `json:"email"     validate:"required,email"`
	TimeZone  string `json:"timeZone"`
	RoleIDs   []int  `json:"roleIds"`
}

// MraGetPMTimeslotsRequest is the request body for fetching PM timeslots (MRA).
type MraGetPMTimeslotsRequest struct {
	Pending       bool    `json:"pending"`
	ProjectID     int64   `json:"projectId"`
	FilterBy      string  `json:"filterBy"`
	FilteredItems []int64 `json:"filteredItems"`
}

// MraGetAvailabilitiesForPMRequest is the request body for fetching PM availabilities (MRA).
type MraGetAvailabilitiesForPMRequest struct {
	FilterBy      string  `json:"filterBy"`
	FilteredItems []int64 `json:"filteredItems"`
}

// LsAddUserRolesRequest is the request body for adding user roles (LS brand).
type LsAddUserRolesRequest struct {
	Email         string `json:"email"         validate:"required,email"`
	RoleID        int    `json:"roleId"        validate:"required,gt=0"`
	ClientID      int64  `json:"clientId"      validate:"required,gt=0"`
	FirstName     string `json:"firstName"`
	LastName      string `json:"lastName"`
	CognitoUserID string `json:"cognitoUserId"`
}

// LsDeleteUserRolesRequest is the request body for deleting user roles (LS brand).
type LsDeleteUserRolesRequest struct {
	Email  string `json:"email"  validate:"required,email"`
	RoleID int    `json:"roleId" validate:"required,gt=0"`
}
