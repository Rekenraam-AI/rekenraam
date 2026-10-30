package db

import (
	"context"
	"database/sql"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func correctionParams(f voidCascadeFixture, root int64, token string) CorrectPriceParams {
	return CorrectPriceParams{ObservationID: root, ExpectedImpact: token, Create: CreatePriceObservationParams{
		BookID: 1, ActorUserID: f.ownerID, Operation: "pricing.price.correct", OriginType: "browser_api", CreatedAt: "2026-06-13T12:00:00Z", ChangeReason: "correct fixing",
		Spec: PriceObservationSpec{BaseCommodityID: f.baseID, QuoteCommodityID: f.quoteID, ValuationDate: "2026-06-01", QuoteType: "manual", AdjustmentBasis: "raw", PriceValue: 125, PriceScale: 2, BaseQuantityValue: 1, SourceID: sql.NullInt64{Int64: f.manualSrcID, Valid: true}, IsManual: true, DerivationJSON: "{}", MetadataJSON: `{"reference":"statement"}`, SeriesMetadataJSON: "{}"},
	}}
}

func TestPriceImpactUsesFullCascadeWithoutWriting(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := newVoidCascadeFixture(t)
	root := f.createObservation(t, 1)
	left := f.createObservation(t, 2, root)
	right := f.createObservation(t, 3, root)
	join := f.createObservation(t, 4, left, right)
	var before int
	require.NoError(t, f.database.QueryRow(`SELECT count(*) FROM audit_events`).Scan(&before))
	impact, err := f.repository.PriceImpact(ctx, 1, root)
	require.NoError(t, err)
	ids := []int64{}
	for _, p := range impact.Prices {
		ids = append(ids, p.ID)
		assert.False(t, p.VoidedAt.Valid)
	}
	assert.Equal(t, []int64{root, left, right, join}, ids)
	assert.Len(t, impact.Token, 64)
	var after int
	require.NoError(t, f.database.QueryRow(`SELECT count(*) FROM audit_events`).Scan(&after))
	assert.Equal(t, before, after)
	retired, err := f.repository.VoidPriceObservation(ctx, VoidPriceObservationParams{BookID: 1, ObservationID: root, ActorUserID: f.ownerID, VoidedAt: "2026-06-13T12:00:00Z", VoidReason: "withdraw", ChangeReason: "withdraw", Operation: "pricing.price.void", OriginType: "browser_api", ExpectedImpact: impact.Token})
	require.NoError(t, err)
	require.Len(t, retired, 4)
	for i, p := range retired {
		assert.Equal(t, ids[i], p.ID)
		assert.True(t, p.VoidedAt.Valid)
	}
}

func TestReviewedCorrectionRefusesNewDependenciesWithoutWrites(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := newVoidCascadeFixture(t)
	root := f.createObservation(t, 1)
	impact, err := f.repository.PriceImpact(ctx, 1, root)
	require.NoError(t, err)
	child := f.createObservation(t, 2, root)
	var before int
	require.NoError(t, f.database.QueryRow(`SELECT count(*) FROM audit_events`).Scan(&before))
	_, err = f.repository.CorrectPrice(ctx, correctionParams(f, root, impact.Token))
	require.ErrorIs(t, err, ErrPriceImpactStale)
	_, err = f.repository.VoidPriceObservation(ctx, VoidPriceObservationParams{BookID: 1, ObservationID: root, ActorUserID: f.ownerID, ExpectedImpact: impact.Token})
	require.ErrorIs(t, err, ErrPriceImpactStale)
	for _, id := range []int64{root, child} {
		p, err := f.repository.PriceObservationByID(ctx, 1, id)
		require.NoError(t, err)
		assert.False(t, p.VoidedAt.Valid)
	}
	var after int
	require.NoError(t, f.database.QueryRow(`SELECT count(*) FROM audit_events`).Scan(&after))
	assert.Equal(t, before, after)
}

func TestPriceCorrectionPreservesEvidenceUnderOneAudit(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := newVoidCascadeFixture(t)
	root := f.createObservation(t, 1)
	child := f.createObservation(t, 2, root)
	original, err := f.repository.PriceObservationByID(ctx, 1, root)
	require.NoError(t, err)
	impact, err := f.repository.PriceImpact(ctx, 1, root)
	require.NoError(t, err)
	result, err := f.repository.CorrectPrice(ctx, correctionParams(f, root, impact.Token))
	require.NoError(t, err)
	require.Len(t, result.Retired, 2)
	assert.Equal(t, root, result.Replacement.SupersedesObservationID.Int64)
	assert.Equal(t, original.ValuationDate, result.Replacement.ValuationDate)
	assert.Equal(t, f.baseID, result.Replacement.BaseCommodityID)
	assert.Equal(t, int64(125), result.Replacement.PriceValue)
	assert.True(t, result.Replacement.IsManual)
	for _, id := range []int64{root, child} {
		var voidAudit, createAudit int64
		require.NoError(t, f.database.QueryRow(`SELECT voided_audit_event_id,(SELECT created_audit_event_id FROM price_observations WHERE id=?) FROM price_observations WHERE id=?`, result.Replacement.ID, id).Scan(&voidAudit, &createAudit))
		assert.Equal(t, voidAudit, createAudit)
	}
	old, err := f.repository.PriceObservationByID(ctx, 1, root)
	require.NoError(t, err)
	assert.Equal(t, original.PriceValue, old.PriceValue)
	assert.Equal(t, original.RecordedAt, old.RecordedAt)
	assert.Equal(t, original.SourceID, old.SourceID)
	_, err = f.repository.CorrectPrice(ctx, correctionParams(f, root, impact.Token))
	require.ErrorIs(t, err, ErrPriceObservationAlreadyVoided)
}

func TestPriceCorrectionLateInsertFailureRollsBackCascadeAndAudit(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := newVoidCascadeFixture(t)
	root := f.createObservation(t, 1)
	child := f.createObservation(t, 2, root)
	impact, err := f.repository.PriceImpact(ctx, 1, root)
	require.NoError(t, err)
	var before int
	require.NoError(t, f.database.QueryRow(`SELECT count(*) FROM audit_events`).Scan(&before))
	_, err = f.database.Exec(`CREATE TRIGGER reject_test_price_replacement BEFORE INSERT ON price_observations WHEN NEW.supersedes_observation_id IS NOT NULL BEGIN SELECT RAISE(ABORT,'late insert test failure'); END`)
	require.NoError(t, err)
	_, err = f.repository.CorrectPrice(ctx, correctionParams(f, root, impact.Token))
	require.ErrorContains(t, err, "late insert test failure")
	for _, id := range []int64{root, child} {
		p, err := f.repository.PriceObservationByID(ctx, 1, id)
		require.NoError(t, err)
		assert.False(t, p.VoidedAt.Valid)
	}
	var after, count int
	require.NoError(t, f.database.QueryRow(`SELECT count(*) FROM audit_events`).Scan(&after))
	assert.Equal(t, before, after)
	require.NoError(t, f.database.QueryRow(`SELECT count(*) FROM price_observations`).Scan(&count))
	assert.Equal(t, 2, count)
}

func TestConcurrentPriceCorrectionsHaveOneWinner(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := newVoidCascadeFixture(t)
	root := f.createObservation(t, 1)
	impact, err := f.repository.PriceImpact(ctx, 1, root)
	require.NoError(t, err)
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for range 2 {
		wg.Go(func() {
			_, err := f.repository.CorrectPrice(ctx, correctionParams(f, root, impact.Token))
			results <- err
		})
	}
	wg.Wait()
	close(results)
	successes, refusals := 0, 0
	for err := range results {
		if err == nil {
			successes++
		} else {
			require.ErrorIs(t, err, ErrPriceObservationAlreadyVoided)
			refusals++
		}
	}
	assert.Equal(t, 1, successes)
	assert.Equal(t, 1, refusals)
	var count int
	require.NoError(t, f.database.QueryRow(`SELECT count(*) FROM price_observations WHERE supersedes_observation_id=?`, root).Scan(&count))
	assert.Equal(t, 1, count)
}

func TestPriceImpactOverDepthRefusesReadAndCorrection(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := newVoidCascadeFixture(t)
	root, _ := f.createChain(t, maxVoidCascadeDepth+1)
	_, err := f.repository.PriceImpact(ctx, 1, root)
	require.ErrorIs(t, err, ErrPriceVoidCascadeTooDeep)
	_, err = f.repository.CorrectPrice(ctx, correctionParams(f, root, "invalid"))
	require.ErrorIs(t, err, ErrPriceVoidCascadeTooDeep)
	p, err := f.repository.PriceObservationByID(ctx, 1, root)
	require.NoError(t, err)
	assert.False(t, p.VoidedAt.Valid)
}

func TestPriceWorkspaceIncludesUnpricedConfiguredInstrumentAndRanksTrust(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := newVoidCascadeFixture(t)
	snapshot, err := f.repository.PriceManagementSnapshot(ctx, 1, "2026-06-12", f.quoteID, nil)
	require.NoError(t, err)
	require.Contains(t, snapshot.Pairs, PricingPairRecord{BaseCommodityID: f.baseID, QuoteCommodityID: f.quoteID})
	assert.Empty(t, snapshot.Candidates)
	manual := f.createObservation(t, 12)
	provider := f.createObservation(t, 12)
	_, err = f.database.Exec(`UPDATE price_observations SET is_manual=0 WHERE id=?`, provider)
	require.NoError(t, err)
	snapshot, err = f.repository.PriceManagementSnapshot(ctx, 1, "2026-06-12", f.quoteID, nil)
	require.NoError(t, err)
	require.Len(t, snapshot.Candidates, 1)
	assert.Equal(t, manual, snapshot.Candidates[0].ID)
	_, err = f.database.Exec(`UPDATE price_observations SET is_manual=0,is_approximate=1 WHERE id=?`, manual)
	require.NoError(t, err)
	snapshot, err = f.repository.PriceManagementSnapshot(ctx, 1, "2026-06-12", f.quoteID, nil)
	require.NoError(t, err)
	assert.Equal(t, provider, snapshot.Candidates[0].ID)
	assert.False(t, snapshot.Candidates[0].IsApproximate)
}
