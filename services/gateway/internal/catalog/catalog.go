// Package catalog is the trusted active snapshot loader (Figure 3). It reads the active-revision
// pointer on every call, "rather than relying indefinitely on a stale cache", and builds one
// coherent snapshot: the catalog revision, its bound signature-feed revision, the limits that
// bound passports and dispatches, and the hybrid security settings. Anything missing or invalid
// is ErrUnavailable: a catalog that cannot be enforced never decides.
package catalog

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"sync"

	"github.com/jackc/pgx/v5"

	"starter/services/gateway/internal/contracts"
	"starter/services/gateway/internal/security"
)

// ErrUnavailable means no valid active snapshot could be read.
var ErrUnavailable = errors.New("active control catalog unavailable")

// maximumCatalogInteger matches the largest integer the TypeScript importer accepts exactly.
const maximumCatalogInteger = 9007199254740991

// Limits are the catalog values that bound a passport and every later dispatch.
type Limits struct {
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

// Snapshot is one coherent active catalog state.
type Snapshot struct {
	RevisionID int64
	// FeedRevisionID is the signature feed bound by the pointer; nil when none is bound.
	FeedRevisionID *int64
	Limits         Limits
	Security       security.Settings
}

// Querier runs one query; a pool or a transaction.
type Querier interface {
	QueryRow(ctx context.Context, sql string, arguments ...any) pgx.Row
}

// snapshotKey identifies an immutable revision pair: revisions and feeds reject UPDATE by trigger,
// so a parsed snapshot stays valid for its key.
type snapshotKey struct {
	revisionID     int64
	feedRevisionID int64
}

// Loader reads the active snapshot. It memoizes parsing per immutable revision pair; the pointer
// itself is read on every call.
type Loader struct {
	mutex  sync.Mutex
	parsed map[snapshotKey]Snapshot
}

// NewLoader returns a loader with an empty parse cache.
func NewLoader() *Loader {
	return &Loader{parsed: make(map[snapshotKey]Snapshot)}
}

// Active reads the pointer and returns the active snapshot, or ErrUnavailable. Pass the caller's
// transaction to read the pointer in it. The snapshot is shared with the parse cache: callers
// must treat its slices and feed as read-only.
func (loader *Loader) Active(ctx context.Context, querier Querier) (Snapshot, error) {
	if loader == nil || ctx == nil || querier == nil {
		return Snapshot{}, ErrUnavailable
	}
	var revisionID int64
	var content []byte
	var feedRevisionID *int64
	var feedSource *string
	var feedDigest *string
	err := querier.QueryRow(ctx, `SELECT revision.id, revision.content, feed.id, feed.source_text, feed.file_digest
		FROM app.control_catalog_pointer AS pointer
		JOIN app.control_catalog_revisions AS revision ON revision.id = pointer.active_revision_id
		LEFT JOIN app.signature_feed_revisions AS feed ON feed.id = pointer.active_feed_revision_id
		WHERE pointer.id = 1`).Scan(&revisionID, &content, &feedRevisionID, &feedSource, &feedDigest)
	if err != nil || revisionID <= 0 {
		return Snapshot{}, ErrUnavailable
	}
	key := snapshotKey{revisionID: revisionID}
	if feedRevisionID != nil {
		key.feedRevisionID = *feedRevisionID
	}
	loader.mutex.Lock()
	cached, found := loader.parsed[key]
	loader.mutex.Unlock()
	if found {
		return cached, nil
	}
	snapshot, err := buildSnapshot(revisionID, content, feedRevisionID, feedSource, feedDigest)
	if err != nil {
		return Snapshot{}, ErrUnavailable
	}
	loader.mutex.Lock()
	loader.parsed[key] = snapshot
	loader.mutex.Unlock()
	return snapshot, nil
}

func buildSnapshot(revisionID int64, content []byte, feedRevisionID *int64, feedSource, feedDigest *string) (Snapshot, error) {
	limits, err := ParseLimits(content)
	if err != nil {
		return Snapshot{}, err
	}
	var feedContent []byte
	digest := ""
	if feedSource != nil && feedDigest != nil {
		feedContent, digest = []byte(*feedSource), *feedDigest
	}
	// Fails closed on a missing or mismatched feed, unknown disabled rules or invalid controls.
	settings, err := security.SettingsFromCatalog(revisionID, content, feedContent, digest)
	if err != nil {
		return Snapshot{}, ErrUnavailable
	}
	return Snapshot{RevisionID: revisionID, FeedRevisionID: feedRevisionID, Limits: limits, Security: settings}, nil
}

// limitsContent mirrors the catalog fields the limits come from. The importer validated the whole
// document; this re-checks every value it relies on and treats anything missing as unavailable.
type limitsContent struct {
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

// ParseLimits reads and checks the limits of one catalog revision's content.
func ParseLimits(raw []byte) (Limits, error) {
	var content limitsContent
	if err := json.Unmarshal(raw, &content); err != nil || content.SchemaVersion == nil || *content.SchemaVersion != 1 {
		return Limits{}, ErrUnavailable
	}
	budgets := content.Budgets
	for _, value := range []*int64{budgets.CallsTotal, budgets.CallsAgent, budgets.CallsSecurity, budgets.TokensTotal,
		budgets.RequestTimeoutSeconds, budgets.LocalMaxConcurrency, budgets.RunExpiryMinutes, budgets.ToolAttempts} {
		if value == nil || *value < 1 || *value > maximumCatalogInteger {
			return Limits{}, ErrUnavailable
		}
	}
	// Corrections may be zero: a run without bounded correction stops at its first denial.
	if budgets.Corrections == nil || *budgets.Corrections < 0 || *budgets.Corrections > maximumCatalogInteger {
		return Limits{}, ErrUnavailable
	}
	if *budgets.CallsAgent > *budgets.CallsTotal || *budgets.CallsSecurity > *budgets.CallsTotal ||
		*budgets.RequestTimeoutSeconds >= *budgets.RunExpiryMinutes*60 {
		return Limits{}, ErrUnavailable
	}
	if len(content.AllowedModels) == 0 || content.Reports == nil {
		return Limits{}, ErrUnavailable
	}
	for _, template := range content.Reports.EnabledTemplates {
		if !template.Valid() {
			return Limits{}, ErrUnavailable
		}
	}
	return Limits{
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

// Effective is what a run may use now: the immutable passport ceiling narrowed by the active
// catalog. Removed models and lowered budgets apply at once; a raised catalog value never
// exceeds the passport ("raising configured limits cannot exceed an existing passport").
type Effective struct {
	AllowedModels         []string
	CallsTotal            int64
	CallsAgent            int64
	CallsSecurity         int64
	TokensTotal           int64
	RequestTimeoutSeconds int64
	LocalMaxConcurrency   int64
	ToolAttempts          int64
	Corrections           int64
	ReportTemplates       []contracts.ReportTemplate
	// AdmissionCatalogRevisionID and EvaluatedCatalogRevisionID are both recorded with decisions.
	AdmissionCatalogRevisionID int64
	EvaluatedCatalogRevisionID int64
}

// EffectiveFor narrows the passport by the snapshot.
func EffectiveFor(passport contracts.Passport, snapshot Snapshot) Effective {
	limits, active := passport.Limits, snapshot.Limits
	effective := Effective{
		CallsTotal:                 min(limits.CallsTotal, active.CallsTotal),
		CallsAgent:                 min(limits.CallsAgent, active.CallsAgent),
		CallsSecurity:              min(limits.CallsSecurity, active.CallsSecurity),
		TokensTotal:                min(limits.TokensTotal, active.TokensTotal),
		RequestTimeoutSeconds:      min(limits.RequestTimeoutSeconds, active.RequestTimeoutSeconds),
		LocalMaxConcurrency:        min(limits.LocalMaxConcurrency, active.LocalMaxConcurrency),
		ToolAttempts:               min(limits.ToolAttempts, active.ToolAttempts),
		Corrections:                min(limits.Corrections, active.Corrections),
		AdmissionCatalogRevisionID: passport.AdmissionCatalogRevisionID,
		EvaluatedCatalogRevisionID: snapshot.RevisionID,
		AllowedModels:              []string{},
		ReportTemplates:            []contracts.ReportTemplate{},
	}
	for _, model := range passport.Scope.AllowedModels {
		if slices.Contains(active.AllowedModels, model) {
			effective.AllowedModels = append(effective.AllowedModels, model)
		}
	}
	for _, template := range passport.Scope.ReportTemplates {
		if slices.Contains(active.EnabledTemplates, template) {
			effective.ReportTemplates = append(effective.ReportTemplates, template)
		}
	}
	return effective
}
