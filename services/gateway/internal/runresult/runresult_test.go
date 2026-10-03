package runresult

import (
	"context"
	"crypto/sha256"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"starter/services/gateway/internal/contracts"
	"starter/services/gateway/internal/repository"
	"starter/services/gateway/internal/testdb"
)

const (
	firstReport  = "4d5e6f70-8192-4a3b-8c5d-6e7f8091a2b3"
	secondReport = "5e6f7081-92a3-4b4c-9d6e-7f8091a2b3c4"
)

func TestParseAcceptsOnlyTheNarrowFormat(t *testing.T) {
	valid := map[string][]string{
		`{"status":"completed","report_ids":["` + firstReport + `"]}`:                                 {firstReport},
		` {"report_ids":["` + firstReport + `","` + secondReport + `"],"status":"completed"} ` + "\n": {firstReport, secondReport},
	}
	for answer, want := range valid {
		result, reason := Parse(answer)
		if reason != "" || !reflect.DeepEqual(result.ReportIDs, want) {
			t.Errorf("%q: result %v reason %q", answer, result, reason)
		}
	}
	rejected := map[string]string{
		"prose":                "The reconciliation is done; see the reports.",
		"empty":                "",
		"text around the JSON": `Done: {"status":"completed","report_ids":["` + firstReport + `"]}`,
		"other status":         `{"status":"failed","report_ids":["` + firstReport + `"]}`,
		"missing status":       `{"report_ids":["` + firstReport + `"]}`,
		"no reports":           `{"status":"completed","report_ids":[]}`,
		"three reports":        `{"status":"completed","report_ids":["` + firstReport + `","` + secondReport + `","6f708192-a3b4-4c5d-8e7f-8091a2b3c4d5"]}`,
		"repeated report":      `{"status":"completed","report_ids":["` + firstReport + `","` + firstReport + `"]}`,
		"uppercase id":         `{"status":"completed","report_ids":["` + strings.ToUpper(firstReport) + `"]}`,
		"not an id":            `{"status":"completed","report_ids":["internal report"]}`,
		"extra prose field":    `{"status":"completed","report_ids":["` + firstReport + `"],"summary":"Fraud confirmed."}`,
		"duplicate key":        `{"status":"completed","status":"completed","report_ids":["` + firstReport + `"]}`,
		"two objects":          `{"status":"completed","report_ids":["` + firstReport + `"]}{}`,
		"oversized":            `{"status":"completed","report_ids":["` + firstReport + `"]}` + strings.Repeat(" ", 5000),
	}
	for name, answer := range rejected {
		if _, reason := Parse(answer); reason != contracts.ReasonInvalidArguments {
			t.Errorf("%s: reason %q", name, reason)
		}
	}
}

func TestFinalAnswerInstructionDescribesTheParsedFormat(t *testing.T) {
	start := strings.Index(FinalAnswerInstruction, "{")
	end := strings.Index(FinalAnswerInstruction, "}")
	example := strings.Replace(FinalAnswerInstruction[start:end+1], "<report id>", firstReport, 1)
	if _, reason := Parse(example); reason != "" {
		t.Errorf("the instruction's example %q does not parse: %s", example, reason)
	}
}

// resultWorld holds two runs of one organization and a run of another, each with reports,
// inside one rolled-back transaction.
type resultWorld struct {
	outer                        pgx.Tx
	repository                   *repository.Repository
	organizationID, otherOrgID   string
	runID, siblingRunID, otherID string
	ownReports                   []string
	siblingReport, otherReport   string
}

func admitRun(t *testing.T, world *resultWorld, organizationID string) string {
	t.Helper()
	issuedAt := time.Now().UTC().Truncate(time.Microsecond)
	passport := contracts.Passport{PassportID: testdb.ID(t), RunID: testdb.ID(t), OrganizationID: organizationID,
		ActorID: testdb.ID(t), TaskVersion: "reconcile_atlas_v1", AdmissionCatalogRevisionID: 1,
		IssuedAt: issuedAt, ExpiresAt: issuedAt.Add(15 * time.Minute)}
	if err := world.repository.InTransaction(context.Background(), func(tx repository.Tx) error {
		return tx.InsertAdmission(context.Background(), passport, repository.NewJob{ID: testdb.ID(t), Kind: contracts.JobKindAgentStep})
	}); err != nil {
		t.Fatalf("admission fixture: %v", err)
	}
	return passport.RunID
}

func addReport(t *testing.T, world *resultWorld, organizationID, runID string, step int) string {
	t.Helper()
	actionID, reportID := testdb.ID(t), testdb.ID(t)
	digest := sha256.Sum256([]byte(actionID))
	if _, err := world.outer.Exec(context.Background(), `INSERT INTO runtime.actions (id, organization_id, run_id, step_number,
		tool, canonical_arguments, canonicalization_version, action_digest, idempotency_key, evaluated_catalog_revision_id, status)
		VALUES ($1, $2, $3, $4, 'create_report', '{}', 1, $5, $6, 1, 'succeeded')`, actionID, organizationID, runID, step, digest[:], actionID); err != nil {
		t.Fatalf("action fixture: %v", err)
	}
	if _, err := world.outer.Exec(context.Background(), `INSERT INTO demo.reports (id, organization_id, run_id, created_by_action_id,
		template, classification, destination_class, title, content, content_hash) VALUES ($1, $2, $3, $4,
		'internal_investigation_v1', 'internal_only', 'internal_reviewers', 'Investigation', 'content',
		sha256(convert_to('content', 'UTF8')))`, reportID, organizationID, runID, actionID); err != nil {
		t.Fatalf("report fixture: %v", err)
	}
	return reportID
}

func newResultWorld(t *testing.T) *resultWorld {
	t.Helper()
	pool := testdb.Open(t)
	outer, err := pool.Begin(context.Background())
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	t.Cleanup(func() { _ = outer.Rollback(context.Background()) })
	world := &resultWorld{outer: outer, repository: repository.New(outer), organizationID: testdb.ID(t), otherOrgID: testdb.ID(t)}
	world.runID = admitRun(t, world, world.organizationID)
	world.siblingRunID = admitRun(t, world, world.organizationID)
	world.otherID = admitRun(t, world, world.otherOrgID)
	world.ownReports = []string{addReport(t, world, world.organizationID, world.runID, 1), addReport(t, world, world.organizationID, world.runID, 2)}
	world.siblingReport = addReport(t, world, world.organizationID, world.siblingRunID, 1)
	world.otherReport = addReport(t, world, world.otherOrgID, world.otherID, 1)
	return world
}

func answerFor(reportIDs ...string) string {
	return `{"status":"completed","report_ids":["` + strings.Join(reportIDs, `","`) + `"]}`
}

func TestPostgresValidateChecksEveryReportBelongsToTheRun(t *testing.T) {
	world := newResultWorld(t)
	ctx := context.Background()
	reference, reason, err := Validate(ctx, world.outer, world.organizationID, world.runID, answerFor(world.ownReports...))
	if err != nil || reason != "" || reference != `{"report_ids":["`+world.ownReports[0]+`","`+world.ownReports[1]+`"]}` {
		t.Fatalf("own reports: reference %q reason %q err %v", reference, reason, err)
	}
	for name, reportID := range map[string]string{
		"a report of another run of the same organization": world.siblingReport,
		"a report of another organization":                 world.otherReport,
		"an unknown report":                                testdb.ID(t),
	} {
		reference, reason, err := Validate(ctx, world.outer, world.organizationID, world.runID, answerFor(world.ownReports[0], reportID))
		if err != nil || reason != contracts.ReasonResourceOutOfScope || reference != "" {
			t.Errorf("%s: reference %q reason %q err %v", name, reference, reason, err)
		}
	}
	if _, reason, _ := Validate(ctx, world.outer, world.organizationID, world.runID, "All done."); reason != contracts.ReasonInvalidArguments {
		t.Errorf("prose: %q", reason)
	}
}

// The lead's case (c): a final answer naming another run's report is rejected and the run never
// completes; the validated reference completes the run and is what the run state exposes.
func TestPostgresOnlyAValidatedResultCompletesTheRun(t *testing.T) {
	world := newResultWorld(t)
	ctx := context.Background()
	runEvent := func(eventType contracts.EventType) repository.NewEvent {
		return repository.NewEvent{OrganizationID: world.organizationID, RunID: &world.runID, EventType: eventType}
	}
	transition := func(to contracts.RunStatus, reference *string, eventType contracts.EventType) (contracts.RunState, error) {
		var state contracts.RunState
		err := world.repository.InTransaction(ctx, func(tx repository.Tx) error {
			var transitionErr error
			state, transitionErr = tx.TransitionRun(ctx, repository.RunTransition{OrganizationID: world.organizationID,
				RunID: world.runID, To: to, ResultReference: reference, Event: runEvent(eventType)})
			return transitionErr
		})
		return state, err
	}
	if _, err := transition(contracts.RunRunning, nil, contracts.EventRunStarted); err != nil {
		t.Fatal(err)
	}

	_, reason, err := Validate(ctx, world.outer, world.organizationID, world.runID, answerFor(world.siblingReport))
	if err != nil || reason != contracts.ReasonResourceOutOfScope {
		t.Fatalf("sibling report: reason %q err %v", reason, err)
	}
	state, err := world.repository.RunState(ctx, world.organizationID, world.runID)
	if err != nil || state.Status != contracts.RunRunning || state.ResultReference != nil {
		t.Fatalf("the run changed after a rejected final answer: %+v %v", state, err)
	}
	// A reference that did not come from Validate is refused, and so is one on another target.
	forged := `{"report_ids":["not-a-report"]}`
	if _, err := transition(contracts.RunCompleted, &forged, contracts.EventRunCompleted); !errors.Is(err, repository.ErrInvalid) {
		t.Errorf("forged reference: %v", err)
	}
	reference, _, err := Validate(ctx, world.outer, world.organizationID, world.runID, answerFor(world.ownReports[1]))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := transition(contracts.RunPaused, &reference, contracts.EventRunPaused); !errors.Is(err, repository.ErrInvalid) {
		t.Errorf("reference on a non-completed target: %v", err)
	}

	state, err = transition(contracts.RunCompleted, &reference, contracts.EventRunCompleted)
	if err != nil || state.Status != contracts.RunCompleted || state.ResultReference == nil ||
		!reflect.DeepEqual(state.ResultReference.ReportIDs, []string{world.ownReports[1]}) {
		t.Fatalf("completion: %+v %v", state, err)
	}
	read, err := world.repository.RunState(ctx, world.organizationID, world.runID)
	if err != nil || !reflect.DeepEqual(read.ResultReference, state.ResultReference) {
		t.Errorf("read back: %+v %v", read, err)
	}
	var stored string
	if err := world.outer.QueryRow(ctx, `SELECT result_reference FROM runtime.runs WHERE id = $1`, world.runID).Scan(&stored); err != nil ||
		stored != reference {
		t.Errorf("stored reference %q, want %q (no prose)", stored, reference)
	}
}
