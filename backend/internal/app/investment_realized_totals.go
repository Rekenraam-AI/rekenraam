package app

import (
	"rekenraam/backend/internal/db"
	"rekenraam/backend/internal/exact"
)

// NativeRealizedTotal is one cost currency's realized gain over a set of
// disposals, summed exactly in that currency with nothing converted (#399).
// Known is the sum of the known-basis gains only: it is the currency's total
// only while UnresolvedCount is zero.
type NativeRealizedTotal struct {
	CostCommodityID int64
	Known           *exact.ScaledInt
	DisposalCount   int
	UnresolvedCount int
}

// NativeRealizedTotals groups disposals by cost currency alone, in order of
// first appearance. Grouping by scale as well used to stand in for "can be
// added without loss", but a gain carries whatever scale its own subtraction
// needed (T-101), so two gains in one currency can legitimately differ in
// scale; splitting them would report one currency twice and make neither row
// the total. exact.ScaledInt deepens as it adds, so the sum is never rounded.
func NativeRealizedTotals(realized []RealizedGainEntry) []NativeRealizedTotal {
	index := map[int64]int{}
	totals := []NativeRealizedTotal{}
	for _, entry := range realized {
		i, seen := index[entry.CostCommodityID]
		if !seen {
			i = len(totals)
			index[entry.CostCommodityID] = i
			totals = append(totals, NativeRealizedTotal{CostCommodityID: entry.CostCommodityID, Known: exact.NewScaledInt()})
		}
		total := &totals[i]
		total.DisposalCount++
		if entry.BasisKnowledge == db.InvestmentBasisUnknown {
			total.UnresolvedCount++
			continue
		}
		total.Known.AddInt64(entry.RealizedGainValue, entry.RealizedGainScale)
	}
	return totals
}

// Gain is the operational total at the full scale its sum needed, or nil
// while any disposal's basis is unresolved. The int64 range is checked first,
// without dropping trailing zeros, so an unrepresentable known sum fails the
// read even when the total itself would be withheld. Statements apply their
// own trailing-zero restatement to Known instead.
func (t NativeRealizedTotal) Gain() (*StatementAmount, error) {
	value, err := t.Known.Int64()
	if err != nil {
		return nil, err
	}
	if t.UnresolvedCount > 0 {
		return nil, nil
	}
	return &StatementAmount{Value: value, Scale: t.Known.Scale()}, nil
}
