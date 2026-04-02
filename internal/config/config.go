package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds all application configuration loaded from environment variables.
type Config struct {
	Port        string
	Environment string
	LogLevel    string

	// IRIS (InCrowd) MySQL database — legacy InCrowdAPI
	IRISDB         DatabaseConfig
	IRISReadOnlyDB DatabaseConfig

	// QS-Tool MySQL database — legacy qual-scheduler
	QSDB DatabaseConfig

	// DocumentDB (MongoDB-compatible) — legacy user/answer store
	DocumentDB DocumentDBConfig

	// AWS Cognito
	Cognito CognitoConfig

	// Central auth service (API Gateway wrapping Cognito)
	AuthAPIURL string
	AuthAPIKey string

	// Conference Service (Chime proxy — API Gateway + Lambda)
	ConferenceService ConferenceServiceConfig

	// Notification Service (SES-backed email API)
	NotificationService NotificationServiceConfig

	// Google Calendar integration (service account)
	GoogleCalendar GoogleCalendarConfig

	// Decipher survey platform
	Decipher DecipherConfig

	// CastingWords transcription
	CastingWords CastingWordsConfig

	// Payment gateways
	Stripe  StripeConfig
	Tango   TangoConfig
	PayPal  PayPalConfig

	// SMS (Bandwidth)
	SMS SMSConfig

	// Event logging service
	EventLog EventLogConfig

	// Google Sheets integration
	GoogleSheets GoogleSheetsConfig

	// S3 buckets
	S3 S3Config

	// AWS Lambda / Step Functions
	AWS AWSConfig

	// Inquiry email recipient
	InquiryEmailRecipient string

	// CORS
	CORSOrigins string
}

type DatabaseConfig struct {
	Host            string
	Port            int
	Name            string
	User            string
	Password        string
	Params          string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
}

// DSN returns a MySQL connection string.
func (d DatabaseConfig) DSN() string {
	return fmt.Sprintf(
		"%s:%s@tcp(%s:%d)/%s?%s",
		d.User, d.Password, d.Host, d.Port, d.Name, d.Params,
	)
}

type DocumentDBConfig struct {
	ConnectionString   string
	ConnectionStringV2 string
}

type CognitoConfig struct {
	Region         string
	UserPoolID     string
	AppClientID    string   // Primary client used for login (USER_PASSWORD_AUTH)
	AllClientIDs   []string // All recognised client IDs for JWT audience validation
	SSOClientID    string   // OAuth client for SSO (authorization code flow)
	SSOClientSecret string  // Secret for the SSO OAuth client
	Domain         string   // Cognito hosted UI domain (full hostname, e.g. "admin-dev-auth.incrowdanswers.com")
	SSORedirectURI string   // Callback URL for SSO code exchange
}

type ConferenceServiceConfig struct {
	BaseURL string // API Gateway URL, e.g. "https://xxx.execute-api.us-east-1.amazonaws.com/prod"
	APIKey  string // x-api-key header value
}

type NotificationServiceConfig struct {
	BaseURL    string // Notification service API Gateway URL
	APIKey     string // x-api-key header value
	BearerToken string // Static bearer token for service-to-service auth
}

type GoogleCalendarConfig struct {
	ServiceAccountKeyPath string // Path to service account JSON key file
	DefaultCalendarID     string
}

type DecipherConfig struct {
	BaseURL string // e.g. "https://surveys.opinionsite.com/api/v1"
	APIKey  string
}

type CastingWordsConfig struct {
	BaseURL string // e.g. "https://castingwords.com/store/API4"
	APIKey  string
}

type StripeConfig struct {
	SecretKey string
}

type TangoConfig struct {
	BaseURL      string
	PlatformName string
	PlatformKey  string
}

type PayPalConfig struct {
	BaseURL      string // e.g. "https://api-m.paypal.com" or sandbox
	ClientID     string
	ClientSecret string
}

type SMSConfig struct {
	BaseURL       string // Bandwidth messaging API
	APIToken      string
	AccountID     string
	ApplicationID string
}

type EventLogConfig struct {
	BaseURL string
	APIKey  string
}

type GoogleSheetsConfig struct {
	SpreadsheetID string
	// Uses same service account key as GoogleCalendar
}

type S3Config struct {
	Region          string
	RecordingBucket string
	ExportBucket    string
	InquiryBucket   string
}

type AWSConfig struct {
	Region      string
	Account     string
	Environment string // stackPrefix: "prd", "data-qa", etc.
	ICApiURL    string // e.g. https://api.incrowdnow.com
}

// Load reads configuration from environment variables with sensible defaults.
// Env var names align with legacy InCrowdAPI conventions (DB_HOST, DB_PORT, etc.)
func Load() *Config {
	mysqlParams := envOr("DB_PARAMS", "parseTime=true&charset=utf8mb4&collation=utf8mb4_general_ci&loc=UTC&timeout=5s")

	return &Config{
		Port:        envOr("PORT", "8080"),
		Environment: envOr("ENVIRONMENT", "local"),
		LogLevel:    envOr("LOG_LEVEL", "info"),
		CORSOrigins: envOr("CORS_ORIGINS", "*"),

		IRISDB: DatabaseConfig{
			Host:            envOr("DB_HOST", "localhost"),
			Port:            envIntOr("DB_PORT", 3306),
			Name:            envOr("DB_SCHEMA", "incrowdprod"),
			User:            envOr("DB_USER_NAME", "root"),
			Password:        envOr("DB_PASSWORD", ""),
			Params:          mysqlParams,
			MaxOpenConns:    envIntOr("DB_MAX_POOL_SIZE", 25),
			MaxIdleConns:    envIntOr("DB_MIN_IDLE", 5),
			ConnMaxLifetime: time.Duration(envIntOr("DB_CONN_MAX_LIFE_MIN", 10)) * time.Minute,
		},

		IRISReadOnlyDB: DatabaseConfig{
			Host:            envOr("DB_HOST_READ_ONLY", envOr("DB_HOST", "localhost")),
			Port:            envIntOr("DB_PORT", 3306),
			Name:            envOr("DB_SCHEMA", "incrowdprod"),
			User:            envOr("DB_USER_NAME", "root"),
			Password:        envOr("DB_PASSWORD", ""),
			Params:          mysqlParams,
			MaxOpenConns:    envIntOr("DB_MAX_POOL_SIZE", 25),
			MaxIdleConns:    envIntOr("DB_MIN_IDLE", 5),
			ConnMaxLifetime: time.Duration(envIntOr("DB_CONN_MAX_LIFE_MIN", 10)) * time.Minute,
		},

		QSDB: DatabaseConfig{
			Host:            envOr("QS_DB_HOST", "localhost"),
			Port:            envIntOr("QS_DB_PORT", 3306),
			Name:            envOr("QS_DB_NAME", "qstool"),
			User:            envOr("QS_DB_USER", "root"),
			Password:        envOr("QS_DB_PASSWORD", ""),
			Params:          mysqlParams,
			MaxOpenConns:    envIntOr("QS_DB_MAX_OPEN", 25),
			MaxIdleConns:    envIntOr("QS_DB_MAX_IDLE", 5),
			ConnMaxLifetime: time.Duration(envIntOr("QS_DB_CONN_MAX_LIFE_MIN", 10)) * time.Minute,
		},

		DocumentDB: DocumentDBConfig{
			ConnectionString:   envOr("DOCUMENT_DB_CONNECTION_STRING", ""),
			ConnectionStringV2: envOr("DOCUMENT_DB_CONNECTION_STRING_V2", ""),
		},

		Cognito: CognitoConfig{
			Region:          envOr("COGNITO_REGION", "us-east-1"),
			UserPoolID:       envOr("COGNITO_USER_POOL_ID", ""),
			AppClientID:      envOr("COGNITO_APP_CLIENT_ID", ""),
			AllClientIDs:     parseClientIDs(envOr("COGNITO_APP_CLIENT_ID", ""), envOr("COGNITO_APP_CLIENT_IDS", "")),
			SSOClientID:      envOr("COGNITO_SSO_CLIENT_ID", ""),
			SSOClientSecret:  envOr("COGNITO_SSO_CLIENT_SECRET", ""),
			Domain:           envOr("COGNITO_DOMAIN", ""),
			SSORedirectURI:   envOr("COGNITO_SSO_REDIRECT_URI", ""),
		},

		AuthAPIURL: envOr("AUTH_API_URL", ""),
		AuthAPIKey: envOr("AUTH_API_KEY", ""),

		ConferenceService: ConferenceServiceConfig{
			BaseURL: envOr("CONFERENCE_SERVICE_URL", ""),
			APIKey:  envOr("CONFERENCE_SERVICE_API_KEY", ""),
		},

		NotificationService: NotificationServiceConfig{
			BaseURL:     envOr("NOTIFICATION_SERVICE_URL", ""),
			APIKey:      envOr("NOTIFICATION_SERVICE_API_KEY", ""),
			BearerToken: envOr("NOTIFICATION_SERVICE_BEARER_TOKEN", ""),
		},

		GoogleCalendar: GoogleCalendarConfig{
			ServiceAccountKeyPath: envOr("GOOGLE_SERVICE_ACCOUNT_KEY_PATH", ""),
			DefaultCalendarID:     envOr("GOOGLE_CALENDAR_DEFAULT_ID", ""),
		},

		Decipher: DecipherConfig{
			BaseURL: envOr("DECIPHER_API_URL", "https://surveys.opinionsite.com/api/v1"),
			APIKey:  envOr("DECIPHER_API_KEY", ""),
		},

		CastingWords: CastingWordsConfig{
			BaseURL: envOr("CASTINGWORDS_API_URL", "https://castingwords.com/store/API4"),
			APIKey:  envOr("CASTINGWORDS_API_KEY", ""),
		},

		Stripe: StripeConfig{
			SecretKey: envOr("STRIPE_SECRET_KEY", ""),
		},

		Tango: TangoConfig{
			BaseURL:      envOr("TANGO_API_URL", "https://integration-api.tangocard.com/raas/v2"),
			PlatformName: envOr("TANGO_PLATFORM_NAME", ""),
			PlatformKey:  envOr("TANGO_PLATFORM_KEY", ""),
		},

		PayPal: PayPalConfig{
			BaseURL:      envOr("PAYPAL_BASE_URL", "https://api-m.paypal.com"),
			ClientID:     envOr("PAYPAL_CLIENT_ID", ""),
			ClientSecret: envOr("PAYPAL_CLIENT_SECRET", ""),
		},

		SMS: SMSConfig{
			BaseURL:       envOr("BANDWIDTH_API_URL", "https://messaging.bandwidth.com/api/v2"),
			APIToken:      envOr("BANDWIDTH_API_TOKEN", ""),
			AccountID:     envOr("BANDWIDTH_ACCOUNT_ID", ""),
			ApplicationID: envOr("BANDWIDTH_APPLICATION_ID", ""),
		},

		EventLog: EventLogConfig{
			BaseURL: envOr("EVENT_LOG_API_URL", ""),
			APIKey:  envOr("EVENT_LOG_API_KEY", ""),
		},

		GoogleSheets: GoogleSheetsConfig{
			SpreadsheetID: envOr("GOOGLE_SHEETS_SPREADSHEET_ID", ""),
		},

		S3: S3Config{
			Region:          envOr("AWS_REGION", envOr("COGNITO_REGION", "us-east-1")),
			RecordingBucket: envOr("S3_RECORDING_BUCKET", ""),
			ExportBucket:    envOr("S3_EXPORT_BUCKET", ""),
			InquiryBucket:   envOr("S3_INQUIRY_BUCKET", ""),
		},

		AWS: AWSConfig{
			Region:      envOr("AWS_REGION", envOr("COGNITO_REGION", "us-east-1")),
			Account:     envOr("AWS_ACCOUNT_ID", ""),
			Environment: envOr("ENVIRONMENT", "local"),
			ICApiURL:    envOr("IC_API_URL", ""),
		},

		InquiryEmailRecipient: envOr("INQUIRY_EMAIL_RECIPIENT", "dev-ni@incrowdnow.com"),
	}
}

// IsDummy returns true when no real DB credentials are configured,
// signalling the app should fall back to dummy handlers.
func (c *Config) IsDummy() bool {
	return c.IRISDB.Password == "" && c.QSDB.Password == ""
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envIntOr(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}

// parseClientIDs builds a deduplicated list of Cognito app client IDs.
// primary is the main login client; extra is a comma-separated list of additional IDs.
func parseClientIDs(primary, extra string) []string {
	seen := map[string]bool{}
	var ids []string
	add := func(id string) {
		id = strings.TrimSpace(id)
		if id != "" && !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	add(primary)
	for _, id := range strings.Split(extra, ",") {
		add(id)
	}
	return ids
}
