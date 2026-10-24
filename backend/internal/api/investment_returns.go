package api

import (
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"rekenraam/backend/internal/app"
)

// R13 returns_v1 live read (docs/plans/investment-returns-plan.md).

type returnsAmountResponse struct {
	Amount *balanceQuantityResponse `json:"amount"`
	Reason *string                  `json:"reason"`
	Date   *string                  `json:"date"`
}

type returnFigureResponse struct {
	Value  *string `json:"value"`
	Reason *string `json:"reason"`
	Date   *string `json:"date"`
}

type returnsFallbackResponse struct {
	TransactionID int64  `json:"transaction_id"`
	OperationID   int64  `json:"operation_id"`
	OperationKind string `json:"operation_kind"`
	Date          string `json:"date"`
	Reason        string `json:"reason"`
	Counted       bool   `json:"counted"`
}

type investmentReturnsResponse struct {
	Policy                 string                    `json:"policy"`
	From                   string                    `json:"from"`
	To                     string                    `json:"to"`
	OpeningDate            string                    `json:"opening_date"`
	Days                   int                       `json:"days"`
	AccountScope           string                    `json:"account_scope"`
	ResolvedAccountIDs     []int64                   `json:"resolved_account_ids"`
	KnowledgeCutoff        string                    `json:"knowledge_cutoff"`
	OpeningValue           returnsAmountResponse     `json:"opening_value"`
	ClosingValue           returnsAmountResponse     `json:"closing_value"`
	Contributions          returnsAmountResponse     `json:"contributions"`
	Withdrawals            returnsAmountResponse     `json:"withdrawals"`
	Gain                   returnsAmountResponse     `json:"gain"`
	TimeWeighted           returnFigureResponse      `json:"time_weighted"`
	TimeWeightedAnnualized returnFigureResponse      `json:"time_weighted_annualized"`
	MoneyWeighted          returnFigureResponse      `json:"money_weighted"`
	MoneyWeightedPeriod    returnFigureResponse      `json:"money_weighted_period"`
	AttributionFallbacks   []returnsFallbackResponse `json:"attribution_fallbacks"`
	Valuation              *valuationResponse        `json:"valuation"`
}

func investmentReturns(logger *slog.Logger, authService *app.AuthService, investmentService *app.InvestmentService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := authenticatedOwner(w, r, logger, authService); !ok {
			return
		}
		query := r.URL.Query()
		input := app.ReturnsInput{From: query.Get("from"), To: query.Get("to")}
		var err error
		if input.ReportingCommodityID, err = optionalPositiveInt64(query.Get("reporting_currency_id")); err != nil {
			writeAPIError(w, http.StatusBadRequest, "VALIDATION_FAILED", "reporting_currency_id must be a positive integer")
			return
		}
		if raw := strings.TrimSpace(query.Get("max_staleness_days")); raw != "" {
			if input.MaxStalenessDays, err = strconv.Atoi(raw); err != nil || input.MaxStalenessDays <= 0 {
				writeAPIError(w, http.StatusBadRequest, "VALIDATION_FAILED", "max_staleness_days must be a positive integer")
				return
			}
		}
		if input.AccountIDs, err = parseRepeatedIDs(query, "account_id"); err != nil {
			writeAPIError(w, http.StatusBadRequest, "VALIDATION_FAILED", "account_id must be a positive integer")
			return
		}
		result, err := investmentService.ReadReturns(r.Context(), input)
		if err != nil {
			writeInvestmentServiceError(w, r, logger, "investment returns", err)
			return
		}
		writeJSON(w, http.StatusOK, toInvestmentReturnsResponse(result))
	}
}

func optionalPositiveInt64(raw string) (int64, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, nil
	}
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || value <= 0 {
		return 0, strconv.ErrSyntax
	}
	return value, nil
}

func toInvestmentReturnsResponse(result app.ReturnsResult) investmentReturnsResponse {
	fallbacks := make([]returnsFallbackResponse, 0, len(result.AttributionFallbacks))
	for _, fallback := range result.AttributionFallbacks {
		fallbacks = append(fallbacks, returnsFallbackResponse(fallback))
	}
	accountIDs := result.ResolvedAccountIDs
	if accountIDs == nil {
		accountIDs = []int64{}
	}
	return investmentReturnsResponse{
		Policy: result.Policy, From: result.From, To: result.To, OpeningDate: result.OpeningDate, Days: result.Days,
		AccountScope: result.AccountScope, ResolvedAccountIDs: accountIDs, KnowledgeCutoff: result.KnowledgeCutoff,
		OpeningValue: toReturnsAmountResponse(result.OpeningValue), ClosingValue: toReturnsAmountResponse(result.ClosingValue),
		Contributions: toReturnsAmountResponse(result.Contributions), Withdrawals: toReturnsAmountResponse(result.Withdrawals),
		Gain:         toReturnsAmountResponse(result.Gain),
		TimeWeighted: toReturnFigureResponse(result.TimeWeighted), TimeWeightedAnnualized: toReturnFigureResponse(result.TimeWeightedAnnualized),
		MoneyWeighted: toReturnFigureResponse(result.MoneyWeighted), MoneyWeightedPeriod: toReturnFigureResponse(result.MoneyWeightedPeriod),
		AttributionFallbacks: fallbacks, Valuation: toValuationResponse(&result.Valuation),
	}
}

func toReturnsAmountResponse(amount app.ReturnsAmount) returnsAmountResponse {
	response := returnsAmountResponse{Reason: optionalText(amount.Reason), Date: optionalText(amount.Date)}
	if amount.Amount != nil {
		quantity := toBalanceQuantityResponse(*amount.Amount)
		response.Amount = &quantity
	}
	return response
}

func toReturnFigureResponse(figure app.ReturnFigure) returnFigureResponse {
	return returnFigureResponse{Value: figure.Value, Reason: optionalText(figure.Reason), Date: optionalText(figure.Date)}
}
