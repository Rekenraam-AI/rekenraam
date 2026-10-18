package api

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

// #275: independent posting-day expectations through the authenticated report
// consumers. The transaction header date is never a report's balance axis.
func TestReportAuditPostingDateAxesAndClearingScope(t *testing.T) {
	t.Parallel()
	handler, _ := newSetupTestHandler(t)
	f := newSpendingFixture(t, handler)
	// A single-account purchase moves both cash and category to May 3.
	createTransactionForSession(t, handler, f.sessionCookie, f.csrfToken, fmt.Sprintf(`{
 "transaction_date":"2026-05-01","journal_entries":[{"postings":[
 {"account_id":%d,"commodity_id":%d,"quantity_value":"-2500","quantity_scale":2,"posted_on":"2026-05-03"},
 {"account_id":%d,"commodity_id":%d,"quantity_value":"2500","quantity_scale":2}]}]}`,
		f.checking.ID, f.usdID, f.groceries.ID, f.usdID), http.StatusCreated)
	// Two dated account legs leave the category on May 10, using clearing to
	// balance each dated journal. May 11/12 cash is financing via that system
	// counterpart; clearing is excluded from the cash scope, retained in worth.
	createTransactionForSession(t, handler, f.sessionCookie, f.csrfToken, fmt.Sprintf(`{
 "transaction_date":"2026-05-10","journal_entries":[{"postings":[
 {"account_id":%d,"commodity_id":%d,"quantity_value":"-15000","quantity_scale":2,"posted_on":"2026-05-11"},
 {"account_id":%d,"commodity_id":%d,"quantity_value":"10000","quantity_scale":2,"posted_on":"2026-05-12"},
 {"account_id":%d,"commodity_id":%d,"quantity_value":"5000","quantity_scale":2}]}]}`,
		f.checking.ID, f.usdID, f.card.ID, f.usdID, f.groceries.ID, f.usdID), http.StatusCreated)
	spending := readSpendingForSession(t, handler, f.sessionCookie, "?start_date=2026-05-01&end_date=2026-05-12&group_by=category")
	require.Len(t, spending.CommodityTotals, 1)
	require.Equal(t, exactValue(t, "75"), exactCoefficientValue(t, spending.CommodityTotals[0].QuantityValue, spending.CommodityTotals[0].QuantityScale))
	for _, day := range []string{"2026-05-01", "2026-05-11", "2026-05-12"} {
		report := readSpendingForSession(t, handler, f.sessionCookie, "?start_date="+day+"&end_date="+day+"&group_by=category")
		require.Empty(t, report.Groups, day)
	}
	for _, tc := range []struct{ day, amount string }{{"2026-05-03", "25"}, {"2026-05-10", "50"}} {
		report := readSpendingForSession(t, handler, f.sessionCookie, "?start_date="+tc.day+"&end_date="+tc.day+"&group_by=category")
		require.Len(t, report.CommodityTotals, 1)
		require.Equal(t, exactValue(t, tc.amount), exactCoefficientValue(t, report.CommodityTotals[0].QuantityValue, report.CommodityTotals[0].QuantityScale))
	}
	query := "?start_date=2026-05-01&end_date=2026-05-12&bucket=day"
	cash := readCashflowForSession(t, handler, f.sessionCookie, query+"&account_id="+strconvFormatInt(f.checking.ID)+"&account_id="+strconvFormatInt(f.card.ID))
	require.Len(t, cash.Buckets, 12)
	for index, want := range map[int]string{0: "0", 2: "-25", 9: "0", 10: "-150", 11: "100"} {
		bucket := cash.Buckets[index]
		actual := "0"
		for _, amount := range bucket.NetMovement {
			if amount.CommodityID == f.usdID {
				actual = exactCoefficientValue(t, amount.QuantityValue, amount.QuantityScale)
			}
		}
		require.Equal(t, exactValue(t, want), actual, bucket.EndDate)
	}
	require.Equal(t, exactValue(t, "25"), exactCoefficientValue(t, cash.Buckets[2].Outflow[0].QuantityValue, cash.Buckets[2].Outflow[0].QuantityScale))
	require.Equal(t, exactValue(t, "150"), exactCoefficientValue(t, cash.Buckets[10].TransferOut[0].QuantityValue, cash.Buckets[10].TransferOut[0].QuantityScale))
	require.Equal(t, exactValue(t, "100"), exactCoefficientValue(t, cash.Buckets[11].TransferIn[0].QuantityValue, cash.Buckets[11].TransferIn[0].QuantityScale))
	worth := readNetWorthSeriesForSession(t, handler, f.sessionCookie, query)
	for index, want := range map[int]string{2: "-25", 9: "-75", 10: "-75", 11: "-75"} {
		require.Len(t, worth.Buckets[index].Totals, 1)
		amount := worth.Buckets[index].Totals[0]
		require.Equal(t, exactValue(t, want), exactCoefficientValue(t, amount.QuantityValue, amount.QuantityScale), worth.Buckets[index].EndDate)
	}
}
