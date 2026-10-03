package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"time"

	"github.com/jackc/pgx/v5"

	"starter/services/gateway/internal/catalog"
)

// activationDatabase is the gateway pool, or an enclosing transaction in tests.
type activationDatabase interface {
	catalog.Beginner
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// Another gateway instance (or this command twice) may hold the activation lock for a moment.
const busyAttempts = 10

var busyWait = 300 * time.Millisecond

// safeCode accepts the fixed identifier the activation records as a rejection code.
var safeCode = regexp.MustCompile(`^[a-z0-9_]{1,64}$`)

// activate runs the gateway's own activation once and decides the exit code from the resulting
// pointer, not only from the outcome: the command exists so the catalog is enforceable, so it
// fails when nothing is active, and when the requested revision is not the active one (a rejected
// request stays visible instead of looking idle). It prints only fixed texts and the recorded
// rejection code, never policy or feed content.
func activate(ctx context.Context, db activationDatabase, stdout, stderr io.Writer) int {
	var outcome catalog.ActivationOutcome
	for attempt := 1; ; attempt++ {
		var err error
		outcome, err = catalog.ActivateRequested(ctx, db)
		if err != nil {
			fmt.Fprintln(stderr, "catalog activation: FAIL (the activation could not run; is the database migrated? pnpm db:migration:run, then pnpm db:roles)")
			return 1
		}
		if outcome != catalog.ActivationBusy {
			break
		}
		if attempt >= busyAttempts {
			fmt.Fprintln(stderr, "catalog activation: FAIL (another instance holds the activation lock)")
			return 1
		}
		select {
		case <-ctx.Done():
			fmt.Fprintln(stderr, "catalog activation: FAIL (interrupted)")
			return 1
		case <-time.After(busyWait):
		}
	}

	var requested, validated, active, feed *int64
	var lastError []byte
	err := db.QueryRow(ctx, `SELECT requested_revision_id, validated_revision_id, active_revision_id, active_feed_revision_id, last_error
		FROM app.control_catalog_pointer WHERE id = 1`).Scan(&requested, &validated, &active, &feed, &lastError)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		fmt.Fprintln(stderr, "catalog activation: FAIL (the pointer could not be read)")
		return 1
	}
	switch {
	case active == nil:
		fmt.Fprintf(stderr, "catalog activation: FAIL (no active catalog%s; import a policy with pnpm policy:import)\n", rejectionNote(lastError))
		return 1
	case requested != nil && *requested != *active:
		fmt.Fprintf(stderr, "catalog activation: FAIL (the requested revision %d was rejected%s; revision %d stays active)\n",
			*requested, rejectionNote(lastError), *active)
		return 1
	case validated == nil || *validated != *active:
		// An active revision the gateway never validated, for example one an old import bootstrapped
		// (validated and feed empty): the activation sees nothing pending, but admission would fail closed.
		fmt.Fprintf(stderr, "catalog activation: FAIL (revision %d is active but was never validated by the gateway; import the policy again with pnpm policy:import)\n", *active)
		return 1
	}
	// The criterion is what the gateway itself loads: the active revision with the feed it needs.
	if _, err := catalog.NewLoader().Active(ctx, db); err != nil {
		fmt.Fprintf(stderr, "catalog activation: FAIL (the active revision %d cannot be loaded as an enforceable catalog, for example a policy that needs a signature feed has none; import the policy again)\n", *active)
		return 1
	}
	switch {
	case outcome == catalog.ActivationActivated:
		fmt.Fprintf(stdout, "catalog activation: activated revision %d%s\n", *active, feedNote(feed))
	default:
		fmt.Fprintf(stdout, "catalog activation: nothing to activate; revision %d is active%s\n", *active, feedNote(feed))
	}
	return 0
}

func feedNote(feed *int64) string {
	if feed == nil {
		return " (no signature feed)"
	}
	return fmt.Sprintf(" with signature feed revision %d", *feed)
}

// rejectionNote returns " (code: <code>)" for the recorded last error when it carries a safe code
// (the gateway's own rejection records a code; the import's records a reason), else "".
func rejectionNote(lastError []byte) string {
	if len(lastError) == 0 {
		return ""
	}
	var record struct {
		Code   string `json:"code"`
		Reason string `json:"reason"`
	}
	if json.Unmarshal(lastError, &record) != nil {
		return ""
	}
	for _, candidate := range []string{record.Code, record.Reason} {
		if safeCode.MatchString(candidate) {
			return ", code: " + candidate
		}
	}
	return ""
}
