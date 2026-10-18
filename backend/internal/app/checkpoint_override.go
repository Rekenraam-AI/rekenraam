package app

// CheckpointOverride is an investment or import write's reconciliation
// override (#304). Allowed lets the write invalidate the checkpoints it
// reaches. Reviewed, when non-nil, is the checkpoint set the owner was shown
// and acknowledged: the write transaction refuses a different set with
// ErrReconciliationImpactStale, invalidating nothing. A nil Reviewed set is an
// internal caller's (or a preview's) unbound override. The generic ledger
// inputs carry the same pair as two fields (#296).
type CheckpointOverride struct {
	Allowed  bool
	Reviewed []int64
}

// unboundOverride lets a preview resolve the complete invalidation set; it
// must never be bound to a set, or a stale review would refuse the preview
// that is meant to show the owner the current one.
var unboundOverride = CheckpointOverride{Allowed: true}
