package app

import (
	"context"
	"database/sql"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// requireKnownBasisSelfCheckPasses is the one strict self-check assertion for
// a healthy known-basis book (#273/#313/#397). Every applicable check must
// pass, not a named selection, so a check added later is asserted at every
// consumer at once; and the run itself must pass, so no check may report
// unavailable coverage. A warning is not a pass either: a test that expects an
// advisory finding asserts it by name. Unknown-basis books use
// requireHealthyUnresolvedBook; damage tests assert their own expected failures.
func requireKnownBasisSelfCheckPasses(t *testing.T, database *sql.DB) {
	t.Helper()
	run, err := selfCheckOver(t, database).RunSelfCheck(context.Background(), "manual")
	require.NoError(t, err)
	for _, result := range run.Results {
		if result.Status != SelfCheckNotApplicable {
			assert.Equalf(t, SelfCheckPassed, result.Status, "%s: %s %v", result.CheckID, result.Summary, result.Sample)
		}
	}
	assert.Equal(t, SelfCheckPassed, run.Status)
}

// requireInvestmentSelfCheckPasses adds the investment fixture's independent
// oracles to the strict run: effective movements against immutable evidence,
// and dated quantity and basis conservation against the journal.
func requireInvestmentSelfCheckPasses(t *testing.T, f *investmentsTestFixture) {
	t.Helper()
	requireKnownBasisSelfCheckPasses(t, f.database)
	requireEffectiveMovementsConsistent(t, f)
	requireConservationOracle(t, f)
}
