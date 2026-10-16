package api

import (
	"log/slog"
	"net/http"

	"rekenraam/backend/internal/app"
)

type gainsStatementValuationResponse struct {
	ValuationAsOf       string            `json:"valuation_as_of"`
	UnrealizedGainValue *moneyCoefficient `json:"unrealized_gain_value"`
	UnrealizedGainScale *int              `json:"unrealized_gain_scale"`
	PositionCount       int               `json:"position_count"`
	UnpricedCount       int               `json:"unpriced_count"`
	UnresolvedCount     int               `json:"unresolved_count"`
	StaleCount          int               `json:"stale_count"`
	UnavailableReason   *string           `json:"unavailable_reason"`
	EarliestPriceDate   *string           `json:"earliest_price_date"`
	LatestPriceDate     *string           `json:"latest_price_date"`
}

type gainsStatementRowResponse struct {
	CapitalReturn           app.GainsStatementCapitalReturn         `json:"capital_return"`
	UnclassifiedDividends   app.GainsStatementUnclassifiedDividends `json:"unclassified_dividends"`
	CommodityID             int64                                   `json:"commodity_id"`
	Opening                 gainsStatementValuationResponse         `json:"opening"`
	Closing                 gainsStatementValuationResponse         `json:"closing"`
	RealizedGainValue       *moneyCoefficient                       `json:"realized_gain_value"`
	RealizedGainScale       *int                                    `json:"realized_gain_scale"`
	DisposalCount           int                                     `json:"disposal_count"`
	RealizedUnresolvedCount int                                     `json:"realized_unresolved_count"`
	IncomeValue             moneyCoefficient                        `json:"income_value"`
	IncomeScale             int                                     `json:"income_scale"`
	IncomeCount             int                                     `json:"income_count"`
	UnrealizedChangeValue   *moneyCoefficient                       `json:"unrealized_change_value"`
	UnrealizedChangeScale   *int                                    `json:"unrealized_change_scale"`
	TotalValue              *moneyCoefficient                       `json:"total_value"`
	TotalScale              *int                                    `json:"total_scale"`
}

type investmentGainsStatementResponse struct {
	CapitalReturns []capitalReturnReadResponse `json:"capital_returns"`
	Reporting      *gainsFXStatementResponse   `json:"reporting,omitempty"`
	Profile        gainsProfileResponse        `json:"profile"`
	From           string                      `json:"from"`
	To             string                      `json:"to"`
	Currencies     []currencyResponse          `json:"currencies"`
	Rows           []gainsStatementRowResponse `json:"rows"`
}

func statementAmountFields(amount *app.StatementAmount) (*moneyCoefficient, *int) {
	if amount == nil {
		return nil, nil
	}
	value, scale := moneyCoefficient(amount.Value), amount.Scale
	return &value, &scale
}

func optionalText(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

func toGainsStatementValuationResponse(valuation app.GainsStatementValuation) gainsStatementValuationResponse {
	value, scale := statementAmountFields(valuation.Gain)
	return gainsStatementValuationResponse{
		ValuationAsOf: valuation.ValuationAsOf, UnrealizedGainValue: value, UnrealizedGainScale: scale,
		PositionCount: valuation.PositionCount, UnpricedCount: valuation.UnpricedCount,
		UnresolvedCount: valuation.UnresolvedCount, StaleCount: valuation.StaleCount,
		UnavailableReason: optionalText(valuation.UnavailableReason),
		EarliestPriceDate: optionalText(valuation.EarliestPriceDate), LatestPriceDate: optionalText(valuation.LatestPriceDate),
	}
}

// investmentGainsStatement serves the R18 period statement (#231) as one
// backend-composed read.
func investmentGainsStatement(logger *slog.Logger, authService *app.AuthService, investmentService *app.InvestmentService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := authenticatedOwner(w, r, logger, authService); !ok {
			return
		}
		query := r.URL.Query()
		fx, err := gainsFXInput(query)
		if err != nil {
			writeInvestmentServiceError(w, r, logger, "statement FX input", err)
			return
		}
		read, err := investmentService.ReadGainsStatement(r.Context(), query.Get("from"), query.Get("to"), fx)
		if err != nil {
			writeInvestmentServiceError(w, r, logger, "investment gains statement", err)
			return
		}
		var reporting *gainsFXStatementResponse
		if converted := read.Reporting; converted != nil {
			reporting = &gainsFXStatementResponse{Profile: toGainsProfileResponse(converted.Profile), Opening: converted.Opening, Realized: converted.Realized, Income: converted.Income, Closing: converted.Closing, UnrealizedChange: converted.UnrealizedChange, Total: converted.Total, CapitalReturnReceipt: converted.CapitalReturnReceipt, CapitalReturnReduction: converted.CapitalReturnReduction, CapitalReturnExcess: converted.CapitalReturnExcess}
		}
		response := gainsStatementResponse(read.Statement, read.Currencies)
		response.Reporting = reporting
		writeJSON(w, http.StatusOK, response)
	}
}

func gainsStatementResponse(statement app.GainsStatement, currencies []app.Currency) investmentGainsStatementResponse {
	rows := make([]gainsStatementRowResponse, 0, len(statement.Rows))
	for _, row := range statement.Rows {
		realizedValue, realizedScale := statementAmountFields(row.Realized)
		changeValue, changeScale := statementAmountFields(row.UnrealizedChange)
		totalValue, totalScale := statementAmountFields(row.Total)
		rows = append(rows, gainsStatementRowResponse{
			CommodityID: row.CommodityID, CapitalReturn: row.CapitalReturn, UnclassifiedDividends: row.UnclassifiedDividends,
			Opening:           toGainsStatementValuationResponse(row.Opening),
			Closing:           toGainsStatementValuationResponse(row.Closing),
			RealizedGainValue: realizedValue, RealizedGainScale: realizedScale,
			DisposalCount: row.DisposalCount, RealizedUnresolvedCount: row.RealizedUnresolvedCount,
			IncomeValue: moneyCoefficient(row.Income.Value), IncomeScale: row.Income.Scale, IncomeCount: row.IncomeCount,
			UnrealizedChangeValue: changeValue, UnrealizedChangeScale: changeScale,
			TotalValue: totalValue, TotalScale: totalScale,
		})
	}
	return investmentGainsStatementResponse{CapitalReturns: toCapitalReturnReads(statement.CapitalReturns), Profile: toGainsProfileResponse(statement.Profile), From: statement.From, To: statement.To, Currencies: toCurrencyResponses(currencies), Rows: rows}
}
