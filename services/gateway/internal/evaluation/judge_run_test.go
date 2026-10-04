package evaluation

import (
	"context"
	"testing"
	"time"

	"starter/services/gateway/internal/catalog"
	"starter/services/gateway/internal/catalog/catalogtest"
	"starter/services/gateway/internal/contracts"
	"starter/services/gateway/internal/repository"
	"starter/services/gateway/internal/security"
	"starter/services/gateway/internal/testdb"
)

// A judge run (no agent job) stays queued, and evaluations accept it as active until it is
// cancelled or its passport expires.
func TestPostgresJudgeRunIsEvaluatedWhileQueuedAndStopsOnCancel(t *testing.T) {
	pool := testdb.Open(t)
	outer, err := pool.Begin(context.Background())
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	t.Cleanup(func() { _ = outer.Rollback(context.Background()) })
	ctx := context.Background()
	revisionID := catalogtest.ActivatePolicy(t, outer)
	runtimeRepository := repository.New(outer)
	operator := contracts.OperatorContext{UserID: testdb.ID(t), OrganizationID: testdb.ID(t), Roles: []string{"operator"}}
	issuedAt := time.Now().UTC().Truncate(time.Microsecond)
	passport := contracts.Passport{PassportID: testdb.ID(t), RunID: testdb.ID(t), OrganizationID: operator.OrganizationID,
		ActorID: operator.UserID, TaskVersion: "reconcile_atlas_v1", AdmissionCatalogRevisionID: revisionID,
		IssuedAt: issuedAt, ExpiresAt: issuedAt.Add(15 * time.Minute)}
	if err := runtimeRepository.InTransaction(ctx, func(tx repository.Tx) error {
		return tx.InsertJudgeAdmission(ctx, passport)
	}); err != nil {
		t.Fatalf("judge admission: %v", err)
	}
	caller := &fixtureCaller{verdict: benignVerdict}
	evaluator := New(Dependencies{Repository: runtimeRepository, Catalog: catalog.NewLoader(), Database: outer,
		Semantic: semanticEvaluator(t, caller), Inspector: security.NewInspector(semanticEvaluator(t, caller))})
	evaluate := func() contracts.ControlEvaluationResponse {
		text, tool := hostileText, contracts.ToolReadInvoice
		response, err := evaluator.Evaluate(ctx, operator, contracts.ControlEvaluationRequest{
			RunID: passport.RunID, Kind: contracts.BoundaryToolResult, Text: &text, Tool: &tool})
		if err != nil {
			t.Fatalf("Evaluate: %v", err)
		}
		return response
	}

	// Queued and jobless: the controls run and decide.
	if response := evaluate(); response.ReasonCode == nil || *response.ReasonCode != contracts.ReasonSignatureMatch {
		t.Fatalf("queued judge run: %+v, want the controls' signature_match decision", response)
	}
	var jobs int
	if err := outer.QueryRow(ctx, `SELECT count(*) FROM runtime.jobs WHERE run_id = $1`, passport.RunID).Scan(&jobs); err != nil || jobs != 0 {
		t.Fatalf("jobs = %d (err %v), want 0: no worker may claim a judge run", jobs, err)
	}

	// Cancelling stops the queued run at once; later evaluations are refused.
	var state contracts.RunState
	if err := runtimeRepository.InTransaction(ctx, func(tx repository.Tx) error {
		var cancelErr error
		state, cancelErr = tx.RequestCancellation(ctx, operator.OrganizationID, passport.RunID)
		return cancelErr
	}); err != nil || state.Status != contracts.RunStopped {
		t.Fatalf("cancel: %+v %v, want stopped", state, err)
	}
	if response := evaluate(); response.ReasonCode == nil || *response.ReasonCode != contracts.ReasonRunCancelled {
		t.Fatalf("cancelled judge run: %+v, want run_cancelled", response)
	}
}
