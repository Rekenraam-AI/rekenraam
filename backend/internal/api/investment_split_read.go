package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"

	"rekenraam/backend/internal/app"
	"rekenraam/backend/internal/db"
	"rekenraam/backend/internal/exact"
)

type splitActivityResponse struct {
	OperationID             int64             `json:"operation_id"`
	TransactionID           *int64            `json:"transaction_id"`
	CorrectionOfOperationID *int64            `json:"correction_of_operation_id"`
	SuccessorOperationID    *int64            `json:"successor_operation_id"`
	HoldingAccountID        int64             `json:"holding_account_id"`
	CommodityID             int64             `json:"commodity_id"`
	AccountName             string            `json:"account_name"`
	InstrumentName          string            `json:"instrument_name"`
	EffectiveOn             string            `json:"effective_on"`
	RatioNumerator          int64             `json:"ratio_numerator"`
	RatioDenominator        int64             `json:"ratio_denominator"`
	SourceEvidence          json.RawMessage   `json:"source_evidence"`
	Memo                    string            `json:"memo"`
	Reason                  string            `json:"reason"`
	CreatedAt               string            `json:"created_at"`
	Standing                string            `json:"standing"`
	Settled                 bool              `json:"settled"`
	QuantityDeltaValue      exact.Coefficient `json:"quantity_delta_value"`
	QuantityDeltaScale      int               `json:"quantity_delta_scale"`
}

type splitActivityPageResponse struct {
	Splits     []splitActivityResponse `json:"splits"`
	NextCursor *string                 `json:"next_cursor"`
}

func investmentSplitActivity(logger *slog.Logger, auth *app.AuthService, service *app.InvestmentService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		owner, ok := authenticatedOwner(w, r, logger, auth)
		if !ok {
			return
		}
		var before int64
		limit := 50
		if value := r.URL.Query().Get("cursor"); value != "" {
			id, err := strconv.ParseInt(value, 10, 64)
			if err != nil || id <= 0 {
				writeAPIError(w, http.StatusBadRequest, "VALIDATION_FAILED", "split cursor is invalid")
				return
			}
			before = id
		}
		if value := r.URL.Query().Get("limit"); value != "" {
			n, err := strconv.Atoi(value)
			if err != nil || n < 1 || n > 100 {
				writeAPIError(w, http.StatusBadRequest, "VALIDATION_FAILED", "split page size is invalid")
				return
			}
			limit = n
		}
		var records []db.SplitActivityRecord
		var next int64
		var err error
		if value := r.URL.Query().Get("operation_id"); value != "" {
			id, parseErr := strconv.ParseInt(value, 10, 64)
			if parseErr != nil || id <= 0 {
				writeAPIError(w, http.StatusBadRequest, "VALIDATION_FAILED", "split operation id is invalid")
				return
			}
			records, err = service.SplitActivityForOperation(r.Context(), owner.ID, id)
		} else {
			records, next, err = service.SplitActivity(r.Context(), owner.ID, before, limit)
		}
		if err != nil {
			writeInvestmentServiceError(w, r, logger, "read split activity", err)
			return
		}
		response := splitActivityPageResponse{Splits: make([]splitActivityResponse, 0, len(records))}
		if next > 0 {
			cursor := strconv.FormatInt(next, 10)
			response.NextCursor = &cursor
		}
		for _, record := range records {
			response.Splits = append(response.Splits, splitActivityResponse{
				OperationID: record.OperationID, TransactionID: splitTransactionRef(record.TransactionID), CorrectionOfOperationID: splitTransactionRef(record.CorrectionOfOperationID), SuccessorOperationID: splitTransactionRef(record.SuccessorOperationID),
				HoldingAccountID: record.AccountID, CommodityID: record.CommodityID, AccountName: record.AccountName, InstrumentName: record.InstrumentName, EffectiveOn: record.EffectiveOn, RatioNumerator: record.RatioNumerator, RatioDenominator: record.RatioDenominator,
				SourceEvidence: json.RawMessage(record.SourceEvidenceJSON), Memo: record.Memo, Reason: record.Reason, CreatedAt: record.CreatedAt, Standing: record.Standing, Settled: record.Settled, QuantityDeltaValue: record.JournalDeltaValue, QuantityDeltaScale: record.JournalDeltaScale})
		}
		writeJSON(w, http.StatusOK, response)
	}
}
