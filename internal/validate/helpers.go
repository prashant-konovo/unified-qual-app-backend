package validate

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
)

// ---- Error accumulator ----

// Errors collects validation errors for handlers that need
// imperative (non-struct-tag) validation or multi-field rules.
type Errors struct {
	errs []string
}

// Add appends a custom error message.
func (e *Errors) Add(msg string) {
	e.errs = append(e.errs, msg)
}

// RequireString checks that val is non-empty after trimming.
func (e *Errors) RequireString(val, field string) {
	if strings.TrimSpace(val) == "" {
		e.errs = append(e.errs, field+" is required")
	}
}

// RequireID checks that an integer ID is positive.
func (e *Errors) RequireID(val int64, field string) {
	if val <= 0 {
		e.errs = append(e.errs, field+" is required")
	}
}

// ValidateEnum checks that val is one of the allowed values.
func (e *Errors) ValidateEnum(val string, allowed []string, field string) {
	for _, a := range allowed {
		if val == a {
			return
		}
	}
	e.errs = append(e.errs, field+" must be one of: "+strings.Join(allowed, ", "))
}

// MaxLength checks that val does not exceed max characters.
func (e *Errors) MaxLength(val string, max int, field string) {
	if len(val) > max {
		e.errs = append(e.errs, field+" must be at most "+strconv.Itoa(max)+" characters")
	}
}

// MinLength checks that val has at least min characters.
func (e *Errors) MinLength(val string, min int, field string) {
	if len(val) < min {
		e.errs = append(e.errs, field+" must be at least "+strconv.Itoa(min)+" characters")
	}
}

// ValidateEmail performs a basic email format check.
func (e *Errors) ValidateEmail(val, field string) {
	at := strings.IndexByte(val, '@')
	dot := strings.LastIndexByte(val, '.')
	if at < 1 || dot < at+2 || dot >= len(val)-1 {
		e.errs = append(e.errs, field+" must be a valid email address")
	}
}

// PositiveInt checks that val > 0.
func (e *Errors) PositiveInt(val int64, field string) {
	if val <= 0 {
		e.errs = append(e.errs, field+" must be a positive integer")
	}
}

// HasErrors returns true if any errors have been collected.
func (e *Errors) HasErrors() bool { return len(e.errs) > 0 }

// Messages returns the collected error messages.
func (e *Errors) Messages() []string { return e.errs }

// ---- URL / query param helpers ----

// ParseIDParam parses a chi URL parameter as int64.
// Returns 0 and an error if the parameter is missing or not a valid integer.
func ParseIDParam(r *http.Request, param string) (int64, error) {
	raw := chi.URLParam(r, param)
	if raw == "" {
		return 0, &ParamError{Param: param, Msg: "is required"}
	}
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0, &ParamError{Param: param, Msg: "must be a valid integer"}
	}
	if id <= 0 {
		return 0, &ParamError{Param: param, Msg: "must be a positive integer"}
	}
	return id, nil
}

// ParseStringParam returns a chi URL parameter, or an error if empty.
func ParseStringParam(r *http.Request, param string) (string, error) {
	raw := chi.URLParam(r, param)
	if raw == "" {
		return "", &ParamError{Param: param, Msg: "is required"}
	}
	return raw, nil
}

// Pagination holds validated page/pageSize values.
type Pagination struct {
	Page     int
	PageSize int
}

// ParsePagination extracts page and pageSize from query params
// with sensible defaults and bounds.
func ParsePagination(r *http.Request, defaultSize, maxSize int) Pagination {
	if defaultSize <= 0 {
		defaultSize = 20
	}
	if maxSize <= 0 {
		maxSize = 100
	}

	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}

	pageSize, _ := strconv.Atoi(r.URL.Query().Get("pageSize"))
	if pageSize < 1 {
		pageSize = defaultSize
	}
	if pageSize > maxSize {
		pageSize = maxSize
	}

	return Pagination{Page: page, PageSize: pageSize}
}

// ---- Type coercion helpers ----

// CoerceInt64 handles JSON's number-or-string ambiguity.
// Useful for `any`-typed fields that may arrive as float64 or string.
func CoerceInt64(v any) int64 {
	switch val := v.(type) {
	case float64:
		return int64(val)
	case int64:
		return val
	case int:
		return int64(val)
	case string:
		n, _ := strconv.ParseInt(strings.TrimSpace(val), 10, 64)
		return n
	case nil:
		return 0
	default:
		return 0
	}
}

// CoerceString extracts a string from an any value.
func CoerceString(v any) string {
	switch val := v.(type) {
	case string:
		return val
	case nil:
		return ""
	default:
		return ""
	}
}

// ---- Error types ----

// ParamError is returned by URL/query param parsing helpers.
type ParamError struct {
	Param string
	Msg   string
}

func (e *ParamError) Error() string {
	return e.Param + " " + e.Msg
}
