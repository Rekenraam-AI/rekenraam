package app

import (
	"context"
	"fmt"
	"sort"
	"time"

	"rekenraam/backend/internal/db"
	"rekenraam/backend/internal/exact"
)

// StatementAmount is an exact figure at the scale its sum needed.
type StatementAmount struct {
	Value int64
	Scale int
}

// GainsStatementValuation is one currency's unrealized gain at a valuation
// date. Gain is nil when any position held then is unpriced or has unresolved
// basis: a sum of the others is not the figure. Nothing held is a known zero.
type GainsStatementValuation struct {
	ValuationAsOf     string
	Gain              *StatementAmount
	PositionCount     int
	UnpricedCount     int
	UnresolvedCount   int
	StaleCount        int
	UnavailableReason string
	EarliestPriceDate string
	LatestPriceDate   string
}

// GainsStatementRow is one currency's period statement (#231).
type GainsStatementRow struct {
	CommodityID             int64
	Opening                 GainsStatementValuation
	Closing                 GainsStatementValuation
	Realized                *StatementAmount
	DisposalCount           int
	RealizedUnresolvedCount int
	Income                  StatementAmount
	IncomeCount             int
	UnrealizedChange        *StatementAmount
	Total                   *StatementAmount
}

// GainsStatement is the period statement under operational_v1. Profile is
// the closing valuation's; the opening valuation differs only in its date.
type GainsStatement struct {
	Profile GainsProfile
	From    string
	To      string
	Rows    []GainsStatementRow
}

// Gains statement unavailability reasons for a valuation's total.
const (
	StatementUnpriced              = "unpriced"
	StatementUnresolvedBasis       = "unresolved_basis"
	StatementUnpricedAndUnresolved = "unpriced_and_unresolved"
)

// GainsStatement reports, per currency, opening unrealized gain at the day
// before from, realized gain and investment income dated from..to, and
// closing unrealized gain at to (R18 slice 2, #231). Both valuations replay
// positions under operational_v1; nothing is written and nothing converted.
func (s *InvestmentService) GainsStatement(ctx context.Context, from, to string) (GainsStatement, error) {
	from, err := cleanRequiredDate(from, "period start")
	if err != nil {
		return GainsStatement{}, err
	}
	to, err = cleanRequiredDate(to, "period end")
	if err != nil {
		return GainsStatement{}, err
	}
	if from > to {
		return GainsStatement{}, ValidationError{Message: "period start cannot be after period end"}
	}
	start, _ := time.Parse(time.DateOnly, from)
	openingDate := start.AddDate(0, 0, -1).Format(time.DateOnly)

	// The closing projection refuses a date after owner-local today.
	profile, closing, err := s.UnrealizedGainsProjection(ctx, to)
	if err != nil {
		return GainsStatement{}, err
	}
	openingRecords, err := s.repository.DatedPositionsWithGains(ctx, BookID, db.ObservationBoundary{AsOf: openingDate, RecordedAt: profile.KnowledgeCutoff})
	if err != nil {
		return GainsStatement{}, err
	}
	opening := unrealizedGainEntries(openingRecords)
	realized, err := s.ListRealizedGains(ctx, GainsReportParams{From: from, To: to})
	if err != nil {
		return GainsStatement{}, err
	}
	income, err := s.repository.InvestmentIncome(ctx, BookID, from, to)
	if err != nil {
		return GainsStatement{}, fmt.Errorf("gains statement income: %w", err)
	}

	type accumulator struct {
		opening, closing      valuationAccumulator
		realized              *exact.ScaledInt
		disposals, unresolved int
		income                *exact.ScaledInt
		incomeCount           int
	}
	byCurrency := map[int64]*accumulator{}
	get := func(commodityID int64) *accumulator {
		if byCurrency[commodityID] == nil {
			byCurrency[commodityID] = &accumulator{
				opening: newValuationAccumulator(), closing: newValuationAccumulator(),
				realized: exact.NewScaledInt(), income: exact.NewScaledInt(),
			}
		}
		return byCurrency[commodityID]
	}
	for _, position := range opening {
		get(position.CostCommodityID).opening.add(position)
	}
	for _, position := range closing {
		get(position.CostCommodityID).closing.add(position)
	}
	for _, disposal := range realized {
		row := get(disposal.CostCommodityID)
		row.disposals++
		if disposal.BasisKnowledge == "unknown" {
			row.unresolved++
			continue
		}
		row.realized.AddInt64(disposal.RealizedGainValue, disposal.RealizedGainScale)
	}
	for _, dividend := range income {
		row := get(dividend.CommodityID)
		row.incomeCount++
		row.income.AddCoefficient(dividend.Value, dividend.Scale)
	}

	commodityIDs := make([]int64, 0, len(byCurrency))
	for id := range byCurrency {
		commodityIDs = append(commodityIDs, id)
	}
	sort.Slice(commodityIDs, func(i, j int) bool { return commodityIDs[i] < commodityIDs[j] })
	rows := make([]GainsStatementRow, 0, len(commodityIDs))
	for _, id := range commodityIDs {
		acc := byCurrency[id]
		row := GainsStatementRow{CommodityID: id, DisposalCount: acc.disposals,
			RealizedUnresolvedCount: acc.unresolved, IncomeCount: acc.incomeCount}
		if row.Opening, err = acc.opening.result(openingDate); err != nil {
			return GainsStatement{}, err
		}
		if row.Closing, err = acc.closing.result(to); err != nil {
			return GainsStatement{}, err
		}
		incomeAmount, err := statementAmount(acc.income)
		if err != nil {
			return GainsStatement{}, err
		}
		row.Income = *incomeAmount
		if acc.unresolved == 0 {
			if row.Realized, err = statementAmount(acc.realized); err != nil {
				return GainsStatement{}, err
			}
		}
		if acc.opening.complete() && acc.closing.complete() {
			change := exact.ScaledIntFromBig(acc.closing.gain.BigInt(), acc.closing.gain.Scale())
			change.SubScaled(acc.opening.gain)
			if row.UnrealizedChange, err = statementAmount(change); err != nil {
				return GainsStatement{}, err
			}
			if row.Realized != nil {
				change.AddScaled(acc.realized)
				change.AddScaled(acc.income)
				if row.Total, err = statementAmount(change); err != nil {
					return GainsStatement{}, err
				}
			}
		}
		rows = append(rows, row)
	}
	return GainsStatement{Profile: profile, From: from, To: to, Rows: rows}, nil
}

// valuationAccumulator sums one currency's positions at a valuation date
// exactly and counts what keeps the sum from being the total.
type valuationAccumulator struct {
	gain                                   *exact.ScaledInt
	positions, unpriced, unresolved, stale int
	earliest, latest                       string
}

func newValuationAccumulator() valuationAccumulator {
	return valuationAccumulator{gain: exact.NewScaledInt()}
}

func (v *valuationAccumulator) add(position UnrealizedGainEntry) {
	v.positions++
	if position.BasisKnowledge == db.InvestmentBasisUnknown {
		v.unresolved++
	}
	if position.MarketValueValue == nil {
		v.unpriced++
	} else {
		if position.PriceStatus == "stale" {
			v.stale++
		}
		if date := position.LatestPriceDate; date != "" {
			if v.earliest == "" || date < v.earliest {
				v.earliest = date
			}
			if date > v.latest {
				v.latest = date
			}
		}
	}
	if position.UnrealizedGainValue != nil {
		v.gain.AddInt64(*position.UnrealizedGainValue, *position.UnrealizedGainScale)
	}
}

func (v *valuationAccumulator) complete() bool { return v.unpriced == 0 && v.unresolved == 0 }

func (v *valuationAccumulator) result(date string) (GainsStatementValuation, error) {
	valuation := GainsStatementValuation{ValuationAsOf: date, PositionCount: v.positions,
		UnpricedCount: v.unpriced, UnresolvedCount: v.unresolved, StaleCount: v.stale,
		EarliestPriceDate: v.earliest, LatestPriceDate: v.latest}
	switch {
	case v.unpriced > 0 && v.unresolved > 0:
		valuation.UnavailableReason = StatementUnpricedAndUnresolved
	case v.unpriced > 0:
		valuation.UnavailableReason = StatementUnpriced
	case v.unresolved > 0:
		valuation.UnavailableReason = StatementUnresolvedBasis
	default:
		gain, err := statementAmount(v.gain)
		if err != nil {
			return GainsStatementValuation{}, err
		}
		valuation.Gain = gain
	}
	return valuation, nil
}

// statementAmount restates an exact sum in int64 columns, dropping only
// trailing zeros when the full-scale coefficient would not fit.
func statementAmount(sum *exact.ScaledInt) (*StatementAmount, error) {
	if value, err := sum.Int64(); err == nil {
		return &StatementAmount{Value: value, Scale: sum.Scale()}, nil
	}
	normalized := sum.Normalized()
	value, err := normalized.Int64()
	if err != nil {
		return nil, fmt.Errorf("gains statement amount out of range: %w", err)
	}
	return &StatementAmount{Value: value, Scale: normalized.Scale()}, nil
}
