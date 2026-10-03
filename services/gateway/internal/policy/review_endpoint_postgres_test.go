package policy

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"starter/services/gateway/internal/contracts"
	"starter/services/gateway/internal/operatorcontext"
	"starter/services/gateway/internal/testdb"
)

func reviewRequest(actionID string, operator *contracts.OperatorContext) *http.Request {
	request := httptest.NewRequest(http.MethodGet, "/internal/actions/"+actionID+"/review", nil)
	request.SetPathValue("actionId", actionID)
	if operator != nil {
		request = request.WithContext(operatorcontext.WithOperator(request.Context(), *operator))
	}
	return request
}

func TestReviewEndpointServesTheFrozenPayloadToReviewersOnly(t *testing.T) {
	world := openApprovalWorld(t)
	handler := ReviewHandler(NewApprovals(world.pool))

	// The reviewer gets the frozen payload; its content equals the stored frozen content byte for byte.
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, reviewRequest(world.actionID, &world.reviewer))
	if recorder.Code != http.StatusOK {
		t.Fatalf("reviewer: status %d, body %s", recorder.Code, recorder.Body)
	}
	var served ReviewPayload
	if err := json.Unmarshal(recorder.Body.Bytes(), &served); err != nil {
		t.Fatal(err)
	}
	var frozenContent, frozenAddress string
	if err := world.pool.QueryRow(context.Background(),
		`SELECT payload->'report'->>'content', payload->'recipient'->>'address' FROM runtime.review_payloads WHERE action_id = $1`,
		world.actionID).Scan(&frozenContent, &frozenAddress); err != nil {
		t.Fatal(err)
	}
	if served.Report == nil || served.Report.Content != frozenContent || frozenContent != world.reportBody ||
		served.Recipient == nil || served.Recipient.Address != frozenAddress || served.Report.ID != world.reportID {
		t.Fatalf("served payload does not equal the frozen payload: %+v", served)
	}

	// A non-reviewer and a reviewer of another organization get the error envelope and no content.
	otherOrganization := testdb.ID(t)
	mustExec(t, world.pool, `INSERT INTO app.organizations (id, name) VALUES ($1, $2)`, otherOrganization, "Other "+otherOrganization[:8])
	outsiderID := testdb.ID(t)
	mustExec(t, world.pool, `INSERT INTO app.users (id, email, name) VALUES ($1, $2, 'Outsider')`, outsiderID, outsiderID+"@example.test")
	mustExec(t, world.pool, `INSERT INTO app.memberships ("userId", "organizationId", roles) VALUES ($1, $2, '{reviewer}')`, outsiderID, otherOrganization)
	outsider := contracts.OperatorContext{UserID: outsiderID, OrganizationID: otherOrganization, Roles: []string{"reviewer"}}
	refusals := []struct {
		name       string
		operator   *contracts.OperatorContext
		wantStatus int
	}{
		{"operator without the reviewer role", &world.operator, http.StatusForbidden},
		{"reviewer of another organization", &outsider, http.StatusNotFound},
		{"no operator context", nil, http.StatusUnauthorized},
	}
	for _, refusal := range refusals {
		t.Run(refusal.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, reviewRequest(world.actionID, refusal.operator))
			body := recorder.Body.String()
			if recorder.Code != refusal.wantStatus || !strings.Contains(body, `"error"`) ||
				strings.Contains(body, world.reportBody) || strings.Contains(body, world.address) {
				t.Fatalf("status %d, body %s", recorder.Code, body)
			}
		})
	}
	t.Logf("evidence X-41: reviewer read %d content bytes equal to the frozen payload; non-reviewer 403, other organization 404, no context 401, none with content",
		len(served.Report.Content))
}
