package app

import (
	"context"
	"fmt"
	"maps"
	"math/big"
	"slices"
	"sort"
	"time"

	"rekenraam/backend/internal/db"
	"rekenraam/backend/internal/exact"
)

// R13 live returns read, policy returns_v1
// (docs/plans/investment-returns-plan.md). Read-only: it classifies posted
// ledger entries and existing investment facts, values the selected accounts
// at the breakpoints the time-weighted chain needs, and writes nothing — no
// posting, no transfer, no audit event. A dividend attributed to the
// portfolio but paid elsewhere is *read* as income plus a same-day
// withdrawal; the ledger keeps the one journal that was recorded.

const ReturnsPolicy = "returns_v1"

// Account scopes a returns read can resolve to.
const (
	ReturnsScopeDefaultInvestment = "default_investment_accounts"
	ReturnsScopeSelectedAccounts  = "selected_accounts"
)

// Why an investment operation's cash fell back to the cash-side rule.
const (
	ReturnsFallbackUnassignedInstrument = "unassigned_instrument"
	ReturnsFallbackHeldInsideAndOutside = "held_inside_and_outside"
	ReturnsFallbackNotHeld              = "not_held"
)

const returnsMaxStalenessDays = 31

// returnsDefaultBaseKinds are the account base kinds the default scope
// starts from; descendants join through the account tree.
var returnsDefaultBaseKinds = map[string]bool{
	"investment_container": true,
	"investment_cash":      true,
	"security_holding":     true,
	"digital_asset":        true,
}

type ReturnsInput struct {
	From                 string
	To                   string
	ReportingCommodityID int64
	AccountIDs           []int64
	MaxStalenessDays     int
}

// ReturnsAmount is a reporting-currency amount, or why it is missing.
type ReturnsAmount struct {
	Amount *BalanceQuantity
	Reason string
	Date   string
}

// ReturnsAttributionFallback is an investment operation's cash that the facts
// could not tie to one position, so the cash side decided it. Counted says
// whether it entered the portfolio's return.
type ReturnsAttributionFallback struct {
	TransactionID int64
	OperationID   int64
	OperationKind string
	Date          string
	Reason        string
	Counted       bool
}

type ReturnsResult struct {
	Policy             string
	From               string
	To                 string
	OpeningDate        string
	Days               int
	AccountScope       string
	ResolvedAccountIDs []int64
	KnowledgeCutoff    string

	OpeningValue  ReturnsAmount
	ClosingValue  ReturnsAmount
	Contributions ReturnsAmount
	Withdrawals   ReturnsAmount
	Gain          ReturnsAmount

	TimeWeighted           ReturnFigure
	TimeWeightedAnnualized ReturnFigure
	MoneyWeighted          ReturnFigure
	MoneyWeightedPeriod    ReturnFigure

	AttributionFallbacks []ReturnsAttributionFallback
	Valuation            ValuationCoverage
}

// ReadReturns computes one live returns read from one disposable image of
// the database, so every figure shares a ledger state and knowledge instant.
func (s *InvestmentService) ReadReturns(ctx context.Context, input ReturnsInput) (ReturnsResult, error) {
	fromDate, err := time.Parse(time.DateOnly, input.From)
	if err != nil {
		return ReturnsResult{}, ValidationError{Message: "from must be a date (YYYY-MM-DD)"}
	}
	toDate, err := time.Parse(time.DateOnly, input.To)
	if err != nil {
		return ReturnsResult{}, ValidationError{Message: "to must be a date (YYYY-MM-DD)"}
	}
	if toDate.Before(fromDate) {
		return ReturnsResult{}, ValidationError{Message: "from must not be after to"}
	}
	if input.ReportingCommodityID <= 0 {
		return ReturnsResult{}, ValidationError{Message: "reporting currency is required"}
	}
	staleness := input.MaxStalenessDays
	if staleness == 0 {
		staleness = DefaultRateStalenessDays
	}
	if staleness < 1 || staleness > returnsMaxStalenessDays {
		return ReturnsResult{}, ValidationError{Message: fmt.Sprintf("max_staleness_days must be between 1 and %d", returnsMaxStalenessDays)}
	}

	snapshot, cleanup, err := s.repository.LiveGainsSnapshot(ctx)
	if err != nil {
		return ReturnsResult{}, fmt.Errorf("capture returns input: %w", err)
	}
	defer cleanup()
	pricing := db.NewPricingRepository(snapshot)
	boundary, err := pricing.CurrentObservationBoundary(ctx, BookID, s.now())
	if err != nil {
		return ReturnsResult{}, err
	}
	if input.To > boundary.AsOf {
		return ReturnsResult{}, ValidationError{Message: "to must not be after today"}
	}
	reader := returnsReader{
		ledger:   db.NewTransactionRepository(snapshot),
		pricing:  pricing,
		openDate: fromDate.AddDate(0, 0, -1).Format(time.DateOnly),
		from:     input.From,
		to:       input.To,
	}
	result := ReturnsResult{
		Policy: ReturnsPolicy, From: input.From, To: input.To, OpeningDate: reader.openDate,
		Days: int(toDate.Sub(fromDate).Hours()/24) + 1, KnowledgeCutoff: boundary.RecordedAt,
	}
	if err := reader.read(ctx, input, staleness, boundary, &result); err != nil {
		return ReturnsResult{}, err
	}
	return result, nil
}

type returnsReader struct {
	ledger   *db.TransactionRepository
	pricing  *db.PricingRepository
	openDate string
	from, to string

	accounts map[int64]db.LedgerAccountRecord
	inScope  map[int64]bool
	holdings map[int64]bool
	kinds    map[int64]string
	postings []db.LedgerPostingRecord
	shorts   []db.ShortLotQuantityEvent
	rates    *RateTable
}

// returnsEntryFlow is one journal entry's external flow, or why it has none.
// identity groups entries whose flows must cancel before they are split into
// inflows and outflows: one investment correction chain, or else the entry.
type returnsEntryFlow struct {
	identity string
	date     string
	amount   *big.Rat
	ok       bool
}

// returnsDeliveryKinds move units across the portfolio boundary against a
// book-value bridge, so their flow is the market value of what moved.
var returnsDeliveryKinds = map[string]bool{
	"external_transfer_in":  true,
	"external_transfer_out": true,
}

func (r *returnsReader) read(ctx context.Context, input ReturnsInput, staleness int, boundary db.ObservationBoundary, result *ReturnsResult) error {
	if err := r.resolveScope(ctx, input.AccountIDs, result); err != nil {
		return err
	}
	var err error
	if r.postings, err = r.ledger.LedgerPostingsThrough(ctx, BookID, r.to, "posted"); err != nil {
		return fmt.Errorf("read returns postings: %w", err)
	}
	if r.kinds, err = r.ledger.LedgerCommodityKinds(ctx, BookID); err != nil {
		return err
	}
	if r.shorts, err = r.ledger.ShortLotQuantityEventsThrough(ctx, BookID, r.to); err != nil {
		return err
	}
	anchors, err := r.ledger.ReturnsOperationAnchors(ctx, BookID)
	if err != nil {
		return err
	}

	commodities := []int64{}
	for _, posting := range r.postings {
		commodities = append(commodities, posting.CommodityID)
	}
	r.rates, err = newRateTableAtBoundary(ctx, r.pricing, ReportingCurrencyInput{CommodityID: input.ReportingCommodityID, MaxStalenessDay: staleness},
		commodities, r.openDate, r.to, boundary)
	if err != nil {
		return err
	}
	defer func() {
		code := ""
		if currency, err := r.pricing.ReportingCurrency(ctx, BookID, input.ReportingCommodityID); err == nil {
			code = currency.Code
		}
		result.Valuation = r.rates.Coverage(code)
	}()

	flows, fallbacks := r.classify(anchors)
	result.AttributionFallbacks = fallbacks
	r.compute(flows, result)
	return nil
}

// resolveScope fixes the portfolio accounts once, as of the period end.
func (r *returnsReader) resolveScope(ctx context.Context, selected []int64, result *ReturnsResult) error {
	accounts, err := r.ledger.LedgerAccountsAsOf(ctx, BookID, r.to)
	if err != nil {
		return fmt.Errorf("read returns accounts: %w", err)
	}
	baseKinds, err := r.ledger.AccountBaseKinds(ctx)
	if err != nil {
		return err
	}
	r.accounts = map[int64]db.LedgerAccountRecord{}
	r.holdings = map[int64]bool{}
	for _, account := range accounts {
		r.accounts[account.ID] = account
		if baseKinds[account.AccountKind] == subledgerManagedBaseKind {
			r.holdings[account.ID] = true
		}
	}
	balanceSheet := func(account db.LedgerAccountRecord) bool {
		return !account.SystemRole.Valid && (account.AccountClass == "asset" || account.AccountClass == "liability")
	}

	roots := dedupeIDs(selected)
	result.AccountScope = ReturnsScopeSelectedAccounts
	if len(roots) == 0 {
		result.AccountScope = ReturnsScopeDefaultInvestment
		for _, account := range accounts {
			if balanceSheet(account) && returnsDefaultBaseKinds[baseKinds[account.AccountKind]] {
				roots = append(roots, account.ID)
			}
		}
	} else {
		for _, id := range roots {
			account, ok := r.accounts[id]
			if !ok {
				return ValidationError{Message: fmt.Sprintf("account %d does not exist on %s", id, r.to)}
			}
			if !balanceSheet(account) {
				return ValidationError{Message: "portfolio accounts must be non-system asset or liability accounts"}
			}
		}
	}
	r.inScope = map[int64]bool{}
	for _, id := range accountSubtreeIDs(accounts, roots) {
		if account, ok := r.accounts[id]; ok && balanceSheet(account) {
			r.inScope[id] = true
			result.ResolvedAccountIDs = append(result.ResolvedAccountIDs, id)
		}
	}
	slices.Sort(result.ResolvedAccountIDs)
	return nil
}

// classify turns every journal entry in the period into an external flow,
// deciding which side of the boundary it belongs to from the position its
// operation names before falling back to where its cash landed.
func (r *returnsReader) classify(anchors []db.ReturnsOperationAnchor) ([]returnsEntryFlow, []ReturnsAttributionFallback) {
	byTransaction := map[int64]db.ReturnsOperationAnchor{}
	operationTransactions := map[int64][]int64{}
	for _, anchor := range anchors {
		byTransaction[anchor.TransactionID] = anchor
		operationTransactions[anchor.OperationID] = append(operationTransactions[anchor.OperationID], anchor.TransactionID)
	}
	holdingsByTransaction := map[int64]map[int64]bool{}
	for _, posting := range r.postings {
		if r.holdings[posting.AccountID] {
			if holdingsByTransaction[posting.TransactionID] == nil {
				holdingsByTransaction[posting.TransactionID] = map[int64]bool{}
			}
			holdingsByTransaction[posting.TransactionID][posting.AccountID] = true
		}
	}
	held := newReturnsHoldingIndex(r.postings, r.holdings)

	var entries [][]db.LedgerPostingRecord
	index := map[int64]int{}
	for _, posting := range r.postings {
		if posting.EntryDate < r.from {
			continue
		}
		i, ok := index[posting.JournalEntryID]
		if !ok {
			i = len(entries)
			index[posting.JournalEntryID] = i
			entries = append(entries, nil)
		}
		entries[i] = append(entries[i], posting)
	}

	var flows []returnsEntryFlow
	var fallbacks []ReturnsAttributionFallback
	for _, entry := range entries {
		first := entry[0]
		touches := false
		for _, posting := range entry {
			touches = touches || r.inScope[posting.AccountID]
		}
		side, fallback := "", ""
		anchor, isOperation := byTransaction[first.TransactionID]
		if isOperation {
			side, fallback = r.anchorSide(anchor, operationTransactions[anchor.OperationID], holdingsByTransaction, held, first.EntryDate)
		}
		belongs := touches
		switch side {
		case "in":
			belongs = true
		case "out":
			belongs = false
		}
		// Every fallback is disclosed, counted or not: it is where the read
		// stopped knowing whose income this was.
		if fallback != "" {
			fallbacks = append(fallbacks, ReturnsAttributionFallback{TransactionID: first.TransactionID, OperationID: anchor.OperationID,
				OperationKind: anchor.OperationKind, Date: first.EntryDate, Reason: fallback, Counted: touches})
		}
		if !belongs && !touches {
			continue
		}
		identity := fmt.Sprintf("entry:%d", first.JournalEntryID)
		if isOperation {
			identity = fmt.Sprintf("operation:%d", anchor.RootOperationID)
		}
		delivery := belongs && isOperation && returnsDeliveryKinds[anchor.SourceKind]
		for _, posting := range entry {
			// An equity counterpart outside the portfolio is a book-value
			// bridge. The commodity-trading conversion leg is equity-class
			// in production too, but it is internal, never a bridge.
			if belongs && !r.inScope[posting.AccountID] && r.isOutsideBalance(posting.AccountID) &&
				r.accounts[posting.AccountID].AccountClass == "equity" {
				delivery = true
			}
		}
		flow := returnsEntryFlow{identity: identity, date: first.EntryDate, amount: new(big.Rat), ok: true}
		for _, posting := range entry {
			var sign int64
			switch {
			case delivery && (r.inScope[posting.AccountID] || r.isPerformance(posting.AccountID)):
				// What entered or left, at market, less performance booked
				// with it; equity counterparts carry book value only.
				sign = 1
			case delivery:
				continue
			case belongs && !r.inScope[posting.AccountID] && r.isOutsideBalance(posting.AccountID):
				sign = -1
			case !belongs && r.inScope[posting.AccountID]:
				sign = 1
			default:
				continue
			}
			value, ok := r.convert(posting.QuantityValue, posting.QuantityScale, posting.CommodityID, posting.EntryDate)
			if !ok {
				flow.ok = false
				continue
			}
			flow.amount.Add(flow.amount, value.Mul(value, big.NewRat(sign, 1)))
		}
		flows = append(flows, flow)
	}
	return flows, fallbacks
}

// isPerformance is a P posting's account: income or expense.
func (r *returnsReader) isPerformance(accountID int64) bool {
	account := r.accounts[accountID]
	return account.AccountClass == "income" || account.AccountClass == "expense"
}

// isOutsideBalance is an O posting's account: anything that is neither income
// or expense (performance) nor the commodity-trading conversion leg.
func (r *returnsReader) isOutsideBalance(accountID int64) bool {
	account, ok := r.accounts[accountID]
	if !ok {
		return true
	}
	if account.SystemRole.Valid && account.SystemRole.String == "commodity_trading" {
		return false
	}
	return account.AccountClass != "income" && account.AccountClass != "expense"
}

// anchorSide places an operation's entry inside ("in") or outside ("out") the
// portfolio from the position its facts name, or returns "" with the reason
// the cash side must decide.
func (r *returnsReader) anchorSide(anchor db.ReturnsOperationAnchor, transactions []int64, holdingsByTransaction map[int64]map[int64]bool,
	held returnsHoldingIndex, entryDate string) (string, string) {
	inside, outside := false, false
	for _, transactionID := range transactions {
		for accountID := range holdingsByTransaction[transactionID] {
			inside = inside || r.inScope[accountID]
			outside = outside || !r.inScope[accountID]
		}
	}
	if !inside && !outside && anchor.HoldingAccountID.Valid {
		inside, outside = r.inScope[anchor.HoldingAccountID.Int64], !r.inScope[anchor.HoldingAccountID.Int64]
	}
	switch {
	case inside && outside:
		return "", "" // mixed: the plain ledger rule is exact
	case inside:
		return "in", ""
	case outside:
		return "out", ""
	}
	if !anchor.InstrumentFact {
		return "", ""
	}
	if !anchor.InstrumentID.Valid {
		return "", ReturnsFallbackUnassignedInstrument
	}
	date := entryDate
	if anchor.ExDate.Valid {
		// Entitlement belongs to whoever held the units before the ex date.
		if parsed, err := time.Parse(time.DateOnly, anchor.ExDate.String); err == nil {
			date = parsed.AddDate(0, 0, -1).Format(time.DateOnly)
		}
	}
	for accountID, quantity := range held.at(anchor.InstrumentID.Int64, date) {
		if quantity.Sign() == 0 {
			continue
		}
		inside = inside || r.inScope[accountID]
		outside = outside || !r.inScope[accountID]
	}
	switch {
	case inside && outside:
		return "", ReturnsFallbackHeldInsideAndOutside
	case inside:
		return "in", ""
	case outside:
		return "out", ""
	}
	return "", ReturnsFallbackNotHeld
}

// convert values a posting exactly. Returns are ratios of these values, so
// they are never rounded to the reporting currency's scale before the chain
// divides by them: a positive holding worth €0.0001 must not read as zero
// capital. Only amounts shown to the reader are rounded, once.
func (r *returnsReader) convert(value exact.Coefficient, scale int, commodityID int64, date string) (*big.Rat, bool) {
	return r.rates.ConvertExact(value, scale, commodityID, date)
}

// returnsValuation is V(d), or why it is unknown.
type returnsValuation struct {
	value  *big.Rat
	reason string
}

// valuations values the portfolio at the end of each date, folding the
// ledger forward once.
func (r *returnsReader) valuations(dates []string) map[string]returnsValuation {
	sort.Strings(dates)
	dates = slices.Compact(dates)
	running := map[int64]map[int64]*exact.ScaledInt{}
	shorts := namedShortQuantities{}
	next, nextShort := 0, 0
	result := map[string]returnsValuation{}
	for _, date := range dates {
		for next < len(r.postings) && r.postings[next].EntryDate <= date {
			if r.inScope[r.postings[next].AccountID] {
				foldPosting(running, r.postings[next])
			}
			next++
		}
		for nextShort < len(r.shorts) && r.shorts[nextShort].EventDate <= date {
			shorts.fold(r.shorts[nextShort])
			nextShort++
		}
		totals := map[int64]*exact.ScaledInt{}
		valuation := returnsValuation{value: new(big.Rat)}
		for accountID, balances := range running {
			for commodityID, amount := range balances {
				if r.kinds[commodityID] != "currency" && shorts.unexplainedNegative(accountID, commodityID, amount) {
					valuation.reason = ReturnReasonUnclassifiedShort
				}
				if totals[commodityID] == nil {
					totals[commodityID] = exact.NewScaledInt()
				}
				totals[commodityID].AddScaled(amount)
			}
		}
		if valuation.reason == "" {
			ids := make([]int64, 0, len(totals))
			for id := range totals {
				ids = append(ids, id)
			}
			slices.Sort(ids)
			for _, id := range ids {
				total := totals[id]
				if total.Sign() == 0 {
					continue
				}
				coefficient, err := exact.FromBig(total.BigInt())
				if err != nil {
					valuation.reason = ReturnReasonValuationUnavailable
					break
				}
				value, ok := r.convert(coefficient, total.Scale(), id, date)
				if !ok {
					valuation.reason = ReturnReasonValuationUnavailable
					continue
				}
				valuation.value.Add(valuation.value, value)
			}
		}
		if valuation.reason != "" {
			valuation.value = nil
		}
		result[date] = valuation
	}
	return result
}

func (r *returnsReader) compute(flows []returnsEntryFlow, result *ReturnsResult) {
	// Net each correction chain (or entry) per day before splitting by sign,
	// so an erroneous payment and its same-day reversal leave no flow, while
	// independent same-day flows keep the inflow-start / outflow-end timing.
	type flowKey struct{ identity, date string }
	netted := map[flowKey]*big.Rat{}
	flowsKnown := true
	for _, flow := range flows {
		if !flow.ok {
			flowsKnown = false
			continue
		}
		key := flowKey{flow.identity, flow.date}
		if netted[key] == nil {
			netted[key] = new(big.Rat)
		}
		netted[key].Add(netted[key], flow.amount)
	}
	byDay := map[string]returnsDayFlow{}
	contributions, withdrawals := new(big.Rat), new(big.Rat)
	for key, amount := range netted {
		if amount.Sign() == 0 {
			continue
		}
		day, ok := byDay[key.date]
		if !ok {
			day = returnsDayFlow{In: new(big.Rat), Out: new(big.Rat)}
			byDay[key.date] = day
		}
		if amount.Sign() > 0 {
			day.In.Add(day.In, amount)
			contributions.Add(contributions, amount)
		} else {
			magnitude := new(big.Rat).Neg(amount)
			day.Out.Add(day.Out, magnitude)
			withdrawals.Add(withdrawals, magnitude)
		}
	}
	dates := make([]string, 0, len(byDay))
	for date := range byDay {
		dates = append(dates, date)
	}
	sort.Strings(dates)

	breakpoints := []string{r.openDate, r.to}
	for _, date := range dates {
		if byDay[date].In.Sign() > 0 {
			breakpoints = append(breakpoints, previousDay(date))
		}
		if byDay[date].Out.Sign() > 0 {
			breakpoints = append(breakpoints, date)
		}
	}
	evaluation := r.evaluationDays(breakpoints)
	values := r.valuations(evaluation)

	result.OpeningValue = r.amount(values[r.openDate], r.openDate)
	result.ClosingValue = r.amount(values[r.to], r.to)
	if flowsKnown {
		result.Contributions = ReturnsAmount{Amount: r.quantity(contributions)}
		result.Withdrawals = ReturnsAmount{Amount: r.quantity(withdrawals)}
	} else {
		result.Contributions = ReturnsAmount{Reason: ReturnReasonFlowUnavailable}
		result.Withdrawals = ReturnsAmount{Reason: ReturnReasonFlowUnavailable}
	}

	// Each figure has its own dependencies: gain and IRR need the flows and
	// the two boundary values; TTWROR also needs every evaluation day.
	var boundaryBlock ReturnFigure
	switch {
	case !flowsKnown:
		boundaryBlock = unavailableReturn(ReturnReasonFlowUnavailable, "")
	case values[r.openDate].reason != "":
		boundaryBlock = unavailableReturn(values[r.openDate].reason, r.openDate)
	case values[r.to].reason != "":
		boundaryBlock = unavailableReturn(values[r.to].reason, r.to)
	}
	twrBlock := boundaryBlock
	if twrBlock.Reason == "" {
		for _, date := range slices.Sorted(maps.Keys(values)) {
			if values[date].reason != "" {
				twrBlock = unavailableReturn(values[date].reason, date)
				break
			}
		}
	}

	if twrBlock.Reason != "" {
		result.TimeWeighted, result.TimeWeightedAnnualized = twrBlock, twrBlock
	} else {
		known := make(map[string]*big.Rat, len(values))
		for date, valuation := range values {
			known[date] = valuation.value
		}
		growth, twr := timeWeightedReturn(r.openDate, known, byDay)
		result.TimeWeighted, result.TimeWeightedAnnualized = twr, twr
		if growth != nil {
			result.TimeWeightedAnnualized = annualizedReturn(growth, result.Days)
		}
	}

	if boundaryBlock.Reason != "" {
		result.Gain = ReturnsAmount{Reason: boundaryBlock.Reason, Date: boundaryBlock.Date}
		result.MoneyWeighted, result.MoneyWeightedPeriod = boundaryBlock, boundaryBlock
		return
	}
	open, closing := values[r.openDate].value, values[r.to].value
	gain := new(big.Rat).Sub(closing, open)
	gain.Sub(gain, contributions)
	gain.Add(gain, withdrawals)
	result.Gain = ReturnsAmount{Amount: r.quantity(gain)}

	if open.Sign() == 0 && len(dates) == 0 {
		result.MoneyWeighted = unavailableReturn(ReturnReasonNoCapital, "")
		result.MoneyWeightedPeriod = result.MoneyWeighted
		return
	}
	openTime, _ := time.Parse(time.DateOnly, r.openDate)
	offset := func(date string) int {
		parsed, _ := time.Parse(time.DateOnly, date)
		return int(parsed.Sub(openTime).Hours() / 24)
	}
	cashflows := []returnsCashflow{{Offset: 0, Amount: new(big.Rat).Neg(open)}}
	for _, date := range dates {
		if byDay[date].In.Sign() > 0 {
			cashflows = append(cashflows, returnsCashflow{Offset: offset(date) - 1, Amount: new(big.Rat).Neg(byDay[date].In)})
		}
		if byDay[date].Out.Sign() > 0 {
			cashflows = append(cashflows, returnsCashflow{Offset: offset(date), Amount: byDay[date].Out})
		}
	}
	cashflows = append(cashflows, returnsCashflow{Offset: result.Days, Amount: closing})
	growth, reason := moneyWeightedGrowth(cashflows, result.Days)
	if reason != "" {
		result.MoneyWeighted = unavailableReturn(reason, "")
		result.MoneyWeightedPeriod = result.MoneyWeighted
		return
	}
	result.MoneyWeightedPeriod = growthReturn(growth, 1)
	if result.Days < 365 {
		result.MoneyWeighted = unavailableReturn(ReturnReasonPeriodShorterThanYear, "")
	} else {
		result.MoneyWeighted = growthReturn(growth, 365/float64(result.Days))
	}
}

// evaluationDays adds to the flow breakpoints every day the daily product's
// validity cannot be proven without a price (the plan's "Evaluation days").
// Prices are positive, so non-negative per-commodity balances with at least
// one positive are a positive value; anything else is checked by valuing.
func (r *returnsReader) evaluationDays(breakpoints []string) []string {
	totals := map[int64]*exact.ScaledInt{}
	state := func() (safe, negative bool) {
		positive := false
		for _, total := range totals {
			switch total.Sign() {
			case -1:
				negative = true
			case 1:
				positive = true
			}
		}
		return positive && !negative, negative
	}
	days := append([]string(nil), breakpoints...)
	addRun := func(after, before string) {
		for day := nextDay(after); day < before && day <= r.to; day = nextDay(day) {
			days = append(days, day)
		}
	}

	next := 0
	fold := func(through string) bool {
		changed := false
		for next < len(r.postings) && r.postings[next].EntryDate <= through {
			posting := r.postings[next]
			next++
			if !r.inScope[posting.AccountID] {
				continue
			}
			if totals[posting.CommodityID] == nil {
				totals[posting.CommodityID] = exact.NewScaledInt()
			}
			totals[posting.CommodityID].AddCoefficient(posting.QuantityValue, posting.QuantityScale)
			changed = true
		}
		return changed
	}
	fold(r.openDate)
	prevDay := r.openDate
	prevSafe, prevNegative := state()
	for next < len(r.postings) {
		date := r.postings[next].EntryDate
		if !fold(date) {
			continue
		}
		if prevNegative {
			// Constant balances that include a negative: prices alone can
			// turn the value's sign on any day of the run.
			addRun(prevDay, date)
		}
		nowSafe, nowNegative := state()
		if !prevSafe || !nowSafe {
			days = append(days, previousDay(date), date)
		}
		prevDay, prevSafe, prevNegative = date, nowSafe, nowNegative
	}
	if prevNegative {
		addRun(prevDay, nextDay(r.to))
	}
	return days
}

func (r *returnsReader) amount(valuation returnsValuation, date string) ReturnsAmount {
	if valuation.reason != "" {
		return ReturnsAmount{Reason: valuation.reason, Date: date}
	}
	return ReturnsAmount{Amount: r.quantity(valuation.value)}
}

// quantity rounds an exact reporting-currency figure once, half away from
// zero, to the currency's scale.
func (r *returnsReader) quantity(value *big.Rat) *BalanceQuantity {
	coefficient, err := exact.FromBig(roundRatToScale(value, r.rates.Scale()))
	if err != nil {
		return nil
	}
	return &BalanceQuantity{CommodityID: r.rates.QuoteCommodityID(), QuantityValue: coefficient,
		QuantityScale: r.rates.Scale(), NormalQuantityValue: coefficient}
}

func nextDay(date string) string {
	parsed, err := time.Parse(time.DateOnly, date)
	if err != nil {
		return date
	}
	return parsed.AddDate(0, 0, 1).Format(time.DateOnly)
}

func previousDay(date string) string {
	parsed, err := time.Parse(time.DateOnly, date)
	if err != nil {
		return date
	}
	return parsed.AddDate(0, 0, -1).Format(time.DateOnly)
}

// returnsHoldingIndex answers how many units of an instrument each holding
// account held at the end of a date.
type returnsHoldingIndex map[int64][]db.LedgerPostingRecord

func newReturnsHoldingIndex(postings []db.LedgerPostingRecord, holdings map[int64]bool) returnsHoldingIndex {
	index := returnsHoldingIndex{}
	for _, posting := range postings {
		if holdings[posting.AccountID] {
			index[posting.CommodityID] = append(index[posting.CommodityID], posting)
		}
	}
	return index
}

func (h returnsHoldingIndex) at(commodityID int64, date string) map[int64]*exact.ScaledInt {
	balances := map[int64]*exact.ScaledInt{}
	for _, posting := range h[commodityID] {
		if posting.EntryDate > date {
			break
		}
		if balances[posting.AccountID] == nil {
			balances[posting.AccountID] = exact.NewScaledInt()
		}
		balances[posting.AccountID].AddCoefficient(posting.QuantityValue, posting.QuantityScale)
	}
	return balances
}
