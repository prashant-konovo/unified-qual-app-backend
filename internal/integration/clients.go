package integration

import (
	"net/http"
	"time"

	"github.com/InCrowd/unified-qual-api/internal/config"
)

// ══════════════════════════════════════════════════════════════════
// ServiceClients — aggregates all external integration HTTP clients.
// Matches the external dependencies of InCrowdAPI (LS) and QS-Tool (MRA).
// ══════════════════════════════════════════════════════════════════

type ServiceClients struct {
	Conference   *ConferenceClient
	Notification *NotificationClient
	GoogleCal    *GoogleCalendarClient
	Decipher     *DecipherClient
	CastingWords *CastingWordsClient
	Stripe       *StripeClient
	Tango        *TangoClient
	PayPal       *PayPalClient
	SMS          *SMSClient
	EventLog     *EventLogClient
	GoogleSheets *GoogleSheetsClient
	S3           *S3Client
	Lambda       *LambdaClient
	StepFn       *StepFunctionsClient
	ICAuth       *InCrowdAPIAuthClient
}

func NewServiceClients(cfg *config.Config) *ServiceClients {
	httpClient := &http.Client{Timeout: 30 * time.Second}

	return &ServiceClients{
		Conference:   newConferenceClient(cfg.ConferenceService, httpClient),
		Notification: newNotificationClient(cfg.NotificationService, httpClient),
		GoogleCal:    newGoogleCalendarClient(cfg.GoogleCalendar, httpClient),
		Decipher:     newDecipherClient(cfg.Decipher, httpClient),
		CastingWords: newCastingWordsClient(cfg.CastingWords, httpClient),
		Stripe:       newStripeClient(cfg.Stripe, httpClient),
		Tango:        newTangoClient(cfg.Tango, httpClient),
		PayPal:       newPayPalClient(cfg.PayPal, httpClient),
		SMS:          newSMSClient(cfg.SMS, httpClient),
		EventLog:     newEventLogClient(cfg.EventLog, httpClient),
		GoogleSheets: newGoogleSheetsClient(cfg.GoogleSheets, cfg.GoogleCalendar.ServiceAccountKeyPath, httpClient),
		S3:           newS3Client(cfg.S3),
		Lambda:       newLambdaClient(cfg.AWS),
		StepFn:       newStepFunctionsClient(cfg.AWS),
		ICAuth:       newInCrowdAPIAuthClient(cfg.AWS.ICApiURL),
	}
}
