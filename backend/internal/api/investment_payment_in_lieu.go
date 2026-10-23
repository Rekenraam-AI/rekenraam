package api

import (
	"log/slog"
	"net/http"

	"rekenraam/backend/internal/app"
)

// #255: a short holder's payment in lieu of a dividend, its reconciliation
// preview and its native reversal and replacement. Reversals reuse the
// trade-reversal request and response shapes.

type paymentInLieuRequest struct {
	HoldingAccountID  nullableField[int64]  `json:"holding_account_id,omitzero"`
	CommodityID       nullableField[int64]  `json:"commodity_id,omitzero"`
	ExDate            nullableField[string] `json:"ex_date,omitzero"`
	TransactionDate   string                `json:"transaction_date"`
	CashAccountID     int64                 `json:"cash_account_id"`
	CashCommodityID   int64                 `json:"cash_commodity_id"`
	ExpenseAccountID  int64                 `json:"expense_account_id"`
	AmountValue       moneyCoefficient      `json:"amount_value"`
	AmountScale       int                   `json:"amount_scale"`
	RecoveryValue     *moneyCoefficient     `json:"recovery_value"`
	RecoveryScale     *int                  `json:"recovery_scale"`
	RecoveryAccountID *int64                `json:"recovery_account_id"`
	Memo              string                `json:"memo"`
	PayeeID           *int64                `json:"payee_id"`
	ChangeReason      string                `json:"change_reason"`
	reconciliationOverrideFields
}

type investmentPaymentInLieuReplacementRequest struct {
	Reason string `json:"reason"`
	reconciliationOverrideFields
	Replacement paymentInLieuRequest `json:"replacement"`
}

func toPaymentInLieuInput(owner app.Owner, r *http.Request, request paymentInLieuRequest) app.PaymentInLieuInput {
	return app.PaymentInLieuInput{
		OwnerUserID: owner.ID, AuthSessionID: authenticatedSessionID(r), RequestID: RequestIDFromContext(r.Context()),
		HoldingAccountID: request.HoldingAccountID.NullablePatch, CommodityID: request.CommodityID.NullablePatch, ExDate: request.ExDate.NullablePatch,
		TransactionDate: request.TransactionDate, CashAccountID: request.CashAccountID, CashCommodityID: request.CashCommodityID,
		ExpenseAccountID: request.ExpenseAccountID, AmountValue: int64(request.AmountValue), AmountScale: request.AmountScale,
		RecoveryValue: moneyInt64Pointer(request.RecoveryValue), RecoveryScale: request.RecoveryScale,
		RecoveryAccountID: request.RecoveryAccountID, Memo: request.Memo, PayeeID: request.PayeeID,
		ChangeReason: request.ChangeReason, ReconciliationOverride: request.checkpointOverride(),
	}
}

func createPaymentInLieu(logger *slog.Logger, authService *app.AuthService, investmentService *app.InvestmentService, options HandlerOptions) http.HandlerFunc {
	return requireAuthenticatedMutation(logger, authService, options, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		owner, ok := authenticatedMutationOwner(w, r)
		if !ok {
			return
		}
		var request paymentInLieuRequest
		if err := decodeJSONBody(r, &request); err != nil {
			writeDecodeError(w, err)
			return
		}
		transaction, err := investmentService.PaymentInLieu(r.Context(), toPaymentInLieuInput(owner, r, request))
		if err != nil {
			writeInvestmentServiceError(w, r, logger, "create payment in lieu", err)
			return
		}
		writeJSON(w, http.StatusCreated, toTransactionResponse(transaction))
	}))
}

func paymentInLieuReconciliationImpact(logger *slog.Logger, authService *app.AuthService, investmentService *app.InvestmentService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		owner, ok := authenticatedOwner(w, r, logger, authService)
		if !ok {
			return
		}
		var request paymentInLieuRequest
		if err := decodeJSONBody(r, &request); err != nil {
			writeDecodeError(w, err)
			return
		}
		impact, err := investmentService.PaymentInLieuReconciliationImpact(r.Context(), toPaymentInLieuInput(owner, r, request))
		if err != nil {
			writeInvestmentServiceError(w, r, logger, "payment in lieu reconciliation impact", err)
			return
		}
		writeReconciliationImpact(w, impact)
	}
}

func reverseInvestmentPaymentInLieu(logger *slog.Logger, authService *app.AuthService, investmentService *app.InvestmentService, options HandlerOptions) http.HandlerFunc {
	return requireAuthenticatedMutation(logger, authService, options, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		owner, ok := authenticatedMutationOwner(w, r)
		if !ok {
			return
		}
		transactionID, request, ok := correctionRoute[investmentSaleReversalRequest](w, r)
		if !ok {
			return
		}
		transaction, err := investmentService.ReversePaymentInLieu(r.Context(), app.ReverseInvestmentPaymentInLieuInput{
			OwnerUserID: owner.ID, AuthSessionID: authenticatedSessionID(r), RequestID: RequestIDFromContext(r.Context()),
			TransactionID: transactionID, Reason: request.Reason, ReconciliationOverride: request.checkpointOverride(),
		})
		if err != nil {
			writeInvestmentServiceError(w, r, logger, "reverse payment in lieu", err)
			return
		}
		writeJSON(w, http.StatusCreated, investmentSaleReversalResponse{Transaction: toTransactionResponse(transaction), CorrectedTransactionID: transactionID})
	}))
}

func reverseInvestmentPaymentInLieuReconciliationImpact(logger *slog.Logger, authService *app.AuthService, investmentService *app.InvestmentService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		owner, ok := authenticatedOwner(w, r, logger, authService)
		if !ok {
			return
		}
		transactionID, request, ok := correctionRoute[investmentSaleReversalRequest](w, r)
		if !ok {
			return
		}
		impact, err := investmentService.ReversePaymentInLieuReconciliationImpact(r.Context(), app.ReverseInvestmentPaymentInLieuInput{
			OwnerUserID: owner.ID, TransactionID: transactionID, Reason: request.Reason,
		})
		if err != nil {
			writeInvestmentServiceError(w, r, logger, "preview payment in lieu reversal reconciliation impact", err)
			return
		}
		writeReconciliationImpact(w, impact)
	}
}

func replaceInvestmentPaymentInLieu(logger *slog.Logger, authService *app.AuthService, investmentService *app.InvestmentService, options HandlerOptions) http.HandlerFunc {
	return requireAuthenticatedMutation(logger, authService, options, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		owner, ok := authenticatedMutationOwner(w, r)
		if !ok {
			return
		}
		transactionID, request, ok := correctionRoute[investmentPaymentInLieuReplacementRequest](w, r)
		if !ok {
			return
		}
		result, err := investmentService.ReplacePaymentInLieu(r.Context(), app.ReplaceInvestmentPaymentInLieuInput{
			OwnerUserID: owner.ID, AuthSessionID: authenticatedSessionID(r), RequestID: RequestIDFromContext(r.Context()),
			TransactionID: transactionID, Reason: request.Reason, ReconciliationOverride: request.checkpointOverride(),
			Replacement: toPaymentInLieuInput(owner, r, request.Replacement),
		})
		if err != nil {
			writeInvestmentServiceError(w, r, logger, "replace payment in lieu", err)
			return
		}
		writeJSON(w, http.StatusCreated, investmentDividendReplacementResponse{
			InverseTransaction:     toTransactionResponse(result.Inverse),
			ReplacementTransaction: toTransactionResponse(result.Replacement),
			CorrectedTransactionID: transactionID,
		})
	}))
}

func replaceInvestmentPaymentInLieuReconciliationImpact(logger *slog.Logger, authService *app.AuthService, investmentService *app.InvestmentService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		owner, ok := authenticatedOwner(w, r, logger, authService)
		if !ok {
			return
		}
		transactionID, request, ok := correctionRoute[investmentPaymentInLieuReplacementRequest](w, r)
		if !ok {
			return
		}
		impact, err := investmentService.ReplacePaymentInLieuReconciliationImpact(r.Context(), app.ReplaceInvestmentPaymentInLieuInput{
			OwnerUserID: owner.ID, TransactionID: transactionID, Reason: request.Reason,
			Replacement: toPaymentInLieuInput(owner, r, request.Replacement),
		})
		if err != nil {
			writeInvestmentServiceError(w, r, logger, "preview payment in lieu replacement reconciliation impact", err)
			return
		}
		writeReconciliationImpact(w, impact)
	}
}
