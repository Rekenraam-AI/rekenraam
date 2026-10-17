package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math/big"
	"time"

	"rekenraam/backend/internal/db"
	"rekenraam/backend/internal/exact"
	"rekenraam/backend/internal/marketdata"
)

// dividendEventWorkKind fetches one assigned listing's dividend announcements
// (#111). It shares the quote worker's lifecycle and durable queue.
const dividendEventWorkKind = "investment.dividend_events"

// dividendEventLookbackDays bounds one fetch. A dividend older than a year is
// still entered manually; the producer is a convenience, not coverage.
const dividendEventLookbackDays = 365

// Provider dividend reasons are stable codes the frontend localizes.
const (
	dividendMissingPaymentDate     = "payment_date"
	dividendMissingCashAccount     = "cash_account"
	dividendMissingReceivedAmount  = "received_amount"
	dividendMissingCurrency        = "dividend_currency"
	dividendMissingAmountPerShare  = "amount_per_share"
	dividendUnsupportedShort       = "short_position"
	proposedTransactionKindInfoRow = "informational"
)

type dividendEventPayload struct {
	Reason       string `json:"reason"`
	AssignmentID int64  `json:"assignment_id"`
}

// providerDecimal is an exact decimal as JSON: coefficient string plus scale.
type providerDecimal struct {
	Value string `json:"value"`
	Scale int    `json:"scale"`
}

type providerDividendHolding struct {
	AccountID int64           `json:"account_id"`
	Quantity  providerDecimal `json:"quantity"`
}

// providerDividendEvidence is investment_provider_events.normalized_json for a
// provider cash dividend announcement (schema 1). It keeps what the source
// states and what the ledger shows separately from what the owner must supply.
type providerDividendEvidence struct {
	Schema          int                       `json:"schema"`
	Provider        string                    `json:"provider"`
	Symbol          string                    `json:"symbol"`
	MIC             string                    `json:"mic"`
	ExDate          string                    `json:"ex_date"`
	EntitlementDate string                    `json:"entitlement_date"`
	AmountPerShare  *providerDecimal          `json:"amount_per_share"`
	ListingCurrency string                    `json:"listing_currency"`
	Holdings        []providerDividendHolding `json:"holdings"`
	ShortHoldings   []providerDividendHolding `json:"short_holdings,omitempty"`
	EntitledUnits   *providerDecimal          `json:"entitled_units"`
	EstimatedGross  *providerDecimal          `json:"estimated_gross"`
	Missing         []string                  `json:"missing"`
	Unsupported     string                    `json:"unsupported,omitempty"`
}

// QueueDividendEventFetch accepts durable work for every listing whose current
// quote assignment uses a verified dividend provider. It never downloads
// synchronously and never writes ledger state.
func (s *PricingService) QueueDividendEventFetch(ctx context.Context, ownerID int64) (int, error) {
	if ownerID <= 0 {
		return 0, ValidationError{Message: "owner user is required"}
	}
	targets, err := s.currentQuoteTargets(ctx, 0)
	if err != nil {
		return 0, err
	}
	now := s.now().UTC().Format(time.RFC3339)
	queued := 0
	for _, target := range targets {
		if _, ok := s.providerRegistry.DividendProvider(target.ProviderKey); !ok || target.InstrumentType == "crypto" {
			continue
		}
		payload, _ := json.Marshal(dividendEventPayload{Reason: "manual", AssignmentID: target.AssignmentID})
		if _, err := s.repository.EnqueueBackgroundWork(ctx, BookID, dividendEventWorkKind, string(payload), now); err != nil {
			return 0, err
		}
		queued++
	}
	select {
	case s.wake <- struct{}{}:
	default:
	}
	return queued, nil
}

func (s *PricingService) runDueDividendEvents(ctx context.Context, logger *slog.Logger, workerID string) {
	for range 4 {
		now := s.now().UTC()
		item, err := s.repository.ClaimBackgroundWork(ctx, dividendEventWorkKind, workerID, now.Format(time.RFC3339), now.Add(15*time.Minute).Format(time.RFC3339))
		if errors.Is(err, db.ErrNotFound) || ctx.Err() != nil {
			return
		}
		if err != nil {
			logger.WarnContext(ctx, "claim dividend event work", slog.Any("err", err))
			return
		}
		s.processDividendEvents(ctx, logger, workerID, item)
	}
}

func (s *PricingService) processDividendEvents(ctx context.Context, logger *slog.Logger, workerID string, item db.BackgroundWorkItemRecord) {
	var payload dividendEventPayload
	var workErr error
	if item.PayloadVersion != 1 || json.Unmarshal([]byte(item.PayloadJSON), &payload) != nil || payload.AssignmentID <= 0 {
		workErr = &marketdata.QuoteError{Status: 400, Terminal: true}
	} else {
		targets, err := s.currentQuoteTargets(ctx, 0)
		workErr = err
		if err == nil {
			// An assignment ended since queueing has nothing left to fetch.
			for _, target := range targets {
				if target.AssignmentID == payload.AssignmentID {
					workErr = s.stageDividendEvents(ctx, target)
					break
				}
			}
		}
	}
	now := s.now().UTC()
	var stateErr error
	if workErr == nil {
		stateErr = s.repository.CompleteBackgroundWork(ctx, item.ID, workerID, now.Format(time.RFC3339))
	} else if terminalQuoteError(workErr) || item.Attempts >= maxFXCoverageAttempts {
		stateErr = s.repository.FailBackgroundWork(ctx, item.ID, workerID, now.Format(time.RFC3339), workErr.Error())
	} else {
		delay := retryDelay(item.Attempts, item.ID)
		var providerErr *marketdata.QuoteError
		if errors.As(workErr, &providerErr) {
			delay = max(delay, providerErr.RetryAfter)
		}
		stateErr = s.repository.RetryBackgroundWork(ctx, item.ID, workerID, now.Add(delay).Format(time.RFC3339), now.Format(time.RFC3339), workErr.Error())
	}
	if stateErr != nil && ctx.Err() == nil {
		logger.WarnContext(ctx, "record dividend event work outcome", slog.Int64("work_id", item.ID), slog.Any("err", stateErr))
	}
}

// stageDividendEvents fetches outside any database transaction, then stages
// each announcement the ledger shows the owner was positioned for at the
// close before its ex-date. Unheld announcements are not this book's events
// and are not retained; a later fetch stages them if holdings are backfilled.
func (s *PricingService) stageDividendEvents(ctx context.Context, target db.QuoteRefreshTarget) error {
	provider, ok := s.providerRegistry.DividendProvider(target.ProviderKey)
	if !ok {
		return marketdata.ErrUnsupportedPair
	}
	var identifiers map[string]json.RawMessage
	if json.Unmarshal([]byte(target.IdentifiersJSON), &identifiers) != nil {
		return marketdata.ErrUnsupportedPair
	}
	var symbol string
	_ = json.Unmarshal(identifiers["twelve_data_symbol"], &symbol)
	if symbol == "" || target.MIC == "" {
		return fmt.Errorf("%w: explicit provider symbol and market are required", marketdata.ErrUnsupportedPair)
	}
	zone, err := s.repository.BookOwnerTimeZone(ctx, BookID)
	if err != nil {
		return err
	}
	location, err := time.LoadLocation(zone)
	if err != nil {
		return err
	}
	today := s.now().In(location)
	observations, err := provider.FetchDividends(ctx, marketdata.DividendRequest{ProviderInstrumentID: symbol, MIC: target.MIC,
		StartDate: today.AddDate(0, 0, -dividendEventLookbackDays).Format(time.DateOnly), EndDate: today.Format(time.DateOnly)})
	if err != nil {
		return err
	}
	ownerID, err := s.repository.CurrentBookOwnerID(ctx, BookID)
	if err != nil {
		return err
	}
	for _, observation := range observations {
		// Entitlement is fixed only once the ex-date has arrived.
		if observation.ExDate > today.Format(time.DateOnly) || observation.ProviderInstrumentID != symbol || observation.MIC != target.MIC {
			continue
		}
		if err := s.stageDividendObservation(ctx, target, ownerID, observation); err != nil {
			return err
		}
	}
	return nil
}

func (s *PricingService) stageDividendObservation(ctx context.Context, target db.QuoteRefreshTarget, ownerID int64, observation marketdata.DividendObservation) error {
	exDate, err := time.Parse(time.DateOnly, observation.ExDate)
	if err != nil {
		return err
	}
	entitlementDate := exDate.AddDate(0, 0, -1).Format(time.DateOnly)
	holdings, err := s.repository.DividendEntitlementHoldings(ctx, BookID, target.BaseCommodityID, entitlementDate)
	if err != nil {
		return err
	}
	if len(holdings.Long) == 0 && len(holdings.Short) == 0 {
		return nil
	}
	evidence := providerDividendEvidence{Schema: 1, Provider: target.ProviderKey, Symbol: observation.ProviderInstrumentID, MIC: observation.MIC,
		ExDate: observation.ExDate, EntitlementDate: entitlementDate, ListingCurrency: observation.ListingCurrency,
		Holdings: providerHoldings(holdings.Long), ShortHoldings: providerHoldings(holdings.Short),
		Missing: []string{dividendMissingPaymentDate, dividendMissingCashAccount, dividendMissingReceivedAmount, dividendMissingCurrency}}
	units := exact.NewScaledInt()
	for _, holding := range holdings.Long {
		units.AddCoefficient(holding.QuantityValue, holding.QuantityScale)
	}
	if units.Sign() > 0 {
		normalized := units.Normalized()
		evidence.EntitledUnits = &providerDecimal{Value: normalized.BigInt().String(), Scale: normalized.Scale()}
	}
	if observation.AmountKnown {
		evidence.AmountPerShare = &providerDecimal{Value: fmt.Sprint(observation.AmountValue), Scale: observation.AmountScale}
		if units.Sign() > 0 {
			gross := exact.ScaledIntFromBig(new(big.Int).Mul(units.BigInt(), big.NewInt(observation.AmountValue)), units.Scale()+observation.AmountScale).Normalized()
			evidence.EstimatedGross = &providerDecimal{Value: gross.BigInt().String(), Scale: gross.Scale()}
		}
	} else {
		evidence.Missing = append(evidence.Missing, dividendMissingAmountPerShare)
	}
	proposal := map[string]any{"kind": proposedTransactionKindDividendIncome, "complete": false, "commodity_id": target.BaseCommodityID,
		"external_ref_hint": observation.ProviderObservationID, "missing": evidence.Missing}
	confidence := 7000
	if !observation.AmountKnown {
		confidence = 3000
	}
	// A short holder owes a dividend payment and the gross is not income;
	// that mapping is not verified (#176). Keep the evidence informational.
	if len(holdings.Short) > 0 {
		evidence.Unsupported = dividendUnsupportedShort
		proposal = map[string]any{"kind": proposedTransactionKindInfoRow, "reason": dividendUnsupportedShort}
		confidence = 0
	}
	normalizedJSON, err := json.Marshal(evidence)
	if err != nil {
		return err
	}
	proposedJSON, err := json.Marshal(proposal)
	if err != nil {
		return err
	}
	_, err = s.repository.StageProviderEvent(ctx, db.StageProviderEventParams{BookID: BookID, ActorUserID: ownerID, SourceID: target.SourceID,
		ProviderEventID: observation.ProviderObservationID, InstrumentID: target.InstrumentID, EventFamily: "dividend", EventDate: observation.ExDate,
		NormalizedJSON: string(normalizedJSON), RawJSON: observation.RawJSON, ConfidenceBPS: confidence, ProposedJSON: string(proposedJSON),
		RecordedAt: s.now().UTC().Format(time.RFC3339)})
	return err
}

func providerHoldings(holdings []db.DatedHolding) []providerDividendHolding {
	result := make([]providerDividendHolding, 0, len(holdings))
	for _, holding := range holdings {
		result = append(result, providerDividendHolding{AccountID: holding.AccountID, Quantity: providerDecimal{Value: holding.QuantityValue.String(), Scale: holding.QuantityScale}})
	}
	return result
}
