package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"rekenraam/backend/internal/exact"
)

// DatedPositionsWithGains values every position, long and short, as it stood
// after the effective events dated on or before boundary.AsOf (#230, R18
// operational_v1). Each position is replayed in a rolled-back read, so one
// held then but closed since is listed and one opened later is not. Prices
// come from the shared live_raw_v1 selection at that date and knowledge
// instant. The book history replayed is today's effective history: a later
// correction restates an earlier valuation date.
func (r *InvestmentRepository) DatedPositionsWithGains(ctx context.Context, bookID int64, boundary ObservationBoundary) ([]UnrealizedGainRecord, error) {
	if !isDisposalCalendarDate(boundary.AsOf) || boundary.RecordedAt == "" {
		return nil, fmt.Errorf("%w: dated valuation needs a calendar date and knowledge cutoff", ErrInvalidDisposalParams)
	}
	tx, err := r.database.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin dated valuation read: %w", err)
	}
	defer rollbackTx(ctx, tx)
	positions, err := datedPositionsTx(ctx, tx, bookID, boundary.AsOf)
	if err != nil {
		return nil, err
	}
	if err := priceDatedPositionsTx(ctx, tx, bookID, boundary, positions); err != nil {
		return nil, err
	}
	return valuePositions(positions)
}

func datedPositionsTx(ctx context.Context, tx *sql.Tx, bookID int64, asOf string) ([]InvestmentPositionRecord, error) {
	rows, err := tx.QueryContext(ctx, `
		SELECT account_id, commodity_id, cost_commodity_id, position_side
		FROM investment_lots
		WHERE book_id = ? AND opened_on <= ?
		GROUP BY account_id, commodity_id, cost_commodity_id, position_side
		ORDER BY account_id, commodity_id, cost_commodity_id, position_side`, bookID, asOf)
	if err != nil {
		return nil, fmt.Errorf("read dated valuation positions: %w", err)
	}
	type sidedKey struct {
		key  investmentReplayPositionKey
		side string
	}
	var keys []sidedKey
	for rows.Next() {
		var k sidedKey
		if err := rows.Scan(&k.key.accountID, &k.key.commodityID, &k.key.costCommodityID, &k.side); err != nil {
			rows.Close()
			return nil, fmt.Errorf("scan dated valuation position: %w", err)
		}
		keys = append(keys, k)
	}
	if err := errors.Join(rows.Err(), rows.Close()); err != nil {
		return nil, fmt.Errorf("iterate dated valuation positions: %w", err)
	}
	positions := make([]InvestmentPositionRecord, 0, len(keys))
	for _, k := range keys {
		holding, ok, err := datedHoldingTx(ctx, tx, bookID, k.key, k.side, asOf)
		if err != nil {
			return nil, err
		}
		if !ok {
			continue
		}
		record := InvestmentPositionRecord{AccountID: holding.AccountID, CommodityID: holding.CommodityID,
			CostCommodityID: holding.CostCommodityID, QuantityValue: holding.QuantityValue,
			QuantityScale: holding.QuantityScale, PositionSide: k.side, BasisKnowledge: InvestmentBasisKnown}
		cost := exact.NewScaledInt()
		for _, lot := range holding.Lots {
			if lot.BasisKnowledge == InvestmentBasisUnknown {
				record.BasisKnowledge = InvestmentBasisUnknown
				continue
			}
			cost.AddInt64(lot.RemainingCostBasisValue, lot.RemainingCostBasisScale)
		}
		if record.BasisKnowledge == InvestmentBasisKnown {
			value, err := cost.Int64()
			if err != nil {
				return nil, fmt.Errorf("dated position cost basis: %w", err)
			}
			record.RemainingCostBasisValue, record.RemainingCostBasisScale = value, cost.Scale()
		}
		positions = append(positions, record)
	}
	return positions, nil
}

// priceDatedPositionsTx attaches the selected observation, or the reason none
// is usable, with the holdings rule: retained, stale past seven days.
func priceDatedPositionsTx(ctx context.Context, tx *sql.Tx, bookID int64, boundary ObservationBoundary, positions []InvestmentPositionRecord) error {
	if len(positions) == 0 {
		return nil
	}
	seen := map[[2]int64]bool{}
	clauses := make([]string, 0, len(positions))
	args := []any{bookID, boundary.AsOf, boundary.RecordedAt}
	for _, position := range positions {
		pair := [2]int64{position.CommodityID, position.CostCommodityID}
		if seen[pair] {
			continue
		}
		seen[pair] = true
		clauses = append(clauses, "(d.base_commodity_id = ? AND d.quote_commodity_id = ?)")
		args = append(args, pair[0], pair[1])
	}
	rows, err := tx.QueryContext(ctx, observationSelectionCTE+`
		SELECT d.base_commodity_id, d.quote_commodity_id,
			po.price_value, po.price_scale, po.valuation_date, po.is_approximate,
			po.base_quantity_value, po.base_quantity_scale,
			CASE WHEN po.id IS NULL THEN d.reason
				WHEN po.valuation_date < date(si.as_of, '-7 days') THEN 'stale' ELSE 'usable' END
		FROM selection_diagnostics d
		CROSS JOIN selection_input si
		LEFT JOIN selection_latest po ON po.base_commodity_id = d.base_commodity_id AND po.quote_commodity_id = d.quote_commodity_id
		WHERE `+strings.Join(clauses, " OR "), args...)
	if err != nil {
		return fmt.Errorf("read dated valuation prices: %w", err)
	}
	defer rows.Close()
	type price struct {
		value, scale, approximate, baseValue, baseScale sql.NullInt64
		date                                            sql.NullString
		status                                          string
	}
	prices := map[[2]int64]price{}
	for rows.Next() {
		var pair [2]int64
		var p price
		if err := rows.Scan(&pair[0], &pair[1], &p.value, &p.scale, &p.date, &p.approximate, &p.baseValue, &p.baseScale, &p.status); err != nil {
			return fmt.Errorf("scan dated valuation price: %w", err)
		}
		prices[pair] = p
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate dated valuation prices: %w", err)
	}
	for i := range positions {
		position := &positions[i]
		position.ValuationAsOf, position.KnowledgeCutoff = boundary.AsOf, boundary.RecordedAt
		p, ok := prices[[2]int64{position.CommodityID, position.CostCommodityID}]
		if !ok {
			position.PriceStatus = ValuationNoPrice
			continue
		}
		position.PriceStatus = p.status
		position.LatestPriceValue, position.LatestPriceScale, position.LatestPriceDate = p.value, p.scale, p.date
		position.LatestPriceApproximate = p.approximate
		position.LatestPriceBaseQuantityValue, position.LatestPriceBaseQuantityScale = p.baseValue, p.baseScale
	}
	return nil
}
