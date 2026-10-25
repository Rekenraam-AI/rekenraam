package db

import (
	"context"
	"fmt"
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
	if len(commodityIDs) == 0 {
		return nil, nil
	}
	clause, args := inClause("f.commodity_id", commodityIDs)
	rows, err := r.database.QueryContext(ctx, `
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
