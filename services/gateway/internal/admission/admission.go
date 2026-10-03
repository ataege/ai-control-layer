// Package admission derives the immutable passport of a new run from the verified operator, the
// Go-registered task template, the active control catalog and the organization's demo records
// (report: "Trusted authority and passport invariants"). A request that exceeds that authority is
// rejected with a reason and the scope or limit that must change; admission never narrows it.
// An admitted passport, its run, its first job and its token ledger commit in one transaction
// (Figure 4).
package admission

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"starter/services/gateway/internal/budget"
	"starter/services/gateway/internal/catalog"
	"starter/services/gateway/internal/contracts"
	"starter/services/gateway/internal/provenance"
	"starter/services/gateway/internal/repository"
)

// TaskTemplateReconcileAtlas is the only registered task template (lead decision for the MVP;
// it becomes a lookup if NestJS adds task configuration).
const TaskTemplateReconcileAtlas = "reconcile_atlas_v1"

// ApprovalRuleReviewQueueReport is the only registered approval rule: review every queue_report.
const ApprovalRuleReviewQueueReport = "review_queue_report"

// maximumRequestedInvoices matches the X-07 schema bound.
const maximumRequestedInvoices = 100

// ErrUnavailable means admission could not decide (catalog, records or storage unavailable).
// It is never an admission: nothing is stored and the caller answers decision_unavailable.
var ErrUnavailable = errors.New("admission decision unavailable")

// Rejection explains why a request exceeds the operator's authority.
type Rejection struct {
	Code contracts.ReasonCode
	// Message names the scope or limit that must change; it never echoes record content.
	Message string
}

func (rejection *Rejection) Error() string { return string(rejection.Code) + ": " + rejection.Message }

func reject(code contracts.ReasonCode, format string, arguments ...any) *Rejection {
	return &Rejection{Code: code, Message: fmt.Sprintf(format, arguments...)}
}

// Admitter issues passports.
type Admitter struct {
	repository *repository.Repository
	catalog    *catalog.Loader
	now        func() time.Time
}

// New returns an admitter that stores through the runtime repository and reads the active
// catalog through the shared snapshot loader.
func New(runtimeRepository *repository.Repository, catalogLoader *catalog.Loader) *Admitter {
	return &Admitter{repository: runtimeRepository, catalog: catalogLoader, now: time.Now}
}

// Admit validates the request against the operator's organization and the active catalog. It
// returns the stored passport, or a Rejection (after recording an admission.rejected event), or
// ErrUnavailable. Identity comes only from the verified operator, never from the request.
func (admitter *Admitter) Admit(ctx context.Context, operator contracts.OperatorContext, request contracts.StartRunRequest) (contracts.Passport, error) {
	if admitter == nil || admitter.repository == nil || admitter.catalog == nil || ctx == nil {
		return contracts.Passport{}, ErrUnavailable
	}
	if rejection := validateRequest(request); rejection != nil {
		return contracts.Passport{}, admitter.recordRejection(ctx, operator, nil, rejection)
	}
	var passport contracts.Passport
	var catalogRevisionID *int64
	err := admitter.repository.InTransaction(ctx, func(tx repository.Tx) error {
		// The full enforceable snapshot (limits and security settings) or nothing: a catalog
		// that cannot be enforced issues no passport.
		snapshot, err := admitter.catalog.Active(ctx, tx.Raw())
		if err != nil {
			return ErrUnavailable
		}
		catalogRevisionID = &snapshot.RevisionID
		vendorID, rejection, err := resolveVendor(ctx, tx.Raw(), operator.OrganizationID, request)
		if err != nil {
			return ErrUnavailable
		}
		if rejection != nil {
			return rejection
		}
		passport, rejection, err = admitter.buildPassport(operator, request, snapshot.RevisionID, snapshot.Limits, vendorID)
		if err != nil {
			return ErrUnavailable
		}
		if rejection != nil {
			return rejection
		}
		if err := tx.InsertAdmission(ctx, passport, repository.NewJob{ID: newUUID(), Kind: contracts.JobKindAgentStep}); err != nil {
			return ErrUnavailable
		}
		// The run's token ledger opens with the passport (alignment decision 6); without it no
		// model request can be reserved.
		switch err := budget.OpenRunLedger(ctx, tx.Raw(), passport.OrganizationID, passport.RunID, passport.Limits); {
		case errors.Is(err, budget.ErrInvalid):
			return reject(contracts.ReasonLimitNotAllowed, "the token limits cannot open a run ledger")
		case err != nil:
			return ErrUnavailable
		}
		_, err = tx.AppendEvent(ctx, repository.NewEvent{
			OrganizationID:    passport.OrganizationID,
			RunID:             &passport.RunID,
			EventType:         contracts.EventRunQueued,
			Decision:          pointer(contracts.DecisionAllow),
			CatalogRevisionID: &snapshot.RevisionID,
			MaskedSummary: contracts.MaskedSummary{
				AdmissionCatalogRevisionID: &snapshot.RevisionID,
				Effect:                     pointer("none"),
			},
		})
		if err != nil {
			return ErrUnavailable
		}
		return nil
	})
	var rejection *Rejection
	if errors.As(err, &rejection) {
		return contracts.Passport{}, admitter.recordRejection(ctx, operator, catalogRevisionID, rejection)
	}
	if err != nil {
		return contracts.Passport{}, ErrUnavailable
	}
	return passport, nil
}

// recordRejection stores the admission.rejected event in its own transaction: the admission
// itself wrote nothing. If the evidence cannot be stored, the caller sees ErrUnavailable.
func (admitter *Admitter) recordRejection(ctx context.Context, operator contracts.OperatorContext, catalogRevisionID *int64, rejection *Rejection) error {
	err := admitter.repository.InTransaction(ctx, func(tx repository.Tx) error {
		_, err := tx.AppendEvent(ctx, repository.NewEvent{
			OrganizationID:    operator.OrganizationID,
			EventType:         contracts.EventAdmissionRejected,
			Decision:          pointer(contracts.DecisionDeny),
			ReasonCode:        &rejection.Code,
			CatalogRevisionID: catalogRevisionID,
			MaskedSummary: contracts.MaskedSummary{
				Effect:      pointer("none"),
				SafeMessage: pointer(rejection.Message),
			},
		})
		return err
	})
	if err != nil {
		return ErrUnavailable
	}
	return rejection
}

// validateRequest checks the request shape before any record is read.
func validateRequest(request contracts.StartRunRequest) *Rejection {
	if request.Template != TaskTemplateReconcileAtlas {
		return reject(contracts.ReasonTemplateNotAllowed, "task template %q is not registered", boundedText(request.Template))
	}
	if len(request.InvoiceIDs) == 0 || len(request.InvoiceIDs) > maximumRequestedInvoices {
		return reject(contracts.ReasonInvalidArguments, "invoiceIds must name between 1 and %d invoices", maximumRequestedInvoices)
	}
	seen := make(map[string]bool, len(request.InvoiceIDs))
	for _, invoiceID := range request.InvoiceIDs {
		if strings.TrimSpace(invoiceID) == "" || len(invoiceID) > 256 {
			return reject(contracts.ReasonInvalidArguments, "invoiceIds must not contain empty or over-long ids")
		}
		if seen[invoiceID] {
			return reject(contracts.ReasonInvalidArguments, "invoiceIds repeats %q", boundedText(invoiceID))
		}
		seen[invoiceID] = true
	}
	if strings.TrimSpace(request.Destination) == "" {
		return reject(contracts.ReasonInvalidArguments, "destination is required")
	}
	if request.ApprovalRequirement != nil && *request.ApprovalRequirement != ApprovalRuleReviewQueueReport {
		return reject(contracts.ReasonInvalidArguments, "approvalRequirement must be %q", ApprovalRuleReviewQueueReport)
	}
	if request.Limits != nil {
		if request.Limits.ModelCalls != nil && *request.Limits.ModelCalls < 1 {
			return reject(contracts.ReasonInvalidArguments, "limits.modelCalls must be positive")
		}
		if request.Limits.TimeoutSeconds != nil && *request.Limits.TimeoutSeconds < 1 {
			return reject(contracts.ReasonInvalidArguments, "limits.timeoutSeconds must be positive")
		}
	}
	return nil
}

// resolveVendor checks that every requested invoice belongs to the organization and that all
// share one vendor, which must also be the requested vendor and the destination with a
// registered reporting address. It returns that vendor.
func resolveVendor(ctx context.Context, transaction pgx.Tx, organizationID string, request contracts.StartRunRequest) (string, *Rejection, error) {
	rows, err := transaction.Query(ctx, `SELECT id, vendor_id FROM demo.invoices
		WHERE organization_id = $1 AND id = ANY($2)`, organizationID, request.InvoiceIDs)
	if err != nil {
		return "", nil, err
	}
	vendorOfInvoice := make(map[string]string, len(request.InvoiceIDs))
	for rows.Next() {
		var invoiceID, vendorID string
		if err := rows.Scan(&invoiceID, &vendorID); err != nil {
			rows.Close()
			return "", nil, err
		}
		vendorOfInvoice[invoiceID] = vendorID
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return "", nil, err
	}
	// Reported in request order; another organization's invoice looks the same as a missing one.
	for _, invoiceID := range request.InvoiceIDs {
		if _, found := vendorOfInvoice[invoiceID]; !found {
			return "", reject(contracts.ReasonResourceOutOfScope, "invoice %q is not available to this organization", boundedText(invoiceID)), nil
		}
	}
	taskVendorID := vendorOfInvoice[request.InvoiceIDs[0]]
	for _, invoiceID := range request.InvoiceIDs {
		if vendorOfInvoice[invoiceID] != taskVendorID {
			return "", reject(contracts.ReasonResourceOutOfScope, "invoice %q belongs to another vendor than %q", boundedText(invoiceID), boundedText(request.InvoiceIDs[0])), nil
		}
	}
	if request.VendorID != nil && *request.VendorID != taskVendorID {
		return "", reject(contracts.ReasonResourceOutOfScope, "vendorId %q is not the vendor of the requested invoices", boundedText(*request.VendorID)), nil
	}
	if request.Destination != taskVendorID {
		return "", reject(contracts.ReasonDestinationNotAllowed, "destination %q is not the task's vendor", boundedText(request.Destination)), nil
	}
	var hasRegisteredAddress bool
	err = transaction.QueryRow(ctx, `SELECT registered_reporting_address IS NOT NULL FROM demo.vendors
		WHERE organization_id = $1 AND id = $2`, organizationID, taskVendorID).Scan(&hasRegisteredAddress)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && !hasRegisteredAddress) {
		return "", reject(contracts.ReasonDestinationNotAllowed, "vendor %q has no registered reporting address", boundedText(taskVendorID)), nil
	}
	if err != nil {
		return "", nil, err
	}
	return taskVendorID, nil, nil
}

// buildPassport derives the grant: the request's scope, capped by the catalog.
func (admitter *Admitter) buildPassport(operator contracts.OperatorContext, request contracts.StartRunRequest, revisionID int64, limits catalog.Limits, vendorID string) (contracts.Passport, *Rejection, error) {
	callsTotal := limits.CallsTotal
	runLifetime := time.Duration(limits.RunExpiryMinutes) * time.Minute
	if request.Limits != nil && request.Limits.ModelCalls != nil {
		if *request.Limits.ModelCalls > limits.CallsTotal {
			return contracts.Passport{}, reject(contracts.ReasonLimitNotAllowed, "limits.modelCalls exceeds the catalog limit of %d", limits.CallsTotal), nil
		}
		callsTotal = *request.Limits.ModelCalls
	}
	if request.Limits != nil && request.Limits.TimeoutSeconds != nil {
		if *request.Limits.TimeoutSeconds > limits.RunExpiryMinutes*60 {
			return contracts.Passport{}, reject(contracts.ReasonLimitNotAllowed, "limits.timeoutSeconds exceeds the catalog run expiry of %d minutes", limits.RunExpiryMinutes), nil
		}
		runLifetime = time.Duration(*request.Limits.TimeoutSeconds) * time.Second
	}
	// The registered task grants both templates so permitted recovery needs no new authority;
	// the catalog can only narrow that set.
	var reportTemplates []contracts.ReportTemplate
	for _, template := range contracts.ReportTemplates {
		if slices.Contains(limits.EnabledTemplates, template) {
			reportTemplates = append(reportTemplates, template)
		}
	}
	var projectionRules []string
	if slices.Contains(reportTemplates, contracts.TemplateVendorReconciliation) {
		projectionRules = []string{provenance.VendorInvoiceFieldsV1.Name}
	}
	// Database timestamps keep microseconds; truncating keeps the stored passport identical.
	issuedAt := admitter.now().UTC().Truncate(time.Microsecond)
	runID := newUUID()
	passport := contracts.Passport{
		PassportID:                 newUUID(),
		RunID:                      runID,
		OrganizationID:             operator.OrganizationID,
		ActorID:                    operator.UserID,
		TaskVersion:                TaskTemplateReconcileAtlas,
		AdmissionCatalogRevisionID: revisionID,
		IssuedAt:                   issuedAt,
		// ExpiresAt is the authoritative deadline; RunExpiryMinutes records the catalog ceiling.
		ExpiresAt: issuedAt.Add(runLifetime),
		Scope: contracts.PassportScope{
			Tools:                 slices.Clone(contracts.ToolNames),
			InvoiceIDs:            slices.Clone(request.InvoiceIDs),
			VendorIDs:             []string{vendorID},
			ReportTemplates:       nonNil(reportTemplates),
			ProjectionRules:       nonNil(projectionRules),
			RecipientReferences:   []string{"recipient:" + runID + ":" + vendorID},
			AllowedModels:         slices.Clone(limits.AllowedModels),
			InternalNoteReadable:  true,
			ApprovalRequiredTools: []contracts.ToolName{contracts.ToolQueueReport},
		},
		Limits: contracts.PassportLimits{
			CallsTotal:            callsTotal,
			CallsAgent:            min(limits.CallsAgent, callsTotal),
			CallsSecurity:         min(limits.CallsSecurity, callsTotal),
			TokensTotal:           limits.TokensTotal,
			RequestTimeoutSeconds: limits.RequestTimeoutSeconds,
			LocalMaxConcurrency:   limits.LocalMaxConcurrency,
			ToolAttempts:          limits.ToolAttempts,
			Corrections:           limits.Corrections,
			RunExpiryMinutes:      limits.RunExpiryMinutes,
		},
	}
	return passport, nil, nil
}

// boundedText keeps a caller-supplied value short in a rejection message.
func boundedText(value string) string {
	const maximumLength = 64
	runes := []rune(value)
	if len(runes) > maximumLength {
		return string(runes[:maximumLength]) + "..."
	}
	return value
}

func nonNil[Value any](values []Value) []Value {
	if values == nil {
		return []Value{}
	}
	return values
}

func pointer[Value any](value Value) *Value { return &value }

// newUUID returns a random version 4 UUID in the lowercase form PostgreSQL returns.
func newUUID() string {
	var bytes [16]byte
	// crypto/rand.Read never returns an error since Go 1.24 (it aborts instead).
	_, _ = rand.Read(bytes[:])
	bytes[6] = (bytes[6] & 0x0f) | 0x40
	bytes[8] = (bytes[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", bytes[0:4], bytes[4:6], bytes[6:8], bytes[8:10], bytes[10:16])
}
