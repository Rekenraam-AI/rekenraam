package db

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"rekenraam/backend/internal/exact"
)

func TestUnusedInstrumentCleanupTreatsSecurityAndCryptoEqually(t *testing.T) {
	t.Parallel()
	for _, kind := range []string{"stock", "crypto"} {
		t.Run(kind, func(t *testing.T) {
			ctx := context.Background()
			database, ownerID, _ := migratedInvestmentTestDatabase(t)
			r := NewInvestmentRepository(database)
			instrument, err := r.CreateInstrument(ctx, CreateInvestmentInstrumentParams{BookID: 1, CreatedByUserID: ownerID, OriginType: "browser_api", Operation: "investment.instrument.create", CreatedAt: "2026-06-12T00:00:00Z", EffectiveFrom: "0001-01-01", ChangeReason: "cleanup control", Spec: InvestmentInstrumentSpec{CommodityCode: "UNUSED", InstrumentType: kind, DisplayName: "Unused", QuantityScale: 6, PriceScale: 6, IdentifiersJSON: "{}", MetadataJSON: "{}"}})
			require.NoError(t, err)
			var auditsBefore int
			require.NoError(t, database.QueryRow("SELECT count(*) FROM audit_events").Scan(&auditsBefore))
			err = r.DeleteUnusedInstrument(ctx, 1, instrument.ID)
			require.NoError(t, err)
			for _, table := range []string{"investment_instruments", "investment_instrument_versions", "commodities", "commodity_versions"} {
				var count int
				// The fixture retains its original default currency only.
				key := "id"
				id := instrument.ID
				if table == "investment_instrument_versions" {
					key = "instrument_id"
				}
				if table == "commodities" || table == "commodity_versions" {
					id = instrument.CommodityID
				}
				if table == "commodity_versions" {
					key = "commodity_id"
				}
				require.NoError(t, database.QueryRow("SELECT count(*) FROM "+table+" WHERE "+key+"=?", id).Scan(&count))
				require.Zero(t, count, table)
			}
			var auditsAfter int
			require.NoError(t, database.QueryRow("SELECT count(*) FROM audit_events").Scan(&auditsAfter))
			require.Equal(t, auditsBefore, auditsAfter)
		})
	}
}

func TestReferencedInstrumentCleanupRefusesBothKindsAtomically(t *testing.T) {
	t.Parallel()
	for _, kind := range []string{"stock", "crypto"} {
		t.Run(kind, func(t *testing.T) {
			ctx := context.Background()
			database, ownerID, _ := migratedInvestmentTestDatabase(t)
			r := NewInvestmentRepository(database)
			instrument, err := r.CreateInstrument(ctx, CreateInvestmentInstrumentParams{BookID: 1, CreatedByUserID: ownerID, OriginType: "browser_api", Operation: "investment.instrument.create", CreatedAt: "2026-06-12T00:00:00Z", EffectiveFrom: "0001-01-01", ChangeReason: "cleanup control", Spec: InvestmentInstrumentSpec{CommodityCode: "USED", InstrumentType: kind, DisplayName: "Used", QuantityScale: 6, PriceScale: 6, IdentifiersJSON: "{}", MetadataJSON: "{}"}})
			require.NoError(t, err)
			createInvestmentTestAccount(t, database, instrument.CommodityID)
			before := captureLedgerState(t, database)
			require.ErrorContains(t, r.DeleteUnusedInstrument(ctx, 1, instrument.ID), "append-only once referenced")
			require.Equal(t, before, captureLedgerState(t, database))
		})
	}
}

func TestPostedInstrumentCleanupRefusesBothKindsBeforeVersionRemoval(t *testing.T) {
	t.Parallel()
	for _, kind := range []string{"stock", "crypto"} {
		t.Run(kind, func(t *testing.T) {
			ctx := context.Background()
			database, ownerID, currencyID := migratedInvestmentTestDatabase(t)
			repository := NewInvestmentRepository(database)
			instrument, err := repository.CreateInstrument(ctx, CreateInvestmentInstrumentParams{BookID: 1, CreatedByUserID: ownerID, OriginType: "browser_api", Operation: "investment.instrument.create", CreatedAt: "2026-06-12T00:00:00Z", EffectiveFrom: "0001-01-01", ChangeReason: "posted cleanup control", Spec: InvestmentInstrumentSpec{CommodityCode: "POSTED", InstrumentType: kind, DisplayName: "Posted", QuantityScale: 6, PriceScale: 6, IdentifiersJSON: "{}", MetadataJSON: "{}"}})
			require.NoError(t, err)
			holdingID := createInvestmentTestAccount(t, database, 0)
			_, err = createPostedBuyLot(t, ctx, repository, CreateInvestmentLotParams{BookID: 1, AccountID: holdingID, CommodityID: instrument.CommodityID, OpenedOn: "2026-06-12", QuantityValue: exact.New(1), QuantityScale: 0, CostBasisValue: 100, CostBasisScale: 2, CostCommodityID: currencyID, MetadataJSON: "{}", CreatedAt: "2026-06-12T00:00:00Z", CreatedByUserID: ownerID, OriginType: "browser_api", ChangeReason: "posted cleanup control", EventKind: "acquisition"})
			require.NoError(t, err)
			before := captureLedgerState(t, database)
			require.ErrorContains(t, repository.DeleteUnusedInstrument(ctx, 1, instrument.ID), "investment_instrument_versions rows are append-only once referenced")
			require.Equal(t, before, captureLedgerState(t, database))
		})
	}
}
