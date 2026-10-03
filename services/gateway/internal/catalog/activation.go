package catalog

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
)

// activationLockKey is the transaction-scoped advisory lock that lets only one gateway instance
// validate a requested revision at a time.
const activationLockKey int64 = 0x7461736b_63617461 // "taskcata"

// Activation outcomes, for logs and tests.
type ActivationOutcome string

const (
	// ActivationIdle: nothing requested, the request is already active, or it already failed.
	ActivationIdle ActivationOutcome = "idle"
	// ActivationBusy: another gateway instance holds the activation lock.
	ActivationBusy ActivationOutcome = "busy"
	// ActivationActivated: the requested revision validated and is now active (Go's acknowledgement).
	ActivationActivated ActivationOutcome = "activated"
	// ActivationRejected: the requested revision failed validation; the last good one stays active.
	ActivationRejected ActivationOutcome = "rejected"
)

// Stable rejection codes written to pointer.last_error; never file content.
const (
	rejectionRevisionMissing = "revision_missing"
	rejectionFeedMissing     = "signature_feed_missing"
	rejectionInvalid         = "catalog_invalid"
)

// TrustedFeedIssuer is the only signature-feed issuer the gateway binds; the import stores the
// repository's config/attack-signatures.json under it.
const TrustedFeedIssuer = "task-passport-security"

// Beginner starts a transaction: the gateway pool, or an enclosing transaction in tests.
type Beginner interface {
	Begin(ctx context.Context) (pgx.Tx, error)
}

// rejection is the safe record of a failed validation (catalog activation protocol, step 2).
type rejection struct {
	Reason     string `json:"reason"`
	Code       string `json:"code"`
	Message    string `json:"message"`
	RevisionID int64  `json:"revision_id"`
	Stage      string `json:"stage"`
}

// ActivateRequested validates the requested revision with the gateway's own parsers (limits and
// security settings, with the trusted issuer's feed of the policy's signatures.revision) and, in the same
// transaction, either makes it validated and active together with its feed and clears the last
// error, or records a safe last error and keeps the last good active revision. A request that
// already failed is not retried; a new import sets a new requested revision.
func ActivateRequested(ctx context.Context, database Beginner) (ActivationOutcome, error) {
	if ctx == nil || database == nil {
		return ActivationIdle, ErrUnavailable
	}
	transaction, err := database.Begin(ctx)
	if err != nil {
		return ActivationIdle, ErrUnavailable
	}
	defer func() { _ = transaction.Rollback(context.WithoutCancel(ctx)) }()

	var locked bool
	if err := transaction.QueryRow(ctx, `SELECT pg_try_advisory_xact_lock($1)`, activationLockKey).Scan(&locked); err != nil {
		return ActivationIdle, ErrUnavailable
	}
	if !locked {
		return ActivationBusy, nil
	}
	var requestedID, activeID *int64
	var lastError []byte
	err = transaction.QueryRow(ctx, `SELECT requested_revision_id, active_revision_id, last_error
		FROM app.control_catalog_pointer WHERE id = 1 FOR UPDATE`).Scan(&requestedID, &activeID, &lastError)
	if errors.Is(err, pgx.ErrNoRows) {
		return ActivationIdle, nil
	}
	if err != nil {
		return ActivationIdle, ErrUnavailable
	}
	if requestedID == nil || (activeID != nil && *activeID == *requestedID) || alreadyRejected(lastError, *requestedID) {
		return ActivationIdle, nil
	}

	feedID, failure := validateRevision(ctx, transaction, *requestedID)
	if failure != nil {
		record, _ := json.Marshal(failure)
		if _, err := transaction.Exec(ctx, `UPDATE app.control_catalog_pointer
			SET last_error = $1, last_error_at = now(), updated_at = now() WHERE id = 1`, record); err != nil {
			return ActivationIdle, ErrUnavailable
		}
		if err := transaction.Commit(ctx); err != nil {
			return ActivationIdle, ErrUnavailable
		}
		return ActivationRejected, nil
	}
	if _, err := transaction.Exec(ctx, `UPDATE app.control_catalog_pointer
		SET validated_revision_id = $1, active_revision_id = $1, active_feed_revision_id = $2,
			last_error = NULL, last_error_at = NULL, updated_at = now()
		WHERE id = 1`, *requestedID, feedID); err != nil {
		return ActivationIdle, ErrUnavailable
	}
	if err := transaction.Commit(ctx); err != nil {
		return ActivationIdle, ErrUnavailable
	}
	return ActivationActivated, nil
}

// alreadyRejected reports whether the last error is the gateway's rejection of this revision.
func alreadyRejected(lastError []byte, revisionID int64) bool {
	if len(lastError) == 0 {
		return false
	}
	var record rejection
	return json.Unmarshal(lastError, &record) == nil && record.Stage == "gateway_validation" && record.RevisionID == revisionID
}

// validateRevision builds the full snapshot of one revision; it returns the feed to bind (nil for
// none) or the rejection to record.
func validateRevision(ctx context.Context, transaction pgx.Tx, revisionID int64) (*int64, *rejection) {
	reject := func(code, message string) *rejection {
		return &rejection{Reason: "policy_reload_rejected", Code: code, Message: message, RevisionID: revisionID, Stage: "gateway_validation"}
	}
	var content []byte
	err := transaction.QueryRow(ctx, `SELECT content FROM app.control_catalog_revisions WHERE id = $1`, revisionID).Scan(&content)
	if err != nil {
		return nil, reject(rejectionRevisionMissing, "The requested catalog revision could not be read.")
	}
	var signatures struct {
		Signatures *struct {
			Revision *string `json:"revision"`
		} `json:"signatures"`
	}
	if json.Unmarshal(content, &signatures) != nil || signatures.Signatures == nil || signatures.Signatures.Revision == nil {
		return nil, reject(rejectionInvalid, "The requested catalog revision names no signature feed revision.")
	}
	// Only the trusted issuer's feed is bound; (issuer, revision) is unique.
	var feedID *int64
	var feedSource, feedDigest *string
	var storedID int64
	var storedSource, storedDigest string
	err = transaction.QueryRow(ctx, `SELECT id, source_text, file_digest FROM app.signature_feed_revisions
		WHERE issuer = $1 AND revision = $2`, TrustedFeedIssuer, *signatures.Signatures.Revision).Scan(&storedID, &storedSource, &storedDigest)
	switch {
	case err == nil:
		feedID, feedSource, feedDigest = &storedID, &storedSource, &storedDigest
	case !errors.Is(err, pgx.ErrNoRows):
		return nil, reject(rejectionInvalid, "The signature feed could not be read.")
	}
	if _, err := ParseLimits(content); err != nil {
		return nil, reject(rejectionInvalid, "The requested catalog revision has invalid limits.")
	}
	if _, err := buildSnapshot(revisionID, content, feedID, feedSource, feedDigest); err != nil {
		if feedID == nil {
			return nil, reject(rejectionFeedMissing, "The policy needs a signature feed revision that has not been imported.")
		}
		return nil, reject(rejectionInvalid, "The requested catalog revision failed the gateway's security validation.")
	}
	return feedID, nil
}

// WatchRequested checks for a requested revision every interval until ctx ends, so an import is
// validated and activated within seconds. Outcomes other than idle are logged without content.
func WatchRequested(ctx context.Context, database Beginner, interval time.Duration, logger *slog.Logger) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	failing := false
	for {
		outcome, err := ActivateRequested(ctx, database)
		switch {
		case err != nil && ctx.Err() == nil:
			// A failed check is retried next tick. It is named once when checks start failing (a
			// lasting failure means no import can activate), not on every tick.
			if !failing {
				logger.Warn("catalog activation checks are failing; no requested revision can be activated")
			}
			failing = true
		case err == nil && failing:
			logger.Info("catalog activation checks work again")
			failing = false
		}
		switch {
		case outcome == ActivationActivated:
			logger.Info("catalog revision validated and activated")
		case outcome == ActivationRejected:
			logger.Warn("catalog revision rejected; the last good revision stays active")
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
