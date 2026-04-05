package httpkit

// PaginatedResponse wraps list endpoints with pagination metadata.
type PaginatedResponse struct {
	Success bool           `json:"success"`
	Data    any            `json:"data"`
	Meta    PaginationMeta `json:"meta"`
}

// PaginationMeta holds pagination info for list responses.
type PaginationMeta struct {
	Page       int `json:"page"`
	PageSize   int `json:"pageSize"`
	TotalCount int `json:"totalCount"`
}

// NewPaginated builds a standard paginated response.
func NewPaginated(data any, page, pageSize, total int) PaginatedResponse {
	return PaginatedResponse{
		Success: true,
		Data:    data,
		Meta:    PaginationMeta{Page: page, PageSize: pageSize, TotalCount: total},
	}
}

// ErrorBody is used for simple JSON error responses: {"error": "..."}.
type ErrorBody struct {
	Error string `json:"error"`
}

// MutationResult is returned by create/update/delete operations.
type MutationResult struct {
	ID              any    `json:"id,omitempty"`
	Updated         bool   `json:"updated,omitempty"`
	Deleted         bool   `json:"deleted,omitempty"`
	Archived        bool   `json:"archived,omitempty"`
	Source          string `json:"source,omitempty"`
	ServiceCategory string `json:"serviceCategory,omitempty"`
}
