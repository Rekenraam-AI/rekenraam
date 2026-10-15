package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"rekenraam/backend/internal/db"
	"rekenraam/backend/internal/exact"
)

// Owner-asserted short links (#245). A provider fill whose type is not a
// documented execution never posts (#176 keeps automatic short admission
// behind verified payloads). When the owner has recorded the short sale or
// cover natively, they can link the pending row to it: the row's dedupe
// identity then names that operation, so a retry or re-fetch is a duplicate
// and nothing posts twice. The link never classifies the row by itself — the
// owner picks the operation, and every comparable source fact must agree.

var (
	// ErrImportShortLinkUnavailable means the row is not a pending
	// unsupported fill of a known security in this batch, or the operation is
	// not an effective, unlinked short sale or cover the row's side can evidence.
	ErrImportShortLinkUnavailable = errors.New("import row cannot be linked to this short operation")
	// ErrImportShortLinkChanged means the operation was corrected or changed
	// after the owner reviewed it.
	ErrImportShortLinkChanged = errors.New("short operation changed since it was reviewed")
	// ErrImportShortLinkMismatch means a known source fact disagrees with the
	// operation; the row is never matched on ticker or amount alone.
	ErrImportShortLinkMismatch = errors.New("import row evidence does not match this short operation")
)

// Short link mismatch codes, in display order.
const (
	ShortLinkMismatchDate         = "date"
	ShortLinkMismatchQuantity     = "quantity"
	ShortLinkMismatchCashAmount   = "cash_amount"
	ShortLinkMismatchCashCurrency = "cash_currency"
	ShortLinkMismatchAccount      = "holding_account"
	ShortLinkMismatchCashAccount  = "cash_account"
)

type ImportShortLinkMismatchError struct {
	Fields []string
}

func (e ImportShortLinkMismatchError) Error() string {
	return fmt.Sprintf("%s: %s", ErrImportShortLinkMismatch, strings.Join(e.Fields, ", "))
}

func (e ImportShortLinkMismatchError) Unwrap() error { return ErrImportShortLinkMismatch }

type Trading212ShortRowInput struct {
	OwnerUserID   int64
	AuthSessionID int64
	RequestID     string
	BatchID       int64
	RowID         int64
}

type LinkTrading212ShortInput struct {
	Trading212ShortRowInput
	OperationID int64
}

// Trading212ShortSource is the row's raw evidence as the provider sent it,
// shown beside the candidates for review. Blank fields are unknown.
type Trading212ShortSource struct {
	OperationKind string
	FillType      string
	Side          string
	Ticker        string
	ISIN          string
	Date          string
	Quantity      string
	NetValue      string
	NetCurrency   string
}

// Trading212ShortCandidate is a recorded short operation and the comparable
// source facts that disagree with it. Only a candidate with no mismatches
// and not already linked can be linked.
type Trading212ShortCandidate struct {
	OperationID      int64
	OperationKind    string
	TransactionID    int64
	HoldingAccountID int64
	CashAccountID    int64
	EventDate        string
	Quantity         string
	NetAmount        string
	CashCurrency     string
	Linked           bool
	Mismatches       []string
}

type Trading212ShortCandidates struct {
	Source     Trading212ShortSource
	Candidates []Trading212ShortCandidate
}

type trading212ShortRow struct {
	row         db.ImportStagedRowRecord
	sourceKind  string
	source      Trading212ShortSource
	commodityID int64
	found       bool
	// Comparable evidence; nil or zero when the source does not supply it.
	quantity         *exact.ScaledInt
	net              *exact.ScaledInt
	holdingAccountID int64
	cashAccountID    int64
}

func (s *ImportService) trading212ShortRow(ctx context.Context, input Trading212ShortRowInput) (trading212ShortRow, error) {
	if input.OwnerUserID <= 0 || input.BatchID <= 0 || input.RowID <= 0 {
		return trading212ShortRow{}, ValidationError{Message: "owner, batch and row ids are required"}
	}
	if s.investmentService == nil {
		return trading212ShortRow{}, ErrImportShortLinkUnavailable
	}
	batch, err := s.repository.ImportBatchByID(ctx, BookID, input.BatchID)
	if errors.Is(err, db.ErrImportBatchNotFound) {
		return trading212ShortRow{}, ErrImportBatchNotFound
	}
	if err != nil {
		return trading212ShortRow{}, fmt.Errorf("read short row batch: %w", err)
	}
	if batch.SourceKind != "trading212" || batch.Status == "discarded" || batch.Status == "rolled_back" {
		return trading212ShortRow{}, ErrImportShortLinkUnavailable
	}
	row, err := s.repository.ImportStagedRowByID(ctx, input.RowID)
	if errors.Is(err, db.ErrNotFound) {
		return trading212ShortRow{}, ErrImportShortLinkUnavailable
	}
	if err != nil {
		return trading212ShortRow{}, fmt.Errorf("read staged short row: %w", err)
	}
	var raw map[string]string
	if row.BatchID != input.BatchID || row.CommitStatus == "committed" || row.DedupeStatus == "duplicate" ||
		json.Unmarshal([]byte(row.RawJSON), &raw) != nil || !unsupportedTrading212Fill(raw) ||
		raw["fill_type"] == trading212FillTypeStockSplit {
		return trading212ShortRow{}, ErrImportShortLinkUnavailable
	}
	// The side names which recorded operation the row could evidence. It is
	// never inferred from the holding's sign.
	side := strings.ToUpper(strings.TrimSpace(raw[rawKeySide]))
	kind := map[string]string{"SELL": "short_sale", "BUY": "short_cover"}[side]
	if kind == "" {
		return trading212ShortRow{}, ErrImportShortLinkUnavailable
	}
	var normalized struct {
		Date          string `json:"date"`
		CommodityHint string `json:"commodity_hint"`
	}
	_ = json.Unmarshal([]byte(row.NormalizedJSON), &normalized)
	result := trading212ShortRow{row: row, sourceKind: batch.SourceKind, source: Trading212ShortSource{
		OperationKind: kind, FillType: raw["fill_type"], Side: side, Ticker: raw[rawKeyTicker], ISIN: raw[rawKeyISIN],
		Date: normalized.Date, Quantity: raw[rawKeyQuantity], NetValue: raw["net_value"], NetCurrency: normalized.CommodityHint,
	}}
	if value, scale, err := parsePositiveCoefficient(raw[rawKeyQuantity]); err == nil && value.Sign() > 0 {
		result.quantity = exact.ScaledIntFromCoefficient(value, scale)
	}
	if value, scale, err := parseDecimalAmount(raw["net_value"]); err == nil && normalized.CommodityHint != "" {
		result.net = exact.ScaledIntFromCoefficient(value, scale)
	}
	// Find, never create: linking must not leave an instrument behind.
	_, commodityID, found, err := s.investmentService.FindInstrumentForImport(ctx, raw[rawKeyISIN], raw[rawKeyTicker])
	if err != nil {
		return trading212ShortRow{}, fmt.Errorf("resolve short row instrument: %w", err)
	}
	result.commodityID, result.found = commodityID, found
	if found && batch.ConnectionID.Valid && s.connectionService != nil {
		connectionID := batch.ConnectionID.Int64
		// Account evidence only: the connection secret is never opened here.
		conn, err := s.connectionService.repository.ImportConnectionByID(ctx, connectionID, BookID)
		if err != nil && !errors.Is(err, db.ErrImportConnectionNotFound) {
			return trading212ShortRow{}, fmt.Errorf("read short row connection: %w", err)
		}
		if err == nil && conn.CashAccountID.Valid {
			result.cashAccountID = conn.CashAccountID.Int64
		}
		holdingID, mapped, err := s.connectionService.HoldingAccountForCommodity(ctx, connectionID, commodityID)
		if err != nil {
			return trading212ShortRow{}, fmt.Errorf("read short row holding account: %w", err)
		}
		if mapped {
			result.holdingAccountID = holdingID
		}
	}
	return result, nil
}

// mismatches lists every known source fact that disagrees with the
// operation. Unknown source facts are not compared, but the side, security
// and book always are (by candidate selection).
func (r trading212ShortRow) mismatches(candidate db.ShortLinkCandidate) ([]string, error) {
	mismatches := []string{}
	if r.source.Date != "" && r.source.Date != candidate.EventDate {
		mismatches = append(mismatches, ShortLinkMismatchDate)
	}
	if r.quantity != nil {
		quantity, err := exact.Parse(candidate.QuantityValue)
		if err != nil {
			return nil, fmt.Errorf("parse short operation quantity: %w", err)
		}
		if r.quantity.Cmp(exact.ScaledIntFromCoefficient(quantity, candidate.QuantityScale)) != 0 {
			mismatches = append(mismatches, ShortLinkMismatchQuantity)
		}
	}
	if r.net != nil {
		if !strings.EqualFold(r.source.NetCurrency, candidate.CashCommodityCode) {
			mismatches = append(mismatches, ShortLinkMismatchCashCurrency)
		} else {
			// Signed: a short sale receives cash and a cover pays it, as the
			// provider's SELL and BUY net values do.
			net, err := exact.Parse(candidate.NetValue)
			if err != nil {
				return nil, fmt.Errorf("parse short operation net settlement: %w", err)
			}
			if r.net.Cmp(exact.ScaledIntFromCoefficient(net, candidate.NetScale)) != 0 {
				mismatches = append(mismatches, ShortLinkMismatchCashAmount)
			}
		}
	}
	if r.holdingAccountID > 0 && r.holdingAccountID != candidate.HoldingAccountID {
		mismatches = append(mismatches, ShortLinkMismatchAccount)
	}
	if r.cashAccountID > 0 && r.cashAccountID != candidate.CashAccountID {
		mismatches = append(mismatches, ShortLinkMismatchCashAccount)
	}
	return mismatches, nil
}

// Trading212ShortCandidates lists the recorded short operations of the row's
// side and security, newest first, with the source facts that disagree with
// each. An unknown security has no candidates.
func (s *ImportService) Trading212ShortCandidates(ctx context.Context, input Trading212ShortRowInput) (Trading212ShortCandidates, error) {
	short, err := s.trading212ShortRow(ctx, input)
	if err != nil {
		return Trading212ShortCandidates{}, err
	}
	result := Trading212ShortCandidates{Source: short.source, Candidates: []Trading212ShortCandidate{}}
	if !short.found {
		return result, nil
	}
	records, err := s.repository.ListShortLinkCandidates(ctx, BookID, short.commodityID, short.source.OperationKind)
	if err != nil {
		return Trading212ShortCandidates{}, err
	}
	for _, record := range records {
		mismatches, err := short.mismatches(record)
		if err != nil {
			return Trading212ShortCandidates{}, err
		}
		quantity, err := exact.Parse(record.QuantityValue)
		if err != nil {
			return Trading212ShortCandidates{}, fmt.Errorf("parse short operation quantity: %w", err)
		}
		net, err := exact.Parse(record.NetValue)
		if err != nil {
			return Trading212ShortCandidates{}, fmt.Errorf("parse short operation net settlement: %w", err)
		}
		result.Candidates = append(result.Candidates, Trading212ShortCandidate{
			OperationID: record.OperationID, OperationKind: record.OperationKind, TransactionID: record.TransactionID,
			HoldingAccountID: record.HoldingAccountID, CashAccountID: record.CashAccountID, EventDate: record.EventDate,
			Quantity: exact.Decimal(quantity, record.QuantityScale), NetAmount: exact.Decimal(net, record.NetScale),
			CashCurrency: record.CashCommodityCode, Linked: record.Linked, Mismatches: mismatches,
		})
	}
	return result, nil
}

// LinkTrading212Short records the owner's assertion that the pending row is
// the source of an existing short sale or cover. The operation must be
// effective, unlinked, of the row's side and security, and agree with every
// comparable source fact; the writer rechecks it all atomically.
func (s *ImportService) LinkTrading212Short(ctx context.Context, input LinkTrading212ShortInput) error {
	if input.OperationID <= 0 {
		return ValidationError{Message: "short operation is required"}
	}
	short, err := s.trading212ShortRow(ctx, input.Trading212ShortRowInput)
	if err != nil {
		return err
	}
	if !short.found {
		return ErrImportShortLinkUnavailable
	}
	candidate, err := s.repository.ShortLinkCandidateByOperation(ctx, BookID, input.OperationID)
	if errors.Is(err, db.ErrNotFound) {
		return ErrImportShortLinkUnavailable
	}
	if err != nil {
		return err
	}
	if candidate.Linked || candidate.OperationKind != short.source.OperationKind || candidate.CommodityID != short.commodityID {
		return ErrImportShortLinkUnavailable
	}
	mismatches, err := short.mismatches(candidate)
	if err != nil {
		return err
	}
	if len(mismatches) > 0 {
		return ImportShortLinkMismatchError{Fields: mismatches}
	}
	_, err = s.repository.LinkStagedRowToShort(ctx, db.LinkStagedRowToShortParams{
		BookID: BookID, BatchID: input.BatchID, RowID: input.RowID,
		DedupeFingerprint: short.row.DedupeFingerprint, SourceKind: short.sourceKind,
		Expected: candidate, ActorUserID: input.OwnerUserID, AuthSessionID: input.AuthSessionID,
		RequestID: input.RequestID, Now: s.now().UTC().Format(time.RFC3339),
	})
	switch {
	case errors.Is(err, db.ErrShortLinkUnavailable):
		return ErrImportShortLinkUnavailable
	case errors.Is(err, db.ErrShortLinkChanged):
		return ErrImportShortLinkChanged
	}
	return err
}
