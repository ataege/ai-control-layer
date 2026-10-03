package agent

import (
	"context"
	"log/slog"
	"time"
)

// approvalExpiryInterval is how often overdue approvals are closed.
const approvalExpiryInterval = 5 * time.Second

// OverdueApprovals closes approvals nobody decided before their expiry (policy.Approvals).
type OverdueApprovals interface {
	ExpireOverdue(ctx context.Context) (int, error)
}

// ApprovalExpiry closes overdue approvals while no worker holds their run (GO-40): each closure,
// its event and the continuation job that resumes the run commit together in policy, and the
// worker then continues the run on the blocked-action path.
type ApprovalExpiry struct {
	approvals OverdueApprovals
	interval  time.Duration
	logger    *slog.Logger
}

// NewApprovalExpiry returns the sweeper; nothing runs until Run.
func NewApprovalExpiry(approvals OverdueApprovals, logger *slog.Logger) (*ApprovalExpiry, error) {
	if approvals == nil || logger == nil {
		return nil, ErrInvalid
	}
	return &ApprovalExpiry{approvals: approvals, interval: approvalExpiryInterval, logger: logger}, nil
}

// Run closes overdue approvals every interval until ctx ends. A failure is logged and retried.
func (expiry *ApprovalExpiry) Run(ctx context.Context) {
	ticker := time.NewTicker(expiry.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			closed, err := expiry.approvals.ExpireOverdue(ctx)
			if err != nil && ctx.Err() == nil {
				expiry.logger.Warn("overdue approvals not closed", "closed", closed, "error", err.Error())
			} else if closed > 0 {
				expiry.logger.Info("overdue approvals closed as expired", "closed", closed)
			}
		}
	}
}
