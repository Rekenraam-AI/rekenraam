package app

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"rekenraam/backend/internal/exact"
)

// Dividend coverage states (#244). Unknown is never zero: an imported cash
// fallback establishes neither gross income nor the absence of withholding.
const (
	CoverageNone       = "none"
	CoverageKnown      = "known"
	CoverageKnownZero  = "known_zero"
	CoverageUnresolved = "unresolved"

	CoveragePayerUnassigned       = "unassigned"
	CoveragePayerSourceIdentified = "source_identified"

	CoverageReasonPayerUnassigned       = "payer_unassigned"
	CoverageReasonPayerUnlinked         = "payer_unlinked"
	CoverageReasonGrossUnresolved       = "gross_unresolved"
	CoverageReasonWithholdingUnresolved = "withholding_unresolved"

	CoverageItemNamedUnassigned = "named_unassigned"
	CoverageItemImportedCash    = "imported_cash"
)

// DividendCoverageInstrument is one instrument's attributed income in one
// cash currency. Gross and withholding come only from owner-confirmed named
// operations; imported cash whose source names this instrument but has no
// named operation is disclosed beside them and never added to them.
type DividendCoverageInstrument struct {
	CommodityID               int64          `json:"commodity_id"`
	CashCommodityID           int64          `json:"cash_commodity_id"`
	CurrencyCode              string         `json:"currency_code"`
	IncomeState               string         `json:"income_state"`
	WithholdingState          string         `json:"withholding_state"`
	Gross                     *GainsFXAmount `json:"gross"`
	Withholding               *GainsFXAmount `json:"withholding"`
	NetCash                   *GainsFXAmount `json:"net_cash"`
	DividendCount             int            `json:"dividend_count"`
	WithholdingPaymentCount   int            `json:"withholding_payment_count"`
	ImportedUnclassifiedCount int            `json:"imported_unclassified_count"`
	ImportedUnclassifiedCash  *GainsFXAmount `json:"imported_unclassified_cash"`
}

// DividendCoverageTotal is the book's identifiable unassigned investment cash
// in one currency: exact known cash, not gross income.
type DividendCoverageTotal struct {
	CashCommodityID          int64         `json:"cash_commodity_id"`
	CurrencyCode             string        `json:"currency_code"`
	Cash                     GainsFXAmount `json:"cash"`
	ItemCount                int           `json:"item_count"`
	PayerUnassignedCount     int           `json:"payer_unassigned_count"`
	EconomicsUnresolvedCount int           `json:"economics_unresolved_count"`
}

// DividendCoverageSource is the original import evidence, shown unchanged.
type DividendCoverageSource struct {
	BatchID    int64  `json:"batch_id"`
	RowID      int64  `json:"row_id"`
	Instrument string `json:"instrument"`
	Date       string `json:"date"`
	CashAmount string `json:"cash_amount"`
	Currency   string `json:"currency"`
}

// DividendCoverageItem is one unassigned or partially known payment.
type DividendCoverageItem struct {
	Kind                   string                  `json:"kind"`
	TransactionID          int64                   `json:"transaction_id"`
	Date                   string                  `json:"date"`
	CashCommodityID        int64                   `json:"cash_commodity_id"`
	CurrencyCode           string                  `json:"currency_code"`
	Cash                   GainsFXAmount           `json:"cash"`
	GrossState             string                  `json:"gross_state"`
	Gross                  *GainsFXAmount          `json:"gross"`
	WithholdingState       string                  `json:"withholding_state"`
	Withholding            *GainsFXAmount          `json:"withholding"`
	PayerState             string                  `json:"payer_state"`
	SourcePayerCommodityID *int64                  `json:"source_payer_commodity_id"`
	Reasons                []string                `json:"reasons"`
	Source                 *DividendCoverageSource `json:"source"`
}

// DividendCoverage is the composed income attribution read (#244).
type DividendCoverage struct {
	Instruments      []DividendCoverageInstrument `json:"instruments"`
	UnassignedTotals []DividendCoverageTotal      `json:"unassigned_totals"`
	Unassigned       []DividendCoverageItem       `json:"unassigned"`
	NextCursor       *string                      `json:"next_cursor"`
}

// SummaryOnly omits the item page for a summary such as an instrument view.
type DividendCoverageInput struct {
	CommodityID int64
	Cursor      string
	Limit       int
	SummaryOnly bool
}

const dividendCoverageDefaultLimit = 50

type coverageCursor struct {
	date            string
	transactionID   int64
	cashCommodityID int64
}

// DividendCoverage reads attributed income, identifiable unassigned cash and
// its unresolved facts without writing anything. Items are ordered newest
// first; CommodityID only narrows the per-instrument rows, while unassigned
// totals always describe the whole book so a subtotal is never mistaken for it.
func (s *InvestmentService) DividendCoverage(ctx context.Context, input DividendCoverageInput) (DividendCoverage, error) {
	result := DividendCoverage{Instruments: []DividendCoverageInstrument{}, UnassignedTotals: []DividendCoverageTotal{}, Unassigned: []DividendCoverageItem{}}
	limit := input.Limit
	if limit == 0 {
		limit = dividendCoverageDefaultLimit
	}
	if input.CommodityID < 0 || limit < 1 || limit > 100 {
		return result, ValidationError{Message: "invalid income coverage filters"}
	}
	var after *coverageCursor
	if input.Cursor != "" {
		parsed, ok := parseCoverageCursor(input.Cursor)
		if !ok {
			return result, ValidationError{Message: "invalid income coverage cursor"}
		}
		after = &parsed
	}

	operations, err := s.repository.DividendCoverageOperations(ctx, BookID)
	if err != nil {
		return result, err
	}
	fallbacks, err := s.repository.ImportedDividendCashFallbacks(ctx, BookID)
	if err != nil {
		return result, err
	}

	type instrumentKey struct{ commodity, cash int64 }
	type instrumentAcc struct {
		row                         DividendCoverageInstrument
		gross, withholding, pending *exact.ScaledInt
	}
	instruments := map[instrumentKey]*instrumentAcc{}
	instrument := func(commodity, cash int64, code string) *instrumentAcc {
		key := instrumentKey{commodity, cash}
		if instruments[key] == nil {
			instruments[key] = &instrumentAcc{
				row:   DividendCoverageInstrument{CommodityID: commodity, CashCommodityID: cash, CurrencyCode: code},
				gross: exact.NewScaledInt(), withholding: exact.NewScaledInt(), pending: exact.NewScaledInt(),
			}
		}
		return instruments[key]
	}
	type totalAcc struct {
		row  DividendCoverageTotal
		cash *exact.ScaledInt
	}
	totals := map[int64]*totalAcc{}
	total := func(cash int64, code string) *totalAcc {
		if totals[cash] == nil {
			totals[cash] = &totalAcc{row: DividendCoverageTotal{CashCommodityID: cash, CurrencyCode: code}, cash: exact.NewScaledInt()}
		}
		return totals[cash]
	}
	var items []DividendCoverageItem

	for _, op := range operations {
		net := exact.ScaledIntFromBig(op.Gross.BigInt(), op.Gross.Scale())
		net.SubScaled(op.Withholding)
		if op.Payer.Valid {
			acc := instrument(op.Payer.Int64, op.CashCommodityID, op.CashCode)
			if op.WithholdingOnly {
				acc.row.WithholdingPaymentCount++
			} else {
				acc.row.DividendCount++
			}
			acc.gross.AddScaled(op.Gross)
			acc.withholding.AddScaled(op.Withholding)
			continue
		}
		cash, err := coverageAmount(net)
		if err != nil {
			return result, err
		}
		item := DividendCoverageItem{Kind: CoverageItemNamedUnassigned, TransactionID: op.TransactionID, Date: op.EventDate,
			CashCommodityID: op.CashCommodityID, CurrencyCode: op.CashCode, Cash: cash,
			GrossState: CoverageKnown, WithholdingState: CoverageKnownZero, PayerState: CoveragePayerUnassigned,
			Reasons: []string{CoverageReasonPayerUnassigned}}
		if op.WithholdingOnly {
			item.GrossState = CoverageNone
		} else if item.Gross, err = coverageAmountPtr(op.Gross); err != nil {
			return result, err
		}
		if op.Withholding.Sign() != 0 {
			item.WithholdingState = CoverageKnown
			if item.Withholding, err = coverageAmountPtr(op.Withholding); err != nil {
				return result, err
			}
		}
		acc := total(op.CashCommodityID, op.CashCode)
		acc.row.ItemCount++
		acc.row.PayerUnassignedCount++
		acc.cash.AddScaled(net)
		items = append(items, item)
	}

	// The adapter's own read-only identity lookup (ISIN, then ticker) names a
	// payer from typed source fields; memo text is never consulted.
	type sourceKey struct{ isin, ticker string }
	payers := map[sourceKey]*int64{}
	for _, fallback := range fallbacks {
		var raw map[string]string
		var normalized importDividendSource
		if err := json.Unmarshal([]byte(fallback.RawJSON), &raw); err != nil {
			return result, fmt.Errorf("imported dividend %d evidence: %w", fallback.RowID, err)
		}
		if err := json.Unmarshal([]byte(fallback.NormalizedJSON), &normalized); err != nil {
			return result, fmt.Errorf("imported dividend %d evidence: %w", fallback.RowID, err)
		}
		key := sourceKey{strings.TrimSpace(raw[rawKeyISIN]), strings.TrimSpace(raw[rawKeyTicker])}
		payer, seen := payers[key]
		if !seen {
			_, commodityID, found, err := s.FindInstrumentForImport(ctx, key.isin, key.ticker)
			if err != nil {
				return result, err
			}
			if found {
				payer = &commodityID
			}
			payers[key] = payer
		}
		cash, err := coverageAmount(fallback.Cash)
		if err != nil {
			return result, err
		}
		item := DividendCoverageItem{Kind: CoverageItemImportedCash, TransactionID: fallback.TransactionID, Date: fallback.TransactionDate,
			CashCommodityID: fallback.CashCommodityID, CurrencyCode: fallback.CashCode, Cash: cash,
			GrossState: CoverageUnresolved, WithholdingState: CoverageUnresolved, PayerState: CoveragePayerUnassigned,
			Reasons: []string{CoverageReasonPayerUnassigned, CoverageReasonGrossUnresolved, CoverageReasonWithholdingUnresolved},
			Source: &DividendCoverageSource{BatchID: fallback.BatchID, RowID: fallback.RowID,
				Instrument: strings.TrimSpace(raw[rawKeyTicker] + " " + raw[rawKeyISIN]),
				Date:       normalized.Date, CashAmount: normalized.Amount, Currency: normalized.CommodityHint}}
		acc := total(fallback.CashCommodityID, fallback.CashCode)
		acc.row.ItemCount++
		acc.row.EconomicsUnresolvedCount++
		acc.cash.AddScaled(fallback.Cash)
		if payer != nil {
			id := *payer
			item.PayerState, item.SourcePayerCommodityID = CoveragePayerSourceIdentified, &id
			item.Reasons[0] = CoverageReasonPayerUnlinked
			pending := instrument(id, fallback.CashCommodityID, fallback.CashCode)
			pending.row.ImportedUnclassifiedCount++
			pending.pending.AddScaled(fallback.Cash)
		} else {
			acc.row.PayerUnassignedCount++
		}
		items = append(items, item)
	}

	for _, acc := range instruments {
		if input.CommodityID > 0 && acc.row.CommodityID != input.CommodityID {
			continue
		}
		row := acc.row
		row.IncomeState, row.WithholdingState = CoverageNone, CoverageNone
		if row.DividendCount > 0 {
			row.IncomeState = CoverageKnown
		}
		if row.DividendCount+row.WithholdingPaymentCount > 0 {
			row.WithholdingState = CoverageKnownZero
			if acc.withholding.Sign() != 0 {
				row.WithholdingState = CoverageKnown
			}
			net := exact.ScaledIntFromBig(acc.gross.BigInt(), acc.gross.Scale())
			net.SubScaled(acc.withholding)
			var err error
			if row.Withholding, err = coverageAmountPtr(acc.withholding); err != nil {
				return result, err
			}
			if row.NetCash, err = coverageAmountPtr(net); err != nil {
				return result, err
			}
			if row.DividendCount > 0 {
				if row.Gross, err = coverageAmountPtr(acc.gross); err != nil {
					return result, err
				}
			}
		}
		if row.ImportedUnclassifiedCount > 0 {
			var err error
			if row.ImportedUnclassifiedCash, err = coverageAmountPtr(acc.pending); err != nil {
				return result, err
			}
		}
		result.Instruments = append(result.Instruments, row)
	}
	sort.Slice(result.Instruments, func(i, j int) bool {
		a, b := result.Instruments[i], result.Instruments[j]
		if a.CommodityID != b.CommodityID {
			return a.CommodityID < b.CommodityID
		}
		return a.CashCommodityID < b.CashCommodityID
	})
	for _, acc := range totals {
		cash, err := coverageAmount(acc.cash)
		if err != nil {
			return result, err
		}
		acc.row.Cash = cash
		result.UnassignedTotals = append(result.UnassignedTotals, acc.row)
	}
	sort.Slice(result.UnassignedTotals, func(i, j int) bool {
		return result.UnassignedTotals[i].CurrencyCode < result.UnassignedTotals[j].CurrencyCode
	})

	if input.SummaryOnly {
		return result, nil
	}
	sort.Slice(items, func(i, j int) bool { return coverageItemBefore(items[i], items[j]) })
	start := 0
	if after != nil {
		start = sort.Search(len(items), func(i int) bool { return coverageItemBefore(after.item(), items[i]) })
	}
	end := start + limit
	if end < len(items) {
		last := items[end-1]
		cursor := formatCoverageCursor(last)
		result.NextCursor = &cursor
	} else {
		end = len(items)
	}
	result.Unassigned = append(result.Unassigned, items[start:end]...)
	return result, nil
}

// coverageItemBefore orders newest first, with a total order for the cursor.
// Named and imported items never share a transaction.
func coverageItemBefore(a, b DividendCoverageItem) bool {
	if a.Date != b.Date {
		return a.Date > b.Date
	}
	if a.TransactionID != b.TransactionID {
		return a.TransactionID > b.TransactionID
	}
	return a.CashCommodityID > b.CashCommodityID
}

func (c coverageCursor) item() DividendCoverageItem {
	return DividendCoverageItem{Date: c.date, TransactionID: c.transactionID, CashCommodityID: c.cashCommodityID}
}

func formatCoverageCursor(item DividendCoverageItem) string {
	return item.Date + "_" + strconv.FormatInt(item.TransactionID, 10) + "_" + strconv.FormatInt(item.CashCommodityID, 10)
}

func parseCoverageCursor(raw string) (coverageCursor, bool) {
	parts := strings.Split(raw, "_")
	if len(parts) != 3 {
		return coverageCursor{}, false
	}
	date, err := cleanRequiredDate(parts[0], "cursor")
	if err != nil {
		return coverageCursor{}, false
	}
	transactionID, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil || transactionID <= 0 {
		return coverageCursor{}, false
	}
	cashID, err := strconv.ParseInt(parts[2], 10, 64)
	if err != nil || cashID <= 0 {
		return coverageCursor{}, false
	}
	return coverageCursor{date: date, transactionID: transactionID, cashCommodityID: cashID}, true
}

func coverageAmount(value *exact.ScaledInt) (GainsFXAmount, error) {
	coefficient, err := value.Coefficient()
	if err != nil {
		return GainsFXAmount{}, fmt.Errorf("income coverage amount: %w", err)
	}
	return GainsFXAmount{Value: coefficient, Scale: value.Scale()}, nil
}

func coverageAmountPtr(value *exact.ScaledInt) (*GainsFXAmount, error) {
	amount, err := coverageAmount(value)
	if err != nil {
		return nil, err
	}
	return &amount, nil
}
