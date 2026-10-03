// Package config loads and validates service settings from environment variables.
package config

import (
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"

	"starter/services/gateway/internal/logging"
)

const minimumServiceTokenLength = 32

const minimumSigningKeyLength = 32

// Kept below the HTTP write timeout so a slow readiness ping can still answer.
const (
	minimumDatabaseTimeoutMilliseconds = 100
	maximumDatabaseTimeoutMilliseconds = 20000
)

// Config holds every runtime setting of the gateway.
type Config struct {
	Host         string
	Port         int
	ServiceToken logging.Secret
	// OperatorContextSigningKey verifies the X-Operator-Context token NestJS signs (decision 4).
	OperatorContextSigningKey logging.Secret
	Postgres                  Postgres
	DatabaseTimeout           time.Duration
	LogLevel                  slog.Level
}

// Postgres holds the discrete connection settings shared with the API.
type Postgres struct {
	Host     string
	Port     int
	User     string
	Password logging.Secret
	Database string
}

// ValidationError lists every configuration problem at once.
// Problems name variables only, never their values.
type ValidationError struct {
	Problems []string
}

func (validationError *ValidationError) Error() string {
	return "invalid configuration: " + strings.Join(validationError.Problems, "; ")
}

// LookupFunc matches os.LookupEnv so tests can supply a map.
type LookupFunc func(name string) (string, bool)

// Load reads the process environment.
func Load() (Config, error) {
	return LoadFrom(os.LookupEnv)
}

// LoadFrom validates all variables and reports all problems together.
func LoadFrom(lookup LookupFunc) (Config, error) {
	reader := environmentReader{lookup: lookup}

	loadedConfig := Config{
		Host: reader.stringOr("GATEWAY_HOST", "127.0.0.1"),
		Port: reader.port("GATEWAY_PORT", 8080),
		Postgres: Postgres{
			Host: reader.stringOr("POSTGRES_HOST", "localhost"),
			Port: reader.port("POSTGRES_PORT", 5432),
			// The gateway's own role (X-35, CreateServiceRoles); the bootstrap user that runs the
			// migrations is not the gateway's user (GO-38). Fail closed without its password.
			User:     GatewayDatabaseRole,
			Password: logging.NewSecret(reader.requiredWithFix("POSTGRES_GATEWAY_PASSWORD", gatewayRolePasswordFix)),
			Database: reader.required("POSTGRES_DB"),
		},
		DatabaseTimeout: reader.milliseconds("DATABASE_TIMEOUT_MS", 3000),
		LogLevel:        reader.logLevel("LOG_LEVEL", "info"),
	}

	serviceToken := reader.required("GATEWAY_SERVICE_TOKEN")
	if serviceToken != "" && len(serviceToken) < minimumServiceTokenLength {
		reader.addProblem(fmt.Sprintf("GATEWAY_SERVICE_TOKEN must be at least %d characters", minimumServiceTokenLength))
	}
	// HTTP strips surrounding whitespace, so no caller could present such a token.
	if serviceToken != strings.TrimSpace(serviceToken) {
		reader.addProblem("GATEWAY_SERVICE_TOKEN must not start or end with whitespace")
	}
	loadedConfig.ServiceToken = logging.NewSecret(serviceToken)

	// Same rule as the API, which signs with this key.
	signingKey := reader.required("OPERATOR_CONTEXT_SIGNING_KEY")
	if signingKey != "" && len(signingKey) < minimumSigningKeyLength {
		reader.addProblem(fmt.Sprintf("OPERATOR_CONTEXT_SIGNING_KEY must be at least %d characters", minimumSigningKeyLength))
	}
	loadedConfig.OperatorContextSigningKey = logging.NewSecret(signingKey)

	if len(reader.problems) > 0 {
		return Config{}, &ValidationError{Problems: reader.problems}
	}
	return loadedConfig, nil
}

// environmentReader collects problems instead of stopping at the first one.
type environmentReader struct {
	lookup   LookupFunc
	problems []string
}

func (reader *environmentReader) addProblem(problem string) {
	reader.problems = append(reader.problems, problem)
}

// stringOr treats an empty variable like an unset one.
func (reader *environmentReader) stringOr(name, fallback string) string {
	if value, isSet := reader.lookup(name); isSet && strings.TrimSpace(value) != "" {
		return strings.TrimSpace(value)
	}
	return fallback
}

// GatewayDatabaseRole is the database role the gateway connects as. Its name is fixed by the
// CreateServiceRoles migration and is never configurable.
const GatewayDatabaseRole = "task_passport_gateway"

// gatewayRolePasswordFix names the steps that give the gateway role its login password.
const gatewayRolePasswordFix = "run `pnpm run setup`, then `pnpm db:migration:run`, then `pnpm db:roles`"

// requiredWithFix is required with the steps that fix a missing value in its problem text.
func (reader *environmentReader) requiredWithFix(name, fix string) string {
	value, isSet := reader.lookup(name)
	if !isSet || strings.TrimSpace(value) == "" {
		reader.addProblem(name + " is required: " + fix)
		return ""
	}
	return value
}

// required rejects blank values but returns the value untrimmed:
// a password may legitimately start or end with a space.
func (reader *environmentReader) required(name string) string {
	value, isSet := reader.lookup(name)
	if !isSet || strings.TrimSpace(value) == "" {
		reader.addProblem(name + " is required")
		return ""
	}
	return value
}

func (reader *environmentReader) port(name string, fallback int) int {
	port, err := strconv.Atoi(reader.stringOr(name, strconv.Itoa(fallback)))
	if err != nil || port < 1 || port > 65535 {
		reader.addProblem(name + " must be a port number between 1 and 65535")
		return fallback
	}
	return port
}

func (reader *environmentReader) milliseconds(name string, fallback int) time.Duration {
	milliseconds, err := strconv.Atoi(reader.stringOr(name, strconv.Itoa(fallback)))
	if err != nil || milliseconds < minimumDatabaseTimeoutMilliseconds || milliseconds > maximumDatabaseTimeoutMilliseconds {
		reader.addProblem(fmt.Sprintf("%s must be a whole number of milliseconds between %d and %d",
			name, minimumDatabaseTimeoutMilliseconds, maximumDatabaseTimeoutMilliseconds))
		return time.Duration(fallback) * time.Millisecond
	}
	return time.Duration(milliseconds) * time.Millisecond
}

func (reader *environmentReader) logLevel(name, fallback string) slog.Level {
	level, err := logging.ParseLevel(reader.stringOr(name, fallback))
	if err != nil {
		reader.addProblem(name + " must be one of debug, info, warn, error")
	}
	return level
}
