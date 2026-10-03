package tools

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"sort"
	"testing"
)

func TestReadInvoiceReturnsExactlyTheAllowlistedFields(t *testing.T) {
	world := openWorld(t, func(world *testWorld) passportScope { return scenarioScope(world, true) })
	request := world.proposeAction(t, world.nextStep(), ToolReadInvoice, map[string]string{"invoice_id": world.invoiceA01})

	result, err := Runner{}.RunEffect(context.Background(), world.tx, request)
	if err != nil {
		t.Fatalf("RunEffect: %v", err)
	}
	if result.Outcome != OutcomeSucceeded || result.ReasonCode != "" {
		t.Fatalf("outcome = %q / %q, want succeeded", result.Outcome, result.ReasonCode)
	}
	serialized, err := json.Marshal(result.ModelFacing)
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]any
	if err := json.Unmarshal(serialized, &fields); err != nil {
		t.Fatal(err)
	}
	keys := make([]string, 0, len(fields))
	for key := range fields {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	want := []string{"currency", "due_on", "external_reference", "internal_note", "invoice_id",
		"issued_on", "total_minor_units", "vendor_id", "version"}
	if !reflect.DeepEqual(keys, want) {
		t.Fatalf("fields = %v, want %v", keys, want)
	}
	note := fields["internal_note"].(map[string]any)
	if note["classification"] != "internal_only" {
		t.Fatalf("note classification = %v, want internal_only", note["classification"])
	}
	if got := world.count(t, `SELECT count(*) FROM runtime.execution_attempts WHERE id = $1 AND outcome = 'succeeded' AND completed_at IS NOT NULL`, request.AttemptID); got != 1 {
		t.Fatalf("completed attempts = %d, want 1", got)
	}
	if got := world.count(t, `SELECT count(*) FROM runtime.audit_events WHERE action_id = $1`, request.ActionID); got != 1 {
		t.Fatalf("audit events = %d, want 1", got)
	}
}

func TestReadInvoiceOmitsTheNoteWhenTheFieldRuleDoesNotAllowIt(t *testing.T) {
	world := openWorld(t, func(world *testWorld) passportScope { return scenarioScope(world, false) })
	request := world.proposeAction(t, world.nextStep(), ToolReadInvoice, map[string]string{"invoice_id": world.invoiceA01})
	result, err := Runner{}.RunEffect(context.Background(), world.tx, request)
	if err != nil {
		t.Fatalf("RunEffect: %v", err)
	}
	if invoice := result.ModelFacing.(InvoiceResult); invoice.InternalNote != nil {
		t.Fatal("the note reached the result although internalNoteReadable is false")
	}
}

func TestReadInvoiceRefusesOutOfScopeAndOtherOrganizationInvoices(t *testing.T) {
	// invoice_C01 belongs to the other organization even though this scope list names it.
	world := openWorld(t, func(world *testWorld) passportScope {
		passport := scenarioScope(world, true)
		passport.InvoiceIDs = append(passport.InvoiceIDs, world.invoiceC01)
		return passport
	})
	for _, invoiceID := range []string{world.invoiceB01, world.invoiceC01} {
		request := world.proposeAction(t, world.nextStep(), ToolReadInvoice, map[string]string{"invoice_id": invoiceID})
		result, err := Runner{}.RunEffect(context.Background(), world.tx, request)
		if err != nil {
			t.Fatalf("%s: RunEffect: %v", invoiceID, err)
		}
		if result.Outcome != OutcomeFailed || result.ReasonCode != ReasonResourceOutOfScope || result.ModelFacing != nil {
			t.Fatalf("%s: result = %+v, want failed resource_out_of_scope with no data", invoiceID, result)
		}
	}
}

func TestReadInvoiceRejectsUnknownArgumentsAndMismatchedRequests(t *testing.T) {
	world := openWorld(t, func(world *testWorld) passportScope { return scenarioScope(world, true) })
	cases := map[string]func(*EffectRequest){
		"unknown argument": func(request *EffectRequest) {
			request.CanonicalArguments = json.RawMessage(`{"invoice_id":"` + world.invoiceA01 + `","classification":"vendor_shareable"}`)
		},
		"other organization": func(request *EffectRequest) { request.OrganizationID = world.otherOrganizationID },
		"changed digest":     func(request *EffectRequest) { request.ActionDigest[0] ^= 0xff },
		"wrong tool":         func(request *EffectRequest) { request.Tool = ToolQueueReport },
		"changed arguments": func(request *EffectRequest) {
			request.CanonicalArguments = json.RawMessage(`{"invoice_id":"` + world.invoiceA02 + `"}`)
		},
	}
	for name, change := range cases {
		t.Run(name, func(t *testing.T) {
			request := world.proposeAction(t, world.nextStep(), ToolReadInvoice, map[string]string{"invoice_id": world.invoiceA01})
			change(&request)
			if name == "unknown argument" {
				// The stored action holds the same arguments, so only the strict decoding can refuse it.
				world.exec(t, `UPDATE runtime.actions SET canonical_arguments = $1 WHERE id = $2`, request.CanonicalArguments, request.ActionID)
			}
			_, err := Runner{}.RunEffect(context.Background(), world.tx, request)
			if !errors.Is(err, errPrecondition) {
				t.Fatalf("err = %v, want a precondition error", err)
			}
		})
	}
}

func TestRunEffectRefusesACompletedAttempt(t *testing.T) {
	world := openWorld(t, func(world *testWorld) passportScope { return scenarioScope(world, true) })
	request := world.proposeAction(t, world.nextStep(), ToolReadInvoice, map[string]string{"invoice_id": world.invoiceA01})
	if _, err := (Runner{}).RunEffect(context.Background(), world.tx, request); err != nil {
		t.Fatalf("first run: %v", err)
	}
	if _, err := (Runner{}).RunEffect(context.Background(), world.tx, request); !errors.Is(err, errPrecondition) {
		t.Fatalf("second run err = %v, want a precondition error", err)
	}
	if got := world.count(t, `SELECT count(*) FROM runtime.audit_events WHERE action_id = $1`, request.ActionID); got != 1 {
		t.Fatalf("audit events = %d, want 1", got)
	}
}

func TestDecodeArgumentsIsStrict(t *testing.T) {
	var arguments readInvoiceArguments
	for _, raw := range []string{`{"invoice_id":"a","extra":1}`, `{"invoice_id":"a"} {"x":1}`, `[1]`, `not json`} {
		if err := decodeArguments(json.RawMessage(raw), &arguments); !errors.Is(err, errPrecondition) {
			t.Errorf("%s: err = %v, want a precondition error", raw, err)
		}
	}
	if err := decodeArguments(json.RawMessage(`{"invoice_id":"a"}`), &arguments); err != nil || arguments.InvoiceID != "a" {
		t.Fatalf("valid arguments: %v %+v", err, arguments)
	}
}
