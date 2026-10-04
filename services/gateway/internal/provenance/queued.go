package provenance

import (
	"context"

	"github.com/jackc/pgx/v5"
)

// rowQuerier is the part of a pool or a transaction the queued check needs.
type rowQuerier interface {
	QueryRow(ctx context.Context, sql string, arguments ...any) pgx.Row
}

// ReportAlreadyQueued reports whether a queue_report of this run has already succeeded for the
// report: its outbox row exists. A report is queued once per run, so a second queue_report for it
// would be a duplicate send. A rejected, cancelled or failed queue leaves no outbox row and does not
// count. exceptActionID, when not empty, leaves out that action's own row, so a retry of the same
// action is still stopped by the outbox's one-row-per-action constraint and not mistaken for a second
// queue.
func ReportAlreadyQueued(ctx context.Context, querier rowQuerier, organizationID, runID, reportID, exceptActionID string) (bool, error) {
	var queued bool
	err := querier.QueryRow(ctx, `SELECT EXISTS (
		SELECT 1 FROM demo.outbox_messages AS message
		  JOIN runtime.actions AS queued_action
		    ON queued_action.id = message.action_id AND queued_action.organization_id = message.organization_id
		 WHERE message.organization_id = $1 AND queued_action.run_id = $2 AND message.report_id = $3
		   AND message.action_id::text <> $4)`,
		organizationID, runID, reportID, exceptActionID).Scan(&queued)
	return queued, err
}
