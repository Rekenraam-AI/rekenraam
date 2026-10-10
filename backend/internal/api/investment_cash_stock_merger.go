package api

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"rekenraam/backend/internal/app"
	"rekenraam/backend/internal/exact"
)

// Cash-and-stock merger (#181): a share exchange that also pays cash, with an
// explicit election of the basis share the cash recovers.
type cashStockMergerRequest struct {
	EffectiveOn               string            `json:"effective_on"`
	PayableOn                 string            `json:"payable_on"`
	HoldingAccountID          int64             `json:"holding_account_id"`
	DestinationHoldingID      int64             `json:"destination_holding_account_id,omitempty"`
	CommodityID               int64             `json:"commodity_id"`
	DestinationCommodityID    int64             `json:"destination_commodity_id"`
	RatioNumerator            int64             `json:"ratio_numerator"`
	RatioDenominator          int64             `json:"ratio_denominator"`
	CashAccountID             int64             `json:"cash_account_id"`
	CurrencyID                int64             `json:"currency_id"`
	CashValue                 moneyCoefficient  `json:"cash_value"`
	CashScale                 int               `json:"cash_scale"`
	CashBasisFractionValue    exact.Coefficient `json:"cash_basis_fraction_value"`
	CashBasisFractionScale    int               `json:"cash_basis_fraction_scale"`
	SourceEvidence            json.RawMessage   `json:"source_evidence,omitempty"`
	Memo                      string            `json:"memo"`
	ChangeReason              string            `json:"change_reason"`
	ReconciliationOverride    bool              `json:"reconciliation_override"`
	GainImpactAcknowledgement string            `json:"gain_impact_acknowledgement,omitempty"`
}

type cashStockMergerLinkResponse struct {
	shareExchangeLinkResponse
	RealizedBasisValue *string `json:"realized_basis_value"`
	RealizedBasisScale *int    `json:"realized_basis_scale"`
	CashValue          string  `json:"cash_value"`
	CashScale          int     `json:"cash_scale"`
}

// cashStockMergerTotalResponse is one cost currency's units, carried and
// realized basis, cash and result; the bases and result are null when any lot
// in it is unknown.
type cashStockMergerTotalResponse struct {
	shareExchangeBasisTotalResponse
	CashValue          exact.Coefficient `json:"cash_value"`
	CashScale          int               `json:"cash_scale"`
	RealizedBasisValue *string           `json:"realized_basis_value"`
	RealizedBasisScale *int              `json:"realized_basis_scale"`
	ResultValue        *string           `json:"result_value"`
	ResultScale        *int              `json:"result_scale"`
}

type cashStockMergerPlanResponse struct {
	RatioNumerator           int64                          `json:"ratio_numerator"`
	RatioDenominator         int64                          `json:"ratio_denominator"`
	CashBasisFractionValue   exact.Coefficient              `json:"cash_basis_fraction_value"`
	CashBasisFractionScale   int                            `json:"cash_basis_fraction_scale"`
	Links                    []cashStockMergerLinkResponse  `json:"links"`
	SourceQuantityValue      exact.Coefficient              `json:"source_quantity_value"`
	SourceQuantityScale      int                            `json:"source_quantity_scale"`
	DestinationQuantityValue exact.Coefficient              `json:"destination_quantity_value"`
	DestinationQuantityScale int                            `json:"destination_quantity_scale"`
	Totals                   []cashStockMergerTotalResponse `json:"totals"`
}

type cashStockMergerPreviewResponse struct {
	Plan   cashStockMergerPlanResponse  `json:"plan"`
	Impact reconciliationImpactResponse `json:"impact"`
}

type cashStockMergerResponse struct {
	Transaction transactionResponse         `json:"transaction"`
	Plan        cashStockMergerPlanResponse `json:"plan"`
}

func cashStockMergerInput(owner app.Owner, r *http.Request, request cashStockMergerRequest) app.CashStockMergerInput {
	evidence := rawJSONText(request.SourceEvidence)
	if evidence == "null" {
		evidence = ""
	}
	return app.CashStockMergerInput{
		OwnerUserID: owner.ID, AuthSessionID: authenticatedSessionID(r),
		RequestID: RequestIDFromContext(r.Context()), EffectiveOn: request.EffectiveOn, PayableOn: request.PayableOn,
		HoldingAccountID: request.HoldingAccountID, DestinationHoldingAccountID: request.DestinationHoldingID,
		CommodityID: request.CommodityID, DestinationCommodityID: request.DestinationCommodityID,
		RatioNumerator: request.RatioNumerator, RatioDenominator: request.RatioDenominator,
		CashAccountID: request.CashAccountID, CurrencyID: request.CurrencyID,
		CashValue: int64(request.CashValue), CashScale: request.CashScale,
		CashBasisFractionValue: request.CashBasisFractionValue, CashBasisFractionScale: request.CashBasisFractionScale,
		SourceEvidenceJSON: evidence, Memo: request.Memo, ChangeReason: request.ChangeReason,
		ReconciliationOverride:    request.ReconciliationOverride,
		GainImpactAcknowledgement: request.GainImpactAcknowledgement,
	}
}

func mergerKnownAmount(value exact.Coefficient, scale int, known bool) (*string, *int) {
	if !known {
		return nil, nil
	}
	text := value.String()
	return &text, &scale
}

func toCashStockMergerPlanResponse(plan app.CashStockMergerPlan) cashStockMergerPlanResponse {
	exchangeLinks := make([]app.ShareExchangeLink, 0, len(plan.Links))
	for _, link := range plan.Links {
		exchangeLinks = append(exchangeLinks, link.ShareExchangeLink)
	}
	exchangeTotals := make([]app.ShareExchangeBasisTotal, 0, len(plan.Totals))
	for _, total := range plan.Totals {
		exchangeTotals = append(exchangeTotals, total.ShareExchangeBasisTotal)
	}
	exchange := toShareExchangePlanResponse(app.ShareExchangePlan{Links: exchangeLinks, BasisTotals: exchangeTotals})
	out := cashStockMergerPlanResponse{RatioNumerator: plan.RatioNumerator, RatioDenominator: plan.RatioDenominator,
		CashBasisFractionValue: plan.CashBasisFractionValue, CashBasisFractionScale: plan.CashBasisFractionScale,
		SourceQuantityValue: plan.SourceQuantityValue, SourceQuantityScale: plan.SourceQuantityScale,
		DestinationQuantityValue: plan.DestinationQuantityValue, DestinationQuantityScale: plan.DestinationQuantityScale,
		Links: make([]cashStockMergerLinkResponse, 0, len(plan.Links)), Totals: make([]cashStockMergerTotalResponse, 0, len(plan.Totals))}
	for index, link := range plan.Links {
		response := cashStockMergerLinkResponse{shareExchangeLinkResponse: exchange.Links[index],
			CashValue: exact.New(link.CashValue).String(), CashScale: link.CashScale}
		response.RealizedBasisValue, response.RealizedBasisScale = mergerKnownAmount(exact.New(link.RealizedBasisValue),
			link.RealizedBasisScale, link.BasisKnowledge != "unknown")
		out.Links = append(out.Links, response)
	}
	for index, total := range plan.Totals {
		known := total.BasisKnowledge != "unknown"
		response := cashStockMergerTotalResponse{shareExchangeBasisTotalResponse: exchange.BasisTotals[index],
			CashValue: total.CashValue, CashScale: total.CashScale}
		response.RealizedBasisValue, response.RealizedBasisScale = mergerKnownAmount(total.RealizedBasisValue, total.RealizedBasisScale, known)
		response.ResultValue, response.ResultScale = mergerKnownAmount(total.ResultValue, total.ResultScale, known)
		out.Totals = append(out.Totals, response)
	}
	return out
}

func cashStockMerger(logger *slog.Logger, authService *app.AuthService, investmentService *app.InvestmentService, options HandlerOptions) http.HandlerFunc {
	return requireAuthenticatedMutation(logger, authService, options, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		owner, ok := authenticatedMutationOwner(w, r)
		if !ok {
			return
		}
		var request cashStockMergerRequest
		if err := decodeJSONBody(r, &request); err != nil {
			writeDecodeError(w, err)
			return
		}
		result, err := investmentService.CashStockMerger(r.Context(), cashStockMergerInput(owner, r, request))
		if err != nil {
			writeInvestmentServiceError(w, r, logger, "cash-and-stock merger", err)
			return
		}
		writeJSON(w, http.StatusCreated, cashStockMergerResponse{
			Transaction: toTransactionResponse(result.Transaction), Plan: toCashStockMergerPlanResponse(result.Plan),
		})
	}))
}

func cashStockMergerPreview(logger *slog.Logger, authService *app.AuthService, investmentService *app.InvestmentService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		owner, ok := authenticatedOwner(w, r, logger, authService)
		if !ok {
			return
		}
		var request cashStockMergerRequest
		if err := decodeJSONBody(r, &request); err != nil {
			writeDecodeError(w, err)
			return
		}
		preview, err := investmentService.PreviewCashStockMerger(r.Context(), cashStockMergerInput(owner, r, request))
		if err != nil {
			writeInvestmentServiceError(w, r, logger, "cash-and-stock merger preview", err)
			return
		}
		impact := toReconciliationImpactResponse(preview.Impact)
		gainImpact, err := toGainImpactResponse(preview.Impact.GainImpact)
		if err != nil {
			writeAPIError(w, http.StatusUnprocessableEntity, "LEDGER_OVERFLOW", "gain impact value exceeds the coefficient range")
			return
		}
		impact.GainImpact = gainImpact
		writeJSON(w, http.StatusOK, cashStockMergerPreviewResponse{
			Plan: toCashStockMergerPlanResponse(preview.Plan), Impact: impact,
		})
	}
}
