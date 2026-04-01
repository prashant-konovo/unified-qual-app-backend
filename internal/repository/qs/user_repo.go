package qs

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"strings"
	"time"
)

// User maps to the QS `user` table (11 columns verified).
type User struct {
	ID                      int64          `json:"id"`
	FirstName               sql.NullString `json:"firstName"`
	LastName                sql.NullString `json:"lastName"`
	Email                   sql.NullString `json:"email"`
	Deleted                 int            `json:"deleted"`
	CognitoUserID           sql.NullInt64  `json:"cognitoUserId"`
	TermsAccepted           int            `json:"termsAccepted"`
	ModifiedOn              time.Time      `json:"modifiedOn"`
	ModeratorBuffer         sql.NullInt64  `json:"moderatorBuffer"`
	ModeratorBufferModified sql.NullTime   `json:"moderatorBufferModifiedOn"`
	TimeZone                sql.NullString `json:"timeZone"`
}

// UserWithRoles is a User enriched with their role IDs (1=Moderator, 2=Manager, 3=Admin).
type UserWithRoles struct {
	User
	RoleIDs []int `json:"roleIds"`
}

// UserListRow is a flattened row for list queries with aggregated roles.
type UserListRow struct {
	ID            int64          `json:"id"`
	FirstName     sql.NullString `json:"firstName"`
	LastName      sql.NullString `json:"lastName"`
	Email         sql.NullString `json:"email"`
	Deleted       int            `json:"deleted"`
	TimeZone      sql.NullString `json:"timeZone"`
	ModifiedOn    time.Time      `json:"modifiedOn"`
	RoleIDs       string         `json:"roleIds"` // comma-separated from GROUP_CONCAT
}

// ModeratorAvailability maps to the QS `moderator_availability` table (6 columns verified).
type ModeratorAvailability struct {
	ID          int64     `json:"id"`
	ModeratorID int64     `json:"moderatorId"`
	ClientID    int64     `json:"clientId"`
	StartTime   time.Time `json:"startTime"`
	EndTime     time.Time `json:"endTime"`
	ModifiedOn  time.Time `json:"modifiedOn"`
}

// UserRepo provides CRUD for the QS user and user_role tables.
type UserRepo struct {
	db *sql.DB
}

func NewUserRepo(db *sql.DB) *UserRepo {
	return &UserRepo{db: db}
}

const qsUserListQuery = `
SELECT u.id, u.first_name, u.last_name, u.email, u.deleted, u.time_zone, u.modified_on,
       COALESCE(GROUP_CONCAT(ur.role_id ORDER BY ur.role_id), '') AS role_ids
FROM user u
LEFT JOIN user_role ur ON ur.user_id = u.id
WHERE u.deleted = 0
`

// List returns QS users with pagination and optional filters.
func (r *UserRepo) List(ctx context.Context, page, pageSize int, roleID *int, search string) ([]UserListRow, int, error) {
	where := ""
	args := []any{}

	if search != "" {
		where += " AND (u.first_name LIKE ? OR u.last_name LIKE ? OR u.email LIKE ?)"
		s := "%" + search + "%"
		args = append(args, s, s, s)
	}

	// Count
	countQ := "SELECT COUNT(DISTINCT u.id) FROM user u LEFT JOIN user_role ur ON ur.user_id = u.id WHERE u.deleted = 0" + where
	countArgs := make([]any, len(args))
	copy(countArgs, args)

	if roleID != nil {
		countQ += " AND ur.role_id = ?"
		countArgs = append(countArgs, *roleID)
	}

	var total int
	if err := r.db.QueryRowContext(ctx, countQ, countArgs...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count qs users: %w", err)
	}

	// Build list query
	listWhere := where
	if roleID != nil {
		listWhere += " AND ur.role_id = ?"
		args = append(args, *roleID)
	}

	offset := (page - 1) * pageSize
	listQ := qsUserListQuery + listWhere + " GROUP BY u.id ORDER BY u.modified_on DESC LIMIT ? OFFSET ?"
	args = append(args, pageSize, offset)

	rows, err := r.db.QueryContext(ctx, listQ, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list qs users: %w", err)
	}
	defer rows.Close()

	var users []UserListRow
	for rows.Next() {
		var u UserListRow
		if err := rows.Scan(&u.ID, &u.FirstName, &u.LastName, &u.Email, &u.Deleted, &u.TimeZone, &u.ModifiedOn, &u.RoleIDs); err != nil {
			return nil, 0, fmt.Errorf("scan qs user row: %w", err)
		}
		users = append(users, u)
	}
	return users, total, rows.Err()
}

// GetByID returns a single QS user with their roles.
func (r *UserRepo) GetByID(ctx context.Context, id int64) (*UserWithRoles, error) {
	q := `SELECT u.id, u.first_name, u.last_name, u.email, u.deleted, u.cognito_user_id,
	             u.terms_accepted, u.modified_on, u.moderator_buffer, u.moderator_buffer_modified_on, u.time_zone
	      FROM user u WHERE u.id = ?`
	var u UserWithRoles
	err := r.db.QueryRowContext(ctx, q, id).Scan(
		&u.ID, &u.FirstName, &u.LastName, &u.Email, &u.Deleted, &u.CognitoUserID,
		&u.TermsAccepted, &u.ModifiedOn, &u.ModeratorBuffer, &u.ModeratorBufferModified, &u.TimeZone,
	)
	if err != nil {
		return nil, fmt.Errorf("get qs user %d: %w", id, err)
	}

	// Fetch roles
	roleRows, err := r.db.QueryContext(ctx, "SELECT role_id FROM user_role WHERE user_id = ?", id)
	if err != nil {
		return nil, fmt.Errorf("get qs user roles %d: %w", id, err)
	}
	defer roleRows.Close()
	for roleRows.Next() {
		var rid int
		if err := roleRows.Scan(&rid); err != nil {
			return nil, fmt.Errorf("scan qs role: %w", err)
		}
		u.RoleIDs = append(u.RoleIDs, rid)
	}
	return &u, roleRows.Err()
}

// GetByEmail returns a QS user by email.
func (r *UserRepo) GetByEmail(ctx context.Context, email string) (*UserWithRoles, error) {
	q := `SELECT id FROM user WHERE email = ? AND deleted = 0 LIMIT 1`
	var id int64
	if err := r.db.QueryRowContext(ctx, q, email).Scan(&id); err != nil {
		return nil, fmt.Errorf("get qs user by email %s: %w", email, err)
	}
	return r.GetByID(ctx, id)
}

// GetModerators returns all users with role_id=1 (moderator).
func (r *UserRepo) GetModerators(ctx context.Context) ([]UserListRow, error) {
	q := `SELECT u.id, u.first_name, u.last_name, u.email, u.deleted, u.time_zone, u.modified_on,
	             COALESCE(GROUP_CONCAT(ur2.role_id ORDER BY ur2.role_id), '') AS role_ids
	      FROM user u
	      JOIN user_role ur ON ur.user_id = u.id AND ur.role_id = 1
	      LEFT JOIN user_role ur2 ON ur2.user_id = u.id
	      WHERE u.deleted = 0
	      GROUP BY u.id
	      ORDER BY u.first_name, u.last_name`
	rows, err := r.db.QueryContext(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("get qs moderators: %w", err)
	}
	defer rows.Close()

	var mods []UserListRow
	for rows.Next() {
		var u UserListRow
		if err := rows.Scan(&u.ID, &u.FirstName, &u.LastName, &u.Email, &u.Deleted, &u.TimeZone, &u.ModifiedOn, &u.RoleIDs); err != nil {
			return nil, fmt.Errorf("scan qs moderator row: %w", err)
		}
		mods = append(mods, u)
	}
	return mods, rows.Err()
}

// ListModeratorAvailability returns availability slots for a moderator.
func (r *UserRepo) ListModeratorAvailability(ctx context.Context, moderatorID int64, clientID *int64, startDate, endDate string) ([]ModeratorAvailability, error) {
	q := `SELECT id, moderator_id, client_id, start_time, end_time, modified_on
	      FROM moderator_availability WHERE moderator_id = ?`
	args := []any{moderatorID}

	if clientID != nil {
		q += " AND client_id = ?"
		args = append(args, *clientID)
	}
	if startDate != "" {
		q += " AND start_time >= ?"
		args = append(args, startDate)
	}
	if endDate != "" {
		q += " AND end_time <= ?"
		args = append(args, endDate+" 23:59:59")
	}
	q += " ORDER BY start_time"

	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("list qs availability: %w", err)
	}
	defer rows.Close()

	var avails []ModeratorAvailability
	for rows.Next() {
		var a ModeratorAvailability
		if err := rows.Scan(&a.ID, &a.ModeratorID, &a.ClientID, &a.StartTime, &a.EndTime, &a.ModifiedOn); err != nil {
			return nil, fmt.Errorf("scan qs availability row: %w", err)
		}
		avails = append(avails, a)
	}
	return avails, rows.Err()
}

// CreateModeratorAvailability inserts a new availability slot.
func (r *UserRepo) CreateModeratorAvailability(ctx context.Context, moderatorID, clientID int64, startTime, endTime time.Time) (*ModeratorAvailability, error) {
	q := `INSERT INTO moderator_availability (moderator_id, client_id, start_time, end_time) VALUES (?, ?, ?, ?)`
	res, err := r.db.ExecContext(ctx, q, moderatorID, clientID, startTime, endTime)
	if err != nil {
		return nil, fmt.Errorf("create qs availability: %w", err)
	}
	id, _ := res.LastInsertId()
	slog.InfoContext(ctx, "created QS moderator availability", "id", id, "moderatorId", moderatorID)
	return &ModeratorAvailability{
		ID:          id,
		ModeratorID: moderatorID,
		ClientID:    clientID,
		StartTime:   startTime,
		EndTime:     endTime,
		ModifiedOn:  time.Now(),
	}, nil
}

// DeleteModeratorAvailability removes an availability slot.
func (r *UserRepo) DeleteModeratorAvailability(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, "DELETE FROM moderator_availability WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("delete qs availability %d: %w", id, err)
	}
	slog.InfoContext(ctx, "deleted QS moderator availability", "id", id)
	return nil
}

// Update updates a QS user's basic fields.
func (r *UserRepo) Update(ctx context.Context, id int64, firstName, lastName, timeZone string) error {
	sets := []string{}
	args := []any{}
	if firstName != "" {
		sets = append(sets, "first_name = ?")
		args = append(args, firstName)
	}
	if lastName != "" {
		sets = append(sets, "last_name = ?")
		args = append(args, lastName)
	}
	if timeZone != "" {
		sets = append(sets, "time_zone = ?")
		args = append(args, timeZone)
	}
	if len(sets) == 0 {
		return nil
	}
	args = append(args, id)
	q := "UPDATE user SET " + strings.Join(sets, ", ") + " WHERE id = ?"
	_, err := r.db.ExecContext(ctx, q, args...)
	if err != nil {
		return fmt.Errorf("update qs user %d: %w", id, err)
	}
	slog.InfoContext(ctx, "updated QS user", "id", id)
	return nil
}

// UpdateModeratorBuffer updates a moderator's buffer setting.
func (r *UserRepo) UpdateModeratorBuffer(ctx context.Context, id int64, buffer int) error {
	q := "UPDATE user SET moderator_buffer = ?, moderator_buffer_modified_on = NOW() WHERE id = ?"
	_, err := r.db.ExecContext(ctx, q, buffer, id)
	if err != nil {
		return fmt.Errorf("update qs moderator buffer %d: %w", id, err)
	}
	slog.InfoContext(ctx, "updated QS moderator buffer", "id", id, "buffer", buffer)
	return nil
}

// Create inserts a new user and assigns the given role IDs.
func (r *UserRepo) Create(ctx context.Context, firstName, lastName, email, timeZone string, roleIDs []int) (int64, error) {
	q := `INSERT INTO user (first_name, last_name, email, deleted, terms_accepted, time_zone) VALUES (?, ?, ?, 0, 0, ?)`
	res, err := r.db.ExecContext(ctx, q, firstName, lastName, email, timeZone)
	if err != nil {
		return 0, fmt.Errorf("create qs user: %w", err)
	}
	uid, _ := res.LastInsertId()
	for _, rid := range roleIDs {
		_, err := r.db.ExecContext(ctx, "INSERT INTO user_role (user_id, role_id) VALUES (?, ?)", uid, rid)
		if err != nil {
			slog.WarnContext(ctx, "failed to assign role", "userId", uid, "roleId", rid, "error", err)
		}
	}
	slog.InfoContext(ctx, "created QS user", "id", uid, "email", email, "roles", roleIDs)
	return uid, nil
}

// SoftDelete marks a user as deleted.
func (r *UserRepo) SoftDelete(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, "UPDATE user SET deleted = 1 WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("soft-delete qs user %d: %w", id, err)
	}
	slog.InfoContext(ctx, "soft-deleted QS user", "id", id)
	return nil
}

// AddRoles adds role IDs to a user (skips duplicates).
func (r *UserRepo) AddRoles(ctx context.Context, userID int64, roleIDs []int) error {
	for _, rid := range roleIDs {
		_, err := r.db.ExecContext(ctx, "INSERT IGNORE INTO user_role (user_id, role_id) VALUES (?, ?)", userID, rid)
		if err != nil {
			return fmt.Errorf("add role %d to user %d: %w", rid, userID, err)
		}
	}
	slog.InfoContext(ctx, "added roles to QS user", "userId", userID, "roles", roleIDs)
	return nil
}

// DeleteRoles removes role IDs from a user.
func (r *UserRepo) DeleteRoles(ctx context.Context, userID int64, roleIDs []int) error {
	for _, rid := range roleIDs {
		_, err := r.db.ExecContext(ctx, "DELETE FROM user_role WHERE user_id = ? AND role_id = ?", userID, rid)
		if err != nil {
			return fmt.Errorf("delete role %d from user %d: %w", rid, userID, err)
		}
	}
	slog.InfoContext(ctx, "deleted roles from QS user", "userId", userID, "roles", roleIDs)
	return nil
}

// GetRoles returns role IDs for a user.
func (r *UserRepo) GetRoles(ctx context.Context, userID int64) ([]int, error) {
	rows, err := r.db.QueryContext(ctx, "SELECT role_id FROM user_role WHERE user_id = ?", userID)
	if err != nil {
		return nil, fmt.Errorf("get roles for user %d: %w", userID, err)
	}
	defer rows.Close()
	var roles []int
	for rows.Next() {
		var rid int
		if err := rows.Scan(&rid); err != nil {
			return nil, err
		}
		roles = append(roles, rid)
	}
	return roles, rows.Err()
}

// GetUserCommPreference returns communication preference for a user.
func (r *UserRepo) GetUserCommPreference(ctx context.Context, userID int64) (map[string]any, error) {
	q := `SELECT u.id, u.email, COALESCE(u.terms_accepted, 0) as opted_in
	      FROM user u WHERE u.id = ? AND u.deleted = 0`
	var id int64
	var email sql.NullString
	var optedIn int
	err := r.db.QueryRowContext(ctx, q, userID).Scan(&id, &email, &optedIn)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get comm pref for user %d: %w", userID, err)
	}
	return map[string]any{
		"userId": id, "email": email.String,
		"optedIn": optedIn == 1, "canUnsubscribe": true,
	}, nil
}

// SetUnsubscribed marks a user as unsubscribed (terms_accepted = 0).
func (r *UserRepo) SetUnsubscribed(ctx context.Context, userID int64) error {
	_, err := r.db.ExecContext(ctx, "UPDATE user SET terms_accepted = 0 WHERE id = ?", userID)
	if err != nil {
		return fmt.Errorf("unsubscribe user %d: %w", userID, err)
	}
	slog.InfoContext(ctx, "unsubscribed QS user", "id", userID)
	return nil
}

// AcceptTerms sets terms_accepted = 1 for a QS user.
func (r *UserRepo) AcceptTerms(ctx context.Context, userID int64) error {
	_, err := r.db.ExecContext(ctx, "UPDATE user SET terms_accepted = 1 WHERE id = ?", userID)
	if err != nil {
		return fmt.Errorf("accept terms user %d: %w", userID, err)
	}
	slog.InfoContext(ctx, "accepted terms for QS user", "id", userID)
	return nil
}

// ListByRole returns QS users filtered by role.
func (r *UserRepo) ListByRole(ctx context.Context, roleID int, page, pageSize int) ([]UserListRow, int, error) {
	return r.List(ctx, page, pageSize, &roleID, "")
}

// UpdateAvailability updates a moderator availability slot.
func (r *UserRepo) UpdateModeratorAvailability(ctx context.Context, id int64, startTime, endTime time.Time) error {
	_, err := r.db.ExecContext(ctx,
		"UPDATE moderator_availability SET start_time = ?, end_time = ?, modified_on = NOW() WHERE id = ?",
		startTime, endTime, id)
	if err != nil {
		return fmt.Errorf("update qs moderator availability %d: %w", id, err)
	}
	return nil
}

// GetByEmailIncludeDeleted returns a QS user by email regardless of deleted status.
func (r *UserRepo) GetByEmailIncludeDeleted(ctx context.Context, email string) (*UserWithRoles, error) {
	q := `SELECT id, first_name, last_name, email, deleted, cognito_user_id, terms_accepted,
	             modified_on, moderator_buffer, moderator_buffer_modified_on, time_zone
	      FROM user WHERE email = ? LIMIT 1`
	var u User
	err := r.db.QueryRowContext(ctx, q, email).Scan(
		&u.ID, &u.FirstName, &u.LastName, &u.Email, &u.Deleted,
		&u.CognitoUserID, &u.TermsAccepted, &u.ModifiedOn,
		&u.ModeratorBuffer, &u.ModeratorBufferModified, &u.TimeZone,
	)
	if err != nil {
		return nil, fmt.Errorf("get qs user by email (include deleted) %s: %w", email, err)
	}
	roles, _ := r.GetRoles(ctx, u.ID)
	return &UserWithRoles{User: u, RoleIDs: roles}, nil
}

// RestoreByEmail sets deleted = 0 for a user found by email.
func (r *UserRepo) RestoreByEmail(ctx context.Context, email string) error {
	_, err := r.db.ExecContext(ctx, "UPDATE user SET deleted = 0 WHERE email = ?", email)
	if err != nil {
		return fmt.Errorf("restore qs user by email %s: %w", email, err)
	}
	slog.InfoContext(ctx, "restored QS user", "email", email)
	return nil
}

// AddUserClient inserts a user_client association.
func (r *UserRepo) AddUserClient(ctx context.Context, userID, clientID int64) error {
	_, err := r.db.ExecContext(ctx, "INSERT IGNORE INTO user_client (user_id, client_id) VALUES (?, ?)", userID, clientID)
	if err != nil {
		return fmt.Errorf("add client %d to user %d: %w", clientID, userID, err)
	}
	return nil
}

// CreateUserCommPrefs inserts initial communication preferences for a user.
func (r *UserRepo) CreateUserCommPrefs(ctx context.Context, userID int64, email, cognitoUserID string) error {
	_, err := r.db.ExecContext(ctx,
		"INSERT IGNORE INTO user_communication_preferences (user_id, email, cognito_user_id) VALUES (?, ?, ?)",
		userID, email, cognitoUserID)
	if err != nil {
		return fmt.Errorf("create comm prefs for user %d: %w", userID, err)
	}
	return nil
}

// CheckUserIsQsToolAndI2 checks if a user exists in QS and has I2 (IRIS) cross-reference.
func (r *UserRepo) CheckUserIsQsToolAndI2(ctx context.Context, email string) (map[string]any, error) {
	u, err := r.GetByEmail(ctx, email)
	if err != nil {
		return nil, err
	}
	if u == nil {
		return map[string]any{"isQsTool": false, "isI2": false, "exists": false}, nil
	}
	return map[string]any{
		"isQsTool": true,
		"isI2":     u.CognitoUserID.Valid,
		"exists":   true,
		"userId":   u.ID,
	}, nil
}
