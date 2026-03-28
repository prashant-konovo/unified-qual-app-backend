package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config holds all application configuration loaded from environment variables.
type Config struct {
	Port        string
	Environment string
	LogLevel    string

	// IRIS (InCrowd) database
	IRISDB DatabaseConfig

	// QS-Tool database
	QSDB DatabaseConfig

	// AWS Cognito
	Cognito CognitoConfig

	// CORS
	CORSOrigins string
}

type DatabaseConfig struct {
	Host            string
	Port            int
	Name            string
	User            string
	Password        string
	SSLMode         string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
}

func (d DatabaseConfig) DSN() string {
	return fmt.Sprintf(
		"host=%s port=%d dbname=%s user=%s password=%s sslmode=%s",
		d.Host, d.Port, d.Name, d.User, d.Password, d.SSLMode,
	)
}

type CognitoConfig struct {
	Region       string
	UserPoolID   string
	AppClientID  string
}

// Load reads configuration from environment variables with sensible defaults.
func Load() *Config {
	return &Config{
		Port:        envOr("PORT", "8080"),
		Environment: envOr("ENVIRONMENT", "local"),
		LogLevel:    envOr("LOG_LEVEL", "info"),
		CORSOrigins: envOr("CORS_ORIGINS", "*"),

		IRISDB: DatabaseConfig{
			Host:            envOr("IRIS_DB_HOST", "localhost"),
			Port:            envIntOr("IRIS_DB_PORT", 5432),
			Name:            envOr("IRIS_DB_NAME", "incrowd"),
			User:            envOr("IRIS_DB_USER", "incrowd"),
			Password:        envOr("IRIS_DB_PASSWORD", ""),
			SSLMode:         envOr("IRIS_DB_SSLMODE", "disable"),
			MaxOpenConns:    envIntOr("IRIS_DB_MAX_OPEN", 25),
			MaxIdleConns:    envIntOr("IRIS_DB_MAX_IDLE", 5),
			ConnMaxLifetime: time.Duration(envIntOr("IRIS_DB_CONN_MAX_LIFE_MIN", 5)) * time.Minute,
		},

		QSDB: DatabaseConfig{
			Host:            envOr("QS_DB_HOST", "localhost"),
			Port:            envIntOr("QS_DB_PORT", 5432),
			Name:            envOr("QS_DB_NAME", "qstool"),
			User:            envOr("QS_DB_USER", "qstool"),
			Password:        envOr("QS_DB_PASSWORD", ""),
			SSLMode:         envOr("QS_DB_SSLMODE", "disable"),
			MaxOpenConns:    envIntOr("QS_DB_MAX_OPEN", 25),
			MaxIdleConns:    envIntOr("QS_DB_MAX_IDLE", 5),
			ConnMaxLifetime: time.Duration(envIntOr("QS_DB_CONN_MAX_LIFE_MIN", 5)) * time.Minute,
		},

		Cognito: CognitoConfig{
			Region:      envOr("AWS_REGION", "us-east-2"),
			UserPoolID:  envOr("COGNITO_USER_POOL_ID", ""),
			AppClientID: envOr("COGNITO_APP_CLIENT_ID", ""),
		},
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
