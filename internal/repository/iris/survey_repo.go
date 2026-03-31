package iris

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// ICSurvey maps core columns from the IRIS survey table.
type ICSurvey struct {
	ID                 int64          `json:"id"`
	SubscriptionID     sql.NullInt64  `json:"subscriptionId"`
	SurveyTypeID       int            `json:"surveyTypeId"`
	NamePublic         string         `json:"namePublic"`
	NamePrivate        sql.NullString `json:"namePrivate"`
	TopicName          sql.NullString `json:"topicName"`
	ProjectID          int64          `json:"projectId"`
	Status             int            `json:"status"`
	CompletionsNeeded  int            `json:"completionsNeeded"`
	CreatedBy          int64          `json:"createdBy"`
	CreatedOn          time.Time      `json:"createdOn"`
	ModifiedOn         sql.NullTime   `json:"modifiedOn"`
	FieldedOn          sql.NullTime   `json:"fieldedOn"`
	ClosedOn           sql.NullTime   `json:"closedOn"`
	IsArchived         bool           `json:"isArchived"`
	LanguageID         int            `json:"languageId"`
	SalesforceProjectID sql.NullString `json:"salesforceProjectId"`
	LengthOfInterview  sql.NullInt64  `json:"lengthOfInterview"`
}

// ICSurveyCrowd maps the survey_crowd join table.
type ICSurveyCrowd struct {
	ID             int64  `json:"id"`
	SurveyID       int64  `json:"surveyId"`
	CrowdID        int64  `json:"crowdId"`
	AnswerRequest  int    `json:"answerRequest"`
	QualHonorarium sql.NullInt64 `json:"qualHonorarium"`
	Excluded       bool   `json:"excluded"`
}

// ICCrowd maps core columns from the IRIS crowd table.
// ICCrowd maps core columns from the IRIS crowd table.
type ICCrowd struct {
	ID                          int64          `json:"id"`
	Name                        string         `json:"name"`
	Description                 sql.NullString `json:"description"`
	SubscriptionID              int64          `json:"subscriptionId"`
	CreatedBy                   int64          `json:"createdBy"`
	TypeID                      int            `json:"typeId"`
	MarketID                    int64          `json:"marketId"`
	Deleted                     bool           `json:"deleted"`
	AndOr                       sql.NullInt64  `json:"andOr"`
	DeletedOn                   sql.NullTime   `json:"deletedOn"`
	DeletedBy                   sql.NullInt64  `json:"deletedBy"`
	CreatedOn                   time.Time      `json:"createdOn"`
	IsArchived                  bool           `json:"isArchived"`
	ModifiedOn                  time.Time      `json:"modifiedOn"`
	CreatedFromSampleTemplateID sql.NullInt64  `json:"createdFromSampleTemplateId"`
	CreatedFromExclusionList    bool           `json:"createdFromExclusionList"`
	IsNewbie                    bool           `json:"isNewbie"`
	IncrowdTPA                  sql.NullString `json:"incrowdTPA"`
	DoximityTPA                 sql.NullString `json:"doximityTPA"`
	CanShareWithDoximity        bool           `json:"canShareWithDoximity"`
	DuplicatedFromS3Key         sql.NullString `json:"duplicatedFromS3Key"`
}

// ICMarket maps the IRIS market table.
type ICMarket struct {
	ID             int64  `json:"id"`
	Name           string `json:"name"`
	CanRegister    bool   `json:"canRegister"`
	CanInterview   bool   `json:"canInterview"`
	IsActive       bool   `json:"isActive"`
}

// ICObserver maps the IRIS observer table.
type ICObserver struct {
	ID         int64          `json:"id"`
	ProjectID  int64          `json:"projectId"`
	Email      string         `json:"email"`
	TimeSlotID sql.NullInt64  `json:"timeSlotId"`
}

// ICInterviewMedia maps the IRIS interview_media table.
type ICInterviewMedia struct {
	ID             int64          `json:"id"`
	Name           string         `json:"name"`
	Description    string         `json:"description"`
	ProjectID      int64          `json:"projectId"`
	S3Key          sql.NullString `json:"s3Key"`
	Hash           sql.NullString `json:"hash"`
	Status         string         `json:"status"`
	CreatedOn      time.Time      `json:"createdOn"`
	CreatedBy      int64          `json:"createdBy"`
	PageCount      int            `json:"pageCount"`
	PagesProcessed int            `json:"pagesProcessed"`
	Shared         bool           `json:"shared"`
}

// ICModeratorAvailability maps IRIS moderator_availability.
type ICModeratorAvailability struct {
	ID             int64     `json:"id"`
	ModeratorID    int64     `json:"moderatorId"`
	SubscriptionID int64     `json:"subscriptionId"`
	StartTime      time.Time `json:"startTime"`
	EndTime        time.Time `json:"endTime"`
}

// ICModeratorTimeSlot maps IRIS moderator_time_slot junction.
type ICModeratorTimeSlot struct {
	ID          int64 `json:"id"`
	ModeratorID int64 `json:"moderatorId"`
	TimeSlotID  int64 `json:"timeSlotId"`
	IsHost      bool  `json:"isHost"`
}

// ICUserProject maps IRIS user_project.
type ICUserProject struct {
	ID        int64 `json:"id"`
	UserID    int64 `json:"userId"`
	ProjectID int64 `json:"projectId"`
	CanWrite  bool  `json:"canWrite"`
	CanRead   bool  `json:"canRead"`
}

// ICSalesforceProject maps the IRIS salesforce_project table.
type ICSalesforceProject struct {
	ID                      int64          `json:"id"`
	SalesforceProjectID     string         `json:"salesforceProjectId"`
	Name                    string         `json:"name"`
	Number                  sql.NullString `json:"number"`
	SalesforceAccountID     sql.NullString `json:"salesforceAccountId"`
	SubscriptionID          sql.NullInt64  `json:"subscriptionId"`
	IsProjectPricing        bool           `json:"isProjectPricing"`
	IsDeleted               bool           `json:"isDeleted"`
	LastModifiedDate        time.Time      `json:"lastModifiedDate"`
	ClientProjectName       sql.NullString `json:"clientProjectName"`
	ClientProjectNumber     sql.NullString `json:"clientProjectNumber"`
	BrandTypeID             int64          `json:"brandTypeId"`
	SalesforceProjectType   int64          `json:"salesforceProjectType"`
	SalesforceProjectStatus sql.NullString `json:"salesforceProjectStatus"`
	OwnerName               sql.NullString `json:"ownerName"`
	ProjectManagerName      sql.NullString `json:"projectManagerName"`
	ProjectReconciled       sql.NullString `json:"projectReconciled"`
}

// ICProjectInquiry maps the IRIS project_inquiry table.
type ICProjectInquiry struct {
	ID                     int64          `json:"id"`
	Description            string         `json:"description"`
	Notes                  sql.NullString `json:"notes"`
	SubscriptionID         int64          `json:"subscriptionId"`
	ProjectID              int64          `json:"projectId"`
	InquiryTypeID          int            `json:"inquiryTypeId"`
	InterviewLength        int            `json:"interviewLength"`
	RequiredCompletionDate time.Time      `json:"requiredCompletionDate"`
	CreatedOn              time.Time      `json:"createdOn"`
	CreatedBy              int64          `json:"createdBy"`
	UnderReview            bool           `json:"underReview"`
	TranscriptsRequested   bool           `json:"transcriptsRequested"`
	RequiresStimuli        bool           `json:"requiresStimuli"`
}

// SurveyRepo handles IRIS survey/crowd/market/observer queries.
type SurveyRepo struct {
	db   *sql.DB
	dbRO *sql.DB
}

// NewSurveyRepo creates a new IRIS survey repository.
func NewSurveyRepo(db, dbRO *sql.DB) *SurveyRepo {
	return &SurveyRepo{db: db, dbRO: dbRO}
}

func (r *SurveyRepo) ro() *sql.DB {
	if r.dbRO != nil {
		return r.dbRO
	}
	return r.db
}

// ListSurveysForProject returns surveys for a given project.
func (r *SurveyRepo) ListSurveysForProject(ctx context.Context, projectID int64) ([]ICSurvey, error) {
	q := `SELECT id, subscription_id, survey_type_id, name_public, name_private, topic_name,
	       project_id, status, completions_needed, created_by, created_on, modified_on,
	       fielded_on, closed_on, is_archived, language_id, salesforce_project_id, length_of_interview
	      FROM survey WHERE project_id = ? AND is_archived = 0 ORDER BY created_on DESC`
	return r.scanSurveys(ctx, q, projectID)
}

// ListSurveysForSubscription returns surveys for a subscription.
func (r *SurveyRepo) ListSurveysForSubscription(ctx context.Context, subscriptionID int64) ([]ICSurvey, error) {
	q := `SELECT id, subscription_id, survey_type_id, name_public, name_private, topic_name,
	       project_id, status, completions_needed, created_by, created_on, modified_on,
	       fielded_on, closed_on, is_archived, language_id, salesforce_project_id, length_of_interview
	      FROM survey WHERE subscription_id = ? AND is_archived = 0 ORDER BY created_on DESC`
	return r.scanSurveys(ctx, q, subscriptionID)
}

// GetSurvey returns a single survey by ID.
func (r *SurveyRepo) GetSurvey(ctx context.Context, id int64) (*ICSurvey, error) {
	q := `SELECT id, subscription_id, survey_type_id, name_public, name_private, topic_name,
	       project_id, status, completions_needed, created_by, created_on, modified_on,
	       fielded_on, closed_on, is_archived, language_id, salesforce_project_id, length_of_interview
	      FROM survey WHERE id = ?`
	rows, err := r.scanSurveys(ctx, q, id)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}
	return &rows[0], nil
}

func (r *SurveyRepo) scanSurveys(ctx context.Context, q string, args ...any) ([]ICSurvey, error) {
	rows, err := r.ro().QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("query surveys: %w", err)
	}
	defer rows.Close()
	var result []ICSurvey
	for rows.Next() {
		var s ICSurvey
		if err := rows.Scan(&s.ID, &s.SubscriptionID, &s.SurveyTypeID, &s.NamePublic, &s.NamePrivate,
			&s.TopicName, &s.ProjectID, &s.Status, &s.CompletionsNeeded, &s.CreatedBy, &s.CreatedOn,
			&s.ModifiedOn, &s.FieldedOn, &s.ClosedOn, &s.IsArchived, &s.LanguageID,
			&s.SalesforceProjectID, &s.LengthOfInterview); err != nil {
			return nil, fmt.Errorf("scan survey: %w", err)
		}
		result = append(result, s)
	}
	return result, rows.Err()
}

// CloseSurvey sets survey status to closed (5) and sets closed_on.
func (r *SurveyRepo) CloseSurvey(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, "UPDATE survey SET status = 5, closed_on = NOW() WHERE id = ?", id)
	return err
}

// ToggleFavorite creates or removes a user_survey_favorite row.
func (r *SurveyRepo) ToggleFavorite(ctx context.Context, surveyID, userID int64, favorite bool) error {
	if favorite {
		_, err := r.db.ExecContext(ctx,
			"INSERT IGNORE INTO user_survey_favorite (user_id, survey_id) VALUES (?, ?)", userID, surveyID)
		return err
	}
	_, err := r.db.ExecContext(ctx,
		"DELETE FROM user_survey_favorite WHERE user_id = ? AND survey_id = ?", userID, surveyID)
	return err
}

// ValidateSurvey checks if a survey can be fielded — returns validation errors.
func (r *SurveyRepo) ValidateSurvey(ctx context.Context, id int64) ([]string, error) {
	s, err := r.GetSurvey(ctx, id)
	if err != nil {
		return nil, err
	}
	if s == nil {
		return []string{"survey not found"}, nil
	}
	var errors []string
	if s.NamePublic == "" {
		errors = append(errors, "survey name is required")
	}
	// Check if has crowds
	var crowdCount int
	_ = r.ro().QueryRowContext(ctx, "SELECT COUNT(*) FROM survey_crowd WHERE survey_id = ? AND excluded = 0", id).Scan(&crowdCount)
	if crowdCount == 0 {
		errors = append(errors, "survey must have at least one crowd assigned")
	}
	return errors, nil
}

// GetSurveyCrowds returns crowds assigned to a survey.
func (r *SurveyRepo) GetSurveyCrowds(ctx context.Context, surveyID int64) ([]map[string]any, error) {
	q := `SELECT sc.id, sc.survey_id, sc.crowd_id, sc.answer_request, sc.qual_honorarium, sc.excluded,
	       c.name, c.subscription_id, c.type_id, c.market_id
	      FROM survey_crowd sc
	      JOIN crowd c ON c.id = sc.crowd_id
	      WHERE sc.survey_id = ? ORDER BY c.name`
	rows, err := r.ro().QueryContext(ctx, q, surveyID)
	if err != nil {
		return nil, fmt.Errorf("get survey crowds: %w", err)
	}
	defer rows.Close()
	var result []map[string]any
	for rows.Next() {
		var scID, survID, crowdID int64
		var ansReq int
		var qualHono sql.NullInt64
		var excluded bool
		var cName string
		var cSubID int64
		var cTypeID, cMarketID int
		if err := rows.Scan(&scID, &survID, &crowdID, &ansReq, &qualHono, &excluded,
			&cName, &cSubID, &cTypeID, &cMarketID); err != nil {
			return nil, fmt.Errorf("scan survey crowd: %w", err)
		}
		m := map[string]any{
			"id": scID, "surveyId": survID, "crowdId": crowdID,
			"answerRequest": ansReq, "excluded": excluded,
			"crowdName": cName, "subscriptionId": cSubID,
			"crowdTypeId": cTypeID, "marketId": cMarketID,
		}
		if qualHono.Valid {
			m["qualHonorarium"] = qualHono.Int64
		}
		result = append(result, m)
	}
	return result, rows.Err()
}

// CrowdFilter holds optional filter/pagination params for listing crowds.
type CrowdFilter struct {
	IncludeExclusionLists bool
	Limit                 int
	Offset                int
}

// ListCrowdsForSubscription returns crowds for a subscription with pagination.
func (r *SurveyRepo) ListCrowdsForSubscription(ctx context.Context, subscriptionID int64, f *CrowdFilter) ([]ICCrowd, int, error) {
	baseCols := `id, name, description, subscription_id, created_by, type_id, market_id,
	       CAST(deleted AS UNSIGNED), and_or, deleted_on, deleted_by, created_on,
	       is_archived, modified_on, created_from_sample_template_id,
	       created_from_exclusion_list, is_newbie, incrowd_tpa, doximity_tpa,
	       can_share_with_doximity, duplicated_from_s3_key`

	where := "subscription_id = ? AND deleted = b'0' AND is_archived = 0 AND is_newbie = 0"
	args := []any{subscriptionID}

	if f != nil && !f.IncludeExclusionLists {
		where += " AND created_from_exclusion_list = 0"
	}

	// Count total
	var total int
	countQ := "SELECT COUNT(*) FROM crowd WHERE " + where
	if err := r.ro().QueryRowContext(ctx, countQ, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count crowds: %w", err)
	}

	// Apply pagination
	limit := 20
	offset := 0
	if f != nil && f.Limit > 0 {
		limit = f.Limit
	}
	if f != nil && f.Offset > 0 {
		offset = f.Offset
	}

	q := fmt.Sprintf("SELECT %s FROM crowd WHERE %s ORDER BY name LIMIT %d OFFSET %d",
		baseCols, where, limit, offset)
	rows, err := r.ro().QueryContext(ctx, q, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list crowds: %w", err)
	}
	defer rows.Close()
	var result []ICCrowd
	for rows.Next() {
		var c ICCrowd
		var deletedInt int
		if err := rows.Scan(&c.ID, &c.Name, &c.Description, &c.SubscriptionID, &c.CreatedBy,
			&c.TypeID, &c.MarketID, &deletedInt, &c.AndOr, &c.DeletedOn, &c.DeletedBy,
			&c.CreatedOn, &c.IsArchived, &c.ModifiedOn,
			&c.CreatedFromSampleTemplateID, &c.CreatedFromExclusionList,
			&c.IsNewbie, &c.IncrowdTPA, &c.DoximityTPA,
			&c.CanShareWithDoximity, &c.DuplicatedFromS3Key); err != nil {
			return nil, 0, fmt.Errorf("scan crowd: %w", err)
		}
		c.Deleted = deletedInt != 0
		result = append(result, c)
	}
	return result, total, rows.Err()
}

// GetCrowdTypeDescription returns the description for a crowd type ID.
func (r *SurveyRepo) GetCrowdTypeDescription(ctx context.Context, typeID int) (string, error) {
	var desc string
	err := r.ro().QueryRowContext(ctx,
		"SELECT description FROM crowd_type WHERE id = ?", typeID).Scan(&desc)
	return desc, err
}

// GetMarketName returns a market's name by ID.
func (r *SurveyRepo) GetMarketName(ctx context.Context, marketID int64) (string, error) {
	var name string
	err := r.ro().QueryRowContext(ctx,
		"SELECT name FROM market WHERE id = ?", marketID).Scan(&name)
	return name, err
}

// GetCrowdBrandIDs returns brand IDs for a crowd from the crowd_brand junction table.
func (r *SurveyRepo) GetCrowdBrandIDs(ctx context.Context, crowdID int64) ([]int64, error) {
	rows, err := r.ro().QueryContext(ctx,
		"SELECT brand_id FROM crowd_brand WHERE crowd_id = ?", crowdID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		result = append(result, id)
	}
	return result, rows.Err()
}

// GetBrandName returns a brand name by ID.
func (r *SurveyRepo) GetBrandName(ctx context.Context, brandID int64) (string, error) {
	var name string
	err := r.ro().QueryRowContext(ctx,
		"SELECT name FROM brand WHERE id = ?", brandID).Scan(&name)
	return name, err
}

// GetAccountIDForSubscription returns the account_id from the subscription table.
func (r *SurveyRepo) GetAccountIDForSubscription(ctx context.Context, subscriptionID int64) *int64 {
	var id int64
	err := r.ro().QueryRowContext(ctx,
		"SELECT account_id FROM subscription WHERE id = ?", subscriptionID).Scan(&id)
	if err != nil {
		return nil
	}
	return &id
}

// GetCrowdCountryID returns the country attribute choice ID for a crowd (attribute_id=29).
func (r *SurveyRepo) GetCrowdCountryID(ctx context.Context, crowdID int64) int64 {
	var id int64
	err := r.ro().QueryRowContext(ctx,
		`SELECT cac.attribute_choice_id FROM crowd_attribute_choice cac
		 JOIN crowd_attribute ca ON ca.id = cac.crowd_attribute_id
		 WHERE ca.crowd_id = ? AND ca.attribute_id = 29 LIMIT 1`, crowdID).Scan(&id)
	if err != nil {
		return 0
	}
	return id
}

// GetAttributeChoiceLabel returns the label for an attribute choice.
func (r *SurveyRepo) GetAttributeChoiceLabel(ctx context.Context, choiceID int64) string {
	var label string
	if err := r.ro().QueryRowContext(ctx,
		"SELECT label FROM attribute_choice WHERE id = ?", choiceID).Scan(&label); err != nil {
		return ""
	}
	return label
}

// GetCountryLanguages returns country-language labels for a country ID.
func (r *SurveyRepo) GetCountryLanguages(ctx context.Context, countryID int64, countryName string) []string {
	rows, err := r.ro().QueryContext(ctx,
		"SELECT label FROM country_language_association WHERE country_id = ?", countryID)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var result []string
	for rows.Next() {
		var label string
		if err := rows.Scan(&label); err != nil {
			continue
		}
		result = append(result, countryName+"-"+label)
	}
	return result
}

// CrowdHasListMatch checks if a crowd was created via list match.
func (r *SurveyRepo) CrowdHasListMatch(ctx context.Context, crowdID int64) bool {
	var cnt int
	if err := r.ro().QueryRowContext(ctx,
		"SELECT COUNT(*) FROM list_match_report WHERE crowd_id = ? LIMIT 1", crowdID).Scan(&cnt); err != nil {
		return false
	}
	return cnt > 0
}

// GetCrowdSpecialtyIDs returns specialty IDs for a crowd.
func (r *SurveyRepo) GetCrowdSpecialtyIDs(ctx context.Context, crowdID int64) []int {
	rows, err := r.ro().QueryContext(ctx,
		"SELECT specialty_id FROM crowd_specialties WHERE crowd_id = ?", crowdID)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var result []int
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			continue
		}
		result = append(result, id)
	}
	return result
}

// GetCrowdEngagementRate returns the engagement rate for a crowd.
func (r *SurveyRepo) GetCrowdEngagementRate(ctx context.Context, crowdID int64, fullMatch bool) *float64 {
	var rate float64
	fm := 0
	if fullMatch {
		fm = 1
	}
	err := r.ro().QueryRowContext(ctx,
		`SELECT engagement_rate FROM crowd_users_survey_engagement_rate
		 WHERE crowd_id = ? AND brand_id IS NULL AND rate_key = 'completes' AND full_match = ?`,
		crowdID, fm).Scan(&rate)
	if err != nil {
		return nil
	}
	return &rate
}

// ListMarkets returns all active markets.
func (r *SurveyRepo) ListMarkets(ctx context.Context) ([]ICMarket, error) {
	q := `SELECT id, name, can_register, can_interview, is_active FROM market WHERE is_active = 1 ORDER BY name`
	rows, err := r.ro().QueryContext(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("list markets: %w", err)
	}
	defer rows.Close()
	var result []ICMarket
	for rows.Next() {
		var m ICMarket
		if err := rows.Scan(&m.ID, &m.Name, &m.CanRegister, &m.CanInterview, &m.IsActive); err != nil {
			return nil, fmt.Errorf("scan market: %w", err)
		}
		result = append(result, m)
	}
	return result, rows.Err()
}

// ListMarketsWithNPI returns markets that have can_interview and NPI association.
func (r *SurveyRepo) ListMarketsWithNPI(ctx context.Context) ([]ICMarket, error) {
	q := `SELECT id, name, can_register, can_interview, is_active FROM market
	      WHERE is_active = 1 AND can_interview = 1 ORDER BY name`
	rows, err := r.ro().QueryContext(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("list npi markets: %w", err)
	}
	defer rows.Close()
	var result []ICMarket
	for rows.Next() {
		var m ICMarket
		if err := rows.Scan(&m.ID, &m.Name, &m.CanRegister, &m.CanInterview, &m.IsActive); err != nil {
			return nil, fmt.Errorf("scan market: %w", err)
		}
		result = append(result, m)
	}
	return result, rows.Err()
}

// GetCrowdableAttributes returns crowdable attributes for a market.
func (r *SurveyRepo) GetCrowdableAttributes(ctx context.Context, marketID int64) ([]map[string]any, error) {
	q := `SELECT ma.id, ma.market_id, a.id AS attr_id, a.name, a.label, a.input_type_id,
	             CAST(a.crowd_selector AS UNSIGNED) AS crowd_selector
	      FROM market_attribute ma
	      JOIN attribute a ON a.id = ma.attribute_id
	      WHERE ma.market_id = ? AND a.crowd_selector = b'1'
	      ORDER BY a.name`
	rows, err := r.ro().QueryContext(ctx, q, marketID)
	if err != nil {
		return nil, fmt.Errorf("get crowdable attrs: %w", err)
	}
	defer rows.Close()
	var result []map[string]any
	for rows.Next() {
		var maID, mktID, attrID int64
		var name, label string
		var inputTypeID int
		var crowdSelector int
		if err := rows.Scan(&maID, &mktID, &attrID, &name, &label, &inputTypeID, &crowdSelector); err != nil {
			return nil, fmt.Errorf("scan crowdable attr: %w", err)
		}
		result = append(result, map[string]any{
			"id": maID, "marketId": mktID, "attributeId": attrID,
			"name": name, "label": label, "inputTypeId": inputTypeID, "crowdSelector": crowdSelector == 1,
		})
	}
	return result, rows.Err()
}

// ListObserversForProject returns observers for a project.
func (r *SurveyRepo) ListObserversForProject(ctx context.Context, projectID int64) ([]ICObserver, error) {
	q := `SELECT id, project_id, email, time_slot_id FROM observer WHERE project_id = ?`
	rows, err := r.ro().QueryContext(ctx, q, projectID)
	if err != nil {
		return nil, fmt.Errorf("list observers: %w", err)
	}
	defer rows.Close()
	var result []ICObserver
	for rows.Next() {
		var o ICObserver
		if err := rows.Scan(&o.ID, &o.ProjectID, &o.Email, &o.TimeSlotID); err != nil {
			return nil, fmt.Errorf("scan observer: %w", err)
		}
		result = append(result, o)
	}
	return result, rows.Err()
}

// ListObserversForTimeSlot returns observers for a specific timeslot.
func (r *SurveyRepo) ListObserversForTimeSlot(ctx context.Context, timeSlotID int64) ([]ICObserver, error) {
	q := `SELECT id, project_id, email, time_slot_id FROM observer WHERE time_slot_id = ?`
	rows, err := r.ro().QueryContext(ctx, q, timeSlotID)
	if err != nil {
		return nil, fmt.Errorf("list timeslot observers: %w", err)
	}
	defer rows.Close()
	var result []ICObserver
	for rows.Next() {
		var o ICObserver
		if err := rows.Scan(&o.ID, &o.ProjectID, &o.Email, &o.TimeSlotID); err != nil {
			return nil, fmt.Errorf("scan observer: %w", err)
		}
		result = append(result, o)
	}
	return result, rows.Err()
}

// PutObserversForTimeSlot adds/removes observers by email.
func (r *SurveyRepo) PutObserversForTimeSlot(ctx context.Context, projectID, timeSlotID int64, toAdd, toDelete []string) error {
	for _, email := range toDelete {
		_, _ = r.db.ExecContext(ctx, "DELETE FROM observer WHERE project_id = ? AND time_slot_id = ? AND email = ?",
			projectID, timeSlotID, email)
	}
	for _, email := range toAdd {
		_, _ = r.db.ExecContext(ctx, "INSERT INTO observer (project_id, email, time_slot_id) VALUES (?, ?, ?)",
			projectID, email, timeSlotID)
	}
	return nil
}

// ListMediaForProject returns interview media for a project.
func (r *SurveyRepo) ListMediaForProject(ctx context.Context, projectID int64) ([]ICInterviewMedia, error) {
	q := `SELECT id, name, description, project_id, s3_key, hash, status, created_on, created_by,
	       page_count, pages_processed, shared
	      FROM interview_media WHERE project_id = ? ORDER BY created_on DESC`
	rows, err := r.ro().QueryContext(ctx, q, projectID)
	if err != nil {
		return nil, fmt.Errorf("list media: %w", err)
	}
	defer rows.Close()
	var result []ICInterviewMedia
	for rows.Next() {
		var m ICInterviewMedia
		if err := rows.Scan(&m.ID, &m.Name, &m.Description, &m.ProjectID, &m.S3Key, &m.Hash,
			&m.Status, &m.CreatedOn, &m.CreatedBy, &m.PageCount, &m.PagesProcessed, &m.Shared); err != nil {
			return nil, fmt.Errorf("scan media: %w", err)
		}
		result = append(result, m)
	}
	return result, rows.Err()
}

// GetMediaByID returns a single interview media item.
func (r *SurveyRepo) GetMediaByID(ctx context.Context, projectID, mediaID int64) (*ICInterviewMedia, error) {
	q := `SELECT id, name, description, project_id, s3_key, hash, status, created_on, created_by,
	       page_count, pages_processed, shared
	      FROM interview_media WHERE id = ? AND project_id = ?`
	var m ICInterviewMedia
	err := r.ro().QueryRowContext(ctx, q, mediaID, projectID).Scan(
		&m.ID, &m.Name, &m.Description, &m.ProjectID, &m.S3Key, &m.Hash,
		&m.Status, &m.CreatedOn, &m.CreatedBy, &m.PageCount, &m.PagesProcessed, &m.Shared)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get media %d: %w", mediaID, err)
	}
	return &m, nil
}

// DeleteMedia soft-deletes interview media.
func (r *SurveyRepo) DeleteMedia(ctx context.Context, projectID, mediaID int64) error {
	_, err := r.db.ExecContext(ctx, "DELETE FROM interview_media WHERE id = ? AND project_id = ?", mediaID, projectID)
	return err
}

// ListModeratorAvailability returns IRIS moderator availability for a subscription.
func (r *SurveyRepo) ListModeratorAvailability(ctx context.Context, moderatorID, subscriptionID int64) ([]ICModeratorAvailability, error) {
	q := `SELECT id, moderator_id, subscription_id, start_time, end_time
	      FROM moderator_availability WHERE moderator_id = ? AND subscription_id = ?
	      ORDER BY start_time`
	rows, err := r.ro().QueryContext(ctx, q, moderatorID, subscriptionID)
	if err != nil {
		return nil, fmt.Errorf("list iris avail: %w", err)
	}
	defer rows.Close()
	var result []ICModeratorAvailability
	for rows.Next() {
		var a ICModeratorAvailability
		if err := rows.Scan(&a.ID, &a.ModeratorID, &a.SubscriptionID, &a.StartTime, &a.EndTime); err != nil {
			return nil, fmt.Errorf("scan iris avail: %w", err)
		}
		result = append(result, a)
	}
	return result, rows.Err()
}

// CreateModeratorAvailability inserts IRIS moderator availability.
func (r *SurveyRepo) CreateModeratorAvailability(ctx context.Context, moderatorID, subscriptionID int64, startTime, endTime time.Time) (int64, error) {
	res, err := r.db.ExecContext(ctx, "INSERT INTO moderator_availability (moderator_id, subscription_id, start_time, end_time) VALUES (?, ?, ?, ?)",
		moderatorID, subscriptionID, startTime, endTime)
	if err != nil {
		return 0, fmt.Errorf("create iris avail: %w", err)
	}
	return res.LastInsertId()
}

// UpdateModeratorAvailability updates an IRIS moderator availability slot.
func (r *SurveyRepo) UpdateModeratorAvailability(ctx context.Context, id int64, startTime, endTime time.Time) error {
	_, err := r.db.ExecContext(ctx, "UPDATE moderator_availability SET start_time = ?, end_time = ? WHERE id = ?",
		startTime, endTime, id)
	return err
}

// DeleteModeratorAvailability deletes an IRIS moderator availability slot.
func (r *SurveyRepo) DeleteModeratorAvailability(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, "DELETE FROM moderator_availability WHERE id = ?", id)
	return err
}

// GetModeratorsForTimeSlot returns moderators assigned to a timeslot (IRIS).
func (r *SurveyRepo) GetModeratorsForTimeSlot(ctx context.Context, timeSlotID int64) ([]map[string]any, error) {
	q := `SELECT mts.id, mts.moderator_id, mts.time_slot_id, mts.is_host,
	       CONCAT(u.first_name, ' ', u.last_name) AS label,
	       COALESCE(uca.address,'') AS email
	      FROM moderator_time_slot mts
	      JOIN ic_user u ON u.id = mts.moderator_id
	      LEFT JOIN user_communication_address uca ON uca.user_id = u.id AND uca.transport_type_id = 1
	      WHERE mts.time_slot_id = ?`
	rows, err := r.ro().QueryContext(ctx, q, timeSlotID)
	if err != nil {
		return nil, fmt.Errorf("get mods for timeslot: %w", err)
	}
	defer rows.Close()
	var result []map[string]any
	for rows.Next() {
		var id, modID, tsID int64
		var isHost bool
		var label, email string
		if err := rows.Scan(&id, &modID, &tsID, &isHost, &label, &email); err != nil {
			return nil, fmt.Errorf("scan mod timeslot: %w", err)
		}
		result = append(result, map[string]any{
			"id": id, "moderatorId": modID, "timeSlotId": tsID,
			"isHost": isHost, "label": label, "email": email,
		})
	}
	return result, rows.Err()
}

// AssignModeratorToTimeSlot inserts a moderator_time_slot row.
func (r *SurveyRepo) AssignModeratorToTimeSlot(ctx context.Context, timeSlotID, moderatorID int64, isHost bool) (int64, error) {
	res, err := r.db.ExecContext(ctx,
		"INSERT INTO moderator_time_slot (moderator_id, time_slot_id, is_host) VALUES (?, ?, ?)",
		moderatorID, timeSlotID, isHost)
	if err != nil {
		return 0, fmt.Errorf("assign mod to timeslot: %w", err)
	}
	return res.LastInsertId()
}

// RemoveModeratorFromTimeSlot removes a moderator_time_slot row.
func (r *SurveyRepo) RemoveModeratorFromTimeSlot(ctx context.Context, timeSlotID, moderatorID int64) error {
	_, err := r.db.ExecContext(ctx,
		"DELETE FROM moderator_time_slot WHERE time_slot_id = ? AND moderator_id = ?",
		timeSlotID, moderatorID)
	return err
}

// ListUserProjects returns users assigned to a project.
func (r *SurveyRepo) ListUserProjects(ctx context.Context, projectID int64) ([]map[string]any, error) {
	q := `SELECT up.id, up.user_id, up.project_id, up.can_write, up.can_read,
	       u.first_name, u.last_name, COALESCE(uca.address,'') AS email
	      FROM user_project up
	      JOIN ic_user u ON u.id = up.user_id
	      LEFT JOIN user_communication_address uca ON uca.user_id = u.id AND uca.transport_type_id = 1
	      WHERE up.project_id = ?`
	rows, err := r.ro().QueryContext(ctx, q, projectID)
	if err != nil {
		return nil, fmt.Errorf("list project users: %w", err)
	}
	defer rows.Close()
	var result []map[string]any
	for rows.Next() {
		var id, uid, pid int64
		var canWrite, canRead bool
		var fn, ln, email string
		if err := rows.Scan(&id, &uid, &pid, &canWrite, &canRead, &fn, &ln, &email); err != nil {
			return nil, fmt.Errorf("scan user project: %w", err)
		}
		result = append(result, map[string]any{
			"id": id, "userId": uid, "projectId": pid,
			"canWrite": canWrite, "canRead": canRead,
			"firstName": fn, "lastName": ln, "email": email,
		})
	}
	return result, rows.Err()
}

// SalesforceProjectFilter holds optional filter parameters for listing salesforce projects.
type SalesforceProjectFilter struct {
	ID             string // salesforce_project_id (exact match)
	AccountID      int64  // maps to account.salesforce_account_id
	ProjectTypeID  int64  // salesforce_project_type filter
	IsProject      bool   // exclude records that already have a project row
	SubscriptionID int64  // filter via subscription.salesforce_subscription_id
	Search         string // LIKE search on name or number
}

// ListSalesforceProjects returns salesforce projects with optional filtering.
func (r *SurveyRepo) ListSalesforceProjects(ctx context.Context, f *SalesforceProjectFilter) ([]ICSalesforceProject, error) {
	// Build query dynamically based on filter
	base := `SELECT sp.id, sp.salesforce_project_id, sp.name, sp.number,
	          sp.salesforce_account_id, sp.subscription_id, sp.is_project_pricing,
	          sp.is_deleted, sp.last_modified_date, sp.client_project_name,
	          sp.client_project_number, sp.brand_type_id, sp.salesforce_project_type,
	          sp.salesforce_project_status, sp.owner_name, sp.project_manager_name,
	          sp.project_reconciled
	         FROM salesforce_project sp`

	var where []string
	var args []any

	where = append(where, "sp.is_deleted = 0")

	if f != nil && f.ID != "" {
		where = append(where, "sp.salesforce_project_id = ?")
		args = append(args, f.ID)
	}

	if f != nil && f.AccountID > 0 {
		// Resolve account's salesforce_account_id
		var sfAccountID string
		err := r.ro().QueryRowContext(ctx, "SELECT salesforce_account_id FROM account WHERE id = ?", f.AccountID).Scan(&sfAccountID)
		if err != nil {
			return nil, fmt.Errorf("resolve account %d: %w", f.AccountID, err)
		}
		where = append(where, "sp.salesforce_account_id = ?")
		args = append(args, sfAccountID)

		if f.ProjectTypeID > 0 {
			where = append(where, "sp.salesforce_project_type = ?")
			args = append(args, f.ProjectTypeID)
		}

		if f.IsProject {
			where = append(where, "NOT EXISTS (SELECT 1 FROM project p WHERE p.salesforce_project_id = sp.salesforce_project_id)")
		}

		if f.SubscriptionID > 0 {
			// Filter by subscription's brand_type_id
			var brandTypeID int64
			err := r.ro().QueryRowContext(ctx, "SELECT brand_type FROM subscription WHERE id = ?", f.SubscriptionID).Scan(&brandTypeID)
			if err == nil && brandTypeID > 0 {
				where = append(where, "sp.brand_type_id = ?")
				args = append(args, brandTypeID)
			}
		}

		if f.Search != "" {
			where = append(where, "(LOWER(sp.name) LIKE ? OR LOWER(sp.number) LIKE ?)")
			like := "%" + strings.ToLower(f.Search) + "%"
			args = append(args, like, like)
		}
	}

	q := base + " WHERE " + strings.Join(where, " AND ") + " ORDER BY sp.name"
	rows, err := r.ro().QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("list sf projects: %w", err)
	}
	defer rows.Close()
	var result []ICSalesforceProject
	for rows.Next() {
		var s ICSalesforceProject
		if err := rows.Scan(&s.ID, &s.SalesforceProjectID, &s.Name, &s.Number,
			&s.SalesforceAccountID, &s.SubscriptionID, &s.IsProjectPricing,
			&s.IsDeleted, &s.LastModifiedDate, &s.ClientProjectName,
			&s.ClientProjectNumber, &s.BrandTypeID, &s.SalesforceProjectType,
			&s.SalesforceProjectStatus, &s.OwnerName, &s.ProjectManagerName,
			&s.ProjectReconciled); err != nil {
			return nil, fmt.Errorf("scan sf project: %w", err)
		}
		result = append(result, s)
	}
	return result, rows.Err()
}

// GetMonoProjectID returns the project.id for a given salesforce_project_id, or nil if none.
func (r *SurveyRepo) GetMonoProjectID(ctx context.Context, salesforceProjectID string) *int64 {
	var id int64
	err := r.ro().QueryRowContext(ctx,
		"SELECT id FROM project WHERE salesforce_project_id = ? LIMIT 1", salesforceProjectID).Scan(&id)
	if err != nil {
		return nil
	}
	return &id
}

// ListProjectInquiries returns inquiries for a subscription.
func (r *SurveyRepo) ListProjectInquiries(ctx context.Context, subscriptionID int64) ([]ICProjectInquiry, error) {
	q := `SELECT id, description, notes, subscription_id, project_id, inquiry_type_id,
	       interview_length, required_completion_date, created_on, created_by,
	       under_review, transcripts_requested, requires_stimuli
	      FROM project_inquiry WHERE subscription_id = ? ORDER BY created_on DESC`
	rows, err := r.ro().QueryContext(ctx, q, subscriptionID)
	if err != nil {
		return nil, fmt.Errorf("list inquiries: %w", err)
	}
	defer rows.Close()
	var result []ICProjectInquiry
	for rows.Next() {
		var pi ICProjectInquiry
		if err := rows.Scan(&pi.ID, &pi.Description, &pi.Notes, &pi.SubscriptionID, &pi.ProjectID,
			&pi.InquiryTypeID, &pi.InterviewLength, &pi.RequiredCompletionDate, &pi.CreatedOn,
			&pi.CreatedBy, &pi.UnderReview, &pi.TranscriptsRequested, &pi.RequiresStimuli); err != nil {
			return nil, fmt.Errorf("scan inquiry: %w", err)
		}
		result = append(result, pi)
	}
	return result, rows.Err()
}

// GetProjectInquiry returns a single project inquiry.
func (r *SurveyRepo) GetProjectInquiry(ctx context.Context, subscriptionID, projectID int64) (*ICProjectInquiry, error) {
	q := `SELECT id, description, notes, subscription_id, project_id, inquiry_type_id,
	       interview_length, required_completion_date, created_on, created_by,
	       under_review, transcripts_requested, requires_stimuli
	      FROM project_inquiry WHERE subscription_id = ? AND project_id = ? LIMIT 1`
	var pi ICProjectInquiry
	err := r.ro().QueryRowContext(ctx, q, subscriptionID, projectID).Scan(
		&pi.ID, &pi.Description, &pi.Notes, &pi.SubscriptionID, &pi.ProjectID,
		&pi.InquiryTypeID, &pi.InterviewLength, &pi.RequiredCompletionDate, &pi.CreatedOn,
		&pi.CreatedBy, &pi.UnderReview, &pi.TranscriptsRequested, &pi.RequiresStimuli)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get inquiry: %w", err)
	}
	return &pi, nil
}

// GetAvailabilityAndTimeslotsForProject returns combined data for project dashboard.
func (r *SurveyRepo) GetAvailabilityAndTimeslotsForProject(ctx context.Context, projectID int64) (map[string]any, error) {
	// Time slots for this project
	var totalSlots, openSlots, bookedSlots int
	_ = r.ro().QueryRowContext(ctx, "SELECT COUNT(*) FROM time_slot WHERE project_id = ? AND is_invalid = 0", projectID).Scan(&totalSlots)
	_ = r.ro().QueryRowContext(ctx, "SELECT COUNT(*) FROM time_slot WHERE project_id = ? AND is_invalid = 0 AND status_id = 1", projectID).Scan(&openSlots)
	_ = r.ro().QueryRowContext(ctx, "SELECT COUNT(*) FROM time_slot WHERE project_id = ? AND is_invalid = 0 AND status_id IN (2,3,4,7,8,9)", projectID).Scan(&bookedSlots)

	// Moderator availability — get moderators assigned to this project
	avails := []map[string]any{}
	q := `SELECT ma.id, ma.moderator_id, ma.start_time, ma.end_time,
	       CONCAT(u.first_name, ' ', u.last_name) AS moderator_name
	      FROM moderator_availability ma
	      JOIN user_project upx ON upx.user_id = ma.moderator_id AND upx.project_id = ?
	      JOIN ic_user u ON u.id = ma.moderator_id
	      ORDER BY ma.start_time`
	rows, err := r.ro().QueryContext(ctx, q, projectID)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var id, modID int64
			var st, et time.Time
			var mName string
			if rows.Scan(&id, &modID, &st, &et, &mName) == nil {
				avails = append(avails, map[string]any{
					"id": id, "moderatorId": modID, "startTime": st.Format(time.RFC3339),
					"endTime": et.Format(time.RFC3339), "moderatorName": mName,
				})
			}
		}
	}

	return map[string]any{
		"totalSlots":    totalSlots,
		"openSlots":     openSlots,
		"bookedSlots":   bookedSlots,
		"availabilities": avails,
	}, nil
}

// GetSchedulerModerators returns moderators + their timeslots for project scheduler.
func (r *SurveyRepo) GetSchedulerModerators(ctx context.Context, projectID int64) ([]map[string]any, error) {
	q := `SELECT DISTINCT mts.moderator_id, CONCAT(u.first_name, ' ', u.last_name) AS name,
	       COALESCE(uca.address,'') AS email
	      FROM moderator_time_slot mts
	      JOIN time_slot ts ON ts.id = mts.time_slot_id AND ts.project_id = ?
	      JOIN ic_user u ON u.id = mts.moderator_id
	      LEFT JOIN user_communication_address uca ON uca.user_id = u.id AND uca.transport_type_id = 1
	      ORDER BY name`
	rows, err := r.ro().QueryContext(ctx, q, projectID)
	if err != nil {
		return nil, fmt.Errorf("get scheduler mods: %w", err)
	}
	defer rows.Close()
	var result []map[string]any
	for rows.Next() {
		var modID int64
		var name, email string
		if err := rows.Scan(&modID, &name, &email); err != nil {
			return nil, fmt.Errorf("scan scheduler mod: %w", err)
		}
		result = append(result, map[string]any{
			"moderatorId": modID, "name": name, "email": email,
		})
	}
	return result, rows.Err()
}

// GetProjectAvailability returns moderator availability for a project's subscription.
func (r *SurveyRepo) GetProjectAvailability(ctx context.Context, projectID int64) ([]ICModeratorAvailability, error) {
	q := `SELECT ma.id, ma.moderator_id, ma.subscription_id, ma.start_time, ma.end_time
	      FROM moderator_availability ma
	      JOIN project p ON p.subscription_id = ma.subscription_id AND p.id = ?
	      ORDER BY ma.start_time`
	rows, err := r.ro().QueryContext(ctx, q, projectID)
	if err != nil {
		return nil, fmt.Errorf("get project avail: %w", err)
	}
	defer rows.Close()
	var result []ICModeratorAvailability
	for rows.Next() {
		var a ICModeratorAvailability
		if err := rows.Scan(&a.ID, &a.ModeratorID, &a.SubscriptionID, &a.StartTime, &a.EndTime); err != nil {
			return nil, fmt.Errorf("scan avail: %w", err)
		}
		result = append(result, a)
	}
	return result, rows.Err()
}

// GetSubscriptionInterviews returns all interviews for a subscription.
func (r *SurveyRepo) GetSubscriptionInterviews(ctx context.Context, subscriptionID int64) ([]map[string]any, error) {
	q := `SELECT
	        ic_user.id AS participant_id,
	        project.id AS project_id,
	        project.name AS project_name,
	        crowd.name AS crowd_name,
	        survey.name_private AS survey_name_private,
	        time_slot.duration,
	        time_slot.start_time,
	        time_slot.end_time,
	        time_slot.id AS time_slot_id,
	        time_slot.conference_hash,
	        moderator_info.id AS host_moderator_id,
	        moderator_info.first_name AS host_moderator_first_name,
	        moderator_info.last_name AS host_moderator_last_name,
	        moderator_info.hash AS host_moderator_hash,
	        project_inquiry.requires_stimuli,
	        research_time_slot_hash.hash AS share_hash
	      FROM ic_user
	      INNER JOIN user_survey ON user_survey.user_id = ic_user.id
	      INNER JOIN crowd ON user_survey.crowd_id = crowd.id
	      INNER JOIN survey ON user_survey.survey_id = survey.id
	      INNER JOIN answer ON answer.user_survey_id = user_survey.id
	      INNER JOIN answer_details ON answer_details.answer_id = answer.id
	      INNER JOIN time_slot ON answer_details.time_slot_id = time_slot.id
	      LEFT JOIN research_time_slot_hash ON research_time_slot_hash.time_slot_id = time_slot.id
	      INNER JOIN (
	        SELECT
	          ic_user.id AS id,
	          ic_user.first_name AS first_name,
	          ic_user.last_name AS last_name,
	          conference_invitation.participant_hash AS hash,
	          moderator_time_slot.time_slot_id
	        FROM moderator_time_slot
	        INNER JOIN conference_invitation ON moderator_time_slot.time_slot_id = conference_invitation.time_slot_id
	          AND conference_invitation.user_id = moderator_time_slot.moderator_id
	          AND moderator_time_slot.is_host = 1
	        INNER JOIN ic_user ON ic_user.id = moderator_time_slot.moderator_id
	        WHERE moderator_time_slot.is_host
	      ) moderator_info ON moderator_info.time_slot_id = time_slot.id
	      INNER JOIN project ON time_slot.project_id = project.id
	      INNER JOIN project_inquiry ON project.id = project_inquiry.project_id
	      INNER JOIN subscription ON subscription.id = project.subscription_id
	      WHERE project.subscription_id = ?
	        AND user_survey.is_test = 0
	        AND user_survey.is_invalid = 0
	      ORDER BY time_slot.start_time ASC`
	rows, err := r.ro().QueryContext(ctx, q, subscriptionID)
	if err != nil {
		return nil, fmt.Errorf("get sub interviews: %w", err)
	}
	defer rows.Close()
	var result []map[string]any
	for rows.Next() {
		var participantID, projectID, timeSlotID, hostModeratorID int64
		var projectName, crowdName, surveyNamePrivate string
		var duration int64
		var startTime, endTime time.Time
		var conferenceHash string
		var hostModeratorFirstName, hostModeratorLastName, hostModeratorHash string
		var requiresStimulus bool
		var shareHash sql.NullString
		if err := rows.Scan(&participantID, &projectID, &projectName, &crowdName,
			&surveyNamePrivate, &duration, &startTime, &endTime, &timeSlotID,
			&conferenceHash, &hostModeratorID, &hostModeratorFirstName,
			&hostModeratorLastName, &hostModeratorHash, &requiresStimulus,
			&shareHash); err != nil {
			return nil, fmt.Errorf("scan sub interview: %w", err)
		}
		var shareHashVal any
		if shareHash.Valid {
			shareHashVal = shareHash.String
		}
		m := map[string]any{
			"participantId":          participantID,
			"projectId":               projectID,
			"projectName":             projectName,
			"crowdName":               crowdName,
			"surveyNamePrviate":       surveyNamePrivate,
			"duration":                duration,
			"startTime":               startTime.Format(time.RFC3339),
			"endTime":                 endTime.Format(time.RFC3339),
			"timeSlotId":              timeSlotID,
			"conferenceHash":          conferenceHash,
			"hostModeratorId":         hostModeratorID,
			"hostModeratorFirstName":  hostModeratorFirstName,
			"hostModeratorLastName":   hostModeratorLastName,
			"hostModeratorHash":       hostModeratorHash,
			"requiresStimulus":        requiresStimulus,
			"shareHash":               shareHashVal,
		}
		result = append(result, m)
	}
	return result, rows.Err()
}

// GetSubscriptionQuestionTypes returns question types for a subscription.
func (r *SurveyRepo) GetSubscriptionQuestionTypes(ctx context.Context, subscriptionID int64) ([]map[string]any, error) {
	q := `SELECT DISTINCT st.id, st.name
	      FROM survey_type st
	      JOIN survey s ON s.survey_type_id = st.id AND s.subscription_id = ?
	      ORDER BY st.name`
	rows, err := r.ro().QueryContext(ctx, q, subscriptionID)
	if err != nil {
		return nil, fmt.Errorf("get question types: %w", err)
	}
	defer rows.Close()
	var result []map[string]any
	for rows.Next() {
		var id int64
		var name string
		if err := rows.Scan(&id, &name); err != nil {
			return nil, fmt.Errorf("scan question type: %w", err)
		}
		result = append(result, map[string]any{"id": id, "name": name})
	}
	return result, rows.Err()
}

// GetQualRescheduleBody returns the qual reschedule email template body for a project.
func (r *SurveyRepo) GetQualRescheduleBody(ctx context.Context, projectID int64) (string, error) {
	var body sql.NullString
	err := r.ro().QueryRowContext(ctx,
		`SELECT ct.body FROM communication_template ct
		 JOIN survey s ON s.invite_template_id = ct.id
		 WHERE s.project_id = ? LIMIT 1`, projectID).Scan(&body)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return body.String, nil
}

// GetNoShowCheck returns the latest timeslot that could be a no-show.
func (r *SurveyRepo) GetNoShowCheck(ctx context.Context) (map[string]any, error) {
	q := `SELECT ts.id, ts.project_id, ts.start_time, ts.end_time, ts.status_id,
	       ts.interviewee_id, CONCAT(u.first_name, ' ', u.last_name) AS name
	      FROM time_slot ts
	      LEFT JOIN ic_user u ON u.id = ts.interviewee_id
	      WHERE ts.status_id IN (2, 7, 8) AND ts.start_time < NOW() AND ts.is_invalid = 0
	      ORDER BY ts.start_time DESC LIMIT 1`
	var tsID, projectID int64
	var startTime, endTime time.Time
	var statusID int
	var intervieweeID sql.NullInt64
	var name sql.NullString
	err := r.ro().QueryRowContext(ctx, q).Scan(&tsID, &projectID, &startTime, &endTime, &statusID, &intervieweeID, &name)
	if err == sql.ErrNoRows {
		return map[string]any{"timeSlot": nil, "interviewee": nil}, nil
	}
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"timeSlot": map[string]any{
			"id": tsID, "projectId": projectID, "startTime": startTime.Format(time.RFC3339),
			"endTime": endTime.Format(time.RFC3339), "statusId": statusID,
		},
		"interviewee": map[string]any{
			"id": intervieweeID.Int64, "name": name.String,
		},
	}, nil
}

// MarkNoShow marks a timeslot as no-show and stops payment.
func (r *SurveyRepo) MarkNoShow(ctx context.Context, projectID, timeSlotID int64) error {
	_, err := r.db.ExecContext(ctx,
		"UPDATE time_slot SET status_id = 10, stop_payment = 1 WHERE id = ? AND project_id = ?",
		timeSlotID, projectID)
	return err
}

// GetPossibleModeratorsForTimeSlot returns moderators who could be assigned to a slot.
func (r *SurveyRepo) GetPossibleModeratorsForTimeSlot(ctx context.Context, timeSlotID int64) ([]map[string]any, error) {
	// Get the timeslot's project and time
	var projectID int64
	var startTime, endTime time.Time
	err := r.ro().QueryRowContext(ctx, "SELECT project_id, start_time, end_time FROM time_slot WHERE id = ?", timeSlotID).
		Scan(&projectID, &startTime, &endTime)
	if err != nil {
		return nil, fmt.Errorf("get timeslot for mod options: %w", err)
	}

	// Find moderators who: (1) are assigned to this project, (2) have availability covering this slot
	q := `SELECT DISTINCT u.id, CONCAT(u.first_name, ' ', u.last_name) AS name,
	       COALESCE(uca.address,'') AS email
	      FROM ic_user u
	      JOIN user_project up ON up.user_id = u.id AND up.project_id = ?
	      LEFT JOIN user_communication_address uca ON uca.user_id = u.id AND uca.transport_type_id = 1
	      JOIN moderator_availability ma ON ma.moderator_id = u.id
	        AND ma.start_time <= ? AND ma.end_time >= ?
	      WHERE u.id NOT IN (
	        SELECT mts.moderator_id FROM moderator_time_slot mts WHERE mts.time_slot_id = ?
	      )
	      ORDER BY name`
	rows, err := r.ro().QueryContext(ctx, q, projectID, startTime, endTime, timeSlotID)
	if err != nil {
		return nil, fmt.Errorf("get possible mods: %w", err)
	}
	defer rows.Close()
	var result []map[string]any
	for rows.Next() {
		var id int64
		var name, email string
		if err := rows.Scan(&id, &name, &email); err != nil {
			return nil, fmt.Errorf("scan possible mod: %w", err)
		}
		result = append(result, map[string]any{"id": id, "name": name, "email": email})
	}
	return result, rows.Err()
}
// UpdateInquiryPreview updates a project inquiry's preview fields.
func (r *SurveyRepo) UpdateInquiryPreview(ctx context.Context, subscriptionID, projectID int64, fields map[string]any) error {
	setClauses := []string{}
	args := []any{}
	for k, v := range fields {
		setClauses = append(setClauses, k+" = ?")
		args = append(args, v)
	}
	if len(setClauses) == 0 {
		return nil
	}
	args = append(args, subscriptionID, projectID)
	q := "UPDATE project_inquiry SET " + strings.Join(setClauses, ", ") + " WHERE subscription_id = ? AND project_id = ?"
	_, err := r.db.ExecContext(ctx, q, args...)
	return err
}

// CreateCustomCrowdInquiry creates a custom crowd inquiry.
func (r *SurveyRepo) CreateCustomCrowdInquiry(ctx context.Context, subscriptionID int64, description string, interviewLength int, inquiryTypeID int) (int64, error) {
	q := `INSERT INTO project_inquiry (subscription_id, description, interview_length, inquiry_type_id, under_review, created_on)
	      VALUES (?, ?, ?, ?, 1, NOW())`
	res, err := r.db.ExecContext(ctx, q, subscriptionID, description, interviewLength, inquiryTypeID)
	if err != nil {
		return 0, fmt.Errorf("create custom crowd inquiry: %w", err)
	}
	return res.LastInsertId()
}

// ResetProjectModerators removes all moderator assignments from project timeslots.
func (r *SurveyRepo) ResetProjectModerators(ctx context.Context, projectID int64) (int64, error) {
	q := `DELETE mts FROM moderator_time_slot mts
	      INNER JOIN time_slot ts ON ts.id = mts.time_slot_id
	      WHERE ts.project_id = ? AND ts.status_id IN (1, 2)` // only open/pending slots
	res, err := r.db.ExecContext(ctx, q, projectID)
	if err != nil {
		return 0, fmt.Errorf("reset project moderators: %w", err)
	}
	n, _ := res.RowsAffected()
	return n, nil
}

// GetEmailTemplateForProject returns email template settings for a project.
func (r *SurveyRepo) GetEmailTemplateForProject(ctx context.Context, projectID int64) (map[string]any, error) {
	q := `SELECT p.id, p.name, COALESCE(p.email_subject, '') as subject,
	             COALESCE(p.email_body, '') as body, COALESCE(p.email_from_name, '') as from_name
	      FROM project p WHERE p.id = ?`
	var id int64
	var name, subject, body, fromName string
	err := r.ro().QueryRowContext(ctx, q, projectID).Scan(&id, &name, &subject, &body, &fromName)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get email template: %w", err)
	}
	return map[string]any{
		"projectId": id, "projectName": name,
		"subject": subject, "body": body, "fromName": fromName,
	}, nil
}

// ExportProjectData returns project interview data for export.
func (r *SurveyRepo) ExportProjectData(ctx context.Context, projectID int64) ([]map[string]any, error) {
	q := `SELECT ts.id, ts.start_time, ts.end_time, ts.status_id, tss.description as status,
	             COALESCE(CONCAT(u.first_name, ' ', u.last_name), '') as moderator,
	             COALESCE(r.first_name, '') as respondent_first, COALESCE(r.last_name, '') as respondent_last
	      FROM time_slot ts
	      LEFT JOIN time_slot_status tss ON tss.id = ts.status_id
	      LEFT JOIN moderator_time_slot mts ON mts.time_slot_id = ts.id AND mts.is_host = 1
	      LEFT JOIN ic_user u ON u.id = mts.moderator_id
	      LEFT JOIN responder r ON r.id = ts.responder_id
	      WHERE ts.project_id = ? AND ts.is_invalid = 0
	      ORDER BY ts.start_time`
	rows, err := r.ro().QueryContext(ctx, q, projectID)
	if err != nil {
		return nil, fmt.Errorf("export project data: %w", err)
	}
	defer rows.Close()
	var result []map[string]any
	for rows.Next() {
		var id, statusID int64
		var st, et time.Time
		var status, moderator, rFirst, rLast string
		if err := rows.Scan(&id, &st, &et, &statusID, &status, &moderator, &rFirst, &rLast); err != nil {
			continue
		}
		result = append(result, map[string]any{
			"timeSlotId": id, "startTime": st.Format(time.RFC3339), "endTime": et.Format(time.RFC3339),
			"statusId": statusID, "status": status, "moderator": moderator,
			"respondentFirstName": rFirst, "respondentLastName": rLast,
		})
	}
	return result, rows.Err()
}

// GetUnavailableModerators returns moderators unavailable for a project.
func (r *SurveyRepo) GetUnavailableModerators(ctx context.Context, projectID int64) ([]map[string]any, error) {
	q := `SELECT DISTINCT u.id, CONCAT(u.first_name, ' ', u.last_name) as name, u.email
	      FROM ic_user u
	      INNER JOIN moderator_time_slot mts ON mts.moderator_id = u.id
	      INNER JOIN time_slot ts ON ts.id = mts.time_slot_id
	      WHERE ts.project_id = ? AND ts.status_id IN (2, 3, 4, 7, 8, 9)
	      ORDER BY name`
	rows, err := r.ro().QueryContext(ctx, q, projectID)
	if err != nil {
		return nil, fmt.Errorf("get unavailable mods: %w", err)
	}
	defer rows.Close()
	var result []map[string]any
	for rows.Next() {
		var id int64
		var name, email string
		if err := rows.Scan(&id, &name, &email); err != nil {
			continue
		}
		result = append(result, map[string]any{"id": id, "name": name, "email": email})
	}
	return result, rows.Err()
}

// GetAvailableModeratorsCount returns count of available moderators for a project/sample size.
func (r *SurveyRepo) GetAvailableModeratorsCount(ctx context.Context, projectID int64) (int, error) {
	var count int
	q := `SELECT COUNT(DISTINCT mts.moderator_id)
	      FROM moderator_time_slot mts
	      INNER JOIN time_slot ts ON ts.id = mts.time_slot_id
	      WHERE ts.project_id = ? AND ts.status_id = 1`
	err := r.ro().QueryRowContext(ctx, q, projectID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count available mods: %w", err)
	}
	return count, nil
}

// ICProjectBasic holds the minimal project fields needed for subscription project survey listing.
type ICProjectBasic struct {
	ID              int64  `json:"id"`
	Name            string `json:"name"`
	ProjectStatusID int    `json:"projectStatusId"`
}

// ListProjectsForSubscription returns non-archived, non-draft projects for a subscription.
// Mirrors legacy: Project.readWhere('subscription_id -> subId, 'project_status_id_not_in -> 1, 'is_archived -> false)
func (r *SurveyRepo) ListProjectsForSubscription(ctx context.Context, subscriptionID int64) ([]ICProjectBasic, error) {
	q := `SELECT id, name, project_status_id
	      FROM project
	      WHERE subscription_id = ? AND project_status_id NOT IN (1) AND is_archived = 0`
	rows, err := r.ro().QueryContext(ctx, q, subscriptionID)
	if err != nil {
		return nil, fmt.Errorf("list projects for subscription: %w", err)
	}
	defer rows.Close()
	var result []ICProjectBasic
	for rows.Next() {
		var p ICProjectBasic
		if err := rows.Scan(&p.ID, &p.Name, &p.ProjectStatusID); err != nil {
			return nil, fmt.Errorf("scan project basic: %w", err)
		}
		result = append(result, p)
	}
	return result, rows.Err()
}

// GetSurveyStatusLabel returns the status and label from the survey_status table.
func (r *SurveyRepo) GetSurveyStatusLabel(ctx context.Context, statusCode int) (int, string, error) {
	var status int
	var label string
	err := r.ro().QueryRowContext(ctx,
		"SELECT status, label FROM survey_status WHERE status = ?", statusCode).Scan(&status, &label)
	if err != nil {
		return statusCode, "", fmt.Errorf("get survey status label: %w", err)
	}
	return status, label, nil
}

// GetFirstSurveyCrowdName returns the crowd name for the first (head) survey_crowd entry.
// Mirrors legacy: SurveyCrowd.readWhere('survey_id -> id).head → Crowd.getWithId(crowdId).name
func (r *SurveyRepo) GetFirstSurveyCrowdName(ctx context.Context, surveyID int64) (string, error) {
	var name string
	q := `SELECT c.name
	      FROM survey_crowd sc
	      JOIN crowd c ON c.id = sc.crowd_id
	      WHERE sc.survey_id = ?
	      LIMIT 1`
	err := r.ro().QueryRowContext(ctx, q, surveyID).Scan(&name)
	if err != nil {
		return "", fmt.Errorf("get first survey crowd name: %w", err)
	}
	return name, nil
}

// CountSurveyQuestions returns the number of questions in a survey.
func (r *SurveyRepo) CountSurveyQuestions(ctx context.Context, surveyID int64) (int, error) {
	var count int
	err := r.ro().QueryRowContext(ctx,
		"SELECT COUNT(*) FROM survey_question WHERE survey_id = ?", surveyID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count survey questions: %w", err)
	}
	return count, nil
}

// CountSurveyCompletions returns the number of completed (non-invalid, non-test) responses.
// Mirrors legacy: UserSurvey.countWhere('survey_id -> id, 'user_survey_status_id -> 3, 'is_invalid -> false, 'is_test -> false)
func (r *SurveyRepo) CountSurveyCompletions(ctx context.Context, surveyID int64) (int, error) {
	var count int
	err := r.ro().QueryRowContext(ctx,
		`SELECT COUNT(*) FROM user_survey
		 WHERE survey_id = ? AND user_survey_status_id = 3 AND is_invalid = 0 AND is_test = 0`,
		surveyID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count survey completions: %w", err)
	}
	return count, nil
}

// IsSurveyFavoriteOf checks if a survey is favorited by a specific user.
func (r *SurveyRepo) IsSurveyFavoriteOf(ctx context.Context, surveyID, userID int64) (bool, error) {
	var count int
	err := r.ro().QueryRowContext(ctx,
		"SELECT COUNT(id) FROM user_survey_favorite WHERE user_id = ? AND survey_id = ?",
		userID, surveyID).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("check survey favorite: %w", err)
	}
	return count > 0, nil
}

// GetSubscriptionCompany returns the company name for a subscription.
func (r *SurveyRepo) GetSubscriptionCompany(ctx context.Context, subscriptionID int64) (string, error) {
	var company string
	err := r.ro().QueryRowContext(ctx,
		"SELECT COALESCE(company, '') FROM subscription WHERE id = ?", subscriptionID).Scan(&company)
	if err != nil {
		return "", fmt.Errorf("get subscription company: %w", err)
	}
	return company, nil
}

// GetProjectName returns the name of a project.
func (r *SurveyRepo) GetProjectName(ctx context.Context, projectID int64) (string, error) {
	var name string
	err := r.ro().QueryRowContext(ctx,
		"SELECT COALESCE(name, '') FROM project WHERE id = ?", projectID).Scan(&name)
	if err != nil {
		return "", fmt.Errorf("get project name: %w", err)
	}
	return name, nil
}

// GetProjectTypeID returns the project_type_id for a project.
func (r *SurveyRepo) GetProjectTypeID(ctx context.Context, projectID int64) (int, error) {
	var ptid int
	err := r.ro().QueryRowContext(ctx,
		"SELECT COALESCE(project_type_id, 1) FROM project WHERE id = ?", projectID).Scan(&ptid)
	if err != nil {
		return 1, fmt.Errorf("get project type id: %w", err)
	}
	return ptid, nil
}

// ICSurveyType maps the survey_type table.
type ICSurveyType struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

// GetSurveyType returns the survey type by ID.
func (r *SurveyRepo) GetSurveyType(ctx context.Context, typeID int) (map[string]any, error) {
	var id int64
	var name string
	err := r.ro().QueryRowContext(ctx,
		"SELECT id, name FROM survey_type WHERE id = ?", typeID).Scan(&id, &name)
	if err != nil {
		return nil, fmt.Errorf("get survey type: %w", err)
	}
	return map[string]any{"id": id, "name": name}, nil
}

// GetSurveyPricing returns pricing info for a survey.
func (r *SurveyRepo) GetSurveyPricing(ctx context.Context, surveyID int64) (map[string]any, error) {
	var pricingTypeID int64
	var freeScreeners int64
	var fixedRate sql.NullInt64
	err := r.ro().QueryRowContext(ctx,
		"SELECT COALESCE(survey_pricing_type_id, 0), COALESCE(free_screeners, 0), fixed_rate FROM survey_pricing WHERE survey_id = ?",
		surveyID).Scan(&pricingTypeID, &freeScreeners, &fixedRate)
	if err != nil {
		// Return defaults if no pricing row
		return map[string]any{"surveyPricingTypeId": 0, "freeScreeners": 0, "fixedRate": nil}, nil
	}
	result := map[string]any{
		"surveyPricingTypeId": pricingTypeID,
		"freeScreeners":       freeScreeners,
	}
	if fixedRate.Valid {
		result["fixedRate"] = fixedRate.Int64
	} else {
		result["fixedRate"] = nil
	}
	return result, nil
}

// GetUserProjectPermissions returns a user's permissions on a project.
func (r *SurveyRepo) GetUserProjectPermissions(ctx context.Context, userID, projectID int64) (map[string]any, error) {
	var id int64
	var canWrite, canRead bool
	err := r.ro().QueryRowContext(ctx,
		"SELECT id, can_write, can_read FROM user_project WHERE user_id = ? AND project_id = ?",
		userID, projectID).Scan(&id, &canWrite, &canRead)
	if err != nil {
		// No user_project row — return defaults
		return map[string]any{
			"userId": userID, "projectId": projectID,
			"canWrite": false, "canRead": false, "favorite": false,
		}, nil
	}
	return map[string]any{
		"id": id, "userId": userID, "projectId": projectID,
		"canWrite": canWrite, "canRead": canRead,
	}, nil
}

// UserCanReadProject checks if a user has read access to a project.
func (r *SurveyRepo) UserCanReadProject(ctx context.Context, userID, projectID int64) (bool, error) {
	var canRead bool
	err := r.ro().QueryRowContext(ctx,
		"SELECT can_read FROM user_project WHERE user_id = ? AND project_id = ?",
		userID, projectID).Scan(&canRead)
	if err != nil {
		return false, nil // no row means no access
	}
	return canRead, nil
}

// CountSurveyCrowds returns the number of non-excluded crowds on a survey.
func (r *SurveyRepo) CountSurveyCrowds(ctx context.Context, surveyID int64) (int, error) {
	var count int
	err := r.ro().QueryRowContext(ctx,
		"SELECT COUNT(*) FROM survey_crowd WHERE survey_id = ? AND excluded = 0", surveyID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count survey crowds: %w", err)
	}
	return count, nil
}

// CountScreenOutCompletions returns completions with screen-out status.
func (r *SurveyRepo) CountScreenOutCompletions(ctx context.Context, surveyID int64) (int, error) {
	var count int
	err := r.ro().QueryRowContext(ctx,
		`SELECT COUNT(*) FROM user_survey
		 WHERE survey_id = ? AND user_survey_status_id IN (3, 5) AND is_invalid = 0 AND is_test = 0`,
		surveyID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count screen out completions: %w", err)
	}
	return count, nil
}