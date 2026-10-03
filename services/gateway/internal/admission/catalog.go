package admission

import (
	"context"
	"encoding/json"
	"errors"
	"slices"

	"github.com/jackc/pgx/v5"

	"starter/services/gateway/internal/contracts"
)

// ErrCatalogUnavailable means no valid active catalog revision could be read. Admission then
// issues nothing ("With no valid initial catalog, the gateway is not ready and cannot dispatch").
var ErrCatalogUnavailable = errors.New("active control catalog unavailable")

// maximumCatalogInteger matches the largest integer the TypeScript importer accepts exactly.
const maximumCatalogInteger = 9007199254740991

// CatalogLimits are the parts of the active catalog revision that bound a new passport.
type CatalogLimits struct {
	RevisionID            int64
	AllowedModels         []string
	CallsTotal            int64
	CallsAgent            int64
	CallsSecurity         int64
	TokensTotal           int64
	RequestTimeoutSeconds int64
	LocalMaxConcurrency   int64
	RunExpiryMinutes      int64
	ToolAttempts          int64
	Corrections           int64
	EnabledTemplates      []contracts.ReportTemplate
}

// catalogContent mirrors the fields admission reads from control_catalog_revisions.content.
// The importer validated the whole document; this reader re-checks what it relies on and
// treats every missing or out-of-range value as an unavailable catalog.
type catalogContent struct {
	SchemaVersion *int     `json:"schema_version"`
	AllowedModels []string `json:"allowed_models"`
	Budgets       struct {
		CallsTotal            *int64 `json:"calls_total"`
		CallsAgent            *int64 `json:"calls_agent"`
		CallsSecurity         *int64 `json:"calls_security"`
		TokensTotal           *int64 `json:"tokens_total"`
		RequestTimeoutSeconds *int64 `json:"request_timeout_seconds"`
		LocalMaxConcurrency   *int64 `json:"local_max_concurrency"`
		RunExpiryMinutes      *int64 `json:"run_expiry_minutes"`
		ToolAttempts          *int64 `json:"tool_attempts"`
		Corrections           *int64 `json:"corrections"`
	} `json:"budgets"`
	Reports *struct {
		EnabledTemplates []contracts.ReportTemplate `json:"enabled_templates"`
	} `json:"reports"`
}

// readActiveCatalog reads the active revision in the caller's transaction, so the passport
// records exactly the revision its limits came from.
func readActiveCatalog(ctx context.Context, transaction pgx.Tx) (CatalogLimits, error) {
	var revisionID int64
	var raw []byte
	err := transaction.QueryRow(ctx, `SELECT revision.id, revision.content
		FROM app.control_catalog_pointer AS pointer
		JOIN app.control_catalog_revisions AS revision ON revision.id = pointer.active_revision_id
		WHERE pointer.id = 1`).Scan(&revisionID, &raw)
	if err != nil || revisionID <= 0 {
		return CatalogLimits{}, ErrCatalogUnavailable
	}
	return parseCatalog(revisionID, raw)
}

func parseCatalog(revisionID int64, raw []byte) (CatalogLimits, error) {
	var content catalogContent
	if err := json.Unmarshal(raw, &content); err != nil || content.SchemaVersion == nil || *content.SchemaVersion != 1 {
		return CatalogLimits{}, ErrCatalogUnavailable
	}
	budgets := content.Budgets
	values := []*int64{budgets.CallsTotal, budgets.CallsAgent, budgets.CallsSecurity, budgets.TokensTotal,
		budgets.RequestTimeoutSeconds, budgets.LocalMaxConcurrency, budgets.RunExpiryMinutes, budgets.ToolAttempts}
	for _, value := range values {
		if value == nil || *value < 1 || *value > maximumCatalogInteger {
			return CatalogLimits{}, ErrCatalogUnavailable
		}
	}
	// Corrections may be zero: a run without bounded correction stops at its first denial.
	if budgets.Corrections == nil || *budgets.Corrections < 0 || *budgets.Corrections > maximumCatalogInteger {
		return CatalogLimits{}, ErrCatalogUnavailable
	}
	if *budgets.CallsAgent > *budgets.CallsTotal || *budgets.CallsSecurity > *budgets.CallsTotal ||
		*budgets.RequestTimeoutSeconds >= *budgets.RunExpiryMinutes*60 {
		return CatalogLimits{}, ErrCatalogUnavailable
	}
	if len(content.AllowedModels) == 0 || content.Reports == nil {
		return CatalogLimits{}, ErrCatalogUnavailable
	}
	for _, template := range content.Reports.EnabledTemplates {
		if !template.Valid() {
			return CatalogLimits{}, ErrCatalogUnavailable
		}
	}
	return CatalogLimits{
		RevisionID:            revisionID,
		AllowedModels:         slices.Clone(content.AllowedModels),
		CallsTotal:            *budgets.CallsTotal,
		CallsAgent:            *budgets.CallsAgent,
		CallsSecurity:         *budgets.CallsSecurity,
		TokensTotal:           *budgets.TokensTotal,
		RequestTimeoutSeconds: *budgets.RequestTimeoutSeconds,
		LocalMaxConcurrency:   *budgets.LocalMaxConcurrency,
		RunExpiryMinutes:      *budgets.RunExpiryMinutes,
		ToolAttempts:          *budgets.ToolAttempts,
		Corrections:           *budgets.Corrections,
		EnabledTemplates:      slices.Clone(content.Reports.EnabledTemplates),
	}, nil
}
