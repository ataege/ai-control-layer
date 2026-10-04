package agent

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"starter/services/gateway/internal/budget"
	"starter/services/gateway/internal/contracts"
	"starter/services/gateway/internal/operatorcontext"
	"starter/services/gateway/internal/reads"
	"starter/services/gateway/internal/testdb"
)

// Critical check "Unknown usage" (X-53, GO-39): a provider answer that carries no usable token counts
// is unknown usage, never zero. The answer is real Ollama-shaped JSON served by a labelled stub through
// the real transport, the real accounted caller, the real stepper and the real loop; no model is used.

// readUsage reads the run's usage through the same handler the operator reads use.
func readUsage(t *testing.T, world *loopWorld) reads.RunUsage {
	t.Helper()
	mux := http.NewServeMux()
	mux.Handle(reads.RunUsageRoutePattern, reads.RunUsageHandler(world.pool))
	operator := contracts.OperatorContext{UserID: testdb.ID(t), OrganizationID: world.organizationID, Roles: []string{"operator"}}
	request := httptest.NewRequest(http.MethodGet, "/internal/runs/"+world.runID+"/usage", nil)
	request = request.WithContext(operatorcontext.WithOperator(request.Context(), operator))
	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, request)
	var usage reads.RunUsage
	if recorder.Code != http.StatusOK || contracts.DecodeStrict(recorder.Body.Bytes(), &usage) != nil {
		t.Fatalf("usage read: status %d: %s", recorder.Code, recorder.Body.String())
	}
	return usage
}

func TestAnswerWithoutTokenCountsIsUnknownUsageNotZero(t *testing.T) {
	const message = `"message":{"role":"assistant","content":"The duplicate reference is INV104."}`
	for name, body := range map[string]string{
		"no counts at all":  `{"model":"test-fixture","done":true,` + message + `}`,
		"input count only":  `{"model":"test-fixture","done":true,` + message + `,"prompt_eval_count":200}`,
		"output count only": `{"model":"test-fixture","done":true,` + message + `,"eval_count":50}`,
	} {
		t.Run(name, func(t *testing.T) {
			world := newLoopWorld(t, passportOptions{})
			var requests atomic.Int64
			provider := providerDouble(t, func(writer http.ResponseWriter, _ *http.Request) {
				requests.Add(1)
				_, _ = writer.Write([]byte(body))
			})
			caller := NewCatalogAccountedCaller(provider, budget.NewPostgresStore(world.pool), fixedAccounting{},
				gatewaySnapshot([]string{"test-fixture"}, 20, 2), world.pool, "test-fixture")
			stepper, err := NewStepper(caller, budget.NewCallLog(world.pool), "test-fixture")
			if err != nil {
				t.Fatal(err)
			}
			loop := newTestLoop(t, world, stepper)
			if _, err := loop.Handle(context.Background(), world.job()); err != nil {
				t.Fatal(err)
			}

			// The run pauses for attention and says why; nothing is resent, not even by a second claim.
			assertRunEnded(t, world, contracts.RunPaused, contracts.ReasonOutcomeUnknown)
			if _, err := loop.Handle(context.Background(), world.job()); err != nil {
				t.Fatal(err)
			}
			if sent := requests.Load(); sent != 1 {
				t.Fatalf("%d provider requests, want exactly one", sent)
			}
			var safeMessage string
			if err := world.pool.QueryRow(context.Background(), `SELECT masked_summary->>'safeMessage' FROM runtime.audit_events
				WHERE organization_id = $1 AND run_id = $2 AND event_type = 'run.paused'`, world.organizationID, world.runID).Scan(&safeMessage); err != nil ||
				safeMessage != messageModelUsageUnknown {
				t.Fatalf("pause explanation %q (%v)", safeMessage, err)
			}

			// The call is recorded unknown, its whole reservation and its slot stay held, and no token is
			// counted as used: unknown is not zero.
			var outcome, reservationStatus string
			var reservedTokens int64
			var slotHeld bool
			var input, output, actual *int64
			if err := world.pool.QueryRow(context.Background(), `SELECT call.outcome, reservation.status, reservation.token_reservation, reservation.slot_held,
					reservation.input_tokens, reservation.output_tokens, reservation.actual_tokens
				FROM runtime.model_calls AS call JOIN runtime.model_token_reservations AS reservation ON reservation.call_id = call.id
				WHERE call.run_id = $1`, world.runID).Scan(&outcome, &reservationStatus, &reservedTokens, &slotHeld, &input, &output, &actual); err != nil {
				t.Fatal(err)
			}
			if outcome != string(budget.CallUsageUnknown) || reservationStatus != "usage_unknown" || reservedTokens <= 0 || !slotHeld ||
				input != nil || output != nil || actual != nil {
				t.Fatalf("call %s, reservation %s (%d tokens, slot held %v), counts %v %v %v", outcome, reservationStatus, reservedTokens, slotHeld, input, output, actual)
			}

			// What the operator reads: one unknown call, the reservation held, nothing settled.
			usage := readUsage(t, world)
			encoded, _ := json.Marshal(usage)
			agent := usage.ModelCalls[0]
			if agent.Purpose != "agent" || agent.Dispatched != 1 || agent.UsageUnknown != 1 || agent.UsageUnknownReservations != 1 ||
				agent.SettledTokens != 0 || agent.HeldTokens != reservedTokens || agent.Completed != 0 {
				t.Fatalf("agent usage: %s", encoded)
			}
			if usage.Ledger == nil || usage.Ledger.Tokens.Used != 0 || usage.Ledger.Tokens.Reserved != reservedTokens || usage.Ledger.CallsInFlight != 1 {
				t.Fatalf("ledger: %s", encoded)
			}
			t.Logf("evidence X-53 (%s): provider requests 1; run paused/outcome_unknown; reservation usage_unknown, %d tokens and the slot held; "+
				"reported tokens 0, usage unknown 1, ledger used 0 reserved %d; explanation %q", name, reservedTokens, reservedTokens, safeMessage)
		})
	}
}
