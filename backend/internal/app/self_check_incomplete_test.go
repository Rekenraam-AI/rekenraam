package app

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

// #313's status contract (data-portability-plan.md, "Self-check outcomes"):
// a finding fails a result whatever else it reports; unavailable coverage
// alone makes it incomplete; and a run takes failed over incomplete over
// passed, counting unavailable coverage on failed results too.
func TestSelfCheckOutcomePrecedence(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		results              []SelfCheckResult
		status               string
		failed, incomplete   int64
		lotStatus, slotState string
	}{
		"clean": {
			results: []SelfCheckResult{{CheckID: CheckLotReconciliation}, {CheckID: CheckAttachments, Status: SelfCheckNotApplicable}},
			status:  SelfCheckPassed, lotStatus: SelfCheckPassed, slotState: SelfCheckNotApplicable,
		},
		"unknown basis only": {
			results: []SelfCheckResult{{CheckID: CheckLotReconciliation, UnavailableCount: 2}, {CheckID: CheckAttachments, Status: SelfCheckNotApplicable}},
			status:  SelfCheckIncomplete, incomplete: 1, lotStatus: SelfCheckIncomplete, slotState: SelfCheckNotApplicable,
		},
		"damage beside unknown basis in the same check": {
			results: []SelfCheckResult{{CheckID: CheckLotReconciliation, FindingCount: 1, UnavailableCount: 1}, {CheckID: CheckAttachments, Status: SelfCheckNotApplicable}},
			status:  SelfCheckFailed, failed: 1, incomplete: 1, lotStatus: SelfCheckFailed, slotState: SelfCheckNotApplicable,
		},
		"damage elsewhere beside unknown basis": {
			results: []SelfCheckResult{{CheckID: CheckLotReconciliation, UnavailableCount: 1}, {CheckID: CheckInvestmentReplay, FindingCount: 1}},
			status:  SelfCheckFailed, failed: 1, incomplete: 1, lotStatus: SelfCheckIncomplete,
		},
		"a check that fails without counting": {
			results: []SelfCheckResult{{CheckID: CheckLotReconciliation, Status: SelfCheckFailed, UnavailableCount: 1}},
			status:  SelfCheckFailed, failed: 1, incomplete: 1, lotStatus: SelfCheckFailed,
		},
	} {
		t.Run(name, func(t *testing.T) {
			for index := range tc.results {
				tc.results[index].Status = resultStatus(tc.results[index])
			}
			status, failed, incomplete := aggregateSelfCheck(tc.results)
			require.Equal(t, tc.status, status)
			require.Equal(t, tc.failed, failed)
			require.Equal(t, tc.incomplete, incomplete)
			require.Equal(t, tc.lotStatus, tc.results[0].Status)
			if tc.slotState != "" {
				require.Equal(t, tc.slotState, tc.results[1].Status, "not_applicable keeps its reserved meaning")
			}
		})
	}
}

// The nightly run chained onto a backup records the same incomplete verdict a
// manual run does, and the latest-run read returns it with its separate
// unavailable scope.
func TestScheduledSelfCheckRecordsIncompleteUnknownBasis(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := newInvestmentsTestFixture(t)
	seedExternalTransferEquity(t, f.database)
	inbound, err := f.investmentService.ExternalTransferIn(ctx, unknownTransferInInput(f, "2026-05-01", "2020-03-01"))
	require.NoError(t, err)

	service := selfCheckOver(t, f.database)
	run, err := service.RunSelfCheck(ctx, "scheduled")
	require.NoError(t, err)
	require.Equal(t, SelfCheckIncomplete, run.Status)
	require.Zero(t, run.FailedCheckCount)
	require.EqualValues(t, 1, run.IncompleteCheckCount)

	latest, hasRun, err := service.LatestSelfCheck(ctx)
	require.NoError(t, err)
	require.True(t, hasRun)
	require.Equal(t, run.ID, latest.ID)
	require.Equal(t, "scheduled", latest.Trigger)
	require.Equal(t, SelfCheckIncomplete, latest.Status)
	require.Zero(t, latest.FailedCheckCount)
	require.EqualValues(t, 1, latest.IncompleteCheckCount)
	lots := resultFor(t, latest, CheckLotReconciliation)
	require.Equal(t, SelfCheckIncomplete, lots.Status)
	require.Zero(t, lots.FindingCount)
	require.Empty(t, lots.Sample)
	require.EqualValues(t, 1, lots.UnavailableCount)
	require.Equal(t, []int64{*inbound.LotID}, lots.UnavailableSample)
	require.NotEmpty(t, lots.NextStep)
	for _, result := range latest.Results {
		if result.CheckID != CheckLotReconciliation {
			require.Zero(t, result.UnavailableCount, result.CheckID)
		}
	}
}
