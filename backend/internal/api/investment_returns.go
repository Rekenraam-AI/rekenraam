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

type returnsSeriesPointResponse struct {
	Date         string               `json:"date"`
	TimeWeighted returnFigureResponse `json:"time_weighted"`
}

type returnsSeriesResponse struct {
	Interval  string                       `json:"interval"`
	Points    []returnsSeriesPointResponse `json:"points"`
	Valuation *valuationResponse           `json:"valuation"`
}

type returnsBenchmarkResponse struct {
	CommodityID            int64                        `json:"commodity_id"`
	Basis                  string                       `json:"basis"`
	TimeWeighted           returnFigureResponse         `json:"time_weighted"`
	TimeWeightedAnnualized returnFigureResponse         `json:"time_weighted_annualized"`
	Points                 []returnsSeriesPointResponse `json:"points"`
	Splits                 []returnsSplitResponse       `json:"splits"`
	Valuation              *valuationResponse           `json:"valuation"`
}

type returnsSplitResponse struct {
	EffectiveOn      string `json:"effective_on"`
	RatioNumerator   int64  `json:"ratio_numerator"`
	RatioDenominator int64  `json:"ratio_denominator"`
}

type returnsAllocationRowResponse struct {
	AccountID  *int64                   `json:"account_id"`
	AssetType  *string                  `json:"asset_type"`
	CurrencyID *int64                   `json:"currency_id"`
	Side       string                   `json:"side"`
	Value      *balanceQuantityResponse `json:"value"`
	Weight     *string                  `json:"weight"`
}

type returnsAllocationUnknownResponse struct {
	AccountID int64                   `json:"account_id"`
	Quantity  balanceQuantityResponse `json:"quantity"`
}

type returnsAllocationResponse struct {
	Date          string                             `json:"date"`
	Reason        *string                            `json:"reason"`
	Basis         *string                            `json:"basis"`
	Denominator   *balanceQuantityResponse           `json:"denominator"`
	WeightsReason *string                            `json:"weights_reason"`
	ByAccount     []returnsAllocationRowResponse     `json:"by_account"`
	ByAssetType   []returnsAllocationRowResponse     `json:"by_asset_type"`
	ByCurrency    []returnsAllocationRowResponse     `json:"by_currency"`
	Unknown       []returnsAllocationUnknownResponse `json:"unknown"`
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
	TimeWeightedSeries     returnsSeriesResponse     `json:"time_weighted_series"`
	Allocation             returnsAllocationResponse `json:"allocation"`
	Benchmark              *returnsBenchmarkResponse `json:"benchmark"`
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
		if input.BenchmarkCommodityID, err = optionalPositiveInt64(query.Get("benchmark_commodity_id")); err != nil {
			writeAPIError(w, http.StatusBadRequest, "VALIDATION_FAILED", "benchmark_commodity_id must be a positive integer")
			return
		}
		input.BenchmarkBasis = strings.TrimSpace(query.Get("benchmark_basis"))
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
	points := toReturnsSeriesPointsResponse(result.TimeWeightedSeries.Points)
	var benchmark *returnsBenchmarkResponse
	if result.Benchmark != nil {
		splits := make([]returnsSplitResponse, 0, len(result.Benchmark.Splits))
		for _, split := range result.Benchmark.Splits {
			splits = append(splits, returnsSplitResponse(split))
		}
		benchmark = &returnsBenchmarkResponse{Splits: splits, CommodityID: result.Benchmark.CommodityID, Basis: result.Benchmark.Basis,
			TimeWeighted:           toReturnFigureResponse(result.Benchmark.TimeWeighted),
			TimeWeightedAnnualized: toReturnFigureResponse(result.Benchmark.TimeWeightedAnnualized),
			Points:                 toReturnsSeriesPointsResponse(result.Benchmark.Points),
			Valuation:              toValuationResponse(&result.Benchmark.Valuation)}
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
		TimeWeightedSeries: returnsSeriesResponse{Interval: result.TimeWeightedSeries.Interval, Points: points,
			Valuation: toValuationResponse(&result.TimeWeightedSeries.Valuation)},
		Allocation:           toReturnsAllocationResponse(result.Allocation),
		Benchmark:            benchmark,
		AttributionFallbacks: fallbacks, Valuation: toValuationResponse(&result.Valuation),
	}
}

func toReturnsSeriesPointsResponse(points []app.ReturnsSeriesPoint) []returnsSeriesPointResponse {
	response := make([]returnsSeriesPointResponse, 0, len(points))
	for _, point := range points {
		response = append(response, returnsSeriesPointResponse{Date: point.Date, TimeWeighted: toReturnFigureResponse(point.TimeWeighted)})
	}
	return response
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

func toReturnsAllocationResponse(allocation app.ReturnsAllocation) returnsAllocationResponse {
	rows := func(rows []app.ReturnsAllocationRow) []returnsAllocationRowResponse {
		response := make([]returnsAllocationRowResponse, 0, len(rows))
		for _, row := range rows {
			item := returnsAllocationRowResponse{AssetType: optionalText(row.AssetType), Side: row.Side, Weight: row.Weight,
				Value: optionalBalanceQuantity(row.Value)}
			if row.AccountID != 0 {
				item.AccountID = &row.AccountID
			}
			// A currency row with no id is a security without a quote currency.
			if row.AssetType == "" && row.AccountID == 0 && row.CurrencyID != 0 {
				item.CurrencyID = &row.CurrencyID
			}
			response = append(response, item)
		}
		return response
	}
	unknown := make([]returnsAllocationUnknownResponse, 0, len(allocation.Unknown))
	for _, item := range allocation.Unknown {
		unknown = append(unknown, returnsAllocationUnknownResponse{AccountID: item.AccountID, Quantity: toBalanceQuantityResponse(item.Quantity)})
	}
	return returnsAllocationResponse{
		Date: allocation.Date, Reason: optionalText(allocation.Reason), Basis: optionalText(allocation.Basis),
		Denominator: optionalBalanceQuantity(allocation.Denominator), WeightsReason: optionalText(allocation.WeightsReason),
		ByAccount: rows(allocation.ByAccount), ByAssetType: rows(allocation.ByAssetType), ByCurrency: rows(allocation.ByCurrency),
		Unknown: unknown,
	}
}

func optionalBalanceQuantity(quantity *app.BalanceQuantity) *balanceQuantityResponse {
	if quantity == nil {
		return nil
	}
	response := toBalanceQuantityResponse(*quantity)
	return &response
}
