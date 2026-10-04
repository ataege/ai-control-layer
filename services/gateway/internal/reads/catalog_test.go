package reads

import (
	"net/http"
	"strings"
	"testing"

	"starter/services/gateway/internal/catalog"
	"starter/services/gateway/internal/catalog/catalogtest"
	"starter/services/gateway/internal/contracts"
)

func TestCatalogStatusRejectsWithoutOperatorAndWithoutDependencies(t *testing.T) {
	expectError(t, serve(t, CatalogStatusRoutePattern, CatalogStatusHandler(nil, catalog.NewLoader()), "/internal/catalog/active", nil),
		http.StatusUnauthorized, "unauthorized")
	expectError(t, serve(t, CatalogStatusRoutePattern, CatalogStatusHandler(nil, catalog.NewLoader()), "/internal/catalog/active", &testOperator),
		http.StatusServiceUnavailable, "unavailable")
}

func TestDecodeLastErrorKeepsOnlyTheDecidedFields(t *testing.T) {
	stored := `{"reason":"policy_reload_rejected","code":"catalog_invalid","message":"The revision failed validation.","revision_id":7,"stage":"gateway_validation","source_text":"must not pass"}`
	got := decodeLastError([]byte(stored))
	want := CatalogLastError{Code: "catalog_invalid", Message: "The revision failed validation.", RevisionID: 7, Stage: "gateway_validation"}
	if got == nil || *got != want {
		t.Fatalf("got %+v, want %+v", got, want)
	}
	if decodeLastError(nil) != nil {
		t.Fatal("no stored error must be null")
	}
	// A record that does not parse is never echoed.
	unreadable := decodeLastError([]byte(`"secret text"`))
	if unreadable == nil || unreadable.Code != "last_error_unreadable" || strings.Contains(unreadable.Message, "secret") {
		t.Fatalf("unreadable record %+v", unreadable)
	}
}

func TestDecodeLastErrorMapsAnImportRejectionWithoutEchoingIt(t *testing.T) {
	// The record the API's importer writes when a policy file fails validation.
	stored := `{"reason":"policy_reload_rejected","source_file_name":"secret-file.yaml","file_digest":"` + strings.Repeat("a", 64) + `","issues":[{"path":"controls.semantic_injection.threshold","message":"secret issue text"}]}`
	got := decodeLastError([]byte(stored))
	want := CatalogLastError{Code: "policy_reload_rejected", Message: "The policy file failed validation.", Stage: "import_validation"}
	if got == nil || *got != want {
		t.Fatalf("got %+v, want %+v", got, want)
	}
	for _, leaked := range []string{"secret", "threshold", "aaaa"} {
		if strings.Contains(got.Message, leaked) || strings.Contains(got.Code, leaked) {
			t.Fatalf("import rejection echoes %q: %+v", leaked, got)
		}
	}
	// The same record with the stage the lead named is the same rejection.
	staged := decodeLastError([]byte(`{"reason":"policy_reload_rejected","stage":"import_validation","issues":[]}`))
	if staged == nil || *staged != want {
		t.Fatalf("staged record %+v", staged)
	}
	// Anything else without a code fails closed.
	for name, record := range map[string]string{
		"another reason":    `{"reason":"something_else","issues":[]}`,
		"a foreign stage":   `{"reason":"policy_reload_rejected","stage":"gateway_validation"}`,
		"no reason":         `{"issues":[]}`,
		"an empty object":   `{}`,
		"not an object":     `[1]`,
		"a mistyped reason": `{"reason":7}`,
	} {
		unreadable := decodeLastError([]byte(record))
		if unreadable == nil || unreadable.Code != "last_error_unreadable" || unreadable.Stage != "unknown" {
			t.Fatalf("%s: got %+v", name, unreadable)
		}
	}
}

func TestPostgresCatalogStatusShowsTheActiveRevisionControlsAndLastError(t *testing.T) {
	_, tx := openTransaction(t)
	revisionID := catalogtest.ActivatePolicy(t, tx)
	handler := CatalogStatusHandler(tx, catalog.NewLoader())

	read := func() CatalogStatus {
		t.Helper()
		recorder := serve(t, CatalogStatusRoutePattern, handler, "/internal/catalog/active", &testOperator)
		var status CatalogStatus
		if recorder.Code != http.StatusOK || contracts.DecodeStrict(recorder.Body.Bytes(), &status) != nil {
			t.Fatalf("status %d: %s", recorder.Code, recorder.Body.String())
		}
		// The policy and feed text never leave the gateway.
		for _, forbidden := range []string{"source_text", "catalogtest", `"pattern"`, "prompt_ignore_previous_v1"} {
			if strings.Contains(recorder.Body.String(), forbidden) {
				t.Fatalf("body carries %q: %s", forbidden, recorder.Body.String())
			}
		}
		return status
	}

	status := read()
	if status.ActiveRevisionID == nil || *status.ActiveRevisionID != revisionID || status.PolicyDigest == nil || *status.PolicyDigest != strings.Repeat("c", 64) {
		t.Fatalf("active revision %+v, want %d", status, revisionID)
	}
	if status.FeedRevision == nil || *status.FeedRevision != "feed_v2" || status.FeedDigest == nil || len(*status.FeedDigest) != 64 ||
		status.FeedRuleCount == nil || *status.FeedRuleCount < 1 || status.LastError != nil {
		t.Fatalf("feed or last error wrong: %+v", status)
	}
	if len(status.Controls) != 3 || status.Controls[0].ControlID != "secret_pattern" || status.Controls[0].Mode == nil || *status.Controls[0].Mode != "redact" {
		t.Fatalf("controls %+v", status.Controls)
	}
	semantic := status.Controls[1]
	if semantic.ControlID != "semantic_injection" || semantic.ControlClass != "semantic" || !semantic.Enabled || semantic.Threshold == nil ||
		*semantic.Threshold != 0.75 || semantic.Mode == nil || *semantic.Mode != "block" || len(semantic.Boundaries) != 3 {
		t.Fatalf("semantic control %+v", semantic)
	}
	signature := status.Controls[2]
	if signature.ControlID != "signature_match" || signature.Mode != nil || signature.Threshold != nil || !signature.Enabled {
		t.Fatalf("signature control %+v", signature)
	}

	// A rejected request is shown beside the revision that stays active.
	insert(t, tx, `UPDATE app.control_catalog_pointer SET last_error = $1::jsonb, last_error_at = now(), validated_revision_id = $2 WHERE id = 1`,
		`{"reason":"policy_reload_rejected","code":"catalog_invalid","message":"The revision failed validation.","revision_id":99,"stage":"gateway_validation"}`, revisionID)
	status = read()
	if status.LastError == nil || status.LastError.Code != "catalog_invalid" || status.LastError.RevisionID != 99 || status.LastError.Stage != "gateway_validation" ||
		status.ValidatedRevisionID == nil || *status.ValidatedRevisionID != revisionID || *status.ActiveRevisionID != revisionID {
		t.Fatalf("after a rejection %+v", status)
	}
}

func TestPostgresCatalogStatusBeforeTheFirstActivationAndWithAnUnenforceableRevision(t *testing.T) {
	_, tx := openTransaction(t)
	handler := CatalogStatusHandler(tx, catalog.NewLoader())
	catalogtest.ActivatePolicy(t, tx)

	// No active revision yet: 200 with null revisions and the last error, no controls.
	insert(t, tx, `UPDATE app.control_catalog_pointer SET active_revision_id = NULL, active_feed_revision_id = NULL, validated_revision_id = NULL,
		last_error = '{"code":"signature_feed_missing","message":"The feed is missing.","revision_id":2,"stage":"gateway_validation"}'::jsonb, last_error_at = now() WHERE id = 1`)
	recorder := serve(t, CatalogStatusRoutePattern, handler, "/internal/catalog/active", &testOperator)
	var status CatalogStatus
	if recorder.Code != http.StatusOK || contracts.DecodeStrict(recorder.Body.Bytes(), &status) != nil {
		t.Fatalf("status %d: %s", recorder.Code, recorder.Body.String())
	}
	if status.ActiveRevisionID != nil || len(status.Controls) != 0 || status.LastError == nil || status.LastError.Code != "signature_feed_missing" ||
		!strings.Contains(recorder.Body.String(), `"controls":[]`) {
		t.Fatalf("before activation %+v: %s", status, recorder.Body.String())
	}

	// An active revision the gateway cannot enforce is 503, never a partial answer.
	catalogtest.Activate(t, tx, `{"schema_version": 2}`, nil)
	expectError(t, serve(t, CatalogStatusRoutePattern, handler, "/internal/catalog/active", &testOperator),
		http.StatusServiceUnavailable, "unavailable")
}
