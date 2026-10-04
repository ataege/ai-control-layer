package reads

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/jackc/pgx/v5"

	"starter/services/gateway/internal/catalog"
	"starter/services/gateway/internal/security"
)

// CatalogStatusRoutePattern is the control catalog status read (WEB-29). The catalog is global to
// the gateway, so the answer is organization-agnostic; the route still runs behind the service
// token and the verified operator context like every internal read.
const CatalogStatusRoutePattern = "GET /internal/catalog/active"

// CatalogStatus is what the security page shows of the active control catalog: revisions, digests,
// the last rejected activation and each registered control's setting. It never carries the policy
// or feed text, a signature pattern or a secret.
type CatalogStatus struct {
	// ActiveRevisionID is null while no revision has been activated yet.
	ActiveRevisionID    *int64 `json:"activeRevisionId"`
	RequestedRevisionID *int64 `json:"requestedRevisionId"`
	ValidatedRevisionID *int64 `json:"validatedRevisionId"`
	// PolicyDigest is the SHA-256 of the active policy file as imported.
	PolicyDigest *string `json:"policyDigest"`
	// Feed fields are null when no signature feed is bound to the active revision.
	FeedRevisionID *int64  `json:"feedRevisionId"`
	FeedRevision   *string `json:"feedRevision"`
	FeedDigest     *string `json:"feedDigest"`
	FeedRuleCount  *int    `json:"feedRuleCount"`
	// LastError is the safe record of the last rejected activation; null when the last one succeeded.
	LastError *CatalogLastError `json:"lastError"`
	Controls  []CatalogControl  `json:"controls"`
	// DisabledRules lists signature rule ids the active revision switches off.
	DisabledRules []string `json:"disabledRules"`
}

// CatalogLastError is the decided shape of pointer.last_error: stable codes and a revision, no
// file content.
type CatalogLastError struct {
	Code       string `json:"code"`
	Message    string `json:"message"`
	RevisionID int64  `json:"revisionId"`
	Stage      string `json:"stage"`
}

// CatalogControl is one registered control's setting in the active revision.
type CatalogControl struct {
	ControlID    string `json:"controlId"`
	ControlClass string `json:"controlClass"`
	Enabled      bool   `json:"enabled"`
	// Mode is null for signature_match: each feed rule carries its own response.
	Mode *string `json:"mode"`
	// Threshold is set on the semantic control only.
	Threshold  *float64 `json:"threshold"`
	Boundaries []string `json:"boundaries"`
}

// CatalogStatusHandler serves the status of the active catalog. A missing pointer or a stored
// revision the gateway cannot enforce is 503, never a partial answer; before the first activation
// it answers 200 with null revisions and the last error, if any.
func CatalogStatusHandler(database Beginner, loader *catalog.Loader) http.Handler {
	return http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		if _, ok := verifiedOrganization(responseWriter, request); !ok {
			return
		}
		if database == nil || loader == nil {
			writeUnavailable(responseWriter, request)
			return
		}
		var status CatalogStatus
		err := readTransaction(request.Context(), database, func(tx pgx.Tx) error {
			var readErr error
			status, readErr = readCatalogStatus(request.Context(), tx, loader)
			return readErr
		})
		if err != nil {
			writeUnavailable(responseWriter, request)
			return
		}
		writeJSON(responseWriter, status)
	})
}

// readCatalogStatus reads the pointer and, when a revision is active, its enforceable snapshot in
// one transaction so both describe the same moment.
func readCatalogStatus(ctx context.Context, tx pgx.Tx, loader *catalog.Loader) (CatalogStatus, error) {
	ctx, cancel := context.WithTimeout(ctx, readTimeout)
	defer cancel()
	status := CatalogStatus{Controls: []CatalogControl{}, DisabledRules: []string{}}
	var lastError []byte
	err := tx.QueryRow(ctx, `SELECT pointer.requested_revision_id, pointer.validated_revision_id,
		pointer.active_revision_id, pointer.last_error, revision.file_digest
		FROM app.control_catalog_pointer AS pointer
		LEFT JOIN app.control_catalog_revisions AS revision ON revision.id = pointer.active_revision_id
		WHERE pointer.id = 1`).Scan(&status.RequestedRevisionID, &status.ValidatedRevisionID,
		&status.ActiveRevisionID, &lastError, &status.PolicyDigest)
	if err != nil {
		return CatalogStatus{}, err
	}
	status.LastError = decodeLastError(lastError)
	if status.ActiveRevisionID == nil {
		return status, nil
	}
	snapshot, err := loader.Active(ctx, tx)
	if err != nil || snapshot.RevisionID != *status.ActiveRevisionID {
		return CatalogStatus{}, errors.Join(errMalformedRecord, err)
	}
	settings := snapshot.Security
	status.FeedRevisionID = snapshot.FeedRevisionID
	if settings.Feed != nil {
		revision, digest, ruleCount := settings.Feed.Revision, settings.Feed.Digest, len(settings.Feed.Rules)
		status.FeedRevision, status.FeedDigest, status.FeedRuleCount = &revision, &digest, &ruleCount
	}
	status.Controls = []CatalogControl{
		guardControl("secret_pattern", security.ClassDeterministic, settings.SecretPattern, true),
		{
			ControlID: "semantic_injection", ControlClass: string(security.ClassSemantic),
			Enabled: settings.SemanticInjection.Enabled, Mode: modeOf(settings.SemanticInjection.GuardSettings),
			Threshold: thresholdOf(settings.SemanticInjection), Boundaries: boundaryNames(settings.SemanticInjection.Boundaries),
		},
		guardControl("signature_match", security.ClassDeterministic, settings.SignatureMatch, false),
	}
	if settings.DisabledRules != nil {
		status.DisabledRules = append([]string{}, settings.DisabledRules...)
	}
	return status, nil
}

func guardControl(controlID string, class security.ControlClass, guard security.GuardSettings, hasMode bool) CatalogControl {
	control := CatalogControl{ControlID: controlID, ControlClass: string(class), Enabled: guard.Enabled, Boundaries: boundaryNames(guard.Boundaries)}
	if hasMode {
		control.Mode = modeOf(guard)
	}
	return control
}

// modeOf is the configured response of an enabled guard; a disabled guard has none to show.
func modeOf(guard security.GuardSettings) *string {
	if !guard.Enabled || guard.Mode == "" {
		return nil
	}
	mode := string(guard.Mode)
	return &mode
}

func thresholdOf(semantic security.SemanticSettings) *float64 {
	if !semantic.Enabled {
		return nil
	}
	threshold := semantic.Threshold
	return &threshold
}

func boundaryNames(boundaries []security.Boundary) []string {
	names := make([]string, 0, len(boundaries))
	for _, boundary := range boundaries {
		names = append(names, string(boundary))
	}
	return names
}

// importRejectionReason is the `reason` the API's importer stores when a policy file fails
// validation (apps/api policy-file.ts). That record carries the file's issues, name and digest,
// none of which are passed on.
const importRejectionReason = "policy_reload_rejected"

// decodeLastError keeps only the four decided fields of the stored record. The gateway's own
// rejection carries them. An importer rejection has no code: it is reported with a fixed code,
// message and the import_validation stage, and nothing else from it. Any other record that does
// not parse is reported with a fixed code, never echoed.
func decodeLastError(stored []byte) *CatalogLastError {
	if len(stored) == 0 {
		return nil
	}
	var record struct {
		Reason     string `json:"reason"`
		Code       string `json:"code"`
		Message    string `json:"message"`
		RevisionID int64  `json:"revision_id"`
		Stage      string `json:"stage"`
	}
	if json.Unmarshal(stored, &record) != nil {
		return unreadableLastError()
	}
	if record.Code != "" {
		return &CatalogLastError{Code: record.Code, Message: record.Message, RevisionID: record.RevisionID, Stage: record.Stage}
	}
	if record.Reason == importRejectionReason && (record.Stage == "" || record.Stage == "import_validation") {
		return &CatalogLastError{Code: importRejectionReason, Message: "The policy file failed validation.", Stage: "import_validation"}
	}
	return unreadableLastError()
}

func unreadableLastError() *CatalogLastError {
	return &CatalogLastError{Code: "last_error_unreadable", Message: "The stored activation error could not be read.", Stage: "unknown"}
}
