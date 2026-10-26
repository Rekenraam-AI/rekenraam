package db

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// dependencyPairs lists the indexed (derived, source) links in a stable order.
func (f voidCascadeFixture) dependencyPairs(t *testing.T) [][2]int64 {
	t.Helper()

	rows, err := f.database.QueryContext(context.Background(), `
		SELECT observation_id, source_observation_id
		FROM price_observation_dependencies
		WHERE book_id = 1
		ORDER BY observation_id, source_observation_id
	`)
	require.NoError(t, err)
	defer rows.Close()
	pairs := [][2]int64{}
	for rows.Next() {
		var pair [2]int64
		require.NoError(t, rows.Scan(&pair[0], &pair[1]))
		pairs = append(pairs, pair)
	}
	require.NoError(t, rows.Err())
	return pairs
}

func (f voidCascadeFixture) countObservations(t *testing.T) int {
	t.Helper()

	var count int
	require.NoError(t, f.database.QueryRowContext(context.Background(),
		`SELECT COUNT(*) FROM price_observations WHERE book_id = 1`).Scan(&count))
	return count
}

// A derived observation's sources are indexed in the transaction that inserts
// it, one link per distinct source (#104).
func TestDerivedObservationIndexesItsSourcesAtInsert(t *testing.T) {
	t.Parallel()
	fixture := newVoidCascadeFixture(t)
	first := fixture.createObservation(t, 1)
	second := fixture.createObservation(t, 2)
	derived := fixture.createObservation(t, 3, first, second, first)

	assert.Equal(t, [][2]int64{{derived, first}, {derived, second}}, fixture.dependencyPairs(t))
}

// A leg naming an observation that does not exist cannot be indexed, so a void
// of the real source would never reach the derived price. The whole insert is
// refused, its audit event included.
func TestDerivedObservationNamingAMissingSourceIsRefused(t *testing.T) {
	t.Parallel()
	fixture := newVoidCascadeFixture(t)
	source := fixture.createObservation(t, 1)
	observationsBefore := fixture.countObservations(t)
	var auditBefore int
	require.NoError(t, fixture.database.QueryRow(`SELECT COUNT(*) FROM audit_events`).Scan(&auditBefore))

	_, err := fixture.repository.CreatePriceObservation(context.Background(), CreatePriceObservationParams{
		BookID: 1, ActorUserID: fixture.ownerID, RequestID: "request-missing-leg", OriginType: "browser_api",
		Operation: "pricing.price.create", CreatedAt: "2026-06-12T12:00:00Z", ChangeReason: "missing leg",
		Spec: PriceObservationSpec{
			BaseCommodityID: fixture.baseID, QuoteCommodityID: fixture.quoteID, QuoteType: "manual", AdjustmentBasis: "raw",
			PriceValue: 1000000, PriceScale: 6, BaseQuantityValue: 1, ValuationDate: "2026-06-05", IsDerived: true,
			DerivationJSON: fmt.Sprintf(`{"legs":[{"observation_id":%d},{"observation_id":%d}]}`, source, source+1000),
			MetadataJSON:   `{}`,
		},
	})

	require.ErrorIs(t, err, ErrPriceDerivationSourceNotFound)
	assert.Equal(t, observationsBefore, fixture.countObservations(t))
	var auditAfter int
	require.NoError(t, fixture.database.QueryRow(`SELECT COUNT(*) FROM audit_events`).Scan(&auditAfter))
	assert.Equal(t, auditBefore, auditAfter)
	assert.Empty(t, fixture.dependencyPairs(t))
}

// Only a derived observation has sources. Legs written on a non-derived
// observation were never followed by the cascade and are not indexed now.
func TestNonDerivedObservationLegsAreNotIndexed(t *testing.T) {
	t.Parallel()
	fixture := newVoidCascadeFixture(t)
	source := fixture.createObservation(t, 1)

	_, err := fixture.repository.CreatePriceObservation(context.Background(), CreatePriceObservationParams{
		BookID: 1, ActorUserID: fixture.ownerID, RequestID: "request-manual-legs", OriginType: "browser_api",
		Operation: "pricing.price.create", CreatedAt: "2026-06-12T12:00:00Z", ChangeReason: "manual with legs",
		Spec: PriceObservationSpec{
			BaseCommodityID: fixture.baseID, QuoteCommodityID: fixture.quoteID, QuoteType: "manual", AdjustmentBasis: "raw",
			PriceValue: 1000000, PriceScale: 6, BaseQuantityValue: 1, ValuationDate: "2026-06-05", IsManual: true,
			DerivationJSON: fmt.Sprintf(`{"legs":[{"observation_id":%d}]}`, source), MetadataJSON: `{}`,
		},
	})

	require.NoError(t, err)
	assert.Empty(t, fixture.dependencyPairs(t))
}

// Dependencies are audit evidence: once written they are neither changed nor
// removed, voided endpoints included.
func TestPriceDependenciesAreImmutable(t *testing.T) {
	t.Parallel()
	fixture := newVoidCascadeFixture(t)
	source := fixture.createObservation(t, 1)
	derived := fixture.createObservation(t, 2, source)
	_, err := fixture.void(t, source)
	require.NoError(t, err)

	_, err = fixture.database.Exec(`UPDATE price_observation_dependencies SET source_observation_id = source_observation_id WHERE observation_id = ?`, derived)
	require.ErrorContains(t, err, "price dependencies are immutable")
	_, err = fixture.database.Exec(`DELETE FROM price_observation_dependencies WHERE observation_id = ?`, derived)
	require.ErrorContains(t, err, "price dependencies are immutable")
	assert.Equal(t, [][2]int64{{derived, source}}, fixture.dependencyPairs(t))
}

// Migration 0005 indexes the links existing derived observations name, with
// exactly the leg rule the old json_each cascade followed: integer
// observation_ids inside a $.legs array on an is_derived observation, naming an
// earlier observation in the book. Malformed metadata is skipped, not fatal.

// A derived observation whose derivation is not JSON, or has no legs array,
// names no sources: it is stored without links rather than failing the write.
func TestDerivedObservationWithoutLegsStoresNoLinks(t *testing.T) {
	t.Parallel()
	fixture := newVoidCascadeFixture(t)
	for index, derivation := range []string{`not json`, `{"legs":{"observation_id":1}}`, `{"legs":["text",7]}`} {
		_, err := fixture.repository.CreatePriceObservation(context.Background(), CreatePriceObservationParams{
			BookID: 1, ActorUserID: fixture.ownerID, RequestID: "request-odd-derivation", OriginType: "browser_api",
			Operation: "pricing.price.create", CreatedAt: "2026-06-12T12:00:00Z", ChangeReason: "odd derivation",
			Spec: PriceObservationSpec{
				BaseCommodityID: fixture.baseID, QuoteCommodityID: fixture.quoteID, QuoteType: "manual", AdjustmentBasis: "raw",
				PriceValue: 1000000, PriceScale: 6, BaseQuantityValue: 1, ValuationDate: fmt.Sprintf("2026-06-%02d", index+1),
				IsDerived: true, DerivationJSON: derivation, MetadataJSON: `{}`,
			},
		})
		require.NoError(t, err, derivation)
	}
	assert.Empty(t, fixture.dependencyPairs(t))
}
