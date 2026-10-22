package app

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"rekenraam/backend/internal/db"
	"rekenraam/backend/internal/exact"
)

// CSVInvestmentMapping opts a saved CSV profile into named income/tax writes.
// AmountColumn remains signed cash; GrossColumn is independent source evidence.
type CSVInvestmentMapping struct {
	Kind                 string `json:"kind"`
	KindColumn           string `json:"kind_column,omitempty"`
	DividendValue        string `json:"dividend_value,omitempty"`
	WithholdingValue     string `json:"withholding_value,omitempty"`
	InstrumentColumn     string `json:"instrument_column"`
	GrossColumn          string `json:"gross_column,omitempty"`
	WithholdingColumn    string `json:"withholding_column,omitempty"`
	CashAccountID        int64  `json:"cash_account_id,omitempty"`
	IncomeAccountID      int64  `json:"income_account_id,omitempty"`
	WithholdingAccountID int64  `json:"withholding_account_id,omitempty"`
}

type CSVInvestmentRow struct {
	CashValid         bool                `json:"cash_valid"`
	Kind              string              `json:"kind"`
	InstrumentCode    string              `json:"instrument_code"`
	GrossAmount       string              `json:"gross_amount"`
	WithholdingAmount string              `json:"withholding_amount"`
	Resolution        ImportRowResolution `json:"-"`
}

func validateCSVInvestmentMapping(config csvProfileConfig) error {
	m := config.Investment
	for _, field := range []*string{&m.Kind, &m.KindColumn, &m.DividendValue, &m.WithholdingValue, &m.InstrumentColumn, &m.GrossColumn, &m.WithholdingColumn} {
		*field = strings.TrimSpace(*field)
	}
	if (m.Kind != "dividend" && m.Kind != "withholding" && m.Kind != "column") || m.InstrumentColumn == "" ||
		(m.Kind != "withholding" && m.GrossColumn == "") ||
		(m.Kind == "column" && (m.KindColumn == "" || m.DividendValue == "" || m.WithholdingValue == "" || m.DividendValue == m.WithholdingValue)) ||
		m.CashAccountID < 0 || m.IncomeAccountID < 0 || m.WithholdingAccountID < 0 {
		return ValidationError{Message: "CSV investment mapping requires an explicit kind, instrument code column and dividend gross column"}
	}
	if m.Kind != "column" {
		m.KindColumn = ""
	}
	if m.Kind == "withholding" {
		m.GrossColumn, m.WithholdingColumn = "", ""
	}
	return nil
}

func parseCSVInvestmentRow(record []string, headers map[string]int, m *CSVInvestmentMapping, separator rune) *CSVInvestmentRow {
	kind := m.Kind
	if kind == "column" {
		kind = ""
		switch csvValue(record, headers, m.KindColumn) {
		case m.DividendValue:
			kind = "dividend"
		case m.WithholdingValue:
			kind = "withholding"
		}
	}
	normalize := func(column string, fallback string) string {
		if column == "" {
			return fallback
		}
		value := csvValue(record, headers, column)
		if value == "" {
			return value
		}
		canonical, err := canonicalDecimal(value, separator)
		if err != nil {
			return ""
		} // Never reinterpret a rejected source in another locale.
		return canonical
	}
	row := &CSVInvestmentRow{Kind: kind, InstrumentCode: csvValue(record, headers, m.InstrumentColumn),
		GrossAmount: normalize(m.GrossColumn, ""), WithholdingAmount: normalize(m.WithholdingColumn, "0"),
		Resolution: ImportRowResolution{AccountID: m.CashAccountID},
	}
	if m.IncomeAccountID > 0 {
		id := m.IncomeAccountID
		row.Resolution.CategoryID = &id
	}
	if m.WithholdingAccountID > 0 {
		id := m.WithholdingAccountID
		row.Resolution.WithholdingAccountID = &id
	}
	return row
}

// One catalog read for the file. Codes are stable unique identities, not symbols.
func (s *ImportService) prepareCSVInvestmentRows(ctx context.Context, parsed *ParseResult) error {
	ctx, referenceErr := s.settings.referenceContext(ctx, s.now())
	if referenceErr != nil {
		return referenceErr
	}

	var first *CSVInvestmentRow
	for _, row := range parsed.Rows {
		if row.Investment != nil {
			first = row.Investment
			break
		}
	}
	if first == nil {
		return nil
	}
	if s.investmentService == nil {
		return ValidationError{Message: "investments are unavailable"}
	}
	instruments, err := s.investmentService.repository.ListInstruments(ctx, BookID)
	if err != nil {
		return fmt.Errorf("read CSV instruments: %w", err)
	}
	codes := map[string]int64{}
	for _, instrument := range instruments {
		if instrument.Status == "active" {
			codes[instrument.CommodityCode] = instrument.CommodityID
		}
	}
	var currencyID int64
	if first.Resolution.AccountID > 0 {
		account, err := s.accountRepository.CurrentAccountByID(ctx, BookID, first.Resolution.AccountID)
		if err != nil && !errors.Is(err, db.ErrNotFound) {
			return fmt.Errorf("read CSV cash account: %w", err)
		}
		currencyID = account.DefaultCommodityID.Int64
	}
	for i := range parsed.Rows {
		if row := parsed.Rows[i].Investment; row != nil {
			row.Resolution.InstrumentCommodityID = codes[row.InstrumentCode]
			row.Resolution.CommodityID = currencyID
		}
	}
	return nil
}

func isCSVInvestmentRow(batch db.ImportBatchRecord, row db.ImportStagedRowRecord) bool {
	if batch.SourceKind != "csv" {
		return false
	}
	var normalized struct {
		Investment *CSVInvestmentRow `json:"investment"`
	}
	return json.Unmarshal([]byte(row.NormalizedJSON), &normalized) == nil && normalized.Investment != nil
}

func (s *ImportService) csvInvestmentDividend(ctx context.Context, row db.ImportStagedRowRecord, ownerID, sessionID int64, requestID string) (DividendInput, error) {
	ctx, referenceErr := s.settings.referenceContext(ctx, s.now())
	if referenceErr != nil {
		return DividendInput{}, referenceErr
	}

	refuse := func() (DividendInput, error) {
		return DividendInput{}, ValidationError{Message: "CSV investment row requires review: choose instrument, kind and accounts; gross minus withholding must equal signed cash"}
	}
	var normalized struct {
		Date        string            `json:"date"`
		Amount      string            `json:"amount"`
		Memo        string            `json:"memo"`
		ExternalRef string            `json:"external_ref"`
		PayeeHint   string            `json:"payee_hint"`
		Investment  *CSVInvestmentRow `json:"investment"`
	}
	if s.investmentService == nil || json.Unmarshal([]byte(row.NormalizedJSON), &normalized) != nil || normalized.Investment == nil {
		return refuse()
	}
	resolution, err := parseResolutionJSON(row.ResolutionJSON)
	if err != nil || resolution.AccountID <= 0 || resolution.CommodityID <= 0 || resolution.InstrumentCommodityID <= 0 || resolution.TransferAccountID != nil || len(resolution.TagIDs) > 0 {
		return refuse()
	}
	instrument, err := s.investmentService.repository.InstrumentByCommodityID(ctx, BookID, resolution.InstrumentCommodityID)
	if errors.Is(err, db.ErrNotFound) {
		return refuse()
	}
	if err != nil {
		return DividendInput{}, fmt.Errorf("read CSV paying instrument: %w", err)
	}
	if instrument.Status != "active" {
		return refuse()
	}
	source := normalized.Investment
	if !source.CashValid {
		return refuse()
	}
	payeeName := resolution.PayeeName
	if payeeName == "" {
		payeeName = normalized.PayeeHint
	}
	if resolution.PayeeID == nil && strings.TrimSpace(payeeName) != "" {
		id, _, err := s.transactionService.payeeRepository.PayeeByNormalizedName(ctx, BookID, normalizePayeeNameForMatch(payeeName))
		if errors.Is(err, db.ErrNotFound) {
			return refuse()
		}
		if err != nil {
			return DividendInput{}, fmt.Errorf("resolve CSV payee: %w", err)
		}
		resolution.PayeeID = &id
	}
	kind := source.Kind
	if resolution.InvestmentKind != "" {
		kind = resolution.InvestmentKind
	}
	cash, cashScale, err := parseCanonicalAmount(normalized.Amount)
	if err != nil {
		return refuse()
	}
	input := DividendInput{OwnerUserID: ownerID, AuthSessionID: sessionID, RequestID: requestID,
		TransactionDate: normalized.Date, CommodityID: &resolution.InstrumentCommodityID,
		CashAccountID: resolution.AccountID, CashCommodityID: resolution.CommodityID,
		WithholdingAccountID: resolution.WithholdingAccountID, Memo: normalized.Memo, PayeeID: resolution.PayeeID,
		NeedsReview: true, ExternalRefHint: normalized.ExternalRef, OriginType: "import", Operation: "investment.dividend.csv",
	}
	if kind == "withholding" {
		if cash.Sign() >= 0 {
			return refuse()
		}
		input.WithholdingOnly = true
		input.AmountScale = cashScale
		input.AmountValue, err = exact.ScaledIntFromCoefficient(cash.Negated(), cashScale).Int64()
	} else if kind == "dividend" {
		grossText, taxText := source.GrossAmount, source.WithholdingAmount
		if resolution.GrossAmount != nil {
			grossText = *resolution.GrossAmount
		}
		if resolution.WithholdingAmount != nil {
			taxText = *resolution.WithholdingAmount
		}
		gross, grossScale, grossErr := parseDecimalAmount(grossText)
		tax, taxScale, taxErr := parseDecimalAmount(taxText)
		if grossErr != nil || taxErr != nil || gross.Sign() <= 0 || tax.Sign() < 0 || resolution.CategoryID == nil {
			return refuse()
		}
		net := exact.ScaledIntFromCoefficient(gross, grossScale)
		net.SubScaled(exact.ScaledIntFromCoefficient(tax, taxScale))
		if net.Cmp(exact.ScaledIntFromCoefficient(cash, cashScale)) != 0 {
			return refuse()
		}
		input.IncomeAccountID = resolution.CategoryID
		input.AmountScale = grossScale
		input.AmountValue, err = exact.ScaledIntFromCoefficient(gross, grossScale).Int64()
		if tax.Sign() > 0 {
			value, taxErr := exact.ScaledIntFromCoefficient(tax, taxScale).Int64()
			if taxErr != nil {
				return refuse()
			}
			input.WithholdingValue, input.WithholdingScale = &value, &taxScale
		}
	} else {
		return refuse()
	}
	if err != nil {
		return refuse()
	}
	return input, nil
}

func (s *ImportService) commitCSVInvestmentRow(ctx context.Context, row db.ImportStagedRowRecord, batch db.ImportBatchRecord, input CommitImportBatchInput, now string) (bool, int64, int64, error) {
	dividend, err := s.csvInvestmentDividend(ctx, row, input.OwnerUserID, input.AuthSessionID, input.RequestID)
	if err != nil {
		return true, 0, 0, err
	}
	dividend.ReconciliationOverride = input.ReconciliationOverride
	transaction, err := s.investmentService.dividendWithPostWrite(ctx, dividend, func(tx *sql.Tx, transactionID int64) error {
		return s.recordCommitIdentityAndMarkRowInTx(ctx, tx, row.ID, row.DedupeFingerprint, batch.SourceKind, dividend.CashAccountID, transactionID, now)
	})
	return true, transaction.ID, dividend.CashAccountID, err
}
