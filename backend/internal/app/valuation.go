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

// Reporting-currency valuation.
//
// A report groups by commodity because unlike commodities cannot be added.
// That stays true: conversion here is **additive**, never replacing. Every
// response keeps its exact per-commodity figures, and a converted total sits
// beside them, labelled with the method that produced it.
//
// The three decisions the R2 acceptance review left open, settled here:
//
//  1. **Which rate date.** A stock is converted at the date it is measured on
//     (a net-worth bucket end); a flow is converted at the date it happened
//     (each posting's entry date), then summed. Converting a period of flows at
//     one range-end rate is cheaper and reproducible from the displayed
//     subtotals, but it misattributes everything that happened before a large
//     mid-period move.
//  2. **Missing coverage.** The nearest earlier observation inside a named
//     window is used and reported as stale; with nothing in the window the
//     converted figure is omitted and the commodity is named, with a reason.
//     Never silently.
//  3. **Prerequisite.** T-37 shipped: a voided observation cascades to every
//     rate triangulated from it, and ReportingRates excludes voided rows. A
//     poisoned rate can be withdrawn, which is what makes rates safe to put
//     behind headline figures.

// ValuationMethodObservedOnOrBefore is the only method implemented. It is named
// in every response rather than assumed, so a second method can arrive without
// changing what an existing figure means.
const ValuationMethodObservedOnOrBefore = "observed_on_or_before"

// DefaultRateStalenessDays bounds how far back the nearest-earlier search
// looks. A week covers weekends and public holidays — the ordinary reasons a
// date has no observation — without letting a month-old rate masquerade as
// today's.
const DefaultRateStalenessDays = 7

// Reasons a commodity could not be converted. Returned rather than described,
// so a screen can translate them.
const (
	RateMissingNoObservation = "no_observation_in_window"
	RateMissingNotACurrency  = "reporting_currency_is_not_a_currency"
)

// ReportingCurrencyInput is what a caller asks for.
type ReportingCurrencyInput struct {
	CommodityID     int64
	MaxStalenessDay int
}

// RateUse records that a conversion happened and on what: the rate's own
// observation date, and whether that date was the one asked for.
type RateUse struct {
	ObservationID   int64  `json:"observation_id"`
	Approximate     bool   `json:"approximate"`
	CommodityID     int64  `json:"commodity_id"`
	ObservationDate string `json:"observation_date"`
	RequestedDate   string `json:"requested_date"`
	Stale           bool   `json:"stale"`
	Derived         bool   `json:"derived"`
	// Via* name the second leg of a chained conversion (#281): the commodity was
	// priced in ViaCommodityID by the observation above, and that currency was
	// converted by ViaObservationID. Zero for a direct rate.
	ViaCommodityID     int64  `json:"via_commodity_id,omitempty"`
	ViaObservationID   int64  `json:"via_observation_id,omitempty"`
	ViaObservationDate string `json:"via_observation_date,omitempty"`
}

// RateGap is a commodity that could not be converted, and why.
type RateGap struct {
	CommodityID int64  `json:"commodity_id"`
	Reason      string `json:"reason"`
	// NearestObservationDate is the closest observation that exists outside the
	// window, when there is one. "No rate" and "no rate recently" are different
	// problems and lead to different fixes.
	NearestObservationDate string `json:"nearest_observation_date,omitempty"`
	// ViaCommodityID is set when the commodity has a usable price in that
	// currency but the currency itself has no usable reporting rate: the
	// missing leg is the currency's, and Reason/NearestObservationDate are
	// about it.
	ViaCommodityID int64 `json:"via_commodity_id,omitempty"`
}

// ValuationCoverage is what a response says about its own conversions.
type ValuationCoverage struct {
	SelectionPolicy  string
	ValuationAsOf    string
	KnowledgeCutoff  string
	CommodityID      int64
	Code             string
	Scale            int
	Method           string
	MaxStalenessDays int
	Used             []RateUse
	Gaps             []RateGap
	// Complete is false when any figure in the response omits a commodity,
	// which is the one thing a reader must not have to infer from a total that
	// looks plausible.
	Complete bool
}

type rateUseKey struct {
	commodityID, observationID, viaObservationID int64
	requestedDate                                string
}

// RateTable resolves a commodity and a date to a rate, once, for a whole
// report. Built from one query over the window the report covers.
type RateTable struct {
	boundary         db.ObservationBoundary
	diagnostics      map[[2]int64]string
	quoteCommodityID int64
	quoteScale       int
	stalenessDays    int
	// byCommodity holds each commodity's observations in date order.
	byCommodity map[int64][]db.ReportingRateRecord
	// chainQuotes holds, per non-currency commodity, its observations in each
	// other currency, in date order: the first legs of chained conversions.
	chainQuotes map[int64]map[int64][]db.ReportingRateRecord

	used map[rateUseKey]RateUse
	gaps map[int64]RateGap
}

// NewRateTable loads every rate that could be needed to convert the given
// commodities into the reporting currency across [fromDate, toDate].
//
// The window is widened backwards by the staleness allowance, because the rate
// that applies to the first day of a report may have been observed before it.
func (s *TransactionService) NewRateTable(
	ctx context.Context,
	reporting ReportingCurrencyInput,
	commodityIDs []int64,
	fromDate string,
	toDate string,
) (*RateTable, error) {
	ctx, referenceErr := s.settings.referenceContext(ctx, s.now())
	if referenceErr != nil {
		return nil, referenceErr
	}

	if s.pricingRepository == nil {
		return nil, fmt.Errorf("reporting currency requires pricing data")
	}
	boundary, err := s.pricingRepository.CurrentObservationBoundary(ctx, BookID, s.now())
	if err != nil {
		return nil, err
	}
	return newRateTableAtBoundary(ctx, s.pricingRepository, reporting, commodityIDs, fromDate, toDate, boundary)
}

// newRateTableAtBoundary lets projections share their observation knowledge instant.
func newRateTableAtBoundary(ctx context.Context, pricing *db.PricingRepository, reporting ReportingCurrencyInput, commodityIDs []int64, fromDate, toDate string, boundary db.ObservationBoundary) (*RateTable, error) {
	if pricing == nil {
		return nil, fmt.Errorf("reporting currency requires pricing data")
	}

	staleness := reporting.MaxStalenessDay
	if staleness <= 0 {
		staleness = DefaultRateStalenessDays
	}

	quote, err := pricing.ReportingCurrency(ctx, BookID, reporting.CommodityID)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			return nil, ValidationError{Message: "reporting currency is invalid"}
		}
		return nil, err
	}
	if quote.Kind != "currency" {
		// A report denominated in a security would be a different feature and a
		// far stranger one; refusing is clearer than converting into it.
		return nil, ValidationError{Message: "reporting currency must be a currency"}
	}

	boundary.AsOf = min(boundary.AsOf, toDate)
	table := &RateTable{
		boundary:         boundary,
		quoteCommodityID: reporting.CommodityID,
		quoteScale:       quote.StandardScale,
		stalenessDays:    staleness,
		byCommodity:      map[int64][]db.ReportingRateRecord{},
		chainQuotes:      map[int64]map[int64][]db.ReportingRateRecord{},

		used: map[rateUseKey]RateUse{},
		gaps: map[int64]RateGap{},
	}

	needed := make([]int64, 0, len(commodityIDs))
	for _, commodityID := range dedupeIDs(commodityIDs) {
		if commodityID != reporting.CommodityID {
			needed = append(needed, commodityID)
		}
	}
	if len(needed) == 0 {
		return table, nil
	}

	windowStart, err := shiftISODate(fromDate, -staleness)
	if err != nil {
		return nil, err
	}
	// A security is usually priced in its quote currency, not the reporting
	// currency, so its rate is chained: security → quote currency → reporting
	// currency, each leg under the same on-or-before and staleness rules. Only
	// one currency hop is attempted; currency-to-currency triangulation stays
	// with the FX refresh worker's derived observations.
	chainQuotes, err := pricing.ReportingChainQuotes(ctx, BookID, reporting.CommodityID, needed, windowStart, toDate, boundary)
	if err != nil {
		return nil, err
	}
	rateBases := append([]int64(nil), needed...)
	for _, quote := range chainQuotes {
		byQuote := table.chainQuotes[quote.BaseCommodityID]
		if byQuote == nil {
			byQuote = map[int64][]db.ReportingRateRecord{}
			table.chainQuotes[quote.BaseCommodityID] = byQuote
		}
		byQuote[quote.QuoteCommodityID] = append(byQuote[quote.QuoteCommodityID], quote)
		rateBases = append(rateBases, quote.QuoteCommodityID)
	}
	rateBases = dedupeIDs(rateBases)

	rates, err := pricing.ReportingRates(ctx, BookID, reporting.CommodityID, rateBases, windowStart, toDate, boundary)
	if err != nil {
		return nil, err
	}
	for _, rate := range rates {
		table.byCommodity[rate.BaseCommodityID] = append(table.byCommodity[rate.BaseCommodityID], rate)
	}

	pairs := make([][2]int64, 0, len(rateBases))
	for _, id := range rateBases {
		pairs = append(pairs, [2]int64{id, reporting.CommodityID})
	}
	table.diagnostics, err = pricing.ObservationDiagnostics(ctx, BookID, pairs, boundary)
	if err != nil {
		return nil, err
	}

	return table, nil
}

// QuoteCommodityID is the currency this table converts into.
func (t *RateTable) QuoteCommodityID() int64 { return t.quoteCommodityID }

// Scale is the reporting currency's own scale — what converted figures are
// rounded to, once, at the end of each conversion.
func (t *RateTable) Scale() int { return t.quoteScale }

// Convert values one amount of one commodity as at a date.
//
// Returns ok=false when no rate applies, having recorded why; the caller omits
// the figure rather than substituting a zero, which would read as "nothing"
// instead of "unknown".
func (t *RateTable) Convert(value exact.Coefficient, scale int, commodityID int64, onDate string) (*big.Int, bool) {
	if commodityID == t.quoteCommodityID {
		// The reporting currency converts to itself at 1, exactly, with no
		// observation involved and nothing to round.
		aligned := exact.ScaledIntFromCoefficient(value, scale)
		aligned.Align(t.quoteScale)
		return aligned.BigInt(), true
	}

	rate, found := t.observationOnOrBefore(t.byCommodity[commodityID], onDate)
	if !found {
		return t.convertChained(value, scale, commodityID, onDate)
	}

	converted, err := exact.MulDivRound(
		value.BigInt(), scale,
		big.NewInt(rate.PriceValue), rate.PriceScale,
		big.NewInt(rate.BaseQuantityValue), rate.BaseQuantityScale,
		t.quoteScale,
	)
	if err != nil {
		t.gaps[commodityID] = RateGap{CommodityID: commodityID, Reason: RateMissingNoObservation}
		return nil, false
	}

	stale := rate.ValuationDate != onDate
	// Preserve each selected observation/date, so a later estimated or stale
	// use cannot disappear behind the first exact quote of that currency.
	t.used[rateUseKey{commodityID, rate.ObservationID, 0, onDate}] = RateUse{
		ObservationID: rate.ObservationID, Approximate: rate.IsApproximate,
		CommodityID: commodityID, ObservationDate: rate.ValuationDate, RequestedDate: onDate, Stale: stale, Derived: rate.IsDerived,
	}

	return converted, true
}

// convertChained values a commodity with no direct reporting rate through a
// currency it is priced in: value × price in that currency × that currency's
// reporting rate, rounded once (#281). A direct rate, when usable, always wins;
// a chain never stands in for a missing leg.
//
// With several usable quote currencies, the freshest first leg wins, then the
// freshest second leg, then the lowest currency id, so the choice is
// deterministic. When a price exists but no quote currency converts, the gap
// names that currency, because its rate is what the owner has to add.
func (t *RateTable) convertChained(value exact.Coefficient, scale int, commodityID int64, onDate string) (*big.Int, bool) {
	byQuote := t.chainQuotes[commodityID]
	quoteIDs := make([]int64, 0, len(byQuote))
	for quoteID := range byQuote {
		quoteIDs = append(quoteIDs, quoteID)
	}
	sort.Slice(quoteIDs, func(i, j int) bool { return quoteIDs[i] < quoteIDs[j] })

	var first, second db.ReportingRateRecord
	var found bool
	var missingLeg int64
	var missingLegDate string
	for _, quoteID := range quoteIDs {
		price, ok := t.observationOnOrBefore(byQuote[quoteID], onDate)
		if !ok {
			continue
		}
		rate, ok := t.observationOnOrBefore(t.byCommodity[quoteID], onDate)
		if !ok {
			if missingLeg == 0 || price.ValuationDate > missingLegDate {
				missingLeg, missingLegDate = quoteID, price.ValuationDate
			}
			continue
		}
		if !found || price.ValuationDate > first.ValuationDate ||
			(price.ValuationDate == first.ValuationDate && rate.ValuationDate > second.ValuationDate) {
			first, second, found = price, rate, true
		}
	}
	if !found {
		if missingLeg != 0 {
			t.recordChainGap(commodityID, missingLeg, onDate)
		} else {
			t.recordGap(commodityID, onDate)
		}
		return nil, false
	}

	converted, err := exact.MulDivRound(
		value.BigInt(), scale,
		new(big.Int).Mul(big.NewInt(first.PriceValue), big.NewInt(second.PriceValue)), first.PriceScale+second.PriceScale,
		new(big.Int).Mul(big.NewInt(first.BaseQuantityValue), big.NewInt(second.BaseQuantityValue)), first.BaseQuantityScale+second.BaseQuantityScale,
		t.quoteScale,
	)
	if err != nil {
		t.gaps[commodityID] = RateGap{CommodityID: commodityID, Reason: RateMissingNoObservation}
		return nil, false
	}

	t.used[rateUseKey{commodityID, first.ObservationID, second.ObservationID, onDate}] = RateUse{
		ObservationID: first.ObservationID, Approximate: first.IsApproximate || second.IsApproximate,
		CommodityID: commodityID, ObservationDate: first.ValuationDate, RequestedDate: onDate,
		Stale:   first.ValuationDate != onDate || second.ValuationDate != onDate,
		Derived: true, ViaCommodityID: second.BaseCommodityID, ViaObservationID: second.ObservationID, ViaObservationDate: second.ValuationDate,
	}
	return converted, true
}

// rateFor finds a commodity's direct reporting rate, recording a gap on a miss.
func (t *RateTable) rateFor(commodityID int64, onDate string) (db.ReportingRateRecord, bool) {
	rate, found := t.observationOnOrBefore(t.byCommodity[commodityID], onDate)
	if !found {
		t.recordGap(commodityID, onDate)
	}
	return rate, found
}

// observationOnOrBefore finds the applicable observation: the latest one on or
// before the date, provided it is inside the staleness window. It records
// nothing; the caller decides which gap a miss is.
func (t *RateTable) observationOnOrBefore(observations []db.ReportingRateRecord, onDate string) (db.ReportingRateRecord, bool) {
	if len(observations) == 0 {
		return db.ReportingRateRecord{}, false
	}

	earliestUsable, err := shiftISODate(onDate, -t.stalenessDays)
	if err != nil {
		return db.ReportingRateRecord{}, false
	}

	var best db.ReportingRateRecord
	var found bool
	for _, observation := range observations {
		if observation.ValuationDate > onDate {
			break
		}
		if observation.ValuationDate < earliestUsable {
			continue
		}
		best, found = observation, true
	}
	return best, found
}

func (t *RateTable) recordGap(commodityID int64, onDate string) {
	if _, ok := t.gaps[commodityID]; ok {
		return
	}
	t.gaps[commodityID] = t.pairGap(commodityID, commodityID, onDate)
}

// recordChainGap records a commodity whose price leg exists but whose quote
// currency has no usable reporting rate; the reason is the currency's.
func (t *RateTable) recordChainGap(commodityID, viaCommodityID int64, onDate string) {
	if _, ok := t.gaps[commodityID]; ok {
		return
	}
	gap := t.pairGap(commodityID, viaCommodityID, onDate)
	gap.ViaCommodityID = viaCommodityID
	t.gaps[commodityID] = gap
}

// pairGap explains why rateCommodityID has no usable reporting rate on onDate,
// reported against commodityID.
func (t *RateTable) pairGap(commodityID, rateCommodityID int64, onDate string) RateGap {
	gap := RateGap{CommodityID: commodityID, Reason: RateMissingNoObservation}
	reason := t.diagnostics[[2]int64{rateCommodityID, t.quoteCommodityID}]
	if reason != "" && reason != "usable" {
		gap.Reason = reason
	}
	// Never identify an observation after this specific bucket/posting as a
	// nearest prior rate, even when a later bucket in the same report has one.
	for _, observation := range t.byCommodity[rateCommodityID] {
		if observation.ValuationDate > onDate {
			break
		}
		gap.NearestObservationDate = observation.ValuationDate
	}
	if gap.NearestObservationDate == "" && reason == "usable" {
		gap.Reason = "future_only"
	}
	return gap
}

// Coverage reports what the table actually did, for the response to carry.
func (t *RateTable) Coverage(code string) ValuationCoverage {
	coverage := ValuationCoverage{
		SelectionPolicy: "live_raw_v1", ValuationAsOf: t.boundary.AsOf, KnowledgeCutoff: t.boundary.RecordedAt,
		CommodityID:      t.quoteCommodityID,
		Code:             code,
		Scale:            t.quoteScale,
		Method:           ValuationMethodObservedOnOrBefore,
		MaxStalenessDays: t.stalenessDays,
		Complete:         len(t.gaps) == 0,
	}
	for _, use := range t.used {
		coverage.Used = append(coverage.Used, use)
	}
	for _, gap := range t.gaps {
		coverage.Gaps = append(coverage.Gaps, gap)
	}
	sort.Slice(coverage.Used, func(i, j int) bool {
		a, b := coverage.Used[i], coverage.Used[j]
		if a.CommodityID != b.CommodityID {
			return a.CommodityID < b.CommodityID
		}
		return a.RequestedDate < b.RequestedDate
	})
	sort.Slice(coverage.Gaps, func(i, j int) bool { return coverage.Gaps[i].CommodityID < coverage.Gaps[j].CommodityID })
	return coverage
}

// ConvertedTotal accumulates converted amounts. It is a plain sum: every
// addend has already been rounded to the reporting currency's scale, so there
// is nothing left to align.
type ConvertedTotal struct {
	total   *big.Int
	scale   int
	partial bool
}

func NewConvertedTotal(scale int) *ConvertedTotal {
	return &ConvertedTotal{total: new(big.Int), scale: scale}
}

// Add includes one converted amount. Adding an unconvertible amount marks the
// total partial rather than skipping it silently.
func (c *ConvertedTotal) Add(value *big.Int, ok bool) {
	if !ok {
		c.partial = true
		return
	}
	c.total.Add(c.total, value)
}

// Value returns the total and whether it accounts for everything it was asked
// to. A partial total is not returned as a number: a figure that quietly means
// "most of it" is worse than no figure.
func (c *ConvertedTotal) Value() (exact.Coefficient, int, bool) {
	if c.partial {
		return "", c.scale, false
	}
	coefficient, err := exact.FromBig(c.total)
	if err != nil {
		return "", c.scale, false
	}
	return coefficient, c.scale, true
}

func shiftISODate(date string, days int) (string, error) {
	parsed, err := time.Parse(time.DateOnly, date)
	if err != nil {
		return "", fmt.Errorf("parse date %q: %w", date, err)
	}
	return parsed.AddDate(0, 0, days).Format(time.DateOnly), nil
}
