package app

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"sort"
	"time"

	"rekenraam/backend/internal/db"
	"rekenraam/backend/internal/exact"
)

var ErrGainsFXPrecision = errors.New("converted gains exceed supported precision")

const GainsFXValuationDateAll = "valuation_date_all"
const GainsFXHistoricBasis = "historic_basis_valuation_close"

type GainsFXInput struct {
	CommodityID int64
	Method      string
	MaxAgeDays  int
}

type GainsFXAmount struct {
	Value exact.Coefficient `json:"value"`
	Scale int               `json:"scale"`
}

type GainsFXComponent struct {
	Role                   string         `json:"role"`
	LotID                  int64          `json:"lot_id,omitempty"`
	CommodityID            int64          `json:"commodity_id"`
	Source                 *GainsFXAmount `json:"source"`
	RequestedDate          string         `json:"requested_date"`
	Amount                 *GainsFXAmount `json:"amount"`
	ObservationID          int64          `json:"observation_id,omitempty"`
	ObservationDate        string         `json:"observation_date,omitempty"`
	Rate                   *GainsFXAmount `json:"rate"`
	BaseQuantity           *GainsFXAmount `json:"base_quantity"`
	Approximate            bool           `json:"approximate"`
	Derived                bool           `json:"derived"`
	UnavailableReason      string         `json:"unavailable_reason,omitempty"`
	NearestObservationDate string         `json:"nearest_observation_date,omitempty"`
}

type GainsFXFigure struct {
	Amount            *GainsFXAmount     `json:"amount"`
	Components        []GainsFXComponent `json:"components"`
	UnavailableReason string             `json:"unavailable_reason,omitempty"`
}

type GainsFXProjection struct {
	Profile         GainsProfile
	Realized        []GainsFXFigure
	Unrealized      []GainsFXFigure
	RealizedTotal   GainsFXFigure
	UnrealizedTotal GainsFXFigure
}

type GainsFXStatement struct {
	Profile                                                     GainsProfile
	Opening, Realized, Income, Closing, UnrealizedChange, Total GainsFXFigure
}

func (input GainsFXInput) validated() (GainsFXInput, error) {
	if input.CommodityID <= 0 || (input.Method != GainsFXValuationDateAll && input.Method != GainsFXHistoricBasis) {
		return input, ValidationError{Message: "reporting currency and a named gains FX method are required together"}
	}
	if input.MaxAgeDays == 0 {
		input.MaxAgeDays = DefaultRateStalenessDays
	}
	if input.MaxAgeDays < 1 || input.MaxAgeDays > 365 {
		return input, ValidationError{Message: "FX maximum age must be between 1 and 365 days"}
	}
	return input, nil
}

type gainsFXConverter struct {
	rates   *RateTable
	profile GainsProfile
}

func (s *InvestmentService) gainsFXConverter(ctx context.Context, profile GainsProfile, input GainsFXInput, components []GainsFXComponent) (*gainsFXConverter, error) {
	input, err := input.validated()
	if err != nil {
		return nil, err
	}
	ids := []int64{}
	from, to := profile.ValuationAsOf, profile.ValuationAsOf
	for _, c := range components {
		ids = append(ids, c.CommodityID)
		if c.RequestedDate != "" {
			from = min(from, c.RequestedDate)
			to = max(to, c.RequestedDate)
		}
	}
	// Historic disposals can be later than an independently selected valuation.
	// The rate ceiling is owner-local today, with the profile's knowledge instant.
	readAt, err := time.Parse(time.RFC3339Nano, profile.KnowledgeCutoff)
	if err != nil {
		return nil, fmt.Errorf("gains FX knowledge cutoff: %w", err)
	}
	boundary, err := s.repository.CurrentObservationBoundary(ctx, BookID, readAt)
	if err != nil {
		return nil, err
	}
	boundary.RecordedAt = profile.KnowledgeCutoff
	rates, err := newRateTableAtBoundary(ctx, s.transactionService.pricingRepository, ReportingCurrencyInput{CommodityID: input.CommodityID, MaxStalenessDay: input.MaxAgeDays}, ids, from, to, boundary)
	if err != nil {
		return nil, err
	}
	profile.FXMethod = input.Method
	profile.ReportingCurrencyID = input.CommodityID
	profile.FXMaxAgeDays = input.MaxAgeDays
	profile.Rounding = "component_half_up_currency_scale"
	return &gainsFXConverter{rates: rates, profile: profile}, nil
}

func fxAmount(value int64, scale int) *GainsFXAmount {
	return &GainsFXAmount{Value: exact.New(value), Scale: scale}
}
func fxComponent(role string, currency int64, value int64, scale int, date string) GainsFXComponent {
	return GainsFXComponent{Role: role, CommodityID: currency, Source: fxAmount(value, scale), RequestedDate: date}
}

func (c *gainsFXConverter) figure(parts []GainsFXComponent) (GainsFXFigure, error) {
	result := GainsFXFigure{Components: make([]GainsFXComponent, 0, len(parts))}
	sum := exact.NewScaledInt()
	for _, part := range parts {
		switch {
		case part.Source == nil:
			part.UnavailableReason = "unresolved_basis"
			if part.Role == "market_value" {
				part.UnavailableReason = "unpriced"
			}
		case part.CommodityID == c.rates.QuoteCommodityID():
			part.Amount = part.Source // exact no-op, including sub-cent values
		case part.RequestedDate == "":
			part.UnavailableReason = "unknown_acquisition_date"
		default:
			delete(c.rates.gaps, part.CommodityID) // Each figure retains its own dated gap.
			rate, ok := c.rates.rateFor(part.CommodityID, part.RequestedDate)
			if !ok {
				gap := c.rates.gaps[part.CommodityID]
				part.UnavailableReason = gap.Reason
				part.NearestObservationDate = gap.NearestObservationDate
			} else {
				converted, err := exact.MulDivRound(part.Source.Value.BigInt(), part.Source.Scale, big.NewInt(rate.PriceValue), rate.PriceScale, big.NewInt(rate.BaseQuantityValue), rate.BaseQuantityScale, c.rates.Scale())
				if err != nil {
					return GainsFXFigure{}, err
				}
				value, err := exact.FromBig(converted)
				if err != nil {
					return GainsFXFigure{}, ErrGainsFXPrecision
				}
				part.Amount = &GainsFXAmount{Value: value, Scale: c.rates.Scale()}
				part.ObservationID = rate.ObservationID
				part.ObservationDate = rate.ValuationDate
				part.Rate = fxAmount(rate.PriceValue, rate.PriceScale)
				part.BaseQuantity = fxAmount(rate.BaseQuantityValue, rate.BaseQuantityScale)
				part.Approximate = rate.IsApproximate
				part.Derived = rate.IsDerived
			}
		}
		if part.Amount == nil {
			result.UnavailableReason = "incomplete_components"
		} else {
			sum.AddCoefficient(part.Amount.Value, part.Amount.Scale)
		}
		result.Components = append(result.Components, part)
	}
	if result.UnavailableReason == "" {
		value, err := sum.Coefficient()
		if err != nil {
			return GainsFXFigure{}, ErrGainsFXPrecision
		}
		result.Amount = &GainsFXAmount{Value: value, Scale: sum.Scale()}
	}
	return result, nil
}

func combineFXFigures(figures ...GainsFXFigure) (GainsFXFigure, error) {
	result := GainsFXFigure{Components: []GainsFXComponent{}}
	sum := exact.NewScaledInt()
	for _, f := range figures {
		if f.Amount == nil {
			result.UnavailableReason = "incomplete_components"
		} else {
			sum.AddCoefficient(f.Amount.Value, f.Amount.Scale)
		}
	}
	if result.UnavailableReason == "" {
		value, err := sum.Coefficient()
		if err != nil {
			return result, ErrGainsFXPrecision
		}
		result.Amount = &GainsFXAmount{Value: value, Scale: sum.Scale()}
	}
	return result, nil
}

// ConvertGains adds reporting figures beside an operational projection. It
// never changes the source values, elections or journals.
func (s *InvestmentService) ConvertGains(ctx context.Context, profile GainsProfile, period GainsReportParams, realized []RealizedGainEntry, unrealized []UnrealizedGainEntry, input GainsFXInput) (GainsFXProjection, error) {
	input, err := input.validated()
	if err != nil {
		return GainsFXProjection{}, err
	}
	for _, date := range []string{period.From, period.To} {
		if date != "" {
			if _, err := cleanRequiredDate(date, "gains period"); err != nil {
				return GainsFXProjection{}, err
			}
		}
	}
	if period.From != "" && period.To != "" && period.From > period.To {
		return GainsFXProjection{}, ValidationError{Message: "period start cannot be after period end"}
	}
	profile.From, profile.To = period.From, period.To
	var basis []db.GainsFXBasis
	if input.Method == GainsFXHistoricBasis {
		basis, err = s.repository.GainsFXBasisComponents(ctx, BookID, profile.ValuationAsOf, db.RealizedGainsParams{From: period.From, To: period.To})
		if err != nil {
			return GainsFXProjection{}, err
		}
		sort.Slice(basis, func(i, j int) bool { return basis[i].LotID < basis[j].LotID })
	}
	type basisKey struct {
		account, instrument, currency, transaction int64
		side, date                                 string
	}
	byFigure := make(map[basisKey][]db.GainsFXBasis)
	for _, b := range basis {
		key := basisKey{b.AccountID, b.CommodityID, b.CostCommodityID, b.TransactionID, b.PositionSide, b.DisposalDate}
		byFigure[key] = append(byFigure[key], b)
	}
	realizedParts := make([][]GainsFXComponent, len(realized))
	unrealizedParts := make([][]GainsFXComponent, len(unrealized))
	all := []GainsFXComponent{}
	addBasis := func(parts []GainsFXComponent, account, instrument, currency int64, side, date string, transactionID int64, value int64, scale int, unknown bool) []GainsFXComponent {
		if input.Method == GainsFXValuationDateAll {
			part := fxComponent("basis", currency, value, scale, profile.ValuationAsOf)
			if unknown {
				part.Source = nil
			}
			return append(parts, part)
		}
		found := false
		for _, b := range byFigure[basisKey{account, instrument, currency, transactionID, side, date}] {
			part := fxComponent("basis", currency, b.Value, b.Scale, b.AcquisitionDate)
			part.LotID = b.LotID
			if b.Unknown {
				part.Source = nil
			}
			if date == "" && side != db.PositionSideShort && part.Source != nil {
				part.Source.Value = part.Source.Value.Negated()
			}
			parts = append(parts, part)
			found = true
		}
		if !found {
			part := fxComponent("basis", currency, 0, 0, "")
			part.Source = nil
			parts = append(parts, part)
		}
		return parts
	}
	for i, r := range realized {
		txID := int64(0)
		if r.TransactionID != nil {
			txID = *r.TransactionID
		}
		date := r.DisposalDate
		if input.Method == GainsFXValuationDateAll {
			date = profile.ValuationAsOf
		}
		parts := []GainsFXComponent{fxComponent("proceeds", r.CostCommodityID, r.ProceedsValue, r.ProceedsScale, date)}
		parts = addBasis(parts, r.AccountID, r.CommodityID, r.CostCommodityID, r.PositionSide, r.DisposalDate, txID, r.DisposedBasisValue, r.DisposedBasisScale, r.BasisKnowledge == db.InvestmentBasisUnknown)
		realizedParts[i] = parts
		all = append(all, parts...)
	}
	for i, r := range unrealized {
		part := GainsFXComponent{Role: "market_value", CommodityID: r.CostCommodityID, RequestedDate: profile.ValuationAsOf}
		if r.MarketValueValue != nil {
			part.Source = fxAmount(*r.MarketValueValue, *r.MarketValueScale)
		}
		value := r.RemainingCostBasisValue
		if r.PositionSide != db.PositionSideShort {
			value = -value
		}
		parts := addBasis([]GainsFXComponent{part}, r.AccountID, r.CommodityID, r.CostCommodityID, r.PositionSide, "", 0, value, r.RemainingCostBasisScale, r.BasisKnowledge == db.InvestmentBasisUnknown)
		unrealizedParts[i] = parts
		all = append(all, parts...)
	}
	converter, err := s.gainsFXConverter(ctx, profile, input, all)
	if err != nil {
		return GainsFXProjection{}, err
	}
	result := GainsFXProjection{Profile: converter.profile, Realized: []GainsFXFigure{}, Unrealized: []GainsFXFigure{}}
	for _, parts := range realizedParts {
		f, err := converter.figure(parts)
		if err != nil {
			return result, err
		}
		result.Realized = append(result.Realized, f)
	}
	for _, parts := range unrealizedParts {
		f, err := converter.figure(parts)
		if err != nil {
			return result, err
		}
		result.Unrealized = append(result.Unrealized, f)
	}
	result.RealizedTotal, err = combineFXFigures(result.Realized...)
	if err != nil {
		return result, err
	}
	result.UnrealizedTotal, err = combineFXFigures(result.Unrealized...)
	return result, err
}

func (s *InvestmentService) ConvertGainsStatement(ctx context.Context, statement GainsStatement, input GainsFXInput) (GainsFXStatement, error) {
	project := func(date string) (GainsFXProjection, error) {
		profile := statement.Profile
		profile.ValuationAsOf = date
		records, err := s.repository.DatedPositionsWithGains(ctx, BookID, db.ObservationBoundary{AsOf: date, RecordedAt: profile.KnowledgeCutoff})
		if err != nil {
			return GainsFXProjection{}, err
		}
		positions := unrealizedGainEntries(records)
		var realized []RealizedGainEntry
		if date == statement.To {
			realized, err = s.ListRealizedGains(ctx, GainsReportParams{From: statement.From, To: statement.To})
			if err != nil {
				return GainsFXProjection{}, err
			}
		}
		return s.ConvertGains(ctx, profile, GainsReportParams{From: statement.From, To: statement.To}, realized, positions, input)
	}
	closing, err := project(statement.To)
	if err != nil {
		return GainsFXStatement{}, err
	}
	openingDate, err := shiftISODate(statement.From, -1)
	if err != nil {
		return GainsFXStatement{}, err
	}
	opening, err := project(openingDate)
	if err != nil {
		return GainsFXStatement{}, err
	}
	income, err := s.repository.InvestmentIncome(ctx, BookID, statement.From, statement.To)
	if err != nil {
		return GainsFXStatement{}, err
	}
	parts := []GainsFXComponent{}
	for _, i := range income {
		date := i.EventDate
		if input.Method == GainsFXValuationDateAll {
			date = statement.To
		}
		part := GainsFXComponent{Role: "income", CommodityID: i.CommodityID, Source: &GainsFXAmount{Value: i.Value, Scale: i.Scale}, RequestedDate: date}
		parts = append(parts, part)
	}
	converter, err := s.gainsFXConverter(ctx, statement.Profile, input, parts)
	if err != nil {
		return GainsFXStatement{}, err
	}
	result := GainsFXStatement{Profile: closing.Profile, Opening: opening.UnrealizedTotal, Closing: closing.UnrealizedTotal, Realized: closing.RealizedTotal}
	// Keep per-figure evidence for every boundary and disposal.
	for _, f := range opening.Unrealized {
		result.Opening.Components = append(result.Opening.Components, f.Components...)
	}
	for _, f := range closing.Unrealized {
		result.Closing.Components = append(result.Closing.Components, f.Components...)
	}
	for _, f := range closing.Realized {
		result.Realized.Components = append(result.Realized.Components, f.Components...)
	}
	result.Profile.From, result.Profile.To = statement.From, statement.To
	result.Income, err = converter.figure(parts)
	if err != nil {
		return result, err
	}
	negativeOpening := result.Opening
	if negativeOpening.Amount != nil {
		copy := *negativeOpening.Amount
		copy.Value = copy.Value.Negated()
		negativeOpening.Amount = &copy
	}
	result.UnrealizedChange, err = combineFXFigures(result.Closing, negativeOpening)
	if err != nil {
		return result, err
	}
	result.Total, err = combineFXFigures(result.Realized, result.Income, result.UnrealizedChange)
	if err != nil {
		return result, fmt.Errorf("converted statement total: %w", err)
	}
	return result, nil
}
