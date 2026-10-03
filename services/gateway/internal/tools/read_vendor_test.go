package tools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestReadVendorReturnsAReferenceNotTheAddress(t *testing.T) {
	world := openWorld(t, func(world *testWorld) passportScope { return scenarioScope(world, true) })
	request := world.proposeAction(t, world.nextStep(), ToolReadVendor, map[string]string{"vendor_id": world.atlasID})
	result, err := Runner{}.RunEffect(context.Background(), world.tx, request)
	if err != nil {
		t.Fatalf("RunEffect: %v", err)
	}
	if result.Outcome != OutcomeSucceeded {
		t.Fatalf("result = %+v, want succeeded", result)
	}
	minimized, err := MinimizeForModel(ToolReadVendor, result)
	if err != nil {
		t.Fatal(err)
	}
	serialized := string(minimized.JSON)
	if strings.Contains(serialized, "@") || strings.Contains(serialized, "atlas.example.com") {
		t.Fatalf("the registered address reached the model-facing result: %s", serialized)
	}
	var fields map[string]any
	if err := json.Unmarshal(minimized.JSON, &fields); err != nil {
		t.Fatal(err)
	}
	if len(fields) != 4 || fields["recipient_reference"] != recipientReference(world.runID, world.atlasID) {
		t.Fatalf("fields = %v, want vendor_id, version, name and the run-scoped reference", fields)
	}
}

func TestReadVendorRefusesOtherOrganizationUnlinkedAndOutOfScopeVendors(t *testing.T) {
	var unlinkedID string
	world := openWorld(t, func(world *testWorld) passportScope {
		unlinkedID = "vendor_Unlinked_" + world.atlasID
		passport := scenarioScope(world, true)
		// Both are listed, but Borealis belongs to the other organization and the unlinked vendor
		// has no passport-scoped invoice.
		passport.VendorIDs = append(passport.VendorIDs, world.borealisID, unlinkedID)
		return passport
	})
	world.exec(t, `INSERT INTO demo.vendors (id, organization_id, name, registered_reporting_address) VALUES ($1, $2, 'Unlinked', 'x@unlinked.example.com')`,
		unlinkedID, world.organizationID)
	for _, vendorID := range []string{world.borealisID, unlinkedID, "vendor_not_in_scope"} {
		request := world.proposeAction(t, world.nextStep(), ToolReadVendor, map[string]string{"vendor_id": vendorID})
		result, err := Runner{}.RunEffect(context.Background(), world.tx, request)
		if err != nil {
			t.Fatalf("%s: RunEffect: %v", vendorID, err)
		}
		if result.Outcome != OutcomeFailed || result.ReasonCode != ReasonResourceOutOfScope || result.ModelFacing != nil {
			t.Fatalf("%s: result = %+v, want failed resource_out_of_scope with no data", vendorID, result)
		}
	}
}
