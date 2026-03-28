package middleware

// Unified role constants matching the LLD spec.
// Each maps to one or more Cognito groups from the legacy systems.
const (
	RoleAdmin     = "admin"     // QUAL_SCHEDULER_ADMIN, ADMIN, AdminUsers, SHG_ADMIN
	RoleManager   = "manager"   // QUAL_SCHEDULER_MANAGER, SUBSCRIPTION_OWNER, SUBSCRIPTION_ADMIN
	RoleModerator = "moderator" // QUAL_SCHEDULER_MODERATOR, CLIENT_MODERATOR
	RoleObserver  = "observer"  // SUBSCRIPTION_USER (read-only project view)
	RoleExternal  = "external"  // External panelist / client
)

// cognitoGroupToRole maps Cognito group names to unified roles.
// A group can map to exactly one unified role. The first match wins when
// a user belongs to multiple groups (the highest-privilege role is listed first).
var cognitoGroupToRole = map[string]string{
	// Admin-level groups
	"QUAL_SCHEDULER_ADMIN": RoleAdmin,
	"ADMIN":                RoleAdmin,
	"AdminUsers":           RoleAdmin,
	"SHG_ADMIN":            RoleAdmin,
	"PANEL_ADMIN":          RoleAdmin,

	// Manager-level groups
	"QUAL_SCHEDULER_MANAGER": RoleManager,
	"SUBSCRIPTION_OWNER":     RoleManager,
	"SUBSCRIPTION_ADMIN":     RoleManager,

	// Moderator-level groups
	"QUAL_SCHEDULER_MODERATOR": RoleModerator,
	"CLIENT_MODERATOR":         RoleModerator,

	// Observer-level groups
	"SUBSCRIPTION_USER": RoleObserver,

	// External
	"EXTERNAL_PANELIST_PROVIDER": RoleExternal,
	"RESPONDER":                  RoleExternal,
	"Responder":                  RoleExternal,
}

// rolePriority defines the precedence order. Lower number = higher privilege.
var rolePriority = map[string]int{
	RoleAdmin:     0,
	RoleManager:   1,
	RoleModerator: 2,
	RoleObserver:  3,
	RoleExternal:  4,
}

// MapCognitoGroupsToRoles converts Cognito groups from the JWT into
// a deduplicated set of unified roles, ordered by priority (highest first).
func MapCognitoGroupsToRoles(groups []string) []string {
	seen := make(map[string]bool)
	var roles []string
	for _, g := range groups {
		if r, ok := cognitoGroupToRole[g]; ok && !seen[r] {
			seen[r] = true
			roles = append(roles, r)
		}
	}
	// Sort by priority
	for i := 0; i < len(roles); i++ {
		for j := i + 1; j < len(roles); j++ {
			if rolePriority[roles[j]] < rolePriority[roles[i]] {
				roles[i], roles[j] = roles[j], roles[i]
			}
		}
	}
	return roles
}

// HasRole checks whether the user holds a specific unified role.
func (u *UserClaims) HasRole(role string) bool {
	for _, r := range u.Roles {
		if r == role {
			return true
		}
	}
	return false
}

// HasAnyRole checks whether the user holds any of the specified unified roles.
func (u *UserClaims) HasAnyRole(roles ...string) bool {
	for _, role := range roles {
		if u.HasRole(role) {
			return true
		}
	}
	return false
}

// IsAdmin is a convenience check for admin role.
func (u *UserClaims) IsAdmin() bool { return u.HasRole(RoleAdmin) }

// IsManager is a convenience check for manager role.
func (u *UserClaims) IsManager() bool { return u.HasRole(RoleManager) }

// IsModerator is a convenience check for moderator role.
func (u *UserClaims) IsModerator() bool { return u.HasRole(RoleModerator) }
