package db

import (
	"context"
	"fmt"
)

// GainsFXBasis is a signed basis contribution under operational elections.
// Realized long basis is negative, short opening proceeds positive; remaining
// basis is positive on both sides. An empty AcquisitionDate is unknown evidence.
type GainsFXBasis struct {
	AccountID, CommodityID, CostCommodityID int64
	PositionSide, DisposalDate              string
	TransactionID                           int64
	LotID                                   int64
	AcquisitionDate                         string
	Value                                   int64
	Scale                                   int
	Unknown                                 bool
}

// GainsFXBasisComponents retains per-lot acquisition evidence for #233. Dated
// remaining amounts use the same rolled-back replay as the operational profile;
// disposals use the shared committed movement owner, including corrections.
func (r *InvestmentRepository) GainsFXBasisComponents(ctx context.Context, bookID int64, asOf string, period RealizedGainsParams) ([]GainsFXBasis, error) {
	tx, err := r.database.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer rollbackTx(ctx, tx)
	type origin struct {
		date string
		key  investmentReplayPositionKey
		side string
	}
	origins := map[int64]origin{}
	keys := map[origin]bool{}
	rows, err := tx.QueryContext(ctx, `SELECT l.id, l.account_id, l.commodity_id, l.cost_commodity_id, l.position_side,
 CASE WHEN link.destination_lot_id IS NULL THEN l.opened_on
 WHEN link.original_date_knowledge = 'known' THEN COALESCE(link.original_acquired_on, '') ELSE '' END
 FROM investment_lots l LEFT JOIN effective_investment_transfer_links link ON link.destination_lot_id = l.id WHERE l.book_id = ? ORDER BY l.id`, bookID)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var id int64
		var o origin
		if err := rows.Scan(&id, &o.key.accountID, &o.key.commodityID, &o.key.costCommodityID, &o.side, &o.date); err != nil {
			rows.Close()
			return nil, err
		}
		origins[id] = o
		o.date = ""
		keys[o] = true
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()
	result := []GainsFXBasis{}
	for o := range keys {
		holding, ok, err := datedHoldingTx(ctx, tx, bookID, o.key, o.side, asOf)
		if err != nil {
			return nil, err
		}
		if !ok {
			continue
		}
		for _, lot := range holding.Lots {
			result = append(result, GainsFXBasis{AccountID: o.key.accountID, CommodityID: o.key.commodityID, CostCommodityID: o.key.costCommodityID,
				PositionSide: o.side, LotID: lot.LotID, AcquisitionDate: origins[lot.LotID].date, Value: lot.RemainingCostBasisValue, Scale: lot.RemainingCostBasisScale, Unknown: lot.BasisKnowledge == InvestmentBasisUnknown})
		}
	}
	movements, err := EffectiveLotMovements(ctx, tx, bookID, EffectiveMovementFilter{DisposalsOnly: true})
	if err != nil {
		return nil, fmt.Errorf("gains FX disposal basis: %w", err)
	}
	for _, m := range movements {
		if m.Kind != "disposal" || (period.From != "" && m.EventDate < period.From) || (period.To != "" && m.EventDate > period.To) {
			continue
		}
		value := m.CostBasisValue
		if m.PositionSide == PositionSideShort {
			value = -value
		}
		result = append(result, GainsFXBasis{AccountID: m.AccountID, CommodityID: m.CommodityID, CostCommodityID: m.CostCommodityID,
			PositionSide: m.PositionSide, DisposalDate: m.EventDate, TransactionID: m.TransactionID.Int64, LotID: m.LotID,
			AcquisitionDate: origins[m.LotID].date, Value: value, Scale: m.CostBasisScale, Unknown: m.BasisKnowledge == InvestmentBasisUnknown})
	}
	return result, nil
}
