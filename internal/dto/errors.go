package dto

import (
	"strconv"
	"strings"
)

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
