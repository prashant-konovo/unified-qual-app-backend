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
