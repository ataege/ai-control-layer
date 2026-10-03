package policy

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

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

// CatalogSecuritySettings loads a revision's security settings from the control catalog: the
// revision's content and the feed on the active pointer, validated by security.SettingsFromCatalog.
// A missing or invalid revision is an error, never default settings.
type CatalogSecuritySettings struct{ pool *pgxpool.Pool }

// NewCatalogSecuritySettings returns a loader on the given pool. It creates nothing.
func NewCatalogSecuritySettings(pool *pgxpool.Pool) *CatalogSecuritySettings {
	return &CatalogSecuritySettings{pool: pool}
}

// SettingsFor reads the revision and the active feed in one read-only transaction.
func (source *CatalogSecuritySettings) SettingsFor(ctx context.Context, revisionID int64) (security.Settings, error) {
	if source.pool == nil {
		return security.Settings{}, ErrSecuritySettingsUnavailable
	}
	var settings security.Settings
	err := pgx.BeginTxFunc(ctx, source.pool, pgx.TxOptions{AccessMode: pgx.ReadOnly}, func(tx pgx.Tx) error {
		var content []byte
		if err := tx.QueryRow(ctx, `SELECT content FROM app.control_catalog_revisions WHERE id = $1`, revisionID).Scan(&content); err != nil {
			return err
		}
		var feedContent *string
		var feedDigest *string
		err := tx.QueryRow(ctx,
			`SELECT feed.source_text, feed.file_digest
			   FROM app.control_catalog_pointer AS pointer
			   LEFT JOIN app.signature_feed_revisions AS feed ON feed.id = pointer.active_feed_revision_id
			  WHERE pointer.id = 1`).Scan(&feedContent, &feedDigest)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		var feedBytes []byte
		var digest string
		if feedContent != nil && feedDigest != nil {
			feedBytes, digest = []byte(*feedContent), *feedDigest
		}
		settings, err = security.SettingsFromCatalog(revisionID, content, feedBytes, digest)
		return err
	})
	if err != nil {
		return security.Settings{}, ErrSecuritySettingsUnavailable
	}
	return settings, nil
}
