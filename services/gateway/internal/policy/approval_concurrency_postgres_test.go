package policy

import (
	"context"
	"sync"
	"testing"
)

// TestConcurrentApprovalDecisionsStoreOneGrant is part of the X-51 evidence: however many
// decisions race for one action, one grant and one continuation are stored.
func TestConcurrentApprovalDecisionsStoreOneGrant(t *testing.T) {
	world := openApprovalWorld(t)
	const decisions = 6
	errorsByRequest := make([]error, decisions)
	var waitGroup sync.WaitGroup
	for index := range decisions {
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			choice := ApprovalApprove
			if index%2 == 1 {
				choice = ApprovalReject
			}
			_, errorsByRequest[index] = NewApprovals(world.pool).Decide(context.Background(), world.reviewer, world.actionID, choice)
		}()
	}
	waitGroup.Wait()
	succeeded := 0
	for _, err := range errorsByRequest {
		if err == nil {
			succeeded++
		}
	}
	approvals, jobs, _ := world.counts(t)
	if succeeded != 1 || approvals != 1 || jobs != 1 {
		t.Fatalf("succeeded %d, approvals %d, jobs %d; want 1, 1, 1 (errors %v)", succeeded, approvals, jobs, errorsByRequest)
	}
	t.Logf("evidence X-51: %d concurrent decisions -> 1 accepted, %d refused; 1 grant, 1 continuation job", decisions, decisions-1)
}
