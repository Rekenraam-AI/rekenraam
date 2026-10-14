package app

import (
	"context"
	"database/sql"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"rekenraam/backend/internal/db"
)

// priceDependencyFixture adds a second currency to the self-check book and
// records USD/EUR observations through the real repository write path.
func priceDependencyFixture(t *testing.T, harness *selfCheckHarness) func(legs ...int64) int64 {
	t.Helper()
	execRaw(t, harness.writer, `
		INSERT INTO commodities (id, book_id, code, kind, is_builtin, created_at, created_by_user_id)
		VALUES (2, 1, 'EUR', 'currency', 1, '2026-01-01T00:00:00Z', 1);
		INSERT INTO commodity_versions (
			commodity_id, version_seq, effective_from, recorded_at, changed_by_user_id, change_reason,
			status, symbol, display_symbol, name, standard_scale, max_quantity_scale
		) VALUES (2, 1, '2026-01-01', '2026-01-01T00:00:00Z', 1, 'seed', 'active', '€', '€', 'Euro', 2, 2);
	`)
	repository := db.NewPricingRepository(harness.writer)
	sourceID, err := repository.ManualSourceID(context.Background())
	require.NoError(t, err)
	day := 0
	return func(legs ...int64) int64 {
		day++
		derivation := "{}"
		if len(legs) > 0 {
			derivation = `{"legs":[`
			for index, leg := range legs {
				if index > 0 {
					derivation += ","
				}
				derivation += fmt.Sprintf(`{"observation_id":%d}`, leg)
			}
			derivation += `]}`
		}
		record, err := repository.CreatePriceObservation(context.Background(), db.CreatePriceObservationParams{
			BookID: BookID, ActorUserID: 1, RequestID: "request-price", OriginType: "browser_api",
			Operation: "pricing.price.create", CreatedAt: "2026-06-12T12:00:00Z", ChangeReason: "fixture",
			Spec: db.PriceObservationSpec{
				BaseCommodityID: 1, QuoteCommodityID: 2, QuoteType: "manual", AdjustmentBasis: "raw",
				PriceValue: 90, PriceScale: 2, BaseQuantityValue: 1, ValuationDate: fmt.Sprintf("2026-06-%02d", day),
				SourceID: sql.NullInt64{Int64: sourceID, Valid: true}, IsManual: len(legs) == 0, IsDerived: len(legs) > 0,
				DerivationJSON: derivation, MetadataJSON: `{}`,
			},
		})
		require.NoError(t, err)
		return record.ID
	}
}

// Derived prices written by the app pass: their links are the derivation's legs.
func TestPriceDependencyCheckPassesOnIndexedDerivations(t *testing.T) {
	t.Parallel()
	harness := newSelfCheckHarness(t)
	create := priceDependencyFixture(t, harness)
	first, second := create(), create()
	create(first, second)

	result := resultFor(t, harness.run(t), CheckPriceDependencies)
	assert.Equal(t, SelfCheckPassed, result.Status, result.Summary)
}

// A derived price still in use over a voided source is the state the void
// cascade exists to prevent; the check names it.
func TestPriceDependencyCheckFlagsADerivedPriceOverAVoidedSource(t *testing.T) {
	t.Parallel()
	harness := newSelfCheckHarness(t)
	create := priceDependencyFixture(t, harness)
	source := create()
	derived := create(source)
	// Only a write outside the cascade can retire the source alone.
	execRaw(t, harness.writer, fmt.Sprintf(`
		UPDATE price_observations SET voided_at = '2026-06-20T00:00:00Z', voided_by_user_id = 1,
			void_reason = 'outside the app', voided_audit_event_id = created_audit_event_id
		WHERE id = %d`, source))

	result := resultFor(t, harness.run(t), CheckPriceDependencies)
	assert.Equal(t, SelfCheckFailed, result.Status)
	assert.Equal(t, int64(1), result.FindingCount)
	assert.Equal(t, []int64{derived}, result.Sample)
	assert.Contains(t, result.Summary, "depend on a voided observation")
}

// A leg the index does not carry is a void that would stop short of this
// price. Rewriting the derivation after the fact is how such a gap appears.
func TestPriceDependencyCheckFlagsLinksThatDisagreeWithTheDerivation(t *testing.T) {
	t.Parallel()
	harness := newSelfCheckHarness(t)
	create := priceDependencyFixture(t, harness)
	first, second := create(), create()
	derived := create(first)
	execRaw(t, harness.writer, fmt.Sprintf(`
		UPDATE price_observations SET derivation_json = '{"legs":[{"observation_id":%d},{"observation_id":%d}]}'
		WHERE id = %d`, first, second, derived))

	result := resultFor(t, harness.run(t), CheckPriceDependencies)
	assert.Equal(t, SelfCheckFailed, result.Status)
	assert.Equal(t, []int64{derived}, result.Sample)
	assert.Contains(t, result.Summary, "links disagree with their derivation")
}
