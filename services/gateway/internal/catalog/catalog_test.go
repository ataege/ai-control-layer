package catalog

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"

	"starter/services/gateway/internal/catalog/catalogtest"
	"starter/services/gateway/internal/contracts"
	"starter/services/gateway/internal/testdb"
)

const policyContent = catalogtest.PolicyContent

// catalogWorld is an isolated catalog state inside one outer transaction, rolled back at the end.
type catalogWorld struct {
	outer  pgx.Tx
	feedID int64
}

func newCatalogWorld(t *testing.T) *catalogWorld {
	t.Helper()
	pool := testdb.Open(t)
	outer, err := pool.Begin(context.Background())
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	t.Cleanup(func() { _ = outer.Rollback(context.Background()) })
	return &catalogWorld{outer: outer, feedID: catalogtest.InsertFeed(t, outer)}
}

func (world *catalogWorld) activate(t *testing.T, content string, feedID *int64) int64 {
	t.Helper()
	return catalogtest.Activate(t, world.outer, content, feedID)
}

func TestPostgresActiveSnapshotFollowsThePointer(t *testing.T) {
	world := newCatalogWorld(t)
	loader := NewLoader()
	firstRevision := world.activate(t, policyContent, &world.feedID)

	snapshot, err := loader.Active(context.Background(), world.outer)
	if err != nil {
		t.Fatalf("Active: %v", err)
	}
	if snapshot.RevisionID != firstRevision || snapshot.FeedRevisionID == nil || *snapshot.FeedRevisionID != world.feedID ||
		snapshot.Security.EvaluatedCatalogRevisionID != firstRevision || snapshot.Security.SemanticInjection.Threshold != 0.75 ||
		snapshot.Security.Feed == nil || snapshot.Limits.CallsTotal != 24 {
		t.Fatalf("unexpected snapshot %+v", snapshot)
	}

	// A changed optional threshold is active at the next read, with the new revision recorded.
	secondRevision := world.activate(t, strings.Replace(policyContent, `"threshold": 0.75`, `"threshold": 0.9`, 1), &world.feedID)
	snapshot, err = loader.Active(context.Background(), world.outer)
	if err != nil || snapshot.RevisionID != secondRevision || snapshot.Security.SemanticInjection.Threshold != 0.9 ||
		snapshot.Security.EvaluatedCatalogRevisionID != secondRevision {
		t.Fatalf("after the change: %+v %v", snapshot, err)
	}
}

func TestPostgresActiveSnapshotFailsClosed(t *testing.T) {
	cases := map[string]func(world *catalogWorld, t *testing.T){
		"no active revision": func(world *catalogWorld, t *testing.T) {
			world.activate(t, policyContent, &world.feedID)
			if _, err := world.outer.Exec(context.Background(), `UPDATE app.control_catalog_pointer SET active_revision_id = NULL WHERE id = 1`); err != nil {
				t.Fatal(err)
			}
		},
		"signature matching without a feed": func(world *catalogWorld, t *testing.T) {
			world.activate(t, policyContent, nil)
		},
		"feed revision differs from the policy": func(world *catalogWorld, t *testing.T) {
			world.activate(t, strings.Replace(policyContent, `"revision": "feed_v2"`, `"revision": "feed_v3"`, 1), &world.feedID)
		},
		"invalid limits": func(world *catalogWorld, t *testing.T) {
			world.activate(t, strings.Replace(policyContent, `"calls_agent": 12`, `"calls_agent": 30`, 1), &world.feedID)
		},
		"unknown disabled rule": func(world *catalogWorld, t *testing.T) {
			world.activate(t, strings.Replace(policyContent, `"disabled_rules": []`, `"disabled_rules": ["no_such_rule_v1"]`, 1), &world.feedID)
		},
	}
	for name, arrange := range cases {
		t.Run(name, func(t *testing.T) {
			world := newCatalogWorld(t)
			arrange(world, t)
			if _, err := NewLoader().Active(context.Background(), world.outer); !errors.Is(err, ErrUnavailable) {
				t.Errorf("got %v, want ErrUnavailable", err)
			}
		})
	}
}

func TestPostgresSignatureMatchingDisabledNeedsNoFeed(t *testing.T) {
	world := newCatalogWorld(t)
	world.activate(t, strings.Replace(policyContent,
		`"signature_match": {"enabled": true`, `"signature_match": {"enabled": false`, 1), nil)
	snapshot, err := NewLoader().Active(context.Background(), world.outer)
	if err != nil || snapshot.FeedRevisionID != nil || snapshot.Security.Feed != nil {
		t.Errorf("snapshot %+v err %v", snapshot, err)
	}
}

func samplePassport() contracts.Passport {
	tokensAgent := int64(5000)
	return contracts.Passport{
		AdmissionCatalogRevisionID: 1,
		Scope: contracts.PassportScope{
			AllowedModels:   []string{"qwen3.5:4b", "older-model"},
			ReportTemplates: contracts.ReportTemplates,
		},
		Limits: contracts.PassportLimits{CallsTotal: 24, CallsAgent: 12, CallsSecurity: 12, TokensTotal: 20000,
			TokensAgent: &tokensAgent, RequestTimeoutSeconds: 20, LocalMaxConcurrency: 2, ToolAttempts: 12, Corrections: 2},
	}
}

func TestEffectiveLimitsNarrowButNeverWidenThePassport(t *testing.T) {
	passport := samplePassport()
	lowered := Snapshot{RevisionID: 2, Limits: Limits{
		AllowedModels: []string{"qwen3.5:4b"}, CallsTotal: 10, CallsAgent: 5, CallsSecurity: 5, TokensTotal: 8000,
		RequestTimeoutSeconds: 10, LocalMaxConcurrency: 1, ToolAttempts: 4, Corrections: 0,
		EnabledTemplates: []contracts.ReportTemplate{contracts.TemplateInternalInvestigation},
	}}
	effective := EffectiveFor(passport, lowered)
	want := Effective{AllowedModels: []string{"qwen3.5:4b"}, CallsTotal: 10, CallsAgent: 5, CallsSecurity: 5,
		TokensTotal: 8000, RequestTimeoutSeconds: 10, LocalMaxConcurrency: 1, ToolAttempts: 4, Corrections: 0,
		ReportTemplates:            []contracts.ReportTemplate{contracts.TemplateInternalInvestigation},
		AdmissionCatalogRevisionID: 1, EvaluatedCatalogRevisionID: 2}
	if !reflect.DeepEqual(effective, want) {
		t.Errorf("lowered catalog\nwant %+v\ngot  %+v", want, effective)
	}

	raised := Snapshot{RevisionID: 3, Limits: Limits{
		AllowedModels: []string{"qwen3.5:4b", "older-model", "bigger-model"}, CallsTotal: 100, CallsAgent: 50,
		CallsSecurity: 50, TokensTotal: 90000, RequestTimeoutSeconds: 60, LocalMaxConcurrency: 8, ToolAttempts: 40,
		Corrections: 9, EnabledTemplates: contracts.ReportTemplates,
	}}
	effective = EffectiveFor(passport, raised)
	if effective.CallsTotal != 24 || effective.TokensTotal != 20000 || effective.LocalMaxConcurrency != 2 ||
		effective.Corrections != 2 || !reflect.DeepEqual(effective.AllowedModels, []string{"qwen3.5:4b", "older-model"}) ||
		effective.EvaluatedCatalogRevisionID != 3 {
		t.Errorf("a raised catalog widened the passport: %+v", effective)
	}
}

func TestParseLimitsFailsClosed(t *testing.T) {
	broken := map[string]string{
		"not json":              `{`,
		"wrong schema version":  strings.Replace(policyContent, `"schema_version": 1`, `"schema_version": 2`, 1),
		"missing calls total":   strings.Replace(policyContent, `"calls_total": 24, `, ``, 1),
		"zero tool attempts":    strings.Replace(policyContent, `"tool_attempts": 12`, `"tool_attempts": 0`, 1),
		"negative corrections":  strings.Replace(policyContent, `"corrections": 2`, `"corrections": -1`, 1),
		"timeout beyond expiry": strings.Replace(policyContent, `"request_timeout_seconds": 20`, `"request_timeout_seconds": 900`, 1),
		"no models":             strings.Replace(policyContent, `["qwen3.5:4b"]`, `[]`, 1),
		"unknown template":      strings.Replace(policyContent, `"vendor_reconciliation_v1"]`, `"public_v1"]`, 1),
	}
	for name, content := range broken {
		if _, err := ParseLimits([]byte(content)); !errors.Is(err, ErrUnavailable) {
			t.Errorf("%s: got %v", name, err)
		}
	}
	if _, err := ParseLimits([]byte(policyContent)); err != nil {
		t.Errorf("valid catalog rejected: %v", err)
	}
}

func TestParseLimitsReadsTheAccountingFieldsWithTheirDefaults(t *testing.T) {
	limits, err := ParseLimits([]byte(policyContent))
	if err != nil || limits.AgentOutputTokens != 512 || limits.SecurityOutputTokens != 256 || limits.InputTemplateTokens != 1024 {
		t.Fatalf("policy values: %+v %v", limits, err)
	}
	withoutAccounting := strings.Replace(policyContent,
		`"agent_output_tokens": 512, "security_output_tokens": 256, "input_template_tokens": 1024,`, ``, 1)
	limits, err = ParseLimits([]byte(withoutAccounting))
	if err != nil || limits.AgentOutputTokens != 512 || limits.SecurityOutputTokens != 256 || limits.InputTemplateTokens != 1024 {
		t.Fatalf("defaults: %+v %v", limits, err)
	}
	limits, err = ParseLimits([]byte(strings.Replace(policyContent, `"agent_output_tokens": 512`, `"agent_output_tokens": 128`, 1)))
	if err != nil || limits.AgentOutputTokens != 128 {
		t.Fatalf("edited value: %+v %v", limits, err)
	}
	for name, value := range map[string]string{"null": "null", "zero": "0", "negative": "-5", "fraction": "1.5", "text": `"512"`} {
		content := strings.Replace(policyContent, `"security_output_tokens": 256`, `"security_output_tokens": `+value, 1)
		if _, err := ParseLimits([]byte(content)); !errors.Is(err, ErrUnavailable) {
			t.Errorf("%s: %v", name, err)
		}
	}
}
