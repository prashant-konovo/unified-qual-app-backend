package dto

import "encoding/json"

// LsCreatePaymentRequest is the request body for creating a payment record (LS brand).
type LsCreatePaymentRequest struct {
	TimeSlotID  int64  `json:"timeSlotId"  validate:"required,gt=0"`
	Amount      int    `json:"amount"      validate:"required"`
	PaymentType string `json:"paymentType" validate:"required"`
}

// LsCreateCustomHonorariumRequest is the request body for creating a custom honorarium (LS brand).
type LsCreateCustomHonorariumRequest struct {
	TimeSlotID int64  `json:"timeSlotId" validate:"required,gt=0"`
	Amount     int    `json:"amount"     validate:"required"`
	Reason     string `json:"reason"     validate:"required"`
}

// LsCreateExternalPaymentRequest is the request body for creating an external payment (LS brand).
type LsCreateExternalPaymentRequest struct {
	TimeSlotID  int64  `json:"timeSlotId"        validate:"required,gt=0"`
	Amount      int    `json:"amount"             validate:"required"`
	PaymentType string `json:"paymentType"        validate:"required"`
	ExternalRef string `json:"externalReference"`
}

// MraAddHonorariumAmountRequest is the request body for adding an honorarium amount (MRA).
type MraAddHonorariumAmountRequest struct {
	ProjectID             json.Number `json:"projectId"`
	Honorarium            json.Number `json:"honorarium"`
	Currency              string      `json:"currency"`
	SessKey               string      `json:"sessKey"`
	ExternalProjectID     string      `json:"externalProjectId"`
	ExternalUserSurveyID  string      `json:"externalUserSurveyId"`
	ExternalUserID        string      `json:"externalUserId"`
	ExternalCreditOrderID string      `json:"externalCreditOrderId"`
	ExternalCountryID     string      `json:"externalCountryId"`
}

// MraAddTimeSlotPaymentsRequest is the request body for bulk time-slot payment creation (MRA).
type MraAddTimeSlotPaymentsRequest struct {
	TimeSlotIDs []int64 `json:"timeSlotIds"`
}

// MraExternalTimeSlotPaymentItem represents a single external time-slot payment entry (MRA).
type MraExternalTimeSlotPaymentItem struct {
	Amount                json.Number `json:"amount"`
	Currency              string      `json:"currency"`
	Source                string      `json:"source"`
	PaymentDate           string      `json:"paymentDate"`
	PaymentUserID         json.Number `json:"paymentUserId"`
	ExternalUserSurveyID  string      `json:"externalUserSurveyId"`
	ExternalCreditOrderID string      `json:"externalCreditOrderId"`
	PaymentTypeCode       string      `json:"paymentTypeCode"`
}

// MraAddTimeSlotCustomHonorariumRequest is the request body for adding a custom honorarium (MRA).
type MraAddTimeSlotCustomHonorariumRequest struct {
	TimeSlotID int64   `json:"timeSlotId"`
	OldValue   float64 `json:"oldValue"`
	NewValue   float64 `json:"newValue"`
	ReasonCode string  `json:"reasonCode"`
}

// ── Inquiry Preview types (contract-identical with legacy Scala InCrowdAPI) ──

// IPCrowdAttributeSpec describes an attribute filter for a crowd in an inquiry preview.
type IPCrowdAttributeSpec struct {
	AttributeID  int64   `json:"attributeId"`
	NumericMin   *int64  `json:"numericMin"`
	NumericMax   *int64  `json:"numericMax"`
	ChoiceIDs    []int64 `json:"choiceIds"`
	QualRequired *bool   `json:"qualRequired"`
}

// IPDifficultyLevelReq is the difficulty level reference in an inquiry preview.
type IPDifficultyLevelReq struct {
	ID int64 `json:"id"`
}

// IPDifficultyAssessmentResp is the assessed difficulty returned in an inquiry preview.
type IPDifficultyAssessmentResp struct {
	ID         int64  `json:"id"`
	Name       string `json:"name"`
	IsHardStop bool   `json:"isHardStop"`
}

// IPCrowdSpec describes a crowd in an inquiry preview request.
type IPCrowdSpec struct {
	Name                 *string                     `json:"name"`
	NumberRequested      *int64                      `json:"numberRequested"`
	Notes                *string                     `json:"notes"`
	Attributes           []IPCrowdAttributeSpec      `json:"attributes"`
	MarketID             int64                       `json:"marketId"`
	MarketName           *string                     `json:"marketName"`
	DifficultyLevel      IPDifficultyLevelReq        `json:"difficultyLevel"`
	DifficultyAssessment *IPDifficultyAssessmentResp `json:"difficultyAssessment"`
	CrowdID              *int64                      `json:"crowdId"`
	IsCustom             bool                        `json:"isCustom"`
	ValidRespondersCount *int64                      `json:"validRespondersCount"`
}

// IPProposal is the top-level inquiry preview request body.
type IPProposal struct {
	InterviewLength      int64         `json:"interviewLength"`
	Name                 string        `json:"name"`
	SalesforceProjectID  *string       `json:"salesforceProjectId"`
	CompletionDate       *string       `json:"completionDate"`
	Notes                *string       `json:"notes"`
	Crowds               []IPCrowdSpec `json:"crowds"`
	CustomCrowds         []IPCrowdSpec `json:"customCrowds"`
	ProjectID            *int64        `json:"projectId"`
	UnderReview          bool          `json:"underReview"`
	TranscriptsRequested bool          `json:"transcriptsRequested"`
	RequiresStimuli      bool          `json:"requiresStimuli"`
	IsDynamicStimulus    bool          `json:"isDynamicStimulus"`
}

// IPFee is a single fee line item in an inquiry preview response.
type IPFee struct {
	GrossSubtotal float64  `json:"grossSubtotal"`
	NetSubtotal   float64  `json:"netSubtotal"`
	DiscountRate  *float64 `json:"discountRate"`
	PricePerUnit  float64  `json:"pricePerUnit"`
	Count         int64    `json:"count"`
	IsHonorarium  bool     `json:"isHonorarium"`
	Name          string   `json:"name"`
	ProductID     int64    `json:"productId"`
}

// IPProjectCosts is the cost summary in an inquiry preview response.
type IPProjectCosts struct {
	GrossTotal float64 `json:"grossTotal"`
	NetTotal   float64 `json:"netTotal"`
	Fees       []IPFee `json:"fees"`
}

// IPResponse is the full inquiry preview response.
type IPResponse struct {
	Proposal              *IPProposal     `json:"proposal"`
	SalesforceProjectName *string         `json:"salesforceProjectName"`
	Costs                 *IPProjectCosts `json:"costs"`
	IsHardStop            bool            `json:"isHardStop"`
}

// IPIsSpecializedCrowd checks whether a crowd is "specialized" per legacy logic.
func IPIsSpecializedCrowd(c *IPCrowdSpec) bool {
	if c.MarketID != 1 {
		return false
	}
	if c.IsCustom {
		return true
	}
	nonGeneralChoices := map[int64]bool{304: true}
	for _, attr := range c.Attributes {
		if attr.AttributeID == 1 {
			for _, cid := range attr.ChoiceIDs {
				if !nonGeneralChoices[cid] {
					return true
				}
			}
		}
	}
	return false
}

// IPCrowdMatchesProduct determines if a crowd matches a product for honorarium calculations.
func IPCrowdMatchesProduct(c *IPCrowdSpec, relatedMarketIDs []int64, isSpecialized bool) bool {
	for _, mid := range relatedMarketIDs {
		if mid == c.MarketID {
			return true
		}
	}
	crowdSpecialized := IPIsSpecializedCrowd(c)
	if isSpecialized && crowdSpecialized {
		return true
	}
	if !isSpecialized && !crowdSpecialized && c.MarketID == 1 {
		return true
	}
	return false
}
