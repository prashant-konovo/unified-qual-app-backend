package dto

import (
"fmt"
"strconv"
"strings"
)

// QsRoleName maps QS role_id to a human-readable name.
func QsRoleName(id int) string {
switch id {
case 1:
return "moderator"
case 2:
return "manager"
case 3:
return "admin"
default:
return fmt.Sprintf("role_%d", id)
}
}

// ParseRoleCSV splits a comma-separated role-id string from GROUP_CONCAT.
func ParseRoleCSV(csv string) []int {
if csv == "" {
return nil
}
parts := strings.Split(csv, ",")
ids := make([]int, 0, len(parts))
for _, p := range parts {
v, err := strconv.Atoi(strings.TrimSpace(p))
if err == nil {
ids = append(ids, v)
}
}
return ids
}
