package core

import (
	"database/sql"
	"time"
)

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
