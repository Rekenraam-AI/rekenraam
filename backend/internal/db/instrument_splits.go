package db

import (
	"context"
	"fmt"
	"math/big"
	"time"
)

// InstrumentSplit is one effective split of an instrument: on EffectiveOn each
// unit became RatioNumerator / RatioDenominator units. AccountID is the
// holding that recorded it; OrderOperationID is its same-day replay slot (its
// correction root), so a same-day trade or delivery with an earlier slot was
// split and one with a later slot was not.
type InstrumentSplit struct {
	CommodityID      int64
	AccountID        int64
	EffectiveOn      string
	RatioNumerator   int64
	RatioDenominator int64
	OrderOperationID int64
}

// correctionRootsCTE maps every investment operation to its correction root,
// the same-day slot replay orders by (investmentReplayOrderOperationIDsQuery).
// Parameters: book, book.
const correctionRootsCTE = `WITH RECURSIVE correction_roots(operation_id, root_id) AS (
	SELECT id, id FROM investment_operations WHERE book_id = ? AND correction_of_operation_id IS NULL
	UNION ALL
	SELECT child.id, correction_roots.root_id FROM investment_operations child
	JOIN correction_roots ON correction_roots.operation_id = child.correction_of_operation_id
	WHERE child.book_id = ?
) `

// TotalReturnSecurities reports whether this repository prices securities by
// total-return observations, which are continuous index levels and are never
// split-adjusted.
func (r *PricingRepository) TotalReturnSecurities() bool {
	return r.selection == totalReturnSelectionCTE
}

// InstrumentSplits returns the effective splits of the given instruments
// recorded in any holding, after `after` and through `through`, ordered by
// instrument and date. Reversed and replaced splits are excluded. The same
// split recorded in several holdings appears once per holding; a holding's
// successive splits on one day each appear. A raw quote prices a unit of its
// own day, so valuation reads these to bring a stale quote to a later day's
// units (#407).
func (r *PricingRepository) InstrumentSplits(ctx context.Context, bookID int64, commodityIDs []int64, after, through string) ([]InstrumentSplit, error) {
	return instrumentSplits(ctx, r.database, bookID, commodityIDs, after, through)
}

func instrumentSplits(ctx context.Context, queryer rowsQueryer, bookID int64, commodityIDs []int64, after, through string) ([]InstrumentSplit, error) {
	if len(commodityIDs) == 0 {
		return nil, nil
	}
	clause, args := inClause("f.commodity_id", commodityIDs)
	rows, err := queryer.QueryContext(ctx, correctionRootsCTE+`
		SELECT f.commodity_id, f.account_id, f.effective_on, f.ratio_numerator, f.ratio_denominator, root.root_id
		FROM investment_split_facts f
		JOIN effective_investment_operations o ON o.id = f.operation_id
		JOIN correction_roots root ON root.operation_id = f.operation_id
		WHERE f.book_id = ? AND `+clause+` AND f.effective_on > ? AND f.effective_on <= ?
		ORDER BY f.commodity_id, f.effective_on, root.root_id`, append(append([]any{bookID, bookID, bookID}, args...), after, through)...)
	if err != nil {
		return nil, fmt.Errorf("read instrument splits: %w", err)
	}
	defer rows.Close()
	var splits []InstrumentSplit
	for rows.Next() {
		var split InstrumentSplit
		if err := rows.Scan(&split.CommodityID, &split.AccountID, &split.EffectiveOn, &split.RatioNumerator, &split.RatioDenominator, &split.OrderOperationID); err != nil {
			return nil, fmt.Errorf("scan instrument split: %w", err)
		}
		splits = append(splits, split)
	}
	return splits, rows.Err()
}

// SplitDay is one day's split factor of an instrument: every unit held at the
// end of the previous day became Factor units (a holding's successive splits
// that day compounded). Exact, so it is never rounded.
type SplitDay struct {
	EffectiveOn string
	Factor      *big.Rat
}

// InstrumentSplitDays combines recorded splits (ordered by instrument, then
// date) into one factor per instrument and day. A holding's successive splits
// on one day compound (2:1 twice is 4:1, as its ledger units are); holdings
// that recorded the same event must agree on the day's factor. A day they
// disagree on has no factor and is a conflict, rather than a guess.
//
// The factor is the instrument's, not a holding's: a quote prices the
// instrument, so a holding that never recorded a split is misvalued by a
// fresh post-split quote just as by a rescaled stale one.
func InstrumentSplitDays(recorded []InstrumentSplit) (map[int64][]SplitDay, map[int64][]string) {
	days := map[int64][]SplitDay{}
	conflicts := map[int64][]string{}
	for start := 0; start < len(recorded); {
		first := recorded[start]
		end := start
		for end < len(recorded) && recorded[end].CommodityID == first.CommodityID && recorded[end].EffectiveOn == first.EffectiveOn {
			end++
		}
		byHolding := map[int64]*big.Rat{}
		for _, split := range recorded[start:end] {
			if byHolding[split.AccountID] == nil {
				byHolding[split.AccountID] = big.NewRat(1, 1)
			}
			byHolding[split.AccountID].Mul(byHolding[split.AccountID], big.NewRat(split.RatioNumerator, split.RatioDenominator))
		}
		var factor *big.Rat
		agree := true
		for _, holding := range byHolding {
			agree = agree && (factor == nil || factor.Cmp(holding) == 0)
			factor = holding
		}
		if agree {
			days[first.CommodityID] = append(days[first.CommodityID], SplitDay{EffectiveOn: first.EffectiveOn, Factor: factor})
		} else {
			conflicts[first.CommodityID] = append(conflicts[first.CommodityID], first.EffectiveOn)
		}
		start = end
	}
	return days, conflicts
}

// SplitFactor is the product of an instrument's split factors effective in
// (observed, onDate]: how many units on onDate one unit of the observation's
// day became. A conflicting day in that span returns that day instead.
func SplitFactor(days []SplitDay, conflicts []string, observed, onDate string) (*big.Rat, string) {
	for _, conflict := range conflicts {
		if conflict > observed && conflict <= onDate {
			return nil, conflict
		}
	}
	factor := big.NewRat(1, 1)
	for _, split := range days {
		if split.EffectiveOn > observed && split.EffectiveOn <= onDate {
			factor.Mul(factor, split.Factor)
		}
	}
	return factor, ""
}

// previousISODate is the calendar day before date, or date when it is not one.
func previousISODate(date string) string {
	parsed, err := time.Parse(time.DateOnly, date)
	if err != nil {
		return date
	}
	return parsed.AddDate(0, 0, -1).Format(time.DateOnly)
}

// PendingSplitFactor is how many end-of-day units one unit of an event earlier
// in a split day became: the product of the splits the same holding recorded
// for the instrument that day in a later replay slot. A same-day trade or
// delivery ordered before the split carried pre-split units (the split split
// them), one ordered after it carried post-split units. Nil when none is
// pending. Splits must be ordered as InstrumentSplits returns them.
func PendingSplitFactor(splits []InstrumentSplit, commodityID, accountID int64, date string, slot int64) *big.Rat {
	var factor *big.Rat
	for _, split := range splits {
		if split.CommodityID != commodityID || split.AccountID != accountID || split.EffectiveOn != date || split.OrderOperationID <= slot {
			continue
		}
		if factor == nil {
			factor = big.NewRat(1, 1)
		}
		factor.Mul(factor, big.NewRat(split.RatioNumerator, split.RatioDenominator))
	}
	return factor
}

// TradeQuotePendingSplits returns, per trade-implied observation of the given
// instruments dated in (after, through], the split factor still pending after
// its trade (PendingSplitFactor): such a quote is cash per unit of the trade,
// and a trade ordered before that day's split priced pre-split units. Only
// observations with a pending split appear.
func (r *PricingRepository) TradeQuotePendingSplits(ctx context.Context, bookID int64, commodityIDs []int64, after, through string) (map[int64]*big.Rat, error) {
	return tradeQuotePendingSplits(ctx, r.database, bookID, commodityIDs, after, through)
}

func tradeQuotePendingSplits(ctx context.Context, queryer rowsQueryer, bookID int64, commodityIDs []int64, after, through string) (map[int64]*big.Rat, error) {
	pending := map[int64]*big.Rat{}
	if len(commodityIDs) == 0 {
		return pending, nil
	}
	clause, args := inClause("po.base_commodity_id", commodityIDs)
	// The trade's holding is the split's holding with a posting of the
	// instrument in the observation's source version; the commodity-trading
	// leg is a system account that never records a split.
	rows, err := queryer.QueryContext(ctx, correctionRootsCTE+`
		SELECT po.id, f.ratio_numerator, f.ratio_denominator
		FROM price_observations po
		JOIN investment_operation_journal_links link ON link.transaction_version_id = po.source_transaction_version_id
		JOIN correction_roots trade ON trade.operation_id = link.operation_id
		JOIN investment_split_facts f ON f.book_id = po.book_id AND f.commodity_id = po.base_commodity_id
			AND f.effective_on = po.valuation_date
		JOIN effective_investment_operations split ON split.id = f.operation_id
		JOIN correction_roots slot ON slot.operation_id = f.operation_id
		WHERE po.book_id = ? AND po.quote_type = 'trade_implied' AND `+clause+`
			AND po.valuation_date > ? AND po.valuation_date <= ? AND slot.root_id > trade.root_id
			AND EXISTS (SELECT 1 FROM posting_versions pv WHERE pv.transaction_version_id = po.source_transaction_version_id
				AND pv.account_id = f.account_id AND pv.commodity_id = po.base_commodity_id)
		ORDER BY po.id, slot.root_id`, append(append([]any{bookID, bookID, bookID}, args...), after, through)...)
	if err != nil {
		return nil, fmt.Errorf("read trade quote pending splits: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var observationID, numerator, denominator int64
		if err := rows.Scan(&observationID, &numerator, &denominator); err != nil {
			return nil, fmt.Errorf("scan trade quote pending split: %w", err)
		}
		if pending[observationID] == nil {
			pending[observationID] = big.NewRat(1, 1)
		}
		pending[observationID].Mul(pending[observationID], big.NewRat(numerator, denominator))
	}
	return pending, rows.Err()
}

// splitAdjustPositionPrices brings each position's latest raw quote to the
// units of its valuation date (#408), the rule RateTable applies to every
// other consumer (#407): a quote observed before a recorded split priced
// pre-split units, so valuePositions divides it by the instrument's split
// factor in between, recorded on the position. A trade-implied quote ordered
// before a split on its own day priced pre-split units too, so the split
// still pending after its trade joins the factor. A conflicting split day in
// between leaves the position unvalued as conflicting_splits. Holdings read
// live_raw_v1 only; total-return levels never reach this path.
func splitAdjustPositionPrices(ctx context.Context, queryer rowsQueryer, bookID int64, positions []InvestmentPositionRecord) error {
	var commodityIDs []int64
	seen := map[int64]bool{}
	after, through := "", ""
	for _, position := range positions {
		observed := position.LatestPriceDate
		if !observed.Valid || observed.String > position.ValuationAsOf {
			continue
		}
		if !seen[position.CommodityID] {
			seen[position.CommodityID] = true
			commodityIDs = append(commodityIDs, position.CommodityID)
		}
		if after == "" || observed.String < after {
			after = observed.String
		}
		through = max(through, position.ValuationAsOf)
	}
	if len(commodityIDs) == 0 {
		return nil
	}
	recorded, err := instrumentSplits(ctx, queryer, bookID, commodityIDs, after, through)
	if err != nil {
		return err
	}
	// A quote's own day can hold a split still pending after its trade.
	pending, err := tradeQuotePendingSplits(ctx, queryer, bookID, commodityIDs, previousISODate(after), through)
	if err != nil {
		return err
	}
	days, conflicts := InstrumentSplitDays(recorded)
	for i := range positions {
		position := &positions[i]
		observed := position.LatestPriceDate
		if !observed.Valid || observed.String > position.ValuationAsOf {
			continue
		}
		factor, conflict := SplitFactor(days[position.CommodityID], conflicts[position.CommodityID], observed.String, position.ValuationAsOf)
		if own := pending[position.latestPriceObservationID.Int64]; own != nil && conflict == "" {
			factor.Mul(factor, own)
		}
		switch {
		case conflict != "":
			position.PriceStatus = ValuationConflictingSplits
		case factor.Cmp(big.NewRat(1, 1)) != 0:
			position.splitFactor = factor
			position.SplitRatioNumerator, position.SplitRatioDenominator = factor.Num().String(), factor.Denom().String()
		}
	}
	return nil
}
