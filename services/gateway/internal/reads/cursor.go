package reads

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

// windowCursor pages the organization-wide records (events and control assessments) exactly
// once, without the per-run lock those writers do not share.
//
// Identity values are allocated at insert, not commit, so paging by id alone could pass a row
// that commits later. Instead each row belongs to the window of its inserting transaction id:
// a read fixes High at the oldest transaction still running (pg_snapshot_xmin), so every
// transaction below High has finished and its rows are final. One window [Low, High) is drained
// in id order, then the next starts at Low = High. Windows never overlap and never leave a gap,
// so every committed row is returned once. Rows of a long transaction appear when it finishes,
// so pages are in id order within a window, not across windows: consumers sort by id.
//
// A long-running or idle-in-transaction session anywhere in the database holds the oldest running
// transaction back, so High stops advancing: pages come back empty and nextCursor stays the same
// until that session ends. Nothing is lost; the rows appear on the next read after it ends.
//
// Limitation: the comparison uses age(xmin), which is exact while a row is younger than about
// two billion transactions; older rows (after transaction id wraparound) are not paged reliably.
type windowCursor struct {
	// Low is the window's first transaction id (xid8); 0 starts at the oldest row.
	Low uint64
	// High is the window's end, fixed by the read that opened it; 0 when no window is open.
	High uint64
	// AfterID is the last record id returned from the open window.
	AfterID int64
}

const cursorVersion = "v1"

// String encodes the cursor as "v1.<low>.<high>.<afterId>".
func (cursor windowCursor) String() string {
	return fmt.Sprintf("%s.%d.%d.%d", cursorVersion, cursor.Low, cursor.High, cursor.AfterID)
}

// parseWindowCursor accepts "" (the start) or a cursor this package issued. A cursor is a
// position, never authority: the organization always comes from the verified operator.
func parseWindowCursor(text string) (windowCursor, bool) {
	if text == "" {
		return windowCursor{}, true
	}
	parts := strings.Split(text, ".")
	if len(parts) != 4 || parts[0] != cursorVersion {
		return windowCursor{}, false
	}
	low, lowErr := strconv.ParseUint(parts[1], 10, 64)
	high, highErr := strconv.ParseUint(parts[2], 10, 64)
	afterID, afterErr := strconv.ParseInt(parts[3], 10, 64)
	if lowErr != nil || highErr != nil || afterErr != nil || afterID < 0 {
		return windowCursor{}, false
	}
	// An open window has an end above its start; a closed one has no record position.
	if (high != 0 && high <= low) || (high == 0 && afterID != 0) {
		return windowCursor{}, false
	}
	return windowCursor{Low: low, High: high, AfterID: afterID}, true
}

// next returns the cursor after a page. more says the window holds rows beyond this page (the
// read fetched one extra); high is the window end the read used: the cursor's own when it was
// open, else the one the read fixed, or 0 when it returned no row and so learned none.
func (cursor windowCursor) next(lastID int64, more bool, high uint64) windowCursor {
	if cursor.High != 0 {
		high = cursor.High
	}
	switch {
	case more && high != 0:
		return windowCursor{Low: cursor.Low, High: high, AfterID: lastID}
	case high != 0:
		return windowCursor{Low: high}
	default:
		// Nothing was final yet: keep the start and open a fresh window next time.
		return windowCursor{Low: cursor.Low}
	}
}

// windowPageQuery reads `cursor` and `limit` (1 to 500, default 100); anything else is 400.
func windowPageQuery(responseWriter http.ResponseWriter, request *http.Request) (windowCursor, int, bool) {
	text := ""
	if values, present := request.URL.Query()["cursor"]; present {
		text = onlyValue(values)
		if text == "" {
			writeBadQuery(responseWriter, request)
			return windowCursor{}, 0, false
		}
	}
	cursor, valid := parseWindowCursor(text)
	if !valid {
		writeBadQuery(responseWriter, request)
		return windowCursor{}, 0, false
	}
	limit, ok := pageLimit(responseWriter, request)
	return cursor, limit, ok
}

// optionalXid8 passes 0 as SQL NULL ("no bound") and other values as text for an xid8 cast.
func optionalXid8(value uint64) *string {
	if value == 0 {
		return nil
	}
	text := strconv.FormatUint(value, 10)
	return &text
}

// windowPredicate restricts a table aliased "record" to the window: $2 Low, $3 High (both text
// xid8 or NULL, High NULL meaning "the oldest running transaction now"), $4 AfterID. It must run
// in the same statement as the horizon so the scan never sees a later snapshot than High.
const windowPredicate = `($2::text IS NULL OR age(record.xmin) <= age($2::text::xid8::xid))
	AND age(record.xmin) > age(horizon.high::xid)
	AND record.id > $4`

// windowHorizon is the CTE that fixes High for one statement.
const windowHorizon = `WITH horizon AS (
	SELECT coalesce($3::text::xid8, pg_snapshot_xmin(pg_current_snapshot())) AS high)`
