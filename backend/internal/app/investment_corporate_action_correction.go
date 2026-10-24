package app

import (
	"context"
	"strings"

	"rekenraam/backend/internal/db"
)

// Corporate-action correction preparation (#401): the split, share exchange,
// spin-off and cash-and-stock merger corrections share the admission and
// verification that follow their typed operation lookup. Each family keeps
// its own lookup and error mapping, inverse journal, entitlement, allocation,
// no-op and acknowledgement rules. The typed writer rechecks the same facts
// inside its transaction; this leaf only refuses early, writing nothing.

// corporateActionCorrection names one family's preparation refusals.
type corporateActionCorrection struct {
	// noun completes "owner and <noun> id are required" and
	// "<noun> correction reason is required".
	noun             string
	alreadyCorrected error
	importedLineage  error
	changed          error
	// journalFree admits an operation that posted no journal: a split whose
	// ratio moves no units (T-129).
	journalFree bool
}

var (
	splitCorrection = corporateActionCorrection{noun: "split",
		alreadyCorrected: ErrInvestmentSplitAlreadyCorrected, importedLineage: ErrInvestmentImportedSplit,
		changed: ErrInvestmentSplitChanged, journalFree: true}
	shareExchangeCorrection = corporateActionCorrection{noun: "exchange",
		alreadyCorrected: ErrShareExchangeAlreadyCorrected, importedLineage: ErrInvestmentImportedTransfer,
		changed: ErrShareExchangeChanged}
	spinOffCorrection = corporateActionCorrection{noun: "spin-off",
		alreadyCorrected: ErrSpinOffAlreadyCorrected, importedLineage: ErrInvestmentImportedTransfer,
		changed: ErrSpinOffChanged}
	// A merger reports a moved holding as the share exchange it contains.
	cashStockMergerCorrection = corporateActionCorrection{noun: "merger",
		alreadyCorrected: ErrCashStockMergerAlreadyCorrected, importedLineage: ErrInvestmentImportedTransfer,
		changed: ErrShareExchangeChanged}
)

// admit checks the owner, that the command names its operation, and the
// reason, which it returns cleaned.
func (family corporateActionCorrection) admit(ownerUserID int64, identified bool, rawReason string) (string, error) {
	if ownerUserID <= 0 || !identified {
		return "", ValidationError{Message: "owner and " + family.noun + " id are required"}
	}
	if strings.TrimSpace(rawReason) == "" {
		return "", ValidationError{Message: family.noun + " correction reason is required"}
	}
	return cleanChangeReason(rawReason, "")
}

// correctionTarget is what verification reads from a typed operation record.
type correctionTarget struct {
	OperationID      int64
	TransactionID    int64
	CurrentVersionID int64
	EventDate        string
	AlreadyCorrected bool
	ImportedLineage  bool
}

// verifyCorrectionTarget applies the correction fences to the effective operation and returns
// its posted original, or no transaction for an admitted journal-free one.
func (s *InvestmentService) verifyCorrectionTarget(ctx context.Context, family corporateActionCorrection, target correctionTarget) (Transaction, error) {
	if target.AlreadyCorrected {
		return Transaction{}, family.alreadyCorrected
	}
	if target.ImportedLineage {
		linked, err := s.repository.HasCommittedImportSource(ctx, BookID, target.OperationID)
		if err != nil {
			return Transaction{}, err
		}
		if !linked {
			return Transaction{}, family.importedLineage
		}
	}
	if target.TransactionID == 0 && family.journalFree {
		return Transaction{}, nil
	}
	original, err := s.transactionService.Transaction(ctx, target.TransactionID)
	if err != nil {
		return Transaction{}, err
	}
	if original.VersionID != target.CurrentVersionID || original.Status != "posted" ||
		original.TransactionKind != "investment" || original.DeletedAt != "" ||
		original.TransactionDate != target.EventDate {
		return Transaction{}, family.changed
	}
	return original, nil
}

// correctionInverseInput plans the inverse journal of a correction command.
func correctionInverseInput(ownerUserID, authSessionID int64, requestID, auditOperation string, correctionOf *int64,
	spec TransactionInput, reason string, override CheckpointOverride) CreateTransactionInput {
	return CreateTransactionInput{
		OwnerUserID: ownerUserID, AuthSessionID: authSessionID, RequestID: requestID, OriginType: "browser_api",
		Operation: auditOperation, CorrectionOfTransactionID: correctionOf,
		Spec: spec, ChangeReason: reason, ReconciliationOverride: override.Allowed, ReconciliationOverrideCheckpointIDs: override.Reviewed,
	}
}

// linkCorrection marks a prepared journal as the reversal or replacement of
// the corrected operation.
func linkCorrection(params *db.CreateTransactionParams, operationID int64, mode, reason string) {
	params.InvestmentCorrectionOfOperationID = operationID
	params.InvestmentCorrectionMode = mode
	params.InvestmentCorrectionReason = reason
}

// sameSourceEvidence reports whether corrected evidence is the recorded
// evidence as a JSON object (the correction form pre-fills it). Compare exact
// number tokens, as for basis provenance, so adjacent large source IDs do not
// become a false no-op through float rounding (#309).
func sameSourceEvidence(corrected, recorded string) bool {
	return sameBasisResolutionEvidence(corrected, recorded)
}
