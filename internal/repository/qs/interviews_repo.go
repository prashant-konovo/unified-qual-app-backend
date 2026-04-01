package qs

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

// InterviewsRepo provides queries for the PM interviews dashboard.
type InterviewsRepo struct {
	db *sql.DB
}

func NewInterviewsRepo(db *sql.DB) *InterviewsRepo {
	return &InterviewsRepo{db: db}
}

const completedPaymentStatus = "COMPLETED"

// GetAllInterviewsByOffsetAndActiveTab returns interview records matching the legacy
// getAllInterviewsByOffsetAndActiveTab query. Supports activeTab: upcoming, invalidate, completed(default).
func (r *InterviewsRepo) GetAllInterviewsByOffsetAndActiveTab(
	ctx context.Context,
	clientID int64,
	externalClientsIDs []string,
	projectsIDs []string,
	search string,
	offset int,
	activeTab string,
	paymentStatusCode string,
) ([]map[string]any, error) {

	if len(externalClientsIDs) == 0 {
		externalClientsIDs = []string{"''"}
	}
	if len(projectsIDs) == 0 {
		projectsIDs = []string{"-1"}
	}

	externalClientsIn := "(" + strings.Join(externalClientsIDs, ",") + ")"
	projectsNotIn := "(" + strings.Join(projectsIDs, ",") + ")"

	// Payment condition
	paymentCondition := ""
	if paymentStatusCode == "CREDITED" {
		paymentCondition = fmt.Sprintf(` AND exists (select 1 from time_slot_payment_history tsph where tsph.time_slot_id=time_slot.id and tsph.payment_status='%s')`, completedPaymentStatus)
	} else if paymentStatusCode == "NOT_CREDITED" {
		paymentCondition = fmt.Sprintf(` AND not exists (select 1 from time_slot_payment_history tsph where tsph.time_slot_id=time_slot.id and tsph.payment_status='%s') AND time_slot.end_time >= CURDATE() - INTERVAL 7 DAY`, completedPaymentStatus)
	} else if paymentStatusCode == "PAST_DUE" {
		paymentCondition = fmt.Sprintf(` AND not exists (select 1 from time_slot_payment_history tsph where tsph.time_slot_id=time_slot.id and tsph.payment_status='%s') AND time_slot.end_time < CURDATE() - INTERVAL 7 DAY`, completedPaymentStatus)
	}

	searchCondition := ""
	if search != "" {
		escapedSearch := strings.ReplaceAll(search, "'", "''")
		searchCondition = fmt.Sprintf(` AND ((project.name like '%%%s%%' or project.external_survey_id like '%%%s%%' or project.salesforce_job_number like '%%%s%%' ))`, escapedSearch, escapedSearch, escapedSearch)
	}

	var selectCols, fromJoins, whereClause, orderBy string

	if activeTab == "upcoming" {
		selectCols = `project.project_external_client AS projectExternalClient, salesforce_account.name as clientName, project.external_survey_id AS externalSurveyId,
			responder.external_responder_id AS participantId, CASE WHEN pes.is_eligible = 0 AND time_slot.start_time >= pes.eligibility_updated_at THEN 1 ELSE NULL END AS Ineligible,
			moderator_info.id AS moderatorId, time_slot.duration,
			(CASE WHEN time_slot.has_imported_overlap IS NULL THEN '0' ELSE time_slot.has_imported_overlap END) AS importedOverLap,
			time_slot.start_time AS startTime, time_slot.end_time AS endTime, time_slot.id AS timeSlotId,
			time_slot.conference_hash AS conferenceHash, time_slot.invalidation_reason_code AS invalidationReasonCode,
			time_slot.is_previous_no_show AS isPreviousNoShow,
			moderator_info.first_name AS firstName, moderator_info.last_name AS lastName,
			project.name AS projectName, project.id AS projectId,
			conference_invitation.conference_link AS conferenceLink,
			project.salesforce_job_number AS salesforceJobNumber, topics.topic_name as topicName,
			defaultHonorarium.amount as defaultHonorariumAmount,
			defaultHonorarium.currency as defaultHonorariumCurrency,
			customHonorarium.new_value as customHonorariumAmount,
			payments.totalAmount, payments.paymentDate, payments.source, payments.currency,
			responder.first_name as participantFirstName, responder.last_name as participantLastName,
			(case
				when exists (select 1 from time_slot_payment_history tsph where tsph.time_slot_id=time_slot.id and tsph.payment_status='` + completedPaymentStatus + `') then 'CREDITED'
				when not exists (select 1 from time_slot_payment_history tsph where tsph.time_slot_id=time_slot.id and tsph.payment_status='` + completedPaymentStatus + `') AND time_slot.end_time < CURDATE() - INTERVAL 7 day then 'PAST_DUE'
				else 'NOT_CREDITED' end) as paymentStatus`

		fromJoins = `FROM responder
			INNER JOIN answer_details ON responder.id = answer_details.responder_id
			INNER JOIN time_slot ON time_slot.id = answer_details.time_slot_id
			INNER JOIN (SELECT user.id AS id, user.first_name AS first_name, user.last_name AS last_name, moderator_time_slot.time_slot_id FROM moderator_time_slot INNER JOIN user ON user.id = moderator_time_slot.moderator_id WHERE moderator_time_slot.is_host AND user.deleted = 0) moderator_info ON moderator_info.time_slot_id = time_slot.id
			INNER JOIN project ON time_slot.project_id = project.id
			LEFT JOIN topics on project.id = topics.project_id and topics.language_id = 1
			INNER JOIN conference_invitation_responder_time_slot ON conference_invitation_responder_time_slot.time_slot_id = time_slot.id
			INNER JOIN conference_invitation ON conference_invitation.id = conference_invitation_responder_time_slot.conference_invitation_id
			INNER JOIN external_client ON project.project_external_client = external_client.id
			INNER JOIN salesforce_account on salesforce_account.salesforce_account_id = external_client.external_client_account_id
			LEFT JOIN participant_eligibility_status pes ON pes.participant_id = responder.external_responder_id AND pes.is_eligible = 0
			LEFT JOIN (select sum(tsph.amount) as totalAmount, max(tsph.payment_date) paymentDate, max(tsph.source) as source, max(tsph.currency) as currency, time_slot_id from time_slot_payment_history tsph where tsph.payment_status='` + completedPaymentStatus + `' group by tsph.time_slot_id) payments on payments.time_slot_id=time_slot.id
			LEFT JOIN (SELECT MAX(ha.honorarium) AS amount, MAX(ha.currency) AS currency, ts.id AS timeSlotId FROM time_slot ts JOIN time_slot_event tse ON tse.time_slot_id = ts.id JOIN responder r ON r.id = tse.responder_id JOIN honorarium_amount ha ON ha.sessKey = r.sess_key GROUP BY ts.id) defaultHonorarium ON defaultHonorarium.timeSlotId = time_slot.id
			LEFT JOIN time_slot_custom_honorarium customHonorarium on customHonorarium.time_slot_id = time_slot.id`

		whereClause = fmt.Sprintf(`WHERE project.client_id = ? AND time_slot.status_id IN (2, 7, 8, 9) AND time_slot.is_invalidated_interview = 0 AND time_slot.start_time >= now() AND external_client.external_client_account_id IN %s%s AND project.id NOT IN %s%s`,
			externalClientsIn, searchCondition, projectsNotIn, paymentCondition)
		orderBy = `ORDER BY time_slot.start_time desc`

	} else if activeTab == "invalidate" {
		selectCols = `project.project_external_client AS projectExternalClient, salesforce_account.name as clientName, project.external_survey_id AS externalSurveyId,
			responder.external_responder_id AS participantId, CASE WHEN pes.is_eligible = 0 AND time_slot.start_time >= pes.eligibility_updated_at THEN 1 ELSE NULL END AS Ineligible,
			moderator_info.id AS moderatorId, time_slot.duration, time_slot.updated_on AS updatedOn,
			(CASE WHEN time_slot.has_imported_overlap IS NULL THEN '0' ELSE time_slot.has_imported_overlap END) AS importedOverLap,
			time_slot.start_time AS startTime, time_slot.end_time AS endTime, time_slot.id AS timeSlotId,
			time_slot.conference_hash AS conferenceHash, time_slot.invalidation_reason_code AS invalidationReasonCode,
			time_slot.invalidation_reason_text AS invalidationReasonText,
			time_slot.is_invalidate_email_sent AS isInvalidateEmailSent,
			time_slot.is_previous_no_show AS isPreviousNoShow,
			moderator_info.first_name AS firstName, moderator_info.last_name AS lastName,
			project.name AS projectName, project.id AS projectId,
			conference_invitation.conference_link AS conferenceLink,
			project.salesforce_job_number AS salesforceJobNumber, topics.topic_name as topicName,
			responder.first_name as participantFirstName, responder.last_name as participantLastName`

		fromJoins = `FROM responder
			INNER JOIN answer_details ON responder.id = answer_details.responder_id
			INNER JOIN time_slot ON time_slot.id = answer_details.time_slot_id
			INNER JOIN (SELECT user.id AS id, user.first_name AS first_name, user.last_name AS last_name, moderator_time_slot.time_slot_id FROM moderator_time_slot INNER JOIN user ON user.id = moderator_time_slot.moderator_id WHERE moderator_time_slot.is_host AND user.deleted = 0) moderator_info ON moderator_info.time_slot_id = time_slot.id
			INNER JOIN project ON time_slot.project_id = project.id
			LEFT JOIN topics on project.id = topics.project_id and topics.language_id = 1
			INNER JOIN conference_invitation_responder_time_slot ON conference_invitation_responder_time_slot.time_slot_id = time_slot.id
			INNER JOIN conference_invitation ON conference_invitation.id = conference_invitation_responder_time_slot.conference_invitation_id
			INNER JOIN external_client ON project.project_external_client = external_client.id
			INNER JOIN salesforce_account on salesforce_account.salesforce_account_id = external_client.external_client_account_id
			LEFT JOIN participant_eligibility_status pes ON pes.participant_id = responder.external_responder_id AND pes.is_eligible = 0`

		whereClause = fmt.Sprintf(`WHERE project.client_id = ? AND time_slot.is_invalidated_interview = 1 AND external_client.external_client_account_id IN %s%s AND project.id NOT IN %s`,
			externalClientsIn, searchCondition, projectsNotIn)
		orderBy = `ORDER BY time_slot.updated_on desc`

	} else {
		// completed / default
		selectCols = `project.project_external_client AS projectExternalClient, salesforce_account.name as clientName, project.external_survey_id AS externalSurveyId,
			responder.external_responder_id AS participantId, CASE WHEN pes.is_eligible = 0 AND time_slot.start_time >= pes.eligibility_updated_at THEN 1 ELSE NULL END AS Ineligible,
			moderator_info.id AS moderatorId, time_slot.duration,
			(CASE WHEN time_slot.has_imported_overlap IS NULL THEN '0' ELSE time_slot.has_imported_overlap END) AS importedOverLap,
			time_slot.start_time AS startTime, time_slot.end_time AS endTime, time_slot.id AS timeSlotId,
			time_slot.conference_hash AS conferenceHash, time_slot.invalidation_reason_code AS invalidationReasonCode,
			time_slot.is_previous_no_show AS isPreviousNoShow,
			moderator_info.first_name AS firstName, moderator_info.last_name AS lastName,
			project.name AS projectName, project.id AS projectId,
			conference_invitation.conference_link AS conferenceLink,
			project.salesforce_job_number AS salesforceJobNumber, topics.topic_name as topicName,
			defaultHonorarium.amount as defaultHonorariumAmount,
			defaultHonorarium.currency as defaultHonorariumCurrency,
			customHonorarium.new_value as customHonorariumAmount,
			payments.totalAmount, payments.paymentDate, payments.source, payments.currency,
			responder.first_name as participantFirstName, responder.last_name as participantLastName,
			(case
				when exists (select 1 from time_slot_payment_history tsph where tsph.time_slot_id=time_slot.id and tsph.payment_status='` + completedPaymentStatus + `') then 'CREDITED'
				when not exists (select 1 from time_slot_payment_history tsph where tsph.time_slot_id=time_slot.id and tsph.payment_status='` + completedPaymentStatus + `') AND time_slot.end_time < CURDATE() - INTERVAL 7 day then 'PAST_DUE'
				else 'NOT_CREDITED' end) as paymentStatus`

		fromJoins = `FROM responder
			INNER JOIN answer_details ON responder.id = answer_details.responder_id
			INNER JOIN time_slot ON time_slot.id = answer_details.time_slot_id
			INNER JOIN (SELECT user.id AS id, user.first_name AS first_name, user.last_name AS last_name, moderator_time_slot.time_slot_id FROM moderator_time_slot INNER JOIN user ON user.id = moderator_time_slot.moderator_id WHERE moderator_time_slot.is_host AND user.deleted = 0) moderator_info ON moderator_info.time_slot_id = time_slot.id
			INNER JOIN project ON time_slot.project_id = project.id
			LEFT JOIN topics on project.id = topics.project_id and topics.language_id = 1
			INNER JOIN conference_invitation_responder_time_slot ON conference_invitation_responder_time_slot.time_slot_id = time_slot.id
			INNER JOIN conference_invitation ON conference_invitation.id = conference_invitation_responder_time_slot.conference_invitation_id
			INNER JOIN external_client ON project.project_external_client = external_client.id
			INNER JOIN salesforce_account on salesforce_account.salesforce_account_id = external_client.external_client_account_id
			LEFT JOIN participant_eligibility_status pes ON pes.participant_id = responder.external_responder_id AND pes.is_eligible = 0
			LEFT JOIN (select sum(tsph.amount) as totalAmount, max(tsph.payment_date) paymentDate, max(tsph.source) as source, max(tsph.currency) as currency, time_slot_id from time_slot_payment_history tsph where tsph.payment_status='` + completedPaymentStatus + `' group by tsph.time_slot_id) payments on payments.time_slot_id=time_slot.id
			LEFT JOIN (SELECT MAX(ha.honorarium) AS amount, MAX(ha.currency) AS currency, ts.id AS timeSlotId FROM time_slot ts JOIN time_slot_event tse ON tse.time_slot_id = ts.id JOIN responder r ON r.id = tse.responder_id JOIN honorarium_amount ha ON ha.sessKey = r.sess_key GROUP BY ts.id) defaultHonorarium ON defaultHonorarium.timeSlotId = time_slot.id
			LEFT JOIN time_slot_custom_honorarium customHonorarium on customHonorarium.time_slot_id = time_slot.id`

		whereClause = fmt.Sprintf(`WHERE project.client_id = ? AND time_slot.status_id IN (2, 7, 8, 9) AND time_slot.is_invalidated_interview = 0 AND time_slot.start_time < now() AND external_client.external_client_account_id IN %s%s AND project.id NOT IN %s%s`,
			externalClientsIn, searchCondition, projectsNotIn, paymentCondition)
		orderBy = `ORDER BY time_slot.start_time desc`
	}

	q := fmt.Sprintf("SELECT %s %s %s %s LIMIT %d, 25", selectCols, fromJoins, whereClause, orderBy, offset)

	rows, err := r.db.QueryContext(ctx, q, clientID)
	if err != nil {
		return nil, fmt.Errorf("get all interviews: %w", err)
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("get columns: %w", err)
	}

	var results []map[string]any
	for rows.Next() {
		values := make([]any, len(columns))
		valuePtrs := make([]any, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}
		if err := rows.Scan(valuePtrs...); err != nil {
			return nil, fmt.Errorf("scan interview row: %w", err)
		}
		record := make(map[string]any, len(columns))
		for i, col := range columns {
			val := values[i]
			if b, ok := val.([]byte); ok {
				record[col] = string(b)
			} else {
				record[col] = val
			}
		}
		results = append(results, record)
	}
	return results, rows.Err()
}
