package reads

import (
	"context"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"starter/services/gateway/internal/contracts"
	"starter/services/gateway/internal/repository"
)

// fakePassports records what the handler asked for and answers from fixed values.
type fakePassports struct {
	passport        contracts.Passport
	err             error
	gotOrganization string
	gotRun          string
	calls           int
}

func (fake *fakePassports) Passport(_ context.Context, organizationID, runID string) (contracts.Passport, error) {
	fake.calls++
	fake.gotOrganization, fake.gotRun = organizationID, runID
	return fake.passport, fake.err
}

func TestPassportServesTheX08PassportOfTheOperatorsOrganization(t *testing.T) {
	var fixture contracts.Passport
	readFixture(t, "passport.atlas.json", &fixture)
	reader := &fakePassports{passport: fixture}
	// The organization in the query string is ignored: only the verified operator counts.
	recorder := serve(t, PassportRoutePattern, PassportHandler(reader),
		"/internal/runs/"+testRunID+"/passport?organizationId=00000000-0000-4000-8000-000000000000", &testOperator)
	if recorder.Code != http.StatusOK || recorder.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("status %d: %s", recorder.Code, recorder.Body.String())
	}
	var served contracts.Passport
	if err := contracts.DecodeStrict(recorder.Body.Bytes(), &served); err != nil || !reflect.DeepEqual(served, fixture) {
		t.Fatalf("served %+v, want the fixture", served)
	}
	if reader.gotOrganization != testOperator.OrganizationID || reader.gotRun != testRunID {
		t.Fatalf("read %s/%s", reader.gotOrganization, reader.gotRun)
	}
}

func TestPassportRejectsWithoutDataBeforeAnyRead(t *testing.T) {
	cases := []struct {
		name     string
		target   string
		operator *contracts.OperatorContext
		status   int
		code     string
	}{
		{"no operator", "/internal/runs/" + testRunID + "/passport", nil, http.StatusUnauthorized, "unauthorized"},
		{"operator without organization", "/internal/runs/" + testRunID + "/passport", &contracts.OperatorContext{UserID: testOperator.UserID}, http.StatusUnauthorized, "unauthorized"},
		{"malformed run id", "/internal/runs/not-a-run/passport", &testOperator, http.StatusNotFound, "not_found"},
	}
	for _, testCase := range cases {
		reader := &fakePassports{passport: contracts.Passport{PassportID: "leak"}}
		recorder := serve(t, PassportRoutePattern, PassportHandler(reader), testCase.target, testCase.operator)
		expectError(t, recorder, testCase.status, testCase.code)
		if reader.calls != 0 {
			t.Fatalf("%s: the reader was called for a rejected request", testCase.name)
		}
	}
}

func TestPassportMapsRepositoryErrorsWithoutLeakingData(t *testing.T) {
	cases := []struct {
		err    error
		status int
		code   string
	}{
		// Another organization's run is not found, exactly like a run that does not exist.
		{repository.ErrNotFound, http.StatusNotFound, "not_found"},
		{repository.ErrUnavailable, http.StatusServiceUnavailable, "unavailable"},
	}
	for _, testCase := range cases {
		reader := &fakePassports{err: testCase.err, passport: contracts.Passport{PassportID: "leak"}}
		recorder := serve(t, PassportRoutePattern, PassportHandler(reader), "/internal/runs/"+testRunID+"/passport", &testOperator)
		expectError(t, recorder, testCase.status, testCase.code)
		if strings.Contains(recorder.Body.String(), "leak") {
			t.Fatal("passport data in an error body")
		}
	}
	expectError(t, serve(t, PassportRoutePattern, PassportHandler(nil), "/internal/runs/"+testRunID+"/passport", &testOperator),
		http.StatusServiceUnavailable, "unavailable")
}
