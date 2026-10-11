package app

import (
	"context"
	"database/sql"
	"sort"
	"testing"

	"github.com/stretchr/testify/require"

	"rekenraam/backend/internal/db"
	"rekenraam/backend/internal/exact"
)

// #190: an independent conservation oracle. Replay equivalence compares the
// stored projection with replay, and #189's owner assembles effective history;
// both can agree while a reader is wrong (#188 passed every self-check). Each
// property here takes its expectation from somewhere else: the posted journal,
// the operation's own facts, or the immutable opening evidence.
//
//  1. Dated quantities: for every holding and every date, the long units the
//     effective movements hold minus the units named shorts owe equal the
//     holding's posted journal balance through that date.
//  2. Basis conservation per operation: an internal transfer, split, spin-off
//     or share exchange moves basis without creating or destroying it; a
//     cash-and-stock merger's exchange legs net to zero beside its cash
//     disposal; a return of capital removes exactly its receipt less the excess
//     it records.
//  3. Unknown basis: a root lot (one no transfer link opened) whose opening
//     basis is unknown and has no effective sourced resolution stays unknown,
//     and nothing taken out of it becomes known.
func requireConservationOracle(t *testing.T, f *investmentsTestFixture) {
	t.Helper()
	requireDatedQuantitiesMatchJournal(t, f)
	requireOperationBasisConserved(t, f)
	requireUnresolvedBasisStaysUnknown(t, f)
}

type oracleHolding struct{ accountID, commodityID int64 }

func requireDatedQuantitiesMatchJournal(t *testing.T, f *investmentsTestFixture) {
	t.Helper()
	ctx := context.Background()
	// Holdings are every account/instrument pair a lot has ever been opened in.
	holdings := map[oracleHolding]bool{}
	rows, err := f.database.Query(`SELECT DISTINCT account_id, commodity_id FROM investment_lots`)
	require.NoError(t, err)
	for rows.Next() {
		var holding oracleHolding
		require.NoError(t, rows.Scan(&holding.accountID, &holding.commodityID))
		holdings[holding] = true
	}
	require.NoError(t, rows.Err())
	require.NoError(t, rows.Close())

	deltas := map[oracleHolding]map[string]*exact.ScaledInt{}
	add := func(holding oracleHolding, date string, value exact.Coefficient, scale int, negate bool) {
		if deltas[holding] == nil {
			deltas[holding] = map[string]*exact.ScaledInt{}
		}
		if deltas[holding][date] == nil {
			deltas[holding][date] = exact.NewScaledInt()
		}
		amount := exact.ScaledIntFromCoefficient(value, scale)
		if negate {
			amount = amount.Negated()
		}
		deltas[holding][date].AddScaled(amount)
	}
	// Expectation: the posted journal, independent of lots and replay.
	postings, err := db.NewTransactionRepository(f.database).LedgerPostingsThrough(ctx, BookID, "", "posted")
	require.NoError(t, err)
	for _, posting := range postings {
		holding := oracleHolding{posting.AccountID, posting.CommodityID}
		if holdings[holding] {
			add(holding, posting.EntryDate, posting.QuantityValue, posting.QuantityScale, true)
		}
	}
	// Subject: long units from effective movements, owed units from the
	// named-short reader that net worth and the sign self-check use.
	for _, movement := range effectiveMovements(t, f, db.EffectiveMovementFilter{PositionSide: db.PositionSideLong}) {
		add(oracleHolding{movement.AccountID, movement.CommodityID}, movement.EventDate,
			movement.QuantityValue, movement.QuantityScale, false)
	}
	shorts, err := db.NewTransactionRepository(f.database).ShortLotQuantityEventsThrough(ctx, BookID, "")
	require.NoError(t, err)
	for _, event := range shorts {
		add(oracleHolding{event.AccountID, event.CommodityID}, event.EventDate, event.QuantityValue, event.QuantityScale, true)
	}
	for holding, byDate := range deltas {
		dates := make([]string, 0, len(byDate))
		for date := range byDate {
			dates = append(dates, date)
		}
		sort.Strings(dates)
		running := exact.NewScaledInt()
		for _, date := range dates {
			running.AddScaled(byDate[date])
			require.Zerof(t, running.Sign(),
				"holding %d/%d on %s: lots and named shorts differ from the journal by %s",
				holding.accountID, holding.commodityID, date, running.String())
		}
	}
}

func requireOperationBasisConserved(t *testing.T, f *investmentsTestFixture) {
	t.Helper()
	type operationBasis struct {
		sum, exchangeLegs *exact.ScaledInt
		unknown           bool
	}
	byOperation := map[int64]*operationBasis{}
	for _, movement := range effectiveMovements(t, f, db.EffectiveMovementFilter{}) {
		if movement.OperationID == 0 {
			continue
		}
		basis := byOperation[movement.OperationID]
		if basis == nil {
			basis = &operationBasis{sum: exact.NewScaledInt(), exchangeLegs: exact.NewScaledInt()}
			byOperation[movement.OperationID] = basis
		}
		if movement.BasisKnowledge == db.InvestmentBasisUnknown {
			basis.unknown = true
			continue
		}
		basis.sum.AddInt64(movement.CostBasisValue, movement.CostBasisScale)
		if movement.Kind != "disposal" {
			basis.exchangeLegs.AddInt64(movement.CostBasisValue, movement.CostBasisScale)
		}
	}
	for operationID, basis := range byOperation {
		if basis.unknown {
			continue // property 3 covers unknown basis
		}
		var kind string
		require.NoError(t, f.database.QueryRow(`SELECT operation_kind FROM investment_operations WHERE id = ?`,
			operationID).Scan(&kind))
		switch kind {
		case "internal_transfer", "split", "spin_off", "share_exchange":
			require.Zerof(t, basis.sum.Sign(), "%s operation %d creates or destroys basis: %s", kind, operationID, basis.sum.String())
		case "cash_and_stock_merger":
			require.Zerof(t, basis.exchangeLegs.Sign(), "merger %d exchange legs do not net to zero: %s", operationID, basis.exchangeLegs.String())
		case "return_of_capital":
			// Expectation: the receipt fact and the excess the effective
			// effect set records; the subject is the basis the lots lost.
			var receipt exact.Coefficient
			var receiptScale int
			require.NoError(t, f.database.QueryRow(`SELECT amount_value, amount_scale
				FROM investment_capital_return_facts WHERE operation_id = ?`, operationID).Scan(&receipt, &receiptScale))
			expected := exact.ScaledIntFromCoefficient(receipt, receiptScale)
			expected.SubScaled(capitalReturnExcess(t, f.database, operationID))
			removed := basis.sum.Negated()
			require.Zerof(t, removed.Cmp(expected), "return of capital %d removed %s of basis, receipt less excess is %s",
				operationID, removed.String(), expected.String())
		}
	}
}

// capitalReturnExcess sums the excess of a return of capital's effective
// effect set: its latest revision's effects, else its original effects.
func capitalReturnExcess(t *testing.T, database *sql.DB, operationID int64) *exact.ScaledInt {
	t.Helper()
	rows, err := database.Query(`
		SELECT e.excess_value, e.excess_scale
		FROM latest_investment_capital_return_revisions revision
		JOIN investment_capital_return_revision_effects e ON e.revision_id = revision.id
		WHERE revision.operation_id = ?
		UNION ALL
		SELECT e.excess_value, e.excess_scale FROM investment_capital_return_effects e
		WHERE e.operation_id = ? AND NOT EXISTS (
			SELECT 1 FROM investment_capital_return_revisions revision WHERE revision.operation_id = ?)`,
		operationID, operationID, operationID)
	require.NoError(t, err)
	defer rows.Close()
	excess := exact.NewScaledInt()
	for rows.Next() {
		var value exact.Coefficient
		var scale int
		require.NoError(t, rows.Scan(&value, &scale))
		excess.AddCoefficient(value, scale)
	}
	require.NoError(t, rows.Err())
	return excess
}

func requireUnresolvedBasisStaysUnknown(t *testing.T, f *investmentsTestFixture) {
	t.Helper()
	// Expectation: the immutable opening evidence and the absence of an
	// effective sourced resolution for the lot.
	rows, err := f.database.Query(`
		SELECT l.id, COALESCE(state.basis_knowledge, ''), state.remaining_cost_basis_value IS NULL
		FROM investment_lots l
		LEFT JOIN investment_lot_state state ON state.lot_id = l.id
		WHERE l.opening_basis_knowledge = 'unknown'
			-- A root lot: no transfer link opened it. A link destination copies
			-- its source's knowledge and becomes known when that source is
			-- resolved, which the outgoing-movement check below covers.
			AND NOT EXISTS (SELECT 1 FROM investment_transfer_lot_links link WHERE link.destination_lot_id = l.id)
			AND NOT EXISTS (
			SELECT 1 FROM investment_basis_resolutions resolution
			JOIN effective_investment_operations operation ON operation.id = resolution.operation_id
			WHERE resolution.lot_id = l.id)`)
	require.NoError(t, err)
	unresolved := map[int64]bool{}
	for rows.Next() {
		var lotID int64
		var knowledge string
		var noAmount bool
		require.NoError(t, rows.Scan(&lotID, &knowledge, &noAmount))
		unresolved[lotID] = true
		if knowledge != "" {
			require.Equalf(t, db.InvestmentBasisUnknown, knowledge, "unresolved lot %d became known", lotID)
			require.Truef(t, noAmount, "unresolved lot %d carries a remaining basis amount", lotID)
		}
	}
	require.NoError(t, rows.Err())
	require.NoError(t, rows.Close())
	for _, movement := range effectiveMovements(t, f, db.EffectiveMovementFilter{}) {
		if unresolved[movement.LotID] && movement.QuantityValue.Sign() < 0 {
			require.Equalf(t, db.InvestmentBasisUnknown, movement.BasisKnowledge,
				"%s %s out of unresolved lot %d is known", movement.Source, movement.Kind, movement.LotID)
		}
	}
}
