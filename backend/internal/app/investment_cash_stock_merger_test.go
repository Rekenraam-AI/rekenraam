package app

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/csv"
	"errors"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"rekenraam/backend/internal/db"
	"rekenraam/backend/internal/exact"
)

// Cash-and-stock merger (#181): the holding becomes the new instrument at an
// exact ratio, and the cash recovers an elected share of each lot's basis.
// The contract example: 100 shares bought for 1,000.00 become 50 new shares
// and 300.00 at a fraction of 0.25; the cash realizes +50.00, the new lot
// opens at 750.00, selling it for 1,000.00 realizes +250.00, and the
// cost-currency clearing residual is -300.00.

func cashStockMergerInput(f *investmentsTestFixture, newCommodityID int64, date, payable string, numerator, denominator,
	cash int64, fraction string, fractionScale int) CashStockMergerInput {
	return CashStockMergerInput{
		OwnerUserID: f.ownerUserID, EffectiveOn: date, PayableOn: payable, HoldingAccountID: f.holdingAccountID,
		CommodityID: f.stockCommodityID, DestinationCommodityID: newCommodityID,
		RatioNumerator: numerator, RatioDenominator: denominator,
		CashAccountID: f.cashAccountID, CurrencyID: f.eurCommodityID, CashValue: cash, CashScale: 2,
		CashBasisFractionValue: exact.Coefficient(fraction), CashBasisFractionScale: fractionScale,
		SourceEvidenceJSON: `{"notice":"merger terms","cash_per_share":"3.00"}`, Memo: "merger",
	}
}

// mergerDecision reads the merger's cash decision as it currently applies.
func mergerDecision(t *testing.T, f *investmentsTestFixture, transactionID int64) (quantity, proceeds, basis *exact.ScaledInt, knowledge string) {
	t.Helper()
	var quantityValue, proceedsValue exact.Coefficient
	var quantityScale, proceedsScale int
	var basisValue *string
	var basisScale *int
	require.NoError(t, f.database.QueryRow(`SELECT d.quantity_value, d.quantity_scale, d.proceeds_value, d.proceeds_scale,
			COALESCE(r.disposed_basis_value, d.disposed_basis_value), COALESCE(r.disposed_basis_scale, d.disposed_basis_scale),
			COALESCE(r.basis_knowledge, d.basis_knowledge)
		FROM investment_disposal_decisions d LEFT JOIN latest_investment_disposal_revisions r ON r.decision_id = d.id
		WHERE d.transaction_id = ?`, transactionID).Scan(&quantityValue, &quantityScale, &proceedsValue, &proceedsScale,
		&basisValue, &basisScale, &knowledge))
	quantity = exact.ScaledIntFromCoefficient(quantityValue, quantityScale)
	proceeds = exact.ScaledIntFromCoefficient(proceedsValue, proceedsScale)
	if basisValue != nil {
		basis = exact.ScaledIntFromCoefficient(exact.Coefficient(*basisValue), *basisScale)
	}
	return quantity, proceeds, basis, knowledge
}

func TestCashStockMergerRealizesElectedBasisAndCarriesTheRest(t *testing.T) {
	t.Parallel()
	f := newInvestmentsTestFixture(t)
	ctx := context.Background()
	newID := seedTestSecurityCommodity(t, f, "NEWCO")
	bought := buyOn(t, f, "2026-02-01", 100, 100000)
	var auditsBefore int
	require.NoError(t, f.database.QueryRow(`SELECT COUNT(*) FROM audit_events`).Scan(&auditsBefore))

	input := cashStockMergerInput(f, newID, "2026-06-01", "2026-06-05", 1, 2, 30000, "25", 2)
	preview, err := f.investmentService.PreviewCashStockMerger(ctx, input)
	require.NoError(t, err)
	result, err := f.investmentService.CashStockMerger(ctx, input)
	require.NoError(t, err)
	require.Len(t, result.Plan.Links, 1)
	committed := result.Plan.Links[0]
	assert.NotZero(t, committed.DestinationLotID)
	committed.DestinationLotID = 0
	assert.Equal(t, preview.Plan.Links[0], committed, "the preview shows what the commit writes")

	// Units and the cash on their own dates; the cash is never pre-dated.
	require.Len(t, result.Transaction.JournalEntries, 2)
	units, cash := result.Transaction.JournalEntries[0], result.Transaction.JournalEntries[1]
	assert.Equal(t, "2026-06-01", units.EntryDate)
	require.Len(t, units.Postings, 4)
	assert.Equal(t, "2026-06-05", cash.EntryDate)
	require.Len(t, cash.Postings, 2)
	for _, posting := range cash.Postings {
		assert.Equal(t, f.eurCommodityID, posting.CommodityID)
		if posting.AccountID == f.cashAccountID {
			assert.Equal(t, "30000", posting.QuantityValue.String())
		}
	}
	var dates int
	require.NoError(t, f.database.QueryRow(`SELECT COUNT(*) FROM investment_operation_dates d
		WHERE d.operation_id = ? AND ((d.date_role = 'effective' AND d.event_date = '2026-06-01')
			OR (d.date_role = 'payable' AND d.event_date = '2026-06-05')) AND d.date_seq = 1`,
		transferOperationID(t, f, result.Transaction.ID)).Scan(&dates))
	assert.Equal(t, 2, dates, "effective and payable dates, each sequence 1")
	var auditsAfter int
	require.NoError(t, f.database.QueryRow(`SELECT COUNT(*) FROM audit_events`).Scan(&auditsAfter))
	assert.Equal(t, auditsBefore+1, auditsAfter, "one audit event for the whole merger")

	// 1,000.00 × 0.25 = 250.00 realized against the 300.00; 750.00 carried.
	link := result.Plan.Links[0]
	assert.Equal(t, *bought.LotID, link.SourceLotID)
	assert.Equal(t, "50", link.DestinationQuantityValue.String())
	requireScaled(t, 25000, 2, exact.ScaledIntFromInt64(link.RealizedBasisValue, link.RealizedBasisScale), "realized basis")
	requireScaled(t, 75000, 2, exact.ScaledIntFromInt64(link.CarriedBasisValue, link.CarriedBasisScale), "carried basis")
	requireScaled(t, 30000, 2, exact.ScaledIntFromInt64(link.CashValue, link.CashScale), "the lot's cash")
	require.Len(t, result.Plan.Totals, 1)
	requireScaled(t, 5000, 2, exact.ScaledIntFromCoefficient(result.Plan.Totals[0].ResultValue, result.Plan.Totals[0].ResultScale), "result")
	quantity, proceeds, basis, knowledge := mergerDecision(t, f, result.Transaction.ID)
	assert.Zero(t, quantity.Sign(), "the cash disposes no units")
	requireScaled(t, 30000, 2, proceeds, "proceeds are the cash")
	requireScaled(t, 25000, 2, basis, "the decision's basis is the realized part")
	assert.Equal(t, db.InvestmentBasisKnown, knowledge)
	_, newBasis := openCommodityPosition(t, f, f.holdingAccountID, newID)
	requireScaled(t, 75000, 2, newBasis, "the new lot opens at the carried basis")
	oldQuantity, _ := openPositionBasis(t, f, f.holdingAccountID)
	assert.Zero(t, oldQuantity.Sign(), "nothing of the old instrument stays open")

	gains, err := f.investmentService.ListRealizedGains(ctx, GainsReportParams{})
	require.NoError(t, err)
	require.Len(t, gains, 1)
	requireScaled(t, 5000, 2, exact.ScaledIntFromInt64(gains[0].RealizedGainValue, gains[0].RealizedGainScale), "the realized-gains report reads the cash result")

	// Selling the new lot for 1,000.00 realizes +250.00; the residual is the
	// whole operational result.
	_, err = f.investmentService.Sell(ctx, InvestmentTradeInput{OwnerUserID: f.ownerUserID, TransactionDate: "2026-07-01",
		CommodityID: newID, HoldingAccountID: f.holdingAccountID, CashAccountID: f.cashAccountID,
		CashCommodityID: f.eurCommodityID, QuantityValue: exact.New(50), CashAmountValue: 100000, CashAmountScale: 2})
	require.NoError(t, err)
	requireScaled(t, -30000, 2, commodityTradingBalance(t, f, f.eurCommodityID), "clearing residual")
	requireScaled(t, 0, 0, commodityTradingBalance(t, f, f.stockCommodityID), "old instrument clears")
	requireScaled(t, 0, 0, commodityTradingBalance(t, f, newID), "new instrument clears")
	requireInvestmentSelfCheckPasses(t, f)
}

// Each lot's realized and carried parts add up to its basis exactly; the
// cash is shared by old units with the remainder on the last lot.
func TestCashStockMergerConservesBasisPerLotAndSharesCashByUnits(t *testing.T) {
	t.Parallel()
	f := newInvestmentsTestFixture(t)
	ctx := context.Background()
	newID := seedTestSecurityCommodity(t, f, "NEWCO")
	buyOn(t, f, "2026-02-01", 2, 1001)
	buyOn(t, f, "2026-03-01", 1, 500)
	// 1/3 of each lot's basis, and 1.00 of cash over 3 shares.
	result, err := f.investmentService.CashStockMerger(ctx, cashStockMergerInput(f, newID, "2026-06-01", "2026-06-01", 1, 1, 100, "333333333333", 12))
	require.NoError(t, err)
	require.Len(t, result.Plan.Links, 2)
	cash := exact.NewScaledInt()
	for index, link := range result.Plan.Links {
		basis := exact.ScaledIntFromInt64(link.RealizedBasisValue, link.RealizedBasisScale)
		basis.AddInt64(link.CarriedBasisValue, link.CarriedBasisScale)
		requireScaled(t, []int64{1001, 500}[index], 2, basis, "realized plus carried is the lot's basis")
		cash.AddInt64(link.CashValue, link.CashScale)
	}
	requireScaled(t, 100, 2, cash, "the shares conserve the cash")
	first := exact.ScaledIntFromInt64(result.Plan.Links[0].CashValue, result.Plan.Links[0].CashScale)
	assert.Equal(t, -1, first.Cmp(exact.ScaledIntFromInt64(67, 2)), "the first share is truncated: %s", first)
	requireInvestmentSelfCheckPasses(t, f)
}

func TestCashStockMergerFractionZeroAndOneAreElections(t *testing.T) {
	t.Parallel()
	for _, election := range []struct {
		name              string
		fraction          string
		realized, carried int64
	}{{"all basis carried", "0", 0, 100000}, {"all basis recovered", "1", 100000, 0}} {
		t.Run(election.name, func(t *testing.T) {
			t.Parallel()
			f := newInvestmentsTestFixture(t)
			newID := seedTestSecurityCommodity(t, f, "NEWCO")
			buyOn(t, f, "2026-02-01", 100, 100000)
			result, err := f.investmentService.CashStockMerger(context.Background(),
				cashStockMergerInput(f, newID, "2026-06-01", "2026-06-05", 1, 2, 30000, election.fraction, 0))
			require.NoError(t, err)
			link := result.Plan.Links[0]
			requireScaled(t, election.realized, 2, exact.ScaledIntFromInt64(link.RealizedBasisValue, link.RealizedBasisScale), "realized")
			requireScaled(t, election.carried, 2, exact.ScaledIntFromInt64(link.CarriedBasisValue, link.CarriedBasisScale), "carried")
			assert.Equal(t, db.InvestmentBasisKnown, link.BasisKnowledge, "a known zero stays known")
			requireInvestmentSelfCheckPasses(t, f)
		})
	}
}

func TestCashStockMergerUnknownBasisLeavesResultUnresolved(t *testing.T) {
	t.Parallel()
	f := newInvestmentsTestFixture(t)
	seedExternalTransferEquity(t, f.database)
	ctx := context.Background()
	newID := seedTestSecurityCommodity(t, f, "NEWCO")
	inbound, err := f.investmentService.ExternalTransferIn(ctx, unknownTransferInInput(f, "2026-05-01", "2020-03-01"))
	require.NoError(t, err)
	result, err := f.investmentService.CashStockMerger(ctx, cashStockMergerInput(f, newID, "2026-06-01", "2026-06-05", 1, 1, 1000, "5", 1))
	require.NoError(t, err)
	link := result.Plan.Links[0]
	assert.Equal(t, db.InvestmentBasisUnknown, link.BasisKnowledge)
	require.Len(t, result.Plan.Totals, 1)
	assert.Equal(t, db.InvestmentBasisUnknown, result.Plan.Totals[0].BasisKnowledge)
	assert.Empty(t, result.Plan.Totals[0].ResultValue, "an unknown basis has no result, never one against zero")
	_, proceeds, basis, knowledge := mergerDecision(t, f, result.Transaction.ID)
	requireScaled(t, 1000, 2, proceeds, "the cash is still the proceeds")
	assert.Nil(t, basis)
	assert.Equal(t, db.InvestmentBasisUnknown, knowledge)

	// A sourced resolution of 80.00 replays through the merger: half is
	// realized against the cash and half carried.
	resolveAcknowledged(t, f, resolveInput(f, inbound.Transaction.ID, 8000))
	_, _, basis, knowledge = mergerDecision(t, f, result.Transaction.ID)
	assert.Equal(t, db.InvestmentBasisKnown, knowledge)
	requireScaled(t, 4000, 2, basis, "the realized half")
	_, newBasis := openCommodityPosition(t, f, f.holdingAccountID, newID)
	requireScaled(t, 4000, 2, newBasis, "the carried half")
	requireInvestmentSelfCheckPasses(t, f)
}

func TestCashStockMergerRefusesUnsupportedTerms(t *testing.T) {
	t.Parallel()
	f := newInvestmentsTestFixture(t)
	ctx := context.Background()
	newID := seedTestSecurityCommodity(t, f, "NEWCO")
	buyOn(t, f, "2026-02-01", 100, 100000)
	before := shareExchangeWriteCounts(t, f.database)
	for name, input := range map[string]CashStockMergerInput{
		"cash before the effective date": cashStockMergerInput(f, newID, "2026-06-01", "2026-05-31", 1, 2, 30000, "25", 2),
		"no cash":                        cashStockMergerInput(f, newID, "2026-06-01", "2026-06-05", 1, 2, 0, "25", 2),
		"fraction above one":             cashStockMergerInput(f, newID, "2026-06-01", "2026-06-05", 1, 2, 30000, "125", 2),
		"same instrument":                cashStockMergerInput(f, f.stockCommodityID, "2026-06-01", "2026-06-05", 1, 2, 30000, "25", 2),
	} {
		_, err := f.investmentService.CashStockMerger(ctx, input)
		var validation ValidationError
		assert.ErrorAsf(t, err, &validation, "%s", name)
	}
	usd := cashStockMergerInput(f, newID, "2026-06-01", "2026-06-05", 1, 2, 30000, "25", 2)
	usd.CurrencyID = seedTestCurrencyCommodity(t, f.database, "USD")
	_, err := f.investmentService.CashStockMerger(ctx, usd)
	var validation ValidationError
	require.ErrorAs(t, err, &validation)
	assert.Contains(t, validation.Message, "cost currency", "cash in another currency than the basis")
	assert.Equal(t, before, shareExchangeWriteCounts(t, f.database), "a refusal writes nothing")
}

// A corrected acquisition before the merger replays it: the realized part
// is revised (the gain needs the shared acknowledgement), and the carried
// part appends a link revision that the new lot replays from.
func TestBuyCorrectionBeforeCashStockMergerRevisesRealizedAndCarriedBasis(t *testing.T) {
	t.Parallel()
	f := newInvestmentsTestFixture(t)
	ctx := context.Background()
	newID := seedTestSecurityCommodity(t, f, "NEWCO")
	bought := buyOn(t, f, "2026-02-01", 100, 100000)
	merged, err := f.investmentService.CashStockMerger(ctx, cashStockMergerInput(f, newID, "2026-06-01", "2026-06-05", 1, 2, 30000, "25", 2))
	require.NoError(t, err)

	correction := replaceBuyPrice(f, bought, "2026-02-01", 100, 120000)
	_, err = f.investmentService.ReplaceBuy(ctx, correction)
	require.ErrorIs(t, err, ErrGainImpactAcknowledgementRequired, "the merger's realized result changes")
	_, err = acknowledgedReplaceBuy(ctx, f.investmentService, correction)
	require.NoError(t, err)

	_, _, basis, _ := mergerDecision(t, f, merged.Transaction.ID)
	requireScaled(t, 30000, 2, basis, "1,200.00 × 0.25 is now realized")
	_, newBasis := openCommodityPosition(t, f, f.holdingAccountID, newID)
	requireScaled(t, 90000, 2, newBasis, "the new lot carries the rest")
	var revisions int
	require.NoError(t, f.database.QueryRow(`SELECT COUNT(*) FROM investment_transfer_link_revisions`).Scan(&revisions))
	assert.Equal(t, 1, revisions)
	requireInvestmentSelfCheckPasses(t, f)
}

func TestBackdatedPurchaseBeforeCashStockMergerIsRefusedWithMergerNamed(t *testing.T) {
	t.Parallel()
	f := newInvestmentsTestFixture(t)
	ctx := context.Background()
	newID := seedTestSecurityCommodity(t, f, "NEWCO")
	buyOn(t, f, "2026-02-01", 100, 100000)
	merged, err := f.investmentService.CashStockMerger(ctx, cashStockMergerInput(f, newID, "2026-06-01", "2026-06-05", 1, 2, 30000, "25", 2))
	require.NoError(t, err)
	mergerOperationID := transferOperationID(t, f, merged.Transaction.ID)
	before := shareExchangeWriteCounts(t, f.database)
	_, err = f.investmentService.Buy(ctx, tradeOn(f, f.holdingAccountID, "2026-03-01", 5, 5000))
	require.Error(t, err)
	var dependency InvestmentBuyDependencyError
	var sale InvestmentSaleDependencyError
	switch {
	case errors.As(err, &dependency):
		assert.Equal(t, mergerOperationID, dependency.OperationID)
	case errors.As(err, &sale):
		assert.Equal(t, mergerOperationID, sale.OperationID)
	default:
		t.Fatalf("backdated purchase error %v does not name the merger", err)
	}
	assert.Equal(t, before, shareExchangeWriteCounts(t, f.database))
}

// A merger has its own correction commands (#184): the exchange and sale
// corrections and the generic void path refuse it.
func TestCashStockMergerIsClosedToOtherCorrectionCommands(t *testing.T) {
	t.Parallel()
	f := newInvestmentsTestFixture(t)
	ctx := context.Background()
	newID := seedTestSecurityCommodity(t, f, "NEWCO")
	buyOn(t, f, "2026-02-01", 100, 100000)
	merged, err := f.investmentService.CashStockMerger(ctx, cashStockMergerInput(f, newID, "2026-06-01", "2026-06-05", 1, 2, 30000, "25", 2))
	require.NoError(t, err)
	_, err = f.investmentService.ReverseShareExchange(ctx, ReverseShareExchangeInput{OwnerUserID: f.ownerUserID,
		TransactionID: merged.Transaction.ID, Reason: "wrong"})
	require.ErrorIs(t, err, ErrShareExchangeNotFound)
	_, err = f.investmentService.ReverseSale(ctx, ReverseInvestmentSaleInput{OwnerUserID: f.ownerUserID,
		TransactionID: merged.Transaction.ID, Reason: "wrong"})
	require.ErrorIs(t, err, ErrInvestmentSaleNotFound)
	_, err = f.transactionService.VoidTransaction(ctx, VoidTransactionInput{OwnerUserID: f.ownerUserID,
		TransactionID: merged.Transaction.ID, ChangeReason: "wrong"})
	require.Error(t, err, "the generic void path refuses an investment journal")
	requireInvestmentSelfCheckPasses(t, f)
}

// The fraction a merger leaves is settled by a cash in lieu linked to it.
func TestCashInLieuOfCashStockMergerFraction(t *testing.T) {
	t.Parallel()
	f := newInvestmentsTestFixture(t)
	ctx := context.Background()
	newID := seedTestSecurityCommodity(t, f, "NEWCO")
	buyOn(t, f, "2026-02-01", 3, 3000)
	merged, err := f.investmentService.CashStockMerger(ctx, cashStockMergerInput(f, newID, "2026-06-01", "2026-06-05", 3, 2, 900, "2", 1))
	require.NoError(t, err)
	_, err = f.investmentService.CashInLieu(ctx, cashInLieuInput(f, merged.Transaction.ID))
	require.NoError(t, err)
	quantity, _ := openCommodityPosition(t, f, f.holdingAccountID, newID)
	requireScaled(t, 4, 0, quantity, "4.5 new units less the 0.5 fraction")
	requireInvestmentSelfCheckPasses(t, f)
}

// The bundle carries the election on the transfer fact, the payable date
// with its sequence, and the quantity-0 cash decision with its proceeds.
func TestCashStockMergerExportCarriesElectionDatesAndDecision(t *testing.T) {
	t.Parallel()
	f := newInvestmentsTestFixture(t)
	ctx := context.Background()
	newID := seedTestSecurityCommodity(t, f, "NEWCO")
	buyOn(t, f, "2026-02-01", 100, 100000)
	merged, err := f.investmentService.CashStockMerger(ctx, cashStockMergerInput(f, newID, "2026-06-01", "2026-06-05", 1, 2, 30000, "25", 2))
	require.NoError(t, err)
	operationID := strconv.FormatInt(transferOperationID(t, f, merged.Transaction.ID), 10)

	var out bytes.Buffer
	require.NoError(t, NewExportService(db.NewExportRepository(f.database)).WriteBundle(ctx, &out, ExportFilter{}))
	archive, err := zip.NewReader(bytes.NewReader(out.Bytes()), int64(out.Len()))
	require.NoError(t, err)
	read := func(name string) []map[string]string {
		t.Helper()
		file, err := archive.Open(name)
		require.NoError(t, err)
		defer file.Close()
		rows, err := csv.NewReader(file).ReadAll()
		require.NoError(t, err)
		rows[0][0] = strings.TrimPrefix(rows[0][0], "\ufeff") // the bundle's CSV byte-order mark
		var records []map[string]string
		for _, row := range rows[1:] {
			record := map[string]string{}
			for index, column := range rows[0] {
				record[column] = row[index]
			}
			records = append(records, record)
		}
		return records
	}
	var fact map[string]string
	for _, record := range read("investment-transfer-facts.csv") {
		if record["operation_id"] == operationID {
			fact = record
		}
	}
	require.NotNil(t, fact)
	assert.Equal(t, [3]string{"exchange", "25", "2"}, [3]string{fact["transfer_kind"], fact["cash_basis_fraction_value"], fact["cash_basis_fraction_scale"]})
	dates := map[string]string{}
	for _, record := range read("investment-operation-dates.csv") {
		if record["operation_id"] == operationID {
			dates[record["date_role"]+"/"+record["date_seq"]] = record["event_date"]
		}
	}
	assert.Equal(t, map[string]string{"effective/1": "2026-06-01", "payable/1": "2026-06-05"}, dates)
	var decision map[string]string
	for _, record := range read("disposal-decisions.csv") {
		if record["transaction_id"] == strconv.FormatInt(merged.Transaction.ID, 10) {
			decision = record
		}
	}
	require.NotNil(t, decision)
	assert.Equal(t, [2]string{"0", "30000"}, [2]string{decision["quantity"], decision["proceeds_value"]})
}
