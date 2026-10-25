package app

import (
	"context"
	"fmt"
	"math/big"
	"sort"
	"time"

	"rekenraam/backend/internal/db"
	"rekenraam/backend/internal/exact"
)

// R13 slice 8: per-instrument returns, policy instrument_returns_v1
// (docs/plans/investment-returns-plan.md, "Per-instrument returns"). Each
// instrument's sleeve is its units in the portfolio scope and never cash, so
// a sale's proceeds or a dividend's cash leave the sleeve wherever they land.
// Like the portfolio read it is read-only: income is attached to an
// instrument at read time and nothing is written.

const (
	InstrumentReturnsPolicy           = "instrument_returns_v1"
	InstrumentIncomeRecognitionExDate = "ex_date"
)

const (
	ReturnReasonShortPosition             = "short_position"
	ReturnReasonIncomeUnattributable      = "income_unattributable"
	ReturnReasonUnclassifiedExchange      = "unclassified_exchange"
	ReturnReasonInstrumentGainUnavailable = "instrument_gain_unavailable"
)

// returnsExchangeKinds are the corporate actions a sleeve values by
// consideration (D6). A journal holding two instruments is not one of them.
var returnsExchangeKinds = map[string]bool{"share_exchange": true, "cash_and_stock_merger": true, "spin_off": true}

// returnsShortKinds open or close a named short (D5).
var returnsShortKinds = map[string]bool{"short_sale": true, "short_cover": true}

// InstrumentReturns is the per-instrument part of a returns read.
type InstrumentReturns struct {
	Policy            string
	IncomeRecognition string
	Rows              []InstrumentReturnsRow
	NotAttributed     []InstrumentIncomeNotAttributed
	// NotAttributedGain is the portfolio gain less every sleeve's gain: a
	// difference, never a computed breakdown.
	NotAttributedGain ReturnsAmount
}

// InstrumentReturnsRow is one sleeve's figures, from the portfolio's own
// calculation over the sleeve's values and flows.
type InstrumentReturnsRow struct {
	CommodityID   int64
	Short         bool
	OpeningValue  ReturnsAmount
	ClosingValue  ReturnsAmount
	Contributions ReturnsAmount
	Withdrawals   ReturnsAmount
	// Income is the signed income attributed to the instrument: dividends,
	// capital returns and reclaims positive, withholding-only payments and
	// payments in lieu negative. Each also counts in contributions or
	// withdrawals by its own sign.
	Income                 ReturnsAmount
	Gain                   ReturnsAmount
	TimeWeighted           ReturnFigure
	TimeWeightedAnnualized ReturnFigure
	MoneyWeighted          ReturnFigure
	MoneyWeightedPeriod    ReturnFigure
	Valuation              ValuationCoverage
}

// InstrumentIncomeNotAttributed is income recognised in the period that no
// sleeve carries, and why. InstrumentID is zero when the facts name none.
type InstrumentIncomeNotAttributed struct {
	TransactionID int64
	OperationID   int64
	OperationKind string
	Date          string
	InstrumentID  int64
	Reason        string
}

// sleeveGroup is one operation's postings recognised on one day, or one plain
// journal entry: the unit a sleeve flow is read from (review 1 of the plan).
type sleeveGroup struct {
	date        string
	identity    string
	anchor      db.ReturnsOperationAnchor
	isOperation bool
	postings    []db.LedgerPostingRecord
}

// sleeveTables are one sleeve's rate tables, sharing loaded observations but
// recording their own coverage.
type sleeveTables struct {
	rates, series *RateTable
}

// instrumentReturns computes every sleeve from the same snapshot as the
// portfolio figures. It reads the portfolio's results only (its exact gain,
// for the residual) and changes none of them.
func (r *returnsReader) instrumentReturns(ctx context.Context, anchors []db.ReturnsOperationAnchor, reporting ReportingCurrencyInput,
	boundary db.ObservationBoundary, days int) (InstrumentReturns, error) {
	result := InstrumentReturns{Policy: InstrumentReturnsPolicy, IncomeRecognition: InstrumentIncomeRecognitionExDate,
		Rows: []InstrumentReturnsRow{}, NotAttributed: []InstrumentIncomeNotAttributed{}}
	commodities := []int64{}
	for _, posting := range r.recognised {
		commodities = append(commodities, posting.CommodityID)
	}
	// Separate tables from the portfolio's, so its coverage is untouched.
	base, err := newRateTableAtBoundary(ctx, r.pricing, reporting, dedupeIDs(commodities), r.openDate, r.to, boundary)
	if err != nil {
		return result, err
	}
	codes, err := r.ledger.ReturnsCommodityCodes(ctx, BookID)
	if err != nil {
		return result, err
	}
	code := ""
	if currency, err := r.pricing.ReportingCurrency(ctx, BookID, reporting.CommodityID); err == nil {
		code = currency.Code
	}
	tables := map[int64]sleeveTables{}
	tableFor := func(commodityID int64) sleeveTables {
		if _, ok := tables[commodityID]; !ok {
			tables[commodityID] = sleeveTables{rates: base.withFreshCoverage(), series: base.withFreshCoverage()}
		}
		return tables[commodityID]
	}

	flows, income, incomeUnknown, notAttributed, involved, shortOps := r.sleeveFlows(anchors, tableFor)
	result.NotAttributed = notAttributed

	// A sleeve is listed when it moved in the period, carried income or had
	// units at either boundary.
	opening, closing := map[int64]*exact.ScaledInt{}, map[int64]*exact.ScaledInt{}
	negative := map[int64]bool{}
	running := map[int64]*exact.ScaledInt{}
	for i := 0; i < len(r.postings); {
		date := r.postings[i].EntryDate
		for ; i < len(r.postings) && r.postings[i].EntryDate == date; i++ {
			posting := r.postings[i]
			if r.kinds[posting.CommodityID] == "currency" || !r.inScope[posting.AccountID] {
				continue
			}
			if running[posting.CommodityID] == nil {
				running[posting.CommodityID] = exact.NewScaledInt()
			}
			running[posting.CommodityID].AddCoefficient(posting.QuantityValue, posting.QuantityScale)
			if date >= r.from {
				involved[posting.CommodityID] = true
			}
		}
		// D5: a scope balance below zero at the end of any day of the period.
		if date >= r.openDate {
			for commodityID, balance := range running {
				if balance.Sign() < 0 {
					negative[commodityID] = true
				}
			}
		}
	}
	for _, posting := range r.postings {
		if r.kinds[posting.CommodityID] == "currency" || !r.inScope[posting.AccountID] {
			continue
		}
		for _, side := range []struct {
			balances map[int64]*exact.ScaledInt
			through  string
		}{{opening, r.openDate}, {closing, r.to}} {
			if posting.EntryDate > side.through {
				continue
			}
			if side.balances[posting.CommodityID] == nil {
				side.balances[posting.CommodityID] = exact.NewScaledInt()
			}
			side.balances[posting.CommodityID].AddCoefficient(posting.QuantityValue, posting.QuantityScale)
		}
	}
	for commodityID, balance := range opening {
		if balance.Sign() != 0 {
			involved[commodityID] = true
			// A balance already negative at the opening boundary is short too.
			if balance.Sign() < 0 {
				negative[commodityID] = true
			}
		}
	}
	for commodityID, balance := range closing {
		if balance.Sign() != 0 {
			involved[commodityID] = true
		}
	}

	ids := make([]int64, 0, len(involved))
	for id := range involved {
		if r.kinds[id] != "currency" {
			ids = append(ids, id)
		}
	}
	sort.Slice(ids, func(i, j int) bool {
		if codes[ids[i]] != codes[ids[j]] {
			return codes[ids[i]] < codes[ids[j]]
		}
		return ids[i] < ids[j]
	})

	sumGains, gainsKnown := new(big.Rat), true
	for _, commodityID := range ids {
		table := tableFor(commodityID)
		sleeve := r.sleeve(commodityID, table)
		figures := ReturnsResult{Days: days}
		sleeve.compute(flows[commodityID], &figures)
		if sleeve.overflow != nil && r.overflow == nil {
			r.overflow = sleeve.overflow
		}
		row := InstrumentReturnsRow{
			CommodityID: commodityID, OpeningValue: figures.OpeningValue, ClosingValue: figures.ClosingValue,
			Contributions: figures.Contributions, Withdrawals: figures.Withdrawals, Gain: figures.Gain,
			TimeWeighted: figures.TimeWeighted, TimeWeightedAnnualized: figures.TimeWeightedAnnualized,
			MoneyWeighted: figures.MoneyWeighted, MoneyWeightedPeriod: figures.MoneyWeightedPeriod,
		}
		if reason := incomeUnknown[commodityID]; reason != "" {
			row.Income = ReturnsAmount{Reason: reason}
		} else {
			amount := income[commodityID]
			if amount == nil {
				amount = new(big.Rat)
			}
			row.Income = ReturnsAmount{Amount: sleeve.quantity(amount)}
		}
		// A sleeve holds no cash to carry a short's capital, so no positive
		// denominator exists (D5); values, flows and gain stay.
		if shortOps[commodityID] || negative[commodityID] {
			row.Short = true
			short := unavailableReturn(ReturnReasonShortPosition, "")
			row.TimeWeighted, row.TimeWeightedAnnualized = short, short
			row.MoneyWeighted, row.MoneyWeightedPeriod = short, short
		}
		row.Valuation = table.rates.Coverage(code)
		if sleeve.gain == nil {
			gainsKnown = false
		} else {
			sumGains.Add(sumGains, sleeve.gain)
		}
		result.Rows = append(result.Rows, row)
	}

	switch {
	case r.gain == nil:
		result.NotAttributedGain = ReturnsAmount{Reason: ReturnReasonInstrumentGainUnavailable}
	case !gainsKnown:
		result.NotAttributedGain = ReturnsAmount{Reason: ReturnReasonInstrumentGainUnavailable}
	default:
		result.NotAttributedGain = ReturnsAmount{Amount: r.quantity(new(big.Rat).Sub(r.gain, sumGains))}
	}
	return result, nil
}

// sleeve is the portfolio reader restricted to one instrument's postings and
// valued through the sleeve's own tables, so compute applies the same
// evaluation days, chain, money-weighted rule and failure policy.
func (r *returnsReader) sleeve(commodityID int64, tables sleeveTables) *returnsReader {
	postings := []db.LedgerPostingRecord{}
	for _, posting := range r.postings {
		if posting.CommodityID == commodityID {
			postings = append(postings, posting)
		}
	}
	return &returnsReader{
		ledger: r.ledger, pricing: r.pricing, openDate: r.openDate, from: r.from, to: r.to,
		accounts: r.accounts, inScope: r.inScope, holdings: r.holdings, kinds: r.kinds,
		postings: postings, shorts: r.shorts, splits: r.splits,
		rates: tables.rates, seriesRates: tables.series,
	}
}

// sleeveFlows reads every sleeve's flows from operation-wide groups. It
// returns, per instrument: flows, signed attributed income and why some of it
// is unknown (unattributable, or a missing conversion); the income no sleeve carries; the instruments a
// flow involved; and those a named short operation touched.
func (r *returnsReader) sleeveFlows(anchors []db.ReturnsOperationAnchor, tableFor func(int64) sleeveTables) (
	map[int64][]returnsEntryFlow, map[int64]*big.Rat, map[int64]string, []InstrumentIncomeNotAttributed, map[int64]bool, map[int64]bool) {
	byTransaction := map[int64]db.ReturnsOperationAnchor{}
	for _, anchor := range anchors {
		byTransaction[anchor.TransactionID] = anchor
	}
	held := newReturnsHoldingIndex(r.postings, r.holdings)

	var groups []*sleeveGroup
	index := map[string]*sleeveGroup{}
	for _, posting := range r.recognised {
		anchor, isOperation := byTransaction[posting.TransactionID]
		date := posting.EntryDate
		// D2: attributed income is recognised on its ex date when recorded.
		if isOperation && returnsIncomeAnchor(anchor) && anchor.ExDate.Valid {
			date = anchor.ExDate.String
		}
		if date < r.from || date > r.to {
			continue
		}
		key := fmt.Sprintf("entry:%d", posting.JournalEntryID)
		identity := key
		if isOperation {
			// By transaction, not operation: a replacement links its inverse
			// and its new journal to one operation, each with its own facts,
			// and each is attributed on its own before the chain nets them.
			key = fmt.Sprintf("transaction:%d:%s", posting.TransactionID, date)
			identity = fmt.Sprintf("operation:%d", anchor.RootOperationID)
		}
		group, ok := index[key]
		if !ok {
			group = &sleeveGroup{date: date, identity: identity, anchor: anchor, isOperation: isOperation}
			index[key] = group
			groups = append(groups, group)
		}
		group.postings = append(group.postings, posting)
	}

	flows := map[int64][]returnsEntryFlow{}
	income := map[int64]*big.Rat{}
	incomeUnknown := map[int64]string{}
	// shortUnits nets each short operation chain's units per instrument, so
	// a short that was reversed leaves no exposure behind (review finding 2).
	shortUnits := map[int64]map[string]*exact.ScaledInt{}
	involved := map[int64]bool{}
	shortOps := map[int64]bool{}
	var notAttributed []InstrumentIncomeNotAttributed
	add := func(commodityID int64, flow returnsEntryFlow) {
		involved[commodityID] = true
		flows[commodityID] = append(flows[commodityID], flow)
	}

	for _, group := range groups {
		units := map[int64][]db.LedgerPostingRecord{}
		for _, posting := range group.postings {
			if r.kinds[posting.CommodityID] != "currency" && r.inScope[posting.AccountID] {
				units[posting.CommodityID] = append(units[posting.CommodityID], posting)
			}
		}
		if group.isOperation && (returnsShortKinds[group.anchor.OperationKind] || returnsShortKinds[group.anchor.SourceKind]) {
			for commodityID, postings := range units {
				if shortUnits[commodityID] == nil {
					shortUnits[commodityID] = map[string]*exact.ScaledInt{}
				}
				if shortUnits[commodityID][group.identity] == nil {
					shortUnits[commodityID][group.identity] = exact.NewScaledInt()
				}
				for _, posting := range postings {
					shortUnits[commodityID][group.identity].AddCoefficient(posting.QuantityValue, posting.QuantityScale)
				}
			}
		}
		// outside values −Σ value(O_i): everything but the sleeve's own units,
		// the commodity-trading leg and performance, on the group's day (D7).
		outside := func(commodityID int64) returnsEntryFlow {
			flow := returnsEntryFlow{identity: group.identity, date: group.date, amount: new(big.Rat), ok: true}
			for _, posting := range group.postings {
				if (posting.CommodityID == commodityID && r.inScope[posting.AccountID]) ||
					!r.isOutsideBalance(posting.AccountID) {
					continue
				}
				value, ok := r.sleeveValue(tableFor(commodityID).rates, group, posting)
				if !ok {
					flow.ok = false
					continue
				}
				flow.amount.Sub(flow.amount, value)
			}
			return flow
		}

		if group.isOperation && returnsIncomeAnchor(group.anchor) {
			commodityID, reason := r.incomeTarget(group, held)
			switch reason {
			case "":
				flow := outside(commodityID)
				add(commodityID, flow)
				if flow.ok {
					if income[commodityID] == nil {
						income[commodityID] = new(big.Rat)
					}
					income[commodityID].Sub(income[commodityID], flow.amount)
				} else if incomeUnknown[commodityID] == "" {
					// The payer is known; only a conversion is missing.
					incomeUnknown[commodityID] = ReturnReasonFlowUnavailable
				}
			case ReturnsFallbackHeldInsideAndOutside:
				// D3: refused, but its direction is known from the cash.
				flow := outside(commodityID)
				sign := 0
				if flow.ok {
					sign = flow.amount.Sign()
				}
				add(commodityID, returnsEntryFlow{identity: group.identity, date: group.date, ok: false,
					reason: ReturnReasonIncomeUnattributable, sign: sign})
				incomeUnknown[commodityID] = ReturnReasonIncomeUnattributable
				notAttributed = append(notAttributed, r.notAttributed(group, commodityID, reason))
			case returnsIncomeOutside:
			default:
				// Listed wherever the cash landed, as the portfolio read
				// lists its fallbacks (review finding 3).
				notAttributed = append(notAttributed, r.notAttributed(group, commodityID, reason))
			}
			continue
		}

		if group.isOperation && group.anchor.Exchange != nil && returnsExchangeKinds[group.anchor.SourceKind] {
			r.exchangeFlows(group, units, tableFor, add)
			continue
		}

		delivery := group.isOperation && returnsDeliveryKinds[group.anchor.SourceKind]
		for _, posting := range group.postings {
			if !r.inScope[posting.AccountID] && r.isOutsideBalance(posting.AccountID) &&
				r.accounts[posting.AccountID].AccountClass == "equity" {
				delivery = true
			}
		}
		switch {
		case len(units) == 0:
		case delivery:
			// What entered or left, at market; performance booked with a
			// single instrument's delivery is netted from it.
			for commodityID := range units {
				flow := returnsEntryFlow{identity: group.identity, date: group.date, amount: new(big.Rat), ok: true}
				for _, posting := range group.postings {
					own := posting.CommodityID == commodityID && r.inScope[posting.AccountID]
					if !own && !(len(units) == 1 && r.isPerformance(posting.AccountID)) {
						continue
					}
					value, ok := r.sleeveValue(tableFor(commodityID).rates, group, posting)
					if !ok {
						flow.ok = false
						continue
					}
					flow.amount.Add(flow.amount, value)
				}
				add(commodityID, flow)
			}
		case len(units) > 1:
			// Two instruments with no exchange fact: the ledger does not say
			// what was given for what, so the read never guesses.
			for commodityID := range units {
				add(commodityID, returnsEntryFlow{identity: group.identity, date: group.date, ok: false,
					reason: ReturnReasonUnclassifiedExchange})
			}
		default:
			for commodityID := range units {
				add(commodityID, outside(commodityID))
			}
		}
	}
	sort.SliceStable(notAttributed, func(i, j int) bool {
		if notAttributed[i].Date != notAttributed[j].Date {
			return notAttributed[i].Date < notAttributed[j].Date
		}
		return notAttributed[i].TransactionID < notAttributed[j].TransactionID
	})
	if notAttributed == nil {
		notAttributed = []InstrumentIncomeNotAttributed{}
	}
	for commodityID, chains := range shortUnits {
		for _, units := range chains {
			if units.Sign() != 0 {
				shortOps[commodityID] = true
			}
		}
	}
	return flows, income, incomeUnknown, notAttributed, involved, shortOps
}

// returnsIncomeOutside is an incomeTarget result: the income belongs to a
// position outside the scope, so it is neither attributed nor listed.
const returnsIncomeOutside = "outside"

// returnsIncomeAnchor is an operation whose facts name income's position: a
// dividend, withholding or payment in lieu (instrument fact), or a capital
// return (holding and instrument).
func returnsIncomeAnchor(anchor db.ReturnsOperationAnchor) bool {
	return anchor.InstrumentFact || anchor.HoldingCommodityID.Valid
}

// incomeTarget is the instrument income is attributed to, or why it is not:
// the fact's holding decides when it names one; otherwise the instrument's
// holdings at the end of the day before the ex date (the entry date when no
// ex date is recorded), the portfolio read's own entitlement rule.
func (r *returnsReader) incomeTarget(group *sleeveGroup, held returnsHoldingIndex) (int64, string) {
	anchor := group.anchor
	commodityID := anchor.HoldingCommodityID.Int64
	if !anchor.HoldingCommodityID.Valid {
		commodityID = anchor.InstrumentID.Int64
	}
	if anchor.HoldingAccountID.Valid {
		if !r.inScope[anchor.HoldingAccountID.Int64] {
			return 0, returnsIncomeOutside
		}
		if commodityID == 0 {
			return 0, ReturnsFallbackUnassignedInstrument
		}
		return commodityID, ""
	}
	if commodityID == 0 {
		return 0, ReturnsFallbackUnassignedInstrument
	}
	date := group.date
	if anchor.ExDate.Valid {
		if parsed, err := time.Parse(time.DateOnly, anchor.ExDate.String); err == nil {
			date = parsed.AddDate(0, 0, -1).Format(time.DateOnly)
		}
	}
	inside, outsideHeld := false, false
	for accountID, quantity := range held.at(commodityID, date) {
		if quantity.Sign() == 0 {
			continue
		}
		inside = inside || r.inScope[accountID]
		outsideHeld = outsideHeld || !r.inScope[accountID]
	}
	switch {
	case inside && outsideHeld:
		return commodityID, ReturnsFallbackHeldInsideAndOutside
	case inside:
		return commodityID, ""
	case outsideHeld:
		return commodityID, returnsIncomeOutside
	}
	return commodityID, ReturnsFallbackNotHeld
}

func (r *returnsReader) notAttributed(group *sleeveGroup, commodityID int64, reason string) InstrumentIncomeNotAttributed {
	return InstrumentIncomeNotAttributed{TransactionID: group.postings[0].TransactionID, OperationID: group.anchor.OperationID,
		OperationKind: group.anchor.OperationKind, Date: group.date, InstrumentID: commodityID, Reason: reason}
}

// exchangeFlows values an exchange by the consideration received at it (D6):
// the source gives up the market value of the new units plus the action's own
// cash, and the destination receives the value of its own units. Cash in lieu
// is a later, separate disposal of the destination's fraction, never part of
// this. A reversal posts every leg mirrored, so its flows cancel the
// original's at the same prices.
func (r *returnsReader) exchangeFlows(group *sleeveGroup, units map[int64][]db.LedgerPostingRecord,
	tableFor func(int64) sleeveTables, add func(int64, returnsEntryFlow)) {
	fact := group.anchor.Exchange
	source, destination := fact.SourceCommodityID, fact.DestinationCommodityID
	sourceInScope := len(units[source]) > 0
	if fact.SourceAccountID.Valid {
		sourceInScope = r.inScope[fact.SourceAccountID.Int64]
	}
	if sourceInScope {
		flow := returnsEntryFlow{identity: group.identity, date: group.date, amount: new(big.Rat), ok: true}
		for _, posting := range group.postings {
			received := posting.CommodityID == destination || r.kinds[posting.CommodityID] == "currency"
			if !received || !r.isOutsideBalance(posting.AccountID) {
				continue
			}
			value, ok := r.sleeveValue(tableFor(source).rates, group, posting)
			if !ok {
				flow.ok = false
				continue
			}
			flow.amount.Sub(flow.amount, value)
		}
		add(source, flow)
	}
	if len(units[destination]) > 0 {
		flow := returnsEntryFlow{identity: group.identity, date: group.date, amount: new(big.Rat), ok: true}
		for _, posting := range units[destination] {
			value, ok := r.sleeveValue(tableFor(destination).rates, group, posting)
			if !ok {
				flow.ok = false
				continue
			}
			flow.amount.Add(flow.amount, value)
		}
		add(destination, flow)
	}
}

// sleeveValue values one posting of a group on the group's day, through a
// sleeve's table. As in the portfolio read, units an operation moved before a
// split that day are pre-split units, each of which the split multiplied.
func (r *returnsReader) sleeveValue(rates *RateTable, group *sleeveGroup, posting db.LedgerPostingRecord) (*big.Rat, bool) {
	value, ok := rates.ConvertExact(posting.QuantityValue, posting.QuantityScale, posting.CommodityID, group.date)
	if !ok {
		return nil, false
	}
	if group.isOperation {
		if pending := db.PendingSplitFactor(r.splits, posting.CommodityID, posting.AccountID, posting.EntryDate, group.anchor.RootOperationID); pending != nil {
			value.Mul(value, pending)
		}
	}
	return value, true
}
