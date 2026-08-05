package handlers

import (
	"testing"
)

// The reconciliation policy in reconcilePending is pure (no DB), so we test it
// directly and exhaustively. See resync.go for the documented policy this
// asserts.

func TestReconcilePending_ClientBehind_AcceptsAllPending(t *testing.T) {
	t.Parallel()

	req := resyncRequest{
		LastRevision: 3,
		Pending: []resyncPendingAction{
			{ClientID: "c:1:9:1", Kind: "play_card"},
			{ClientID: "c:1:9:2", Kind: "go"},
		},
	}
	accepted, rejected := reconcilePending(req, 5)

	if len(accepted) != 2 {
		t.Fatalf("expected 2 accepted, got %d (%v)", len(accepted), accepted)
	}
	if got := accepted[0]; got != "c:1:9:1" {
		t.Errorf("accepted[0] = %q, want c:1:9:1", got)
	}
	if got := accepted[1]; got != "c:1:9:2" {
		t.Errorf("accepted[1] = %q, want c:1:9:2", got)
	}
	if len(rejected) != 0 {
		t.Errorf("expected 0 rejected, got %d (%v)", len(rejected), rejected)
	}
}

func TestReconcilePending_ClientUpToDate_KeepsPending(t *testing.T) {
	t.Parallel()

	req := resyncRequest{
		LastRevision: 5,
		Pending: []resyncPendingAction{
			{ClientID: "c:1:9:1", Kind: "play_card"},
		},
	}
	accepted, rejected := reconcilePending(req, 5)

	if len(accepted) != 0 {
		t.Errorf("expected nothing accepted when up to date, got %v", accepted)
	}
	if len(rejected) != 0 {
		t.Errorf("expected nothing rejected when up to date, got %v", rejected)
	}
}

func TestReconcilePending_ClientAhead_KeepsPending(t *testing.T) {
	t.Parallel()

	// A client claiming to be ahead of the server should not have its actions
	// accepted or rejected; the server simply leaves them outstanding.
	req := resyncRequest{
		LastRevision: 9,
		Pending: []resyncPendingAction{
			{ClientID: "c:1:9:1", Kind: "discard"},
		},
	}
	accepted, rejected := reconcilePending(req, 5)

	if len(accepted) != 0 || len(rejected) != 0 {
		t.Errorf("expected no reconciliation when client is ahead, got accepted=%v rejected=%v", accepted, rejected)
	}
}

func TestReconcilePending_EmptyPending_ReturnsEmptySlices(t *testing.T) {
	t.Parallel()

	accepted, rejected := reconcilePending(resyncRequest{LastRevision: 0}, 3)

	// Must be non-nil so JSON serializes as [] not null.
	if accepted == nil {
		t.Error("accepted should be non-nil")
	}
	if rejected == nil {
		t.Error("rejected should be non-nil")
	}
	if len(accepted) != 0 || len(rejected) != 0 {
		t.Errorf("expected empty slices, got accepted=%v rejected=%v", accepted, rejected)
	}
}

func TestReconcilePending_BlankClientID_IsRejected(t *testing.T) {
	t.Parallel()

	req := resyncRequest{
		LastRevision: 1,
		Pending: []resyncPendingAction{
			{ClientID: "", Kind: "go"},
			{ClientID: "c:1:9:2", Kind: "play_card"},
		},
	}
	accepted, rejected := reconcilePending(req, 4)

	if len(rejected) != 1 || rejected[0] != "" {
		t.Errorf("blank client id should be rejected, got rejected=%v", rejected)
	}
	if len(accepted) != 1 || accepted[0] != "c:1:9:2" {
		t.Errorf("valid client id should be accepted, got accepted=%v", accepted)
	}
}

func TestReconcilePending_IsDeterministic(t *testing.T) {
	t.Parallel()

	req := resyncRequest{
		LastRevision: 0,
		Pending: []resyncPendingAction{
			{ClientID: "a", Kind: "go"},
			{ClientID: "b", Kind: "go"},
			{ClientID: "c", Kind: "go"},
		},
	}
	a1, r1 := reconcilePending(req, 2)
	a2, r2 := reconcilePending(req, 2)

	if len(a1) != len(a2) || len(r1) != len(r2) {
		t.Fatalf("non-deterministic result: %v/%v vs %v/%v", a1, r1, a2, r2)
	}
	for i := range a1 {
		if a1[i] != a2[i] {
			t.Errorf("accepted order differs at %d: %q vs %q", i, a1[i], a2[i])
		}
	}
}
