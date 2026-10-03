package policy

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5/pgxpool"

	"starter/services/gateway/internal/catalog"
	"starter/services/gateway/internal/security"
)

// ErrSecuritySettingsUnavailable means the security settings of a revision could not be loaded.
var ErrSecuritySettingsUnavailable = errors.New("security settings unavailable")

// SecuritySettingsSource returns the validated security settings of one catalog revision.
type SecuritySettingsSource interface {
	SettingsFor(ctx context.Context, revisionID int64) (security.Settings, error)
}

// SecurityActionEvaluator adapts c1's Inspector.EvaluateAction to the gate's ActionEvaluator
// (GO-77). The inspector runs the field limit, the signatures and then the metered semantic check
// at the action_proposal boundary; its answer is no_objection, block or pause, never "allow".
type SecurityActionEvaluator struct {
	inspector *security.Inspector
	settings  SecuritySettingsSource
}

// NewSecurityActionEvaluator returns the adapter.
func NewSecurityActionEvaluator(inspector *security.Inspector, settings SecuritySettingsSource) *SecurityActionEvaluator {
	return &SecurityActionEvaluator{inspector: inspector, settings: settings}
}

// EvaluateAction maps no_objection to allow (the gate's outcome stays as it was), block to deny
// with the control's reason, and pause or any failure to an error, which the gate denies and the
// worker treats as a pause. Control records are returned in every case.
func (evaluator *SecurityActionEvaluator) EvaluateAction(ctx context.Context, run RunIdentity, action StoredAction) (ActionCheck, error) {
	unavailable := ActionCheck{Outcome: OutcomeDeny, ReasonCode: ReasonCode(security.ReasonSecurityEvaluatorUnavailable)}
	if evaluator.inspector == nil || evaluator.settings == nil {
		return unavailable, ErrSecuritySettingsUnavailable
	}
	settings, err := evaluator.settings.SettingsFor(ctx, action.EvaluatedRevisionID)
	if err != nil {
		return unavailable, ErrSecuritySettingsUnavailable
	}
	assessment, err := evaluator.inspector.EvaluateAction(ctx, security.ActionInput{
		RunID: run.RunID, ActionID: action.ActionID, Tool: string(action.Tool), CanonicalArguments: action.CanonicalArguments,
	}, settings)
	check := ActionCheck{Records: assessment.Records, ReasonCode: ReasonCode(assessment.ReasonCode)}
	switch {
	case err != nil || assessment.Decision == security.ActionPause:
		check.Outcome = OutcomeDeny
		if check.ReasonCode == "" {
			check.ReasonCode = ReasonCode(security.ReasonSecurityEvaluatorUnavailable)
		}
		if err == nil {
			err = security.ErrEvaluatorUnavailable
		}
		return check, err
	case assessment.Decision == security.ActionBlock:
		check.Outcome = OutcomeDeny
		return check, nil
	case assessment.Decision == security.ActionNoObjection:
		check.Outcome = OutcomeAllow
		return check, nil
	}
	check.Outcome = OutcomeDeny
	return check, security.ErrEvaluatorUnavailable
}

// CatalogSecuritySettings returns the security settings of the active catalog snapshot through
// 3c's catalog.Loader, the single active-snapshot source. The action's evaluated revision must be
// the active one: a different active revision means the action needs a fresh evaluation, so the
// check pauses instead of judging it under other rules.
type CatalogSecuritySettings struct {
	pool   *pgxpool.Pool
	loader *catalog.Loader
}

// NewCatalogSecuritySettings returns a settings source with its own parse cache. It creates
// nothing in the database.
func NewCatalogSecuritySettings(pool *pgxpool.Pool) *CatalogSecuritySettings {
	return &CatalogSecuritySettings{pool: pool, loader: catalog.NewLoader()}
}

// SettingsFor returns the active snapshot's settings when it is the given revision.
func (source *CatalogSecuritySettings) SettingsFor(ctx context.Context, revisionID int64) (security.Settings, error) {
	if source.pool == nil {
		return security.Settings{}, ErrSecuritySettingsUnavailable
	}
	snapshot, err := source.loader.Active(ctx, source.pool)
	if err != nil || snapshot.RevisionID != revisionID {
		return security.Settings{}, ErrSecuritySettingsUnavailable
	}
	return snapshot.Security, nil
}
