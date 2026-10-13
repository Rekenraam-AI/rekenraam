package app

import (
	"context"

	"rekenraam/backend/internal/db"
	"rekenraam/backend/internal/exact"
)

// CapitalReturnRead totals one receipt's recorded allocations. Unknown effects
// leave totals unknown while independent known effects remain inspectable.
// Excess is deliberately separate from the disposal decision/gain set.
type CapitalReturnRead struct {
	db.CapitalReturnRead
	ReductionValue, ExcessValue exact.Coefficient
	ReductionScale, ExcessScale int
	UnknownLots                 int
	HasExcess                   bool
}

func (s *InvestmentService) ListCapitalReturns(ctx context.Context, params GainsReportParams) ([]CapitalReturnRead, error) {
	return s.capitalReturnReads(ctx, true, params, nil)
}

func (s *InvestmentService) capitalReturnReads(ctx context.Context, currentOnly bool, params GainsReportParams, operationIDs []int64) ([]CapitalReturnRead, error) {
	records, err := s.repository.ListCapitalReturnReads(ctx, BookID, currentOnly, params.From, params.To, operationIDs)
	if err != nil {
		return nil, err
	}
	out := make([]CapitalReturnRead, 0, len(records))
	for _, record := range records {
		read := CapitalReturnRead{CapitalReturnRead: record}
		reduction, excess := exact.NewScaledInt(), exact.NewScaledInt()
		for _, effect := range record.Effects {
			if effect.BasisKnowledge == db.InvestmentBasisUnknown {
				read.UnknownLots++
				continue
			}
			reduction.AddCoefficient(effect.ReductionValue, effect.ReductionScale)
			excess.AddCoefficient(effect.ExcessValue, effect.ExcessScale)
			read.HasExcess = read.HasExcess || effect.ExcessValue != "0"
		}
		if read.ReductionValue, err = reduction.Coefficient(); err != nil {
			return nil, err
		}
		if read.ExcessValue, err = excess.Coefficient(); err != nil {
			return nil, err
		}
		read.ReductionScale, read.ExcessScale = reduction.Scale(), excess.Scale()
		out = append(out, read)
	}
	return out, nil
}
