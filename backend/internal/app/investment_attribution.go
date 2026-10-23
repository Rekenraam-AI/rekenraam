package app

import (
	"context"
	"database/sql"
	"fmt"

	"rekenraam/backend/internal/db"
)

type InvestmentAttributionDiagnostic struct {
	Status      string
	Reasons     []string
	Unavailable bool
}

func investmentAttributionDiagnostic(record db.InvestmentAttributionDiagnostic) InvestmentAttributionDiagnostic {
	status := SelfCheckPassed
	if record.Unavailable {
		status = SelfCheckIncomplete
	}
	if len(record.WarningReasons) > 0 {
		status = SelfCheckWarning
	}
	return InvestmentAttributionDiagnostic{Status: status, Reasons: record.WarningReasons, Unavailable: record.Unavailable}
}

func (s *SelfCheckService) investmentAttributionCheck(ctx context.Context, snapshot *sql.Tx) (SelfCheckResult, error) {
	records, err := s.repository.PaymentInLieuAttributions(ctx, snapshot, BookID)
	if err != nil {
		return SelfCheckResult{}, err
	}
	cash, err := s.repository.CashInLieuAttributions(ctx, snapshot, BookID)
	if err != nil {
		return SelfCheckResult{}, err
	}
	dividends, err := s.repository.DividendAttributions(ctx, snapshot, BookID)
	if err != nil {
		return SelfCheckResult{}, err
	}
	return investmentAttributionResult(append(append(records, cash...), dividends...)), nil
}

func investmentAttributionResult(records []db.InvestmentAttributionDiagnostic) SelfCheckResult {
	result := SelfCheckResult{CheckID: CheckInvestmentAttribution}
	for _, record := range records {
		if record.Unavailable {
			result.UnavailableCount++
			if len(result.UnavailableSample) < db.SelfCheckSampleLimit {
				result.UnavailableSample = append(result.UnavailableSample, record.TransactionID)
			}
		}
		for _, reason := range record.WarningReasons {
			result.WarningCount++
			if len(result.WarningSample) < db.SelfCheckSampleLimit {
				result.WarningSample = append(result.WarningSample, SelfCheckWarningSample{TransactionID: record.TransactionID, Reason: reason})
			}
		}
	}
	result.Status = resultStatus(result)
	result.Summary = fmt.Sprintf("%d investment payments checked for attribution; %d warnings; %d with unavailable context", len(records), result.WarningCount, result.UnavailableCount)
	return result
}
