package utilities

import (
	"database/sql"
	"strconv"
	"strings"
	"time"
)

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

// NullStr converts a sql.NullString to a JSON-safe value (string or nil).
func NullStr(ns sql.NullString) any {
	if ns.Valid {
		return ns.String
	}
	return nil
}

// NullInt64 converts a sql.NullInt64 to a JSON-safe value (int64 or nil).
func NullInt64(ni sql.NullInt64) any {
	if ni.Valid {
		return ni.Int64
	}
	return nil
}

// NullTime converts a sql.NullTime to a JSON-safe value (RFC3339 string or nil).
func NullTime(nt sql.NullTime) any {
	if nt.Valid {
		return nt.Time.Format(time.RFC3339)
	}
	return nil
}

// ToNullStr converts a string to sql.NullString (empty string → invalid).
func ToNullStr(s string) sql.NullString {
	if s == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: s, Valid: true}
}

// ToNullInt64 converts an int64 to sql.NullInt64 (zero → invalid).
func ToNullInt64(n int64) sql.NullInt64 {
	if n == 0 {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: n, Valid: true}
}

// NullStrPtr converts a sql.NullString to *string (nil if not valid).
func NullStrPtr(s sql.NullString) *string {
	if s.Valid {
		return &s.String
	}
	return nil
}

// NullInt64Ptr converts a sql.NullInt64 to *int64 (nil if not valid).
func NullInt64Ptr(n sql.NullInt64) *int64 {
	if n.Valid {
		return &n.Int64
	}
	return nil
}

// NullTimeStr converts a sql.NullTime to *string (RFC3339) or nil.
func NullTimeStr(t sql.NullTime) *string {
	if t.Valid {
		s := t.Time.Format(time.RFC3339)
		return &s
	}
	return nil
}
