package config

import (
	"errors"
	"strings"
	"testing"
)

const validServiceToken = "config-test-token-0123456789abcdefghij"

const validSigningKey = "config-test-signing-key-0123456789abcdef"

func lookupFromMap(environment map[string]string) LookupFunc {
	return func(name string) (string, bool) {
		value, isSet := environment[name]
		return value, isSet
	}
}

func validEnvironment() map[string]string {
	return map[string]string{
		"GATEWAY_SERVICE_TOKEN":        validServiceToken,
		"OPERATOR_CONTEXT_SIGNING_KEY": validSigningKey,
		"POSTGRES_USER":                "starter",
		"POSTGRES_PASSWORD":            "database-password-value",
		"POSTGRES_DB":                  "starter",
	}
}

func TestLoadFromAcceptsMinimalEnvironment(t *testing.T) {
	// Only the required variables are set; optional ones must not block startup.
	loadedConfig, err := LoadFrom(lookupFromMap(validEnvironment()))
	if err != nil {
		t.Fatalf("LoadFrom: %v", err)
	}
	if loadedConfig.ServiceToken.Reveal() != validServiceToken {
		t.Error("service token was not loaded")
	}
	if loadedConfig.Postgres.Password.Reveal() != "database-password-value" {
		t.Error("database password was not loaded")
	}
	if loadedConfig.OperatorContextSigningKey.Reveal() != validSigningKey {
		t.Error("operator context signing key was not loaded")
	}
}

func TestLoadFromRejectsMissingOrShortSigningKey(t *testing.T) {
	for name, signingKey := range map[string]*string{"unset": nil, "31 characters": pointerTo(strings.Repeat("k", 31))} {
		environment := validEnvironment()
		delete(environment, "OPERATOR_CONTEXT_SIGNING_KEY")
		if signingKey != nil {
			environment["OPERATOR_CONTEXT_SIGNING_KEY"] = *signingKey
		}
		_, err := LoadFrom(lookupFromMap(environment))
		if err == nil || !strings.Contains(err.Error(), "OPERATOR_CONTEXT_SIGNING_KEY") {
			t.Errorf("%s: error = %v", name, err)
		}
		if signingKey != nil && strings.Contains(err.Error(), *signingKey) {
			t.Errorf("%s: error echoes the key", name)
		}
	}
}

func pointerTo(value string) *string { return &value }

func TestLoadFromReportsEveryProblemWithoutValues(t *testing.T) {
	const shortToken = "short-token-value"
	const badPort = "port-9x9x9"
	const badTimeout = "timeout-7z7z7"
	const badLevel = "level-loudest"

	environment := map[string]string{
		"GATEWAY_SERVICE_TOKEN": shortToken,
		"GATEWAY_PORT":          badPort,
		"POSTGRES_PORT":         "70000",
		"POSTGRES_USER":         "starter",
		"POSTGRES_DB":           "starter",
		"DATABASE_TIMEOUT_MS":   badTimeout,
		"LOG_LEVEL":             badLevel,
	}
	_, err := LoadFrom(lookupFromMap(environment))

	var validationError *ValidationError
	if !errors.As(err, &validationError) {
		t.Fatalf("error = %v, want *ValidationError", err)
	}
	errorText := err.Error()

	expectedVariables := []string{
		"GATEWAY_SERVICE_TOKEN", "OPERATOR_CONTEXT_SIGNING_KEY", "GATEWAY_PORT", "POSTGRES_PORT",
		"POSTGRES_PASSWORD", "DATABASE_TIMEOUT_MS", "LOG_LEVEL",
	}
	for _, variableName := range expectedVariables {
		if !strings.Contains(errorText, variableName) {
			t.Errorf("error does not name %s: %s", variableName, errorText)
		}
	}
	if len(validationError.Problems) != len(expectedVariables) {
		t.Errorf("problems = %d, want %d: %v", len(validationError.Problems), len(expectedVariables), validationError.Problems)
	}
	for _, rejectedValue := range []string{shortToken, badPort, badTimeout, badLevel, "70000"} {
		if strings.Contains(errorText, rejectedValue) {
			t.Errorf("error echoes a rejected value (%q): %s", rejectedValue, errorText)
		}
	}
}

func TestLoadFromRequiresServiceToken(t *testing.T) {
	tests := []struct {
		name        string
		token       string
		isSet       bool
		wantProblem string
	}{
		{"unset", "", false, "GATEWAY_SERVICE_TOKEN is required"},
		{"empty", "", true, "GATEWAY_SERVICE_TOKEN is required"},
		{"31 characters", strings.Repeat("a", 31), true, "GATEWAY_SERVICE_TOKEN must be at least 32 characters"},
		{"32 characters", strings.Repeat("a", 32), true, ""},
		{"only spaces", strings.Repeat(" ", 34), true, "GATEWAY_SERVICE_TOKEN is required"},
		{"padded with spaces", " " + strings.Repeat("a", 32) + " ", true, "GATEWAY_SERVICE_TOKEN must not start or end with whitespace"},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			environment := validEnvironment()
			delete(environment, "GATEWAY_SERVICE_TOKEN")
			if testCase.isSet {
				environment["GATEWAY_SERVICE_TOKEN"] = testCase.token
			}
			_, err := LoadFrom(lookupFromMap(environment))

			if testCase.wantProblem == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), testCase.wantProblem) {
				t.Fatalf("error = %v, want it to contain %q", err, testCase.wantProblem)
			}
		})
	}
}

func TestLoadFromRejectsBlankRequiredVariables(t *testing.T) {
	for _, variableName := range []string{"POSTGRES_USER", "POSTGRES_PASSWORD", "POSTGRES_DB"} {
		t.Run(variableName, func(t *testing.T) {
			environment := validEnvironment()
			environment[variableName] = "   "
			_, err := LoadFrom(lookupFromMap(environment))

			if err == nil || !strings.Contains(err.Error(), variableName+" is required") {
				t.Fatalf("error = %v, want %s to be required", err, variableName)
			}
		})
	}
}

func TestLoadFromBoundsDatabaseTimeout(t *testing.T) {
	// The range keeps a readiness ping inside the HTTP write timeout.
	tests := []struct {
		timeoutValue string
		isAccepted   bool
	}{
		{"99", false},
		{"100", true},
		{"20000", true},
		{"20001", false},
	}
	for _, testCase := range tests {
		t.Run(testCase.timeoutValue, func(t *testing.T) {
			environment := validEnvironment()
			environment["DATABASE_TIMEOUT_MS"] = testCase.timeoutValue
			_, err := LoadFrom(lookupFromMap(environment))

			if testCase.isAccepted && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !testCase.isAccepted && (err == nil || !strings.Contains(err.Error(), "DATABASE_TIMEOUT_MS must be")) {
				t.Fatalf("error = %v, want a DATABASE_TIMEOUT_MS range problem", err)
			}
		})
	}
}
