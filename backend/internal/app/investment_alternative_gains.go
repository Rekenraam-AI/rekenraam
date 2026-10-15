package app

import (
	"context"
	"errors"
	"fmt"

	"rekenraam/backend/internal/db"
)

// AlternativeGains is a live read-only projection. Specific-lot decisions
// use the selected method, and transfer quantities retain their recorded links.
type AlternativeGains struct {
	Profile    GainsProfile
	Realized   []RealizedGainEntry
	Unrealized []UnrealizedGainEntry
}

// AlternativeGainsDependencyError identifies a recorded structural entitlement
// the hypothetical method cannot satisfy; no partial projection is returned.
type AlternativeGainsDependencyError struct {
	OperationID int64
}

func (e AlternativeGainsDependencyError) Error() string {
	return fmt.Sprintf("alternative projection cannot preserve the lot entitlements of operation %d", e.OperationID)
}

func (s *InvestmentService) AlternativeGainsProjection(ctx context.Context, method, valuationDate string, period GainsReportParams) (AlternativeGains, error) {
	boundary, err := s.repository.CurrentObservationBoundary(ctx, BookID, s.now())
	if err != nil {
		return AlternativeGains{}, err
	}
	if valuationDate != "" {
		date, err := cleanRequiredDate(valuationDate, "valuation date")
		if err != nil {
			return AlternativeGains{}, err
		}
		if date > boundary.AsOf {
			return AlternativeGains{}, ValidationError{Message: "valuation date cannot be after today"}
		}
		boundary.AsOf = date
	}
	return s.alternativeGainsAtBoundary(ctx, method, boundary, period)
}

// AlternativeGainsForProfile shares the operational read's price/date boundary
// so observations recorded between the two computations cannot skew comparison.
func (s *InvestmentService) AlternativeGainsForProfile(ctx context.Context, method string, operational GainsProfile, period GainsReportParams) (AlternativeGains, error) {
	return s.alternativeGainsAtBoundary(ctx, method, db.ObservationBoundary{AsOf: operational.ValuationAsOf, RecordedAt: operational.KnowledgeCutoff}, period)
}

func (s *InvestmentService) alternativeGainsAtBoundary(ctx context.Context, method string, boundary db.ObservationBoundary, period GainsReportParams) (AlternativeGains, error) {
	if method != "fifo" && method != "lifo" && method != "average_cost" {
		return AlternativeGains{}, ValidationError{Message: "alternative method must be fifo, lifo or average_cost"}
	}
	for _, date := range []string{period.From, period.To} {
		if date != "" {
			if _, err := cleanRequiredDate(date, "disposal date"); err != nil {
				return AlternativeGains{}, err
			}
		}
	}
	if period.From != "" && period.To != "" && period.From > period.To {
		return AlternativeGains{}, ValidationError{Message: "period start cannot be after period end"}
	}
	realized, unrealized, err := s.repository.AlternativeMethodGains(ctx, BookID, method, boundary, db.RealizedGainsParams{From: period.From, To: period.To})
	if err != nil {
		var dependency *db.InvestmentReplayDependencyError
		if errors.As(err, &dependency) && (errors.Is(err, db.ErrInsufficientLots) ||
			errors.Is(err, db.ErrNotFound) || errors.Is(err, db.ErrInvalidDisposalParams) ||
			errors.Is(err, db.ErrTransferBasisKnowledgeChanged) || errors.Is(err, db.ErrSpinOffEntitlementChanged) ||
			errors.Is(err, db.ErrCapitalReturnEntitlementUnavailable) || errors.Is(err, db.ErrShareExchangeIncomplete) ||
			errors.Is(err, db.ErrSplitFractionUnrepresentable)) {
			return AlternativeGains{}, AlternativeGainsDependencyError{OperationID: dependency.OperationID}
		}
		return AlternativeGains{}, fmt.Errorf("project alternative gains: %w", err)
	}
	profile := operationalGainsProfile(boundary)
	profile.Name, profile.Purpose, profile.BasisMethod = "alternative_method_v1", "alternative_method", method
	profile.SpecificLotFallback, profile.TransferPolicy = "selected_method", "selected_method_fixed_link_quantities"
	profile.StructuralActionPolicy = "recorded_entitlements"
	profile.From, profile.To = period.From, period.To
	return AlternativeGains{Profile: profile, Realized: realizedGainEntries(realized), Unrealized: unrealizedGainEntries(unrealized)}, nil
}
