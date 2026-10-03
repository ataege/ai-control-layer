package config

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestAccountingCatalogDefaultsAndTrustedOverrides(t *testing.T) {
	catalog := `{"schema_version":1,"allowed_models":["custom:changed"],"budgets":{"tokens_total":10000,"request_timeout_seconds":45},"policy_other":{"future":"allowed"}}`
	result, err := LoadAccountingCatalog([]byte(catalog))
	if err != nil || result.TokensTotal != 10000 || result.AgentOutputTokens != 512 || result.SecurityOutputTokens != 256 || result.TemplateTokens != 1024 || result.RequestTimeout != 45*time.Second || result.AllowedModels[0] != "custom:changed" {
		t.Fatalf("defaults missing: %+v %v", result, err)
	}
	catalog = strings.Replace(catalog, `"request_timeout_seconds":45`, `"request_timeout_seconds":60,"agent_output_tokens":128,"security_output_tokens":64,"input_template_tokens":800`, 1)
	result, err = LoadAccountingCatalog([]byte(catalog))
	if err != nil || result.AgentOutputTokens != 128 || result.SecurityOutputTokens != 64 || result.TemplateTokens != 800 || result.RequestTimeout != time.Minute {
		t.Fatalf("overrides ignored: %+v %v", result, err)
	}
}

func TestAccountingCatalogRejectsAmbiguousOrInvalidValues(t *testing.T) {
	valid := `{"schema_version":1,"allowed_models":["fixture:tag"],"budgets":{"tokens_total":10000,"request_timeout_seconds":45}}`
	cases := []string{
		`{}`, `null`, `[]`, valid + `{}`,
		strings.Replace(valid, `"schema_version":1`, `"schema_version":2`, 1),
		strings.Replace(valid, `"schema_version":1`, `"schema_version":1,"Schema_Version":1`, 1),
		strings.Replace(valid, `"tokens_total":10000`, `"tokens_total":10000,"tokens_total":1`, 1),
		strings.Replace(valid, `"tokens_total":10000`, `"Tokens_Total":10000`, 1),
		strings.Replace(valid, `"tokens_total":10000`, `"tokens_total":null`, 1),
		strings.Replace(valid, `"tokens_total":10000`, `"tokens_total":1.5`, 1),
		strings.Replace(valid, `"tokens_total":10000`, `"tokens_total":-1`, 1),
		strings.Replace(valid, `"tokens_total":10000`, `"tokens_total":9007199254740992`, 1),
		strings.Replace(valid, `"tokens_total":10000,`, ``, 1),
		strings.Replace(valid, `"request_timeout_seconds":45`, `"request_timeout_seconds":86401`, 1),
		strings.Replace(valid, `"request_timeout_seconds":45`, `"request_timeout_seconds":0`, 1),
		strings.Replace(valid, `"request_timeout_seconds":45`, `"request_timeout_seconds":45,"agent_output_tokens":null`, 1),
		strings.Replace(valid, `"request_timeout_seconds":45`, `"request_timeout_seconds":45,"security_output_tokens":9007199254740992`, 1),
		strings.Replace(valid, `"request_timeout_seconds":45`, `"request_timeout_seconds":45,"input_template_tokens":0`, 1),
		strings.Replace(valid, `["fixture:tag"]`, `[]`, 1),
		strings.Replace(valid, `["fixture:tag"]`, `null`, 1),
		strings.Replace(valid, `["fixture:tag"]`, `["fixture:tag","fixture:tag"]`, 1),
		strings.Replace(valid, `["fixture:tag"]`, `["private-secret\nvalue"]`, 1),
		strings.Replace(valid, `"budgets":{`, `"budgets":null,"extra":{`, 1),
		strings.Replace(valid, `"schema_version":1`, `"unknown":{"nested":1,"nested":2},"schema_version":1`, 1),
	}
	for index, catalog := range cases {
		_, err := LoadAccountingCatalog([]byte(catalog))
		var validationError *ValidationError
		if !errors.As(err, &validationError) {
			t.Fatalf("case %d accepted", index)
		}
		if strings.Contains(err.Error(), "private-secret") || strings.Contains(err.Error(), "fixture:tag") {
			t.Fatal("error leaked supplied values")
		}
	}
}
