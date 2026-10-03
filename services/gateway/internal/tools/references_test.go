package tools

import (
	"context"
	"strings"
	"testing"

	"starter/services/gateway/internal/testdb"
)

func TestResolveRecipientOnlyInsideItsRunAndScope(t *testing.T) {
	var unaddressedID string
	world := openWorld(t, func(world *testWorld) passportScope {
		unaddressedID = "vendor_NoAddress_" + world.atlasID
		passport := scenarioScope(world, true)
		passport.VendorIDs = append(passport.VendorIDs, world.borealisID, unaddressedID)
		passport.RecipientReferences = []string{
			recipientReference(world.runID, world.atlasID),
			recipientReference(world.runID, world.borealisID),
			recipientReference(world.runID, unaddressedID),
		}
		return passport
	})
	// A same-organization vendor with a scoped invoice but no registered address.
	world.exec(t, `INSERT INTO demo.vendors (id, organization_id, name) VALUES ($1, $2, 'No address')`, unaddressedID, world.organizationID)
	world.exec(t, `INSERT INTO demo.invoices (id, organization_id, vendor_id, external_reference, currency, total_minor_units, issued_on, due_on)
	               VALUES ($1, $2, $3, 'INV900', 'EUR', 1, '2026-09-01', '2026-09-30')`,
		world.invoiceA02+"_x", world.organizationID, unaddressedID)
	current, err := loadScope(context.Background(), world.tx, EffectRequest{
		OrganizationID: world.organizationID, RunID: world.runID, PassportID: world.passportID,
	})
	if err != nil {
		t.Fatalf("loadScope: %v", err)
	}
	current.passport.InvoiceIDs = append(current.passport.InvoiceIDs, world.invoiceA02+"_x")

	recipient, reason, err := resolveRecipient(context.Background(), world.tx, current, recipientReference(world.runID, world.atlasID))
	if err != nil || reason != "" || recipient.address != "reports@atlas.example.com" || recipient.vendorID != world.atlasID {
		t.Fatalf("valid reference: %+v %q %v", recipient, reason, err)
	}

	refused := map[string]string{
		"another run":                 recipientReference(testdb.ID(t), world.atlasID),
		"another organization vendor": recipientReference(world.runID, world.borealisID),
		"vendor without address":      recipientReference(world.runID, unaddressedID),
		"not listed in the passport":  recipientReference(world.runID, "vendor_unknown"),
		"malformed":                   "recipient:" + world.runID,
		"raw address":                 "reports@atlas.example.com",
	}
	for name, reference := range refused {
		recipient, reason, err := resolveRecipient(context.Background(), world.tx, current, reference)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if reason != ReasonDestinationNotAllowed || recipient.address != "" {
			t.Errorf("%s: resolved to %+v (%q), want destination_not_allowed", name, recipient, reason)
		}
	}
}

func TestNoToolResultCarriesTheRegisteredAddress(t *testing.T) {
	world := openWorld(t, func(world *testWorld) passportScope { return scenarioScope(world, true) })
	for _, call := range []struct {
		tool      string
		arguments map[string]string
	}{
		{ToolReadInvoice, map[string]string{"invoice_id": world.invoiceA01}},
		{ToolReadVendor, map[string]string{"vendor_id": world.atlasID}},
	} {
		request := world.proposeAction(t, world.nextStep(), call.tool, call.arguments)
		result, err := Runner{}.RunEffect(context.Background(), world.tx, request)
		if err != nil {
			t.Fatalf("%s: %v", call.tool, err)
		}
		minimized, err := MinimizeForModel(call.tool, result)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(minimized.JSON), "atlas.example.com") {
			t.Errorf("%s result holds the registered address: %s", call.tool, minimized.JSON)
		}
	}
}
