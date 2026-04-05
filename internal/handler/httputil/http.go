package httputil

import (
"encoding/json"
"net/http"
"strconv"
"strings"
"time"

"github.com/google/uuid"
)

// WriteJSON encodes v as JSON and writes it to w with the given status code.
func WriteJSON(w http.ResponseWriter, status int, v any) {
w.Header().Set("Content-Type", "application/json")
w.Header().Set("X-Brand", "unified")
w.WriteHeader(status)
_ = json.NewEncoder(w).Encode(v)
}

// ID generates a new UUID string.
func ID() string { return uuid.New().String() }

// Now returns the current UTC time formatted as RFC3339.
func Now() string { return time.Now().UTC().Format(time.RFC3339) }

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

// ExtractBearerToken extracts the JWT bearer token from request headers.
func ExtractBearerToken(r *http.Request) string {
auth := r.Header.Get("Authorization")
if strings.HasPrefix(auth, "Bearer ") {
return auth[7:]
}
if t := r.Header.Get("IC-Auth"); t != "" {
return t
}
if t := r.Header.Get("CognitoToken"); t != "" {
return t
}
return ""
}

// ResolveSource extracts the data source ("iris" or "qs") from query params.
func ResolveSource(r *http.Request) string {
if s := r.URL.Query().Get("source"); s != "" {
return strings.ToLower(s)
}
if sc := strings.ToUpper(r.URL.Query().Get("serviceCategory")); sc != "" {
switch sc {
case "LS":
return "iris"
case "MRA":
return "qs"
}
}
return ""
}
