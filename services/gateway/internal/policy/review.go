package policy

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"starter/services/gateway/internal/provenance"
	"starter/services/gateway/internal/repository"
	"starter/services/gateway/internal/tools"
)

// ErrReviewUnavailable means the review material could not be frozen; the gate then denies.
var ErrReviewUnavailable = errors.New("review material unavailable")

// ReviewPayload is the exact material a reviewer approves (GO-43), frozen before review in
// runtime.review_payloads. Its field order is fixed, so its canonical JSON and digest are
// deterministic. It is restricted review storage: never copied into events.
type ReviewPayload struct {
	CanonicalizationVersion int                `json:"canonicalization_version"`
	ActionID                string             `json:"action_id"`
	RunID                   string             `json:"run_id"`
	PassportID              string             `json:"passport_id"`
	PolicyRevisionID        int64              `json:"policy_revision_id"`
	Tool                    ToolName           `json:"tool"`
	CanonicalArguments      json.RawMessage    `json:"canonical_arguments"`
	Recipient               *ReviewedRecipient `json:"recipient"`
	Report                  *ReviewedReport    `json:"report"`
	ExpiresAt               string             `json:"expires_at"`
}

// ReviewedRecipient is the exact destination the reviewer sees.
type ReviewedRecipient struct {
	Reference string `json:"reference"`
	VendorID  string `json:"vendor_id"`
	Address   string `json:"address"`
}

// ReviewedReport is the stored, server-rendered report exactly as it would be queued, with its
// trusted source manifest.
type ReviewedReport struct {
	ID                    string           `json:"id"`
	Version               int              `json:"version"`
	Template              string           `json:"template"`
	TemplateVersion       int              `json:"template_version"`
	ProjectionRule        *string          `json:"projection_rule"`
	ProjectionRuleVersion *int             `json:"projection_rule_version"`
	Classification        string           `json:"classification"`
	ContentHash           string           `json:"content_hash"`
	Content               string           `json:"content"`
	Sources               []ReviewedSource `json:"sources"`
	SourceManifestDigest  string           `json:"source_manifest_digest"`
}

// ReviewedSource is one lineage source with the version the report was rendered from.
type ReviewedSource struct {
	Kind           string   `json:"kind"`
	ID             string   `json:"id"`
	Version        int      `json:"version"`
	Classification string   `json:"classification"`
	ConsumedFields []string `json:"consumed_fields"`
}

// Digest is SHA-256 over the payload's canonical JSON. It detects any change to the reviewed
// material; it never authenticates or authorizes.
func (payload ReviewPayload) Digest() ([sha256.Size]byte, []byte, error) {
	canonicalPayload, err := compactJSON(payload)
	if err != nil {
		return [sha256.Size]byte{}, nil, err
	}
	return sha256.Sum256(canonicalPayload), canonicalPayload, nil
}

// FrozenReview is what the gate records with an approval request: the payload row and the safe
// references the approval.requested event may carry.
type FrozenReview struct {
	PayloadID      string
	PayloadDigest  [sha256.Size]byte
	ReportID       string
	Template       string
	Classification string
}

// ReviewFreezer freezes the review material of an action that needs approval.
type ReviewFreezer interface {
	Freeze(ctx context.Context, run RunIdentity, action StoredAction, scope PassportScope) (FrozenReview, error)
}

// PostgresReviewFreezer builds the payload from trusted rows and stores it immutably.
type PostgresReviewFreezer struct {
	pool       *pgxpool.Pool
	repository *repository.Repository
}

// NewPostgresReviewFreezer returns a freezer. It creates nothing at construction.
func NewPostgresReviewFreezer(pool *pgxpool.Pool) *PostgresReviewFreezer {
	return &PostgresReviewFreezer{pool: pool, repository: repository.New(pool)}
}

// Freeze builds the review payload of the stored action and inserts it in one transaction. A
// recipient that does not resolve, a missing report or any read failure freezes nothing.
func (freezer *PostgresReviewFreezer) Freeze(ctx context.Context, run RunIdentity, action StoredAction, scope PassportScope) (FrozenReview, error) {
	if freezer.pool == nil {
		return FrozenReview{}, ErrReviewUnavailable
	}
	passport, err := freezer.repository.Passport(ctx, run.OrganizationID, run.RunID)
	if err != nil || passport.PassportID != scope.PassportID {
		return FrozenReview{}, ErrReviewUnavailable
	}
	arguments, err := DecodeArguments(action.Tool, action.CanonicalArguments)
	if err != nil {
		return FrozenReview{}, ErrReviewUnavailable
	}
	payload := ReviewPayload{
		CanonicalizationVersion: CanonicalizationVersion,
		ActionID:                action.ActionID,
		RunID:                   run.RunID,
		PassportID:              scope.PassportID,
		PolicyRevisionID:        action.EvaluatedRevisionID,
		Tool:                    action.Tool,
		CanonicalArguments:      action.CanonicalArguments,
		// The review cannot outlive the run: it expires with the passport.
		ExpiresAt: scope.ExpiresAt.UTC().Format(time.RFC3339Nano),
	}

	var frozen FrozenReview
	err = freezer.repository.InTransaction(ctx, func(tx repository.Tx) error {
		if queueArguments, isQueue := arguments.(QueueReportArguments); isQueue {
			// Worker 2's rule, the same one queue_report applies; it loads the stored passport itself.
			vendorID, address, reason, err := tools.ResolveRecipientForReview(ctx, tx.Raw(), run.OrganizationID, run.RunID,
				queueArguments.RecipientReference)
			if err != nil || reason != "" {
				return ErrReviewUnavailable
			}
			payload.Recipient = &ReviewedRecipient{Reference: queueArguments.RecipientReference, VendorID: vendorID, Address: address}
			report, err := provenance.LoadReport(ctx, tx.Raw(), run.OrganizationID, run.RunID, queueArguments.ReportID)
			if err != nil {
				return ErrReviewUnavailable
			}
			payload.Report, err = reviewedReport(report)
			if err != nil {
				return err
			}
			frozen.ReportID, frozen.Template, frozen.Classification = report.ID, report.TemplateName, report.Classification
		}
		digest, canonicalPayload, err := payload.Digest()
		if err != nil {
			return ErrReviewUnavailable
		}
		frozen.PayloadDigest = digest
		return tx.Raw().QueryRow(ctx,
			`INSERT INTO runtime.review_payloads
			   (organization_id, run_id, action_id, payload, payload_digest, canonicalization_version, expires_at)
			 VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING id::text`,
			run.OrganizationID, run.RunID, action.ActionID, canonicalPayload, digest[:], CanonicalizationVersion, scope.ExpiresAt,
		).Scan(&frozen.PayloadID)
	})
	if err != nil {
		return FrozenReview{}, ErrReviewUnavailable
	}
	return frozen, nil
}

// reviewedReport turns a stored report into its frozen form. Sources are sorted, so the manifest
// digest does not depend on read order; consumed fields keep their stored order.
func reviewedReport(report provenance.StoredReport) (*ReviewedReport, error) {
	template, registered := provenance.LookupTemplate(report.TemplateName)
	if !registered || len(report.Lineage) == 0 {
		return nil, ErrReviewUnavailable
	}
	reviewed := &ReviewedReport{
		ID: report.ID, Version: report.Version, Template: report.TemplateName, TemplateVersion: template.Version,
		Classification: report.Classification, ContentHash: hex.EncodeToString(report.ContentHash[:]), Content: report.Content,
	}
	first := report.Lineage[0]
	reviewed.ProjectionRule, reviewed.ProjectionRuleVersion = first.ProjectionRule, first.ProjectionRuleVersion
	for _, entry := range report.Lineage {
		reviewed.Sources = append(reviewed.Sources, ReviewedSource{
			Kind: entry.Kind, ID: entry.ID, Version: entry.Version,
			Classification: entry.Classification, ConsumedFields: entry.ConsumedFields,
		})
	}
	sort.Slice(reviewed.Sources, func(left, right int) bool {
		if reviewed.Sources[left].Kind != reviewed.Sources[right].Kind {
			return reviewed.Sources[left].Kind < reviewed.Sources[right].Kind
		}
		return reviewed.Sources[left].ID < reviewed.Sources[right].ID
	})
	manifest, err := compactJSON(reviewed.Sources)
	if err != nil {
		return nil, fmt.Errorf("%w: encode source manifest", ErrReviewUnavailable)
	}
	manifestDigest := sha256.Sum256(manifest)
	reviewed.SourceManifestDigest = hex.EncodeToString(manifestDigest[:])
	return reviewed, nil
}

// StaleSources returns the frozen report sources whose current version differs from the version
// the reviewed report was rendered from (exact integer equality, the `record versions` rule). A
// source that no longer exists counts as stale.
func StaleSources(payload ReviewPayload, currentVersions map[string]int) []string {
	if payload.Report == nil {
		return nil
	}
	var stale []string
	for _, source := range payload.Report.Sources {
		current, found := currentVersions[source.ID]
		if !found || current != source.Version {
			stale = append(stale, source.ID)
		}
	}
	return stale
}
