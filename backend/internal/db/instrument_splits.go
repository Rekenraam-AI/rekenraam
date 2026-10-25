package db

import (
	"context"
	"fmt"
	"math/big"
)

// InstrumentSplit is one effective split of an instrument: on EffectiveOn each
// unit became RatioNumerator / RatioDenominator units. AccountID is the
// holding that recorded it.
type InstrumentSplit struct {
	CommodityID      int64
	AccountID        int64
	EffectiveOn      string
	RatioNumerator   int64
	RatioDenominator int64
}

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
	rows, err := queryer.QueryContext(ctx, `
		SELECT f.commodity_id, f.account_id, f.effective_on, f.ratio_numerator, f.ratio_denominator
		FROM investment_split_facts f
		JOIN effective_investment_operations o ON o.id = f.operation_id
		WHERE f.book_id = ? AND `+clause+` AND f.effective_on > ? AND f.effective_on <= ?
		ORDER BY f.commodity_id, f.effective_on, f.operation_id`, append(append([]any{bookID}, args...), after, through)...)
	if err != nil {
		return nil, fmt.Errorf("read instrument splits: %w", err)
	}
	defer rows.Close()
	var splits []InstrumentSplit
	for rows.Next() {
		var split InstrumentSplit
		if err := rows.Scan(&split.CommodityID, &split.AccountID, &split.EffectiveOn, &split.RatioNumerator, &split.RatioDenominator); err != nil {
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

// splitAdjustPositionPrices brings each position's latest raw quote to the
// units of its valuation date (#408), the rule RateTable applies to every
// other consumer (#407): a quote observed before a recorded split priced
// pre-split units, so valuePositions divides it by the instrument's split
// factor in between, recorded on the position. A conflicting split day in
// between leaves the position unvalued as conflicting_splits. Holdings read
// live_raw_v1 only; total-return levels never reach this path.
func splitAdjustPositionPrices(ctx context.Context, queryer rowsQueryer, bookID int64, positions []InvestmentPositionRecord) error {
	var commodityIDs []int64
	seen := map[int64]bool{}
	after, through := "", ""
	for _, position := range positions {
		observed := position.LatestPriceDate
		if !observed.Valid || observed.String >= position.ValuationAsOf {
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
	days, conflicts := InstrumentSplitDays(recorded)
	for i := range positions {
		position := &positions[i]
		observed := position.LatestPriceDate
		if !observed.Valid || observed.String >= position.ValuationAsOf {
			continue
		}
		factor, conflict := SplitFactor(days[position.CommodityID], conflicts[position.CommodityID], observed.String, position.ValuationAsOf)
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
