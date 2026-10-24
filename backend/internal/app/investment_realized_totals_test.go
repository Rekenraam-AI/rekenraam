package app

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"rekenraam/backend/internal/exact"
)

func realizedGain(currency, value int64, scale int, knowledge string) RealizedGainEntry {
	return RealizedGainEntry{CostCommodityID: currency, RealizedGainValue: value, RealizedGainScale: scale, BasisKnowledge: knowledge}
}

func TestNativeRealizedTotalsGroupByCurrencyInFirstAppearanceOrder(t *testing.T) {
	t.Parallel()
	assert.Empty(t, NativeRealizedTotals(nil))
	totals := NativeRealizedTotals([]RealizedGainEntry{
		realizedGain(9, 1, 2, "known"), realizedGain(3, 0, 0, "unknown"), realizedGain(9, 20, 0, "known"),
		realizedGain(3, 5, 1, "known"), realizedGain(9, -2, 2, "known"), realizedGain(3, 0, 0, "unknown"),
	})
	require.Len(t, totals, 2)

	assert.Equal(t, int64(9), totals[0].CostCommodityID)
	assert.Equal(t, 3, totals[0].DisposalCount)
	assert.Zero(t, totals[0].UnresolvedCount)
	gain, err := totals[0].Gain()
	require.NoError(t, err)
	assert.Equal(t, &StatementAmount{Value: 1999, Scale: 2}, gain, "0.01 + 20 − 0.02")

	assert.Equal(t, int64(3), totals[1].CostCommodityID)
	assert.Equal(t, 3, totals[1].DisposalCount)
	assert.Equal(t, 2, totals[1].UnresolvedCount)
	assert.Equal(t, "5e-1", totals[1].Known.String(), "unknown disposals add nothing to the known sum")
	gain, err = totals[1].Gain()
	require.NoError(t, err)
	assert.Nil(t, gain, "a sum of only the known disposals is not the total")
}

// The operational total keeps the full scale and refuses a sum that fits
// only once trailing zeros are dropped; the statement restates the same
// grouped sum. The policies stay distinct over one owner.
func TestNativeRealizedTotalRangePolicyDiffersFromStatement(t *testing.T) {
	t.Parallel()
	totals := NativeRealizedTotals([]RealizedGainEntry{
		realizedGain(9, 922337203685477581, 0, "known"), realizedGain(9, 0, 1, "known"),
	})
	require.Len(t, totals, 1)
	_, err := totals[0].Gain()
	require.ErrorIs(t, err, exact.ErrInt64Range)
	statement, err := statementAmount(totals[0].Known)
	require.NoError(t, err)
	assert.Equal(t, &StatementAmount{Value: 922337203685477581, Scale: 0}, statement)

	withheld := NativeRealizedTotals([]RealizedGainEntry{
		realizedGain(9, 9223372036854775807, 0, "known"), realizedGain(9, 0, 0, "unknown"), realizedGain(9, 1, 0, "known"),
	})
	_, err = withheld[0].Gain()
	require.ErrorIs(t, err, exact.ErrInt64Range, "range failure precedes withholding")
}
