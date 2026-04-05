// Package validate provides request body decoding, struct-tag validation,
// and imperative validation helpers for the unified-qual API handlers.
package utilities

import (
	"encoding/json"
	"fmt"
	"strconv"
	"net/http"
	"reflect"
	"strings"

	"github.com/go-playground/validator/v10"
)

// V is the package-level validator instance.
var V = validator.New(validator.WithRequiredStructEnabled())

func init() {
	// Use JSON tag names in error messages instead of Go struct field names.
	V.RegisterTagNameFunc(func(fld reflect.StructField) string {
		name := strings.SplitN(fld.Tag.Get("json"), ",", 2)[0]
		if name == "-" || name == "" {
			return ""
		}
		return name
	})
}

// DecodeAndValidate decodes the JSON request body into dst and runs
// struct-tag validation. Returns nil on success or a slice of
// human-readable error messages on failure.
func DecodeAndValidate(r *http.Request, dst any) []string {
	if err := json.NewDecoder(r.Body).Decode(dst); err != nil {
		return []string{"invalid JSON request body"}
	}
	return Struct(dst)
}

// Struct runs struct-tag validation on v and returns error messages.
// Returns nil if valid.
func Struct(v any) []string {
	if err := V.Struct(v); err != nil {
		return FormatErrors(err)
	}
	return nil
}

// FormatErrors converts a validator error into user-friendly messages.
func FormatErrors(err error) []string {
	ve, ok := err.(validator.ValidationErrors)
	if !ok {
		return []string{err.Error()}
	}
	msgs := make([]string, 0, len(ve))
	for _, fe := range ve {
		msgs = append(msgs, formatFieldError(fe))
	}
	return msgs
}

func formatFieldError(fe validator.FieldError) string {
	field := fe.Field()
	switch fe.Tag() {
	case "required":
		return field + " is required"
	case "email":
		return field + " must be a valid email address"
	case "min":
		return fmt.Sprintf("%s must be at least %s characters", field, fe.Param())
	case "max":
		return fmt.Sprintf("%s must be at most %s characters", field, fe.Param())
	case "len":
		return fmt.Sprintf("%s must be exactly %s characters", field, fe.Param())
	case "oneof":
		return fmt.Sprintf("%s must be one of: %s", field, fe.Param())
	case "gt":
		return fmt.Sprintf("%s must be greater than %s", field, fe.Param())
	case "gte":
		return fmt.Sprintf("%s must be greater than or equal to %s", field, fe.Param())
	case "lt":
		return fmt.Sprintf("%s must be less than %s", field, fe.Param())
	case "lte":
		return fmt.Sprintf("%s must be less than or equal to %s", field, fe.Param())
	case "url":
		return field + " must be a valid URL"
	case "uuid":
		return field + " must be a valid UUID"
	case "numeric":
		return field + " must be numeric"
	default:
		return fmt.Sprintf("%s failed validation: %s", field, fe.Tag())
	}
}

// WriteError writes a standardised 400 validation-error response.
// It matches the existing handler convention: {"error": ...}.
// When there is a single error it writes a string; multiple errors
// are written as a string slice.
func WriteError(w http.ResponseWriter, errors []string) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Brand", "unified")
	w.WriteHeader(http.StatusBadRequest)

	var payload map[string]any
	if len(errors) == 1 {
		payload = map[string]any{"error": errors[0]}
	} else {
		payload = map[string]any{"error": errors}
	}
	_ = json.NewEncoder(w).Encode(payload)
}

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
