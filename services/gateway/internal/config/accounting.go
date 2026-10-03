package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

// ModelAccounting projects trusted, approved catalog accounting fields. It does
// not validate the complete policy or activate another configuration authority.
type ModelAccounting struct {
	TokensTotal          int64
	AgentOutputTokens    int
	SecurityOutputTokens int
	TemplateTokens       int64
	RequestTimeout       time.Duration
	AllowedModels        []string
}

const maximumSafeCatalogInteger int64 = 9007199254740991

var catalogModelTag = regexp.MustCompile(`^[a-z0-9][a-z0-9_.:-]{0,63}$`)

func LoadAccountingCatalog(raw []byte) (ModelAccounting, error) {
	var result ModelAccounting
	if !utf8.Valid(raw) || !catalogUniqueJSON(raw) {
		return result, accountingCatalogError("catalog JSON")
	}
	root, err := catalogObject(raw, []string{"schema_version", "allowed_models", "budgets"})
	if err != nil {
		return result, err
	}
	version, err := catalogInteger(root["schema_version"], "schema_version", 1)
	if err != nil || version != 1 {
		return result, accountingCatalogError("schema_version")
	}
	if err = json.Unmarshal(root["allowed_models"], &result.AllowedModels); err != nil || len(result.AllowedModels) == 0 {
		return ModelAccounting{}, accountingCatalogError("allowed_models")
	}
	seen := make(map[string]bool)
	for _, name := range result.AllowedModels {
		if !catalogModelTag.MatchString(name) || seen[name] {
			return ModelAccounting{}, accountingCatalogError("allowed_models")
		}
		seen[name] = true
	}
	budgets, err := catalogObject(root["budgets"], []string{"tokens_total", "request_timeout_seconds", "agent_output_tokens", "security_output_tokens", "input_template_tokens"})
	if err != nil {
		return ModelAccounting{}, accountingCatalogError("budgets")
	}
	if result.TokensTotal, err = catalogInteger(budgets["tokens_total"], "budgets.tokens_total", maximumSafeCatalogInteger); err != nil {
		return ModelAccounting{}, err
	}
	seconds, err := catalogInteger(budgets["request_timeout_seconds"], "budgets.request_timeout_seconds", 86400)
	if err != nil {
		return ModelAccounting{}, err
	}
	result.RequestTimeout = time.Duration(seconds) * time.Second
	agent, err := catalogOptionalInteger(budgets, "agent_output_tokens", 512, maximumSafeCatalogInteger)
	if err != nil {
		return ModelAccounting{}, err
	}
	security, err := catalogOptionalInteger(budgets, "security_output_tokens", 256, maximumSafeCatalogInteger)
	if err != nil {
		return ModelAccounting{}, err
	}
	result.TemplateTokens, err = catalogOptionalInteger(budgets, "input_template_tokens", 1024, maximumSafeCatalogInteger)
	if err != nil {
		return ModelAccounting{}, err
	}
	result.AgentOutputTokens = int(agent)
	result.SecurityOutputTokens = int(security)
	return result, nil
}

func accountingCatalogError(field string) error {
	return &ValidationError{Problems: []string{field + " is invalid in accounting catalog"}}
}

func catalogInteger(raw json.RawMessage, field string, maximum int64) (int64, error) {
	var value int64
	if len(raw) == 0 || string(raw) == "null" || json.Unmarshal(raw, &value) != nil || value <= 0 || value > maximum {
		return 0, accountingCatalogError(field)
	}
	return value, nil
}

func catalogOptionalInteger(fields map[string]json.RawMessage, name string, fallback, maximum int64) (int64, error) {
	raw, present := fields[name]
	if !present {
		if fallback > maximum {
			return 0, accountingCatalogError("budgets." + name)
		}
		return fallback, nil
	}
	return catalogInteger(raw, "budgets."+name, maximum)
}

func catalogObject(raw json.RawMessage, known []string) (map[string]json.RawMessage, error) {
	if len(bytes.TrimSpace(raw)) == 0 || bytes.TrimSpace(raw)[0] != '{' {
		return nil, accountingCatalogError("catalog object")
	}
	var fields map[string]json.RawMessage
	if json.Unmarshal(raw, &fields) != nil {
		return nil, accountingCatalogError("catalog object")
	}
	for name := range fields {
		for _, expected := range known {
			if strings.EqualFold(name, expected) && name != expected {
				return nil, accountingCatalogError(expected)
			}
		}
	}
	return fields, nil
}

// Reject duplicates in the whole document so downstream policy fields cannot
// acquire a different meaning from the same serialized catalog.
func catalogUniqueJSON(raw []byte) bool {
	if !json.Valid(raw) {
		return false
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var value func() error
	value = func() error {
		token, err := decoder.Token()
		if err != nil {
			return err
		}
		delimiter, compound := token.(json.Delim)
		if !compound {
			return nil
		}
		switch delimiter {
		case '{':
			seen := make(map[string]bool)
			for decoder.More() {
				token, err = decoder.Token()
				if err != nil {
					return err
				}
				name, ok := token.(string)
				if !ok || seen[name] {
					return errors.New("ambiguous catalog")
				}
				seen[name] = true
				if err = value(); err != nil {
					return err
				}
			}
			token, err = decoder.Token()
			if err != nil || token != json.Delim('}') {
				return errors.New("invalid catalog")
			}
		case '[':
			for decoder.More() {
				if err = value(); err != nil {
					return err
				}
			}
			token, err = decoder.Token()
			if err != nil || token != json.Delim(']') {
				return errors.New("invalid catalog")
			}
		default:
			return errors.New("invalid catalog")
		}
		return nil
	}
	if value() != nil {
		return false
	}
	_, err := decoder.Token()
	return errors.Is(err, io.EOF)
}
