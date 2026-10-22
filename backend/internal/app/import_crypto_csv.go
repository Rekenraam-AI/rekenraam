package app

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"rekenraam/backend/internal/db"
	"rekenraam/backend/internal/exact"
)

// CryptoCSVAdapter is deliberately a small interchange format, not an exchange
// integration. The owner chooses one existing holding and settlement account.
type CryptoCSVAdapter struct{}

func (*CryptoCSVAdapter) Kind() string               { return "crypto_csv" }
func (*CryptoCSVAdapter) Detect(RawInput) Confidence { return ConfidenceNone }

var cryptoDecimalPattern = regexp.MustCompile(`^[0-9]+(?:\.[0-9]+)?$`)

func (*CryptoCSVAdapter) Parse(_ context.Context, input RawInput, _ *ImportProfile) (ParseResult, error) {
	if !utf8.Valid(input.Bytes) {
		return ParseResult{}, ValidationError{Message: "crypto CSV must use UTF-8"}
	}
	reader := csv.NewReader(bytes.NewReader(input.Bytes))
	header, err := reader.Read()
	if err != nil {
		return ParseResult{}, ValidationError{Message: "crypto CSV header is required"}
	}
	expected := []string{"date", "side", "quantity", "amount", "external_ref", "memo"}
	if len(header) != len(expected) {
		return ParseResult{}, ValidationError{Message: "crypto CSV columns must be date,side,quantity,amount,external_ref,memo"}
	}
	for i, column := range header {
		if strings.TrimPrefix(column, "\ufeff") != expected[i] {
			return ParseResult{}, ValidationError{Message: "crypto CSV columns must be date,side,quantity,amount,external_ref,memo"}
		}
	}
	result := ParseResult{}
	for index := 1; ; index++ {
		row, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return ParseResult{}, ValidationError{Message: fmt.Sprintf("invalid crypto CSV row %d", index)}
		}
		if index > 10000 {
			return ParseResult{}, ValidationError{Message: "crypto CSV is limited to 10000 rows"}
		}
		for i := range row {
			row[i] = strings.TrimSpace(row[i])
		}
		if _, err := time.Parse(time.DateOnly, row[0]); err != nil {
			return ParseResult{}, ValidationError{Message: fmt.Sprintf("invalid crypto date on row %d", index)}
		}
		if row[1] != "BUY" && row[1] != "SELL" {
			return ParseResult{}, ValidationError{Message: fmt.Sprintf("crypto row %d side must be BUY or SELL", index)}
		}
		for i, scale := range []int{24, 12} {
			text := row[i+2]
			if len(text) > 64 || !cryptoDecimalPattern.MatchString(text) {
				return ParseResult{}, ValidationError{Message: fmt.Sprintf("invalid unsigned dot-decimal on crypto row %d", index)}
			}
			value, places, err := parseDecimalAmount(text)
			if err != nil || value.Sign() <= 0 || places > scale {
				return ParseResult{}, ValidationError{Message: fmt.Sprintf("invalid amount or precision on crypto row %d", index)}
			}
			if i == 1 {
				if _, err := exact.ScaledIntFromCoefficient(value, places).Int64(); err != nil {
					return ParseResult{}, ValidationError{Message: "crypto cash amount exceeds supported range"}
				}
			}
		}
		if row[4] == "" || len(row[4]) > 200 || len(row[5]) > 500 {
			return ParseResult{}, ValidationError{Message: "crypto CSV needs a unique external_ref and memo at most 500 bytes"}
		}
		amount := row[3]
		if row[1] == "BUY" {
			amount = "-" + amount
		}
		result.Rows = append(result.Rows, StagedRow{Date: row[0], Amount: amount, ExternalRef: row[4], Memo: row[5], Raw: map[string]string{"kind": "crypto_csv", "date": row[0], "side": row[1], "quantity": row[2], "amount": row[3], "external_ref": row[4], "memo": row[5]}})
	}
	if len(result.Rows) == 0 {
		return ParseResult{}, ValidationError{Message: "crypto CSV has no trades"}
	}
	return result, nil
}

func (s *ImportService) prepareCryptoCSV(ctx context.Context, input StartImportInput, result *ParseResult) error {
	ctx, referenceErr := s.settings.referenceContext(ctx, s.now())
	if referenceErr != nil {
		return referenceErr
	}

	if s.investmentService == nil {
		return ValidationError{Message: "investments are unavailable"}
	}
	holding, err := s.accountRepository.CurrentAccountByID(ctx, BookID, input.HoldingAccountID)
	if errors.Is(err, db.ErrNotFound) {
		return ValidationError{Message: "crypto holding account is required"}
	}
	if err != nil {
		return fmt.Errorf("read crypto holding: %w", err)
	}
	cash, err := s.accountRepository.CurrentAccountByID(ctx, BookID, input.CashAccountID)
	if errors.Is(err, db.ErrNotFound) {
		return ValidationError{Message: "crypto settlement account is required"}
	}
	if err != nil {
		return fmt.Errorf("read crypto settlement account: %w", err)
	}
	instrument, err := s.investmentService.repository.InstrumentByCommodityID(ctx, BookID, holding.DefaultCommodityID.Int64)
	if err != nil && !errors.Is(err, db.ErrNotFound) {
		return fmt.Errorf("read crypto instrument: %w", err)
	}
	if err != nil || instrument.InstrumentType != "crypto" || instrument.Status != "active" || holding.Status != "active" || holding.AccountKind != "security_holding" || cash.Status != "active" || cash.AccountClass != "asset" || cash.DefaultCommodityID.Int64 == 0 {
		return ValidationError{Message: "select an active crypto holding and currency settlement account"}
	}
	currency, err := s.accountRepository.CurrentCommodityByID(ctx, BookID, cash.DefaultCommodityID.Int64)
	if err != nil && !errors.Is(err, db.ErrNotFound) {
		return fmt.Errorf("read crypto settlement currency: %w", err)
	}
	if err != nil || currency.Kind != "currency" {
		return ValidationError{Message: "settlement account must use a currency"}
	}
	for i := range result.Rows {
		row := &result.Rows[i]
		row.Raw["commodity_id"] = strconv.FormatInt(instrument.CommodityID, 10)
		row.Raw["holding_account_id"] = strconv.FormatInt(holding.ID, 10)
		row.Raw["cash_account_id"] = strconv.FormatInt(cash.ID, 10)
		row.Raw["cash_commodity_id"] = strconv.FormatInt(currency.ID, 10)
		row.CommodityHint = currency.Code
		// Stable source identity within the chosen position. Changed evidence with
		// the same reference stays duplicate; corrections use the native workflow.
		row.DedupeFingerprint = fmt.Sprintf("crypto_csv:%d:%d:%s", holding.ID, cash.ID, row.ExternalRef)
	}
	return nil
}

func (s *ImportService) cryptoCSVTrade(row db.ImportStagedRowRecord, ownerID, sessionID int64, requestID string) (InvestmentTradeInput, string, error) {
	var raw map[string]string
	if err := json.Unmarshal([]byte(row.RawJSON), &raw); err != nil || raw["kind"] != "crypto_csv" {
		return InvestmentTradeInput{}, "", ValidationError{Message: "invalid crypto import evidence"}
	}
	ids := make([]int64, 4)
	for i, key := range []string{"commodity_id", "holding_account_id", "cash_account_id", "cash_commodity_id"} {
		id, err := strconv.ParseInt(raw[key], 10, 64)
		if err != nil || id <= 0 {
			return InvestmentTradeInput{}, "", ValidationError{Message: "invalid crypto import account"}
		}
		ids[i] = id
	}
	if raw["side"] != "BUY" && raw["side"] != "SELL" {
		return InvestmentTradeInput{}, "", ValidationError{Message: "invalid crypto trade side"}
	}
	quantity, scale, err := parseDecimalAmount(raw["quantity"])
	if err != nil || quantity.Sign() <= 0 {
		return InvestmentTradeInput{}, "", ValidationError{Message: "invalid crypto quantity"}
	}
	cash, cashScale, err := parseDecimalAmount(raw["amount"])
	if err != nil || cash.Sign() <= 0 {
		return InvestmentTradeInput{}, "", ValidationError{Message: "invalid crypto cash amount"}
	}
	cashValue, err := exact.ScaledIntFromCoefficient(cash, cashScale).Int64()
	if err != nil {
		return InvestmentTradeInput{}, "", err
	}
	return InvestmentTradeInput{OwnerUserID: ownerID, AuthSessionID: sessionID, RequestID: requestID, TransactionDate: raw["date"], CommodityID: ids[0], HoldingAccountID: ids[1], CashAccountID: ids[2], CashCommodityID: ids[3], QuantityValue: quantity, QuantityScale: scale, CashAmountValue: cashValue, CashAmountScale: cashScale, Memo: raw["memo"], ExternalRefHint: raw["external_ref"], Status: "posted", OriginType: "import", Operation: "investment.crypto_csv." + strings.ToLower(raw["side"])}, raw["side"], nil
}

func (s *ImportService) commitInvestmentRow(ctx context.Context, row db.ImportStagedRowRecord, batch db.ImportBatchRecord, input CommitImportBatchInput, now string) (bool, int64, int64, error) {
	if isCSVInvestmentRow(batch, row) {
		return s.commitCSVInvestmentRow(ctx, row, batch, input, now)
	}
	if batch.SourceKind != "crypto_csv" {
		return s.commitTrading212InvestmentRow(ctx, row, batch, input, now)
	}
	trade, side, err := s.cryptoCSVTrade(row, input.OwnerUserID, input.AuthSessionID, input.RequestID)
	if err != nil {
		return true, 0, 0, err
	}
	trade.ReconciliationOverride = input.ReconciliationOverride
	trade.GainImpactAcknowledgement = input.GainImpactAcknowledgements[row.ID]
	postWrite := func(tx *sql.Tx, transactionID int64) error {
		return s.recordCommitIdentityAndMarkRowInTx(ctx, tx, row.ID, row.DedupeFingerprint, batch.SourceKind, trade.HoldingAccountID, transactionID, now)
	}
	var result InvestmentTradeResult
	if side == "BUY" {
		result, err = s.investmentService.buyWithPostWrite(ctx, trade, postWrite)
	} else {
		result, err = s.investmentService.sellWithPostWrite(ctx, trade, postWrite)
	}
	return true, result.Transaction.ID, trade.HoldingAccountID, err
}
