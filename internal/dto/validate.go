// Package validate provides request body decoding, struct-tag validation,
// and imperative validation helpers for the unified-qual API handlers.
package dto

import (
	"encoding/json"
	"fmt"
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
