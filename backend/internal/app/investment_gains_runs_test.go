package app

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/pressly/goose/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"rekenraam/backend/internal/db"
	"rekenraam/backend/migrations"
)

// The production formatter lives with the API wire DTOs. This test formatter
// retains the projection and descriptor to exercise snapshot isolation/failure.
func testGainsRunFormatter(projection GainsRunProjection) (json.RawMessage, error) {
	quantities := []string{}
	for _, position := range projection.Unrealized {
		quantities = append(quantities, position.QuantityValue.String())
	}
	return json.Marshal(struct {
		Profile    GainsProfile `json:"profile"`
		Quantities []string     `json:"quantities"`
	}{projection.Profile, quantities})
}

func TestGainsRunSnapshotFailureLeavesEveryFinancialRowUnchanged(t *testing.T) {
	t.Parallel()
	f := newInvestmentsTestFixture(t)
	buyOn(t, f, "2026-01-01", 10, 10000)
	_, err := f.investmentService.Sell(context.Background(), sellInput(f, "2026-02-01", 4))
	require.NoError(t, err)
	before := buyReplacementPreviewSnapshot(t, f.database)
	wanted := errors.New("failed to encode report")
	_, err = f.investmentService.CreateGainsRun(context.Background(), GainsRunInput{OwnerUserID: f.ownerUserID,
		Request: GainsRunRequest{ReportType: "gains", ValuationAsOf: "2026-01-15"}}, func(p GainsRunProjection) (json.RawMessage, error) {
		require.Len(t, p.Unrealized, 1)
		assert.Equal(t, "10", p.Unrealized[0].QuantityValue.String(), "dated replay uses the isolated pre-sale position")
		return nil, wanted
	})
	require.ErrorIs(t, err, wanted)
	assert.Equal(t, before, buyReplacementPreviewSnapshot(t, f.database))
	var count int
	require.NoError(t, f.database.QueryRow(`SELECT COUNT(*) FROM investment_gains_runs`).Scan(&count))
	assert.Zero(t, count)

	// Even after successful calculation, a failed insert rolls its audit back.
	_, err = f.investmentService.CreateGainsRun(context.Background(), GainsRunInput{OwnerUserID: 999999,
		Request: GainsRunRequest{ReportType: "gains", ValuationAsOf: "2026-01-15"}}, testGainsRunFormatter)
	require.Error(t, err)
	assert.Equal(t, before, buyReplacementPreviewSnapshot(t, f.database))
	require.NoError(t, f.database.QueryRow(`SELECT COUNT(*) FROM investment_gains_runs`).Scan(&count))
	assert.Zero(t, count)
}

func TestGainsRunSnapshotIgnoresWritesBeforePersistence(t *testing.T) {
	t.Parallel()
	f := newInvestmentsTestFixture(t)
	buyOn(t, f, "2026-01-01", 10, 10000)
	input := GainsRunInput{OwnerUserID: f.ownerUserID, Request: GainsRunRequest{ReportType: "gains", ValuationAsOf: "2026-03-01"}}
	ctx := context.Background()
	run, err := f.investmentService.CreateGainsRun(ctx, input, func(p GainsRunProjection) (json.RawMessage, error) {
		// A source write during computation/presentation must neither leak into the
		// captured output nor be rolled back by the saved run's dated replay.
		_, err := f.investmentService.Sell(ctx, sellInput(f, "2026-02-01", 4))
		require.NoError(t, err)
		require.Len(t, p.Unrealized, 1)
		assert.Equal(t, "10", p.Unrealized[0].QuantityValue.String())
		return testGainsRunFormatter(p)
	})
	require.NoError(t, err)
	require.True(t, db.ValidGainsRunDocument(run))
	saved, err := f.investmentService.GainsRun(ctx, run.ID)
	require.NoError(t, err)
	assert.Equal(t, run, saved)
	_, live, err := f.investmentService.UnrealizedGainsProjection(ctx, "2026-03-01")
	require.NoError(t, err)
	require.Len(t, live, 1)
	assert.Equal(t, "6", live[0].QuantityValue.String())
}

func TestGainsRunMigrationPreservesFinancialFactsAndRefusesPopulatedDowngrade(t *testing.T) {
	t.Parallel()
	f := newInvestmentsTestFixture(t)
	buyOn(t, f, "2026-01-01", 10, 10000)
	before := buyReplacementPreviewSnapshot(t, f.database)
	ctx := context.Background()
	provider, err := goose.NewProvider(goose.DialectSQLite3, f.database, migrations.FS)
	require.NoError(t, err)
	_, err = provider.DownTo(ctx, 6)
	require.NoError(t, err)
	_, err = provider.Up(ctx)
	require.NoError(t, err)
	assert.Equal(t, before, buyReplacementPreviewSnapshot(t, f.database))
	run, err := f.investmentService.CreateGainsRun(ctx, GainsRunInput{OwnerUserID: f.ownerUserID,
		Request: GainsRunRequest{ReportType: "gains", ValuationAsOf: "2026-03-01"}}, testGainsRunFormatter)
	require.NoError(t, err)
	after := buyReplacementPreviewSnapshot(t, f.database)
	require.Len(t, after["audit_events"], len(before["audit_events"])+1)
	// An explicit report save adds its own audit; all financial facts stay exact.
	after["audit_events"] = before["audit_events"]
	assert.Equal(t, before, after)
	_, err = provider.DownTo(ctx, 6)
	require.Error(t, err, "populated downgrade must not discard immutable reports")
	saved, err := f.investmentService.GainsRun(ctx, run.ID)
	require.NoError(t, err)
	assert.Equal(t, run, saved)
}
