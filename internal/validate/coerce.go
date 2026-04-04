package validate

import (
	"strconv"
	"strings"
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
