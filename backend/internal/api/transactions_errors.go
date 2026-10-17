package api

import (
	"errors"
	"log/slog"
	"net/http"

	"rekenraam/backend/internal/app"
)

func writeTransactionServiceError(w http.ResponseWriter, r *http.Request, logger *slog.Logger, action string, err error) {
	var validationError app.ValidationError
	var overflowError app.LedgerOverflowError
	var impactError app.CheckpointImpactError
	if errors.As(err, &impactError) {
		// A guard refusal names the set the write would invalidate now, so a
		// caller without a preview can show it and bind its override (#296).
		switch {
		case errors.Is(err, app.ErrReconciliationImpactStale):
			writeCheckpointRefusal(w, "RECONCILIATION_IMPACT_STALE", "the checkpoints this write would invalidate changed after they were reviewed; review them again", impactError.CheckpointIDs)
			return
		case errors.Is(err, app.ErrReconciliationOverrideRequired):
			writeCheckpointRefusal(w, "CONFLICT", "reconciliation override is required", impactError.CheckpointIDs)
			return
		}
	}
	switch {
	case errors.Is(err, app.ErrTransactionDraftNotUserCreatable):
		writeAPIError(w, http.StatusBadRequest, "TRANSACTION_DRAFT_NOT_USER_CREATABLE", "draft transactions can only be created by a system producer")
	case errors.As(err, &validationError):
		writeAPIError(w, http.StatusBadRequest, "VALIDATION_FAILED", validationError.Error())
	case errors.As(err, &overflowError):
		writeAPIError(w, http.StatusUnprocessableEntity, "LEDGER_OVERFLOW", overflowError.Error())
	case errors.Is(err, app.ErrTransactionNotFound):
		writeAPIError(w, http.StatusNotFound, "NOT_FOUND", "transaction not found")
	case errors.Is(err, app.ErrTransactionProtected):
		writeAPIError(w, http.StatusConflict, "CONFLICT", "transaction requires a corrective workflow")
	case errors.Is(err, app.ErrTransactionPosted):
		writeAPIError(w, http.StatusConflict, "CONFLICT", "posted or voided transaction cannot be deleted")
	case errors.Is(err, app.ErrTransactionVoided):
		writeAPIError(w, http.StatusConflict, "CONFLICT", "voided transaction cannot be edited")
	case errors.Is(err, app.ErrTransactionVersionStale):
		writeAPIError(w, http.StatusConflict, "TRANSACTION_VERSION_STALE", "transaction changed after this edit was prepared; reload it and try again")
	case errors.Is(err, app.ErrPostingAccountVersionStale):
		writeAPIError(w, http.StatusConflict, "POSTING_ACCOUNT_VERSION_STALE", "a posting account changed after this write was prepared; reload it and try again")
	case errors.Is(err, app.ErrTransactionDeleted):
		writeAPIError(w, http.StatusConflict, "CONFLICT", "soft-deleted transaction must be restored first")
	case errors.Is(err, app.ErrTransactionDraftNotVoidable):
		writeAPIError(w, http.StatusConflict, "CONFLICT", "draft transaction cannot be voided; post or delete it instead")
	case errors.Is(err, app.ErrInvestmentWorkflowRequired):
		writeAPIError(w, http.StatusConflict, "INVESTMENT_WORKFLOW_REQUIRED", "investment-linked transaction requires an investment workflow")
	case errors.Is(err, app.ErrTransactionTag):
		writeAPIError(w, http.StatusConflict, "CONFLICT", "transaction tag is invalid")
	case errors.Is(err, app.ErrReconciliationOverrideRequired):
		writeAPIError(w, http.StatusConflict, "CONFLICT", "reconciliation override is required")
	case errors.Is(err, app.ErrReconciliationImpactStale):
		writeAPIError(w, http.StatusConflict, "RECONCILIATION_IMPACT_STALE", "the checkpoints this write would invalidate changed after they were reviewed; review them again")
	case errors.Is(err, app.ErrReconciliationNotFound):
		writeAPIError(w, http.StatusNotFound, "NOT_FOUND", "reconciliation not found")
	case errors.Is(err, app.ErrReconciliationClosed):
		writeAPIError(w, http.StatusConflict, "CONFLICT", "reconciliation is not open")
	case errors.Is(err, app.ErrReconciliationNotBalanced):
		writeAPIError(w, http.StatusConflict, "CONFLICT", "reconciliation difference must be zero")
	case errors.Is(err, app.ErrReconciliationStartStale):
		writeAPIError(w, http.StatusConflict, "RECONCILIATION_START_STALE", "reconciliation starting balance is no longer current; void it and start a new reconciliation")
	case errors.Is(err, app.ErrReconciliationPosting):
		writeAPIError(w, http.StatusConflict, "CONFLICT", "reconciliation posting is invalid")
	default:
		writeServiceInternalError(w, r, logger, action, err)
	}
}

func writeCheckpointRefusal(w http.ResponseWriter, code, message string, checkpointIDs []int64) {
	writeJSON(w, http.StatusConflict, errorResponse{Error: errorBody{Code: code, Message: message, CheckpointIDs: checkpointIDs}})
}
