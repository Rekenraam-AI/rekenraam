package api

import (
	"rekenraam/backend/internal/app"
	"rekenraam/backend/internal/exact"
)

type capitalReturnReadResponse struct {
	OperationID         int64                         `json:"operation_id"`
	TransactionID       int64                         `json:"transaction_id"`
	HoldingAccountID    int64                         `json:"holding_account_id"`
	CommodityID         int64                         `json:"commodity_id"`
	CurrencyID          int64                         `json:"currency_id"`
	HoldingName         string                        `json:"holding_name"`
	InstrumentName      string                        `json:"instrument_name"`
	CurrencyCode        string                        `json:"currency_code"`
	CurrencyScale       int                           `json:"currency_scale"`
	EffectiveOn         string                        `json:"effective_on"`
	PaymentOn           string                        `json:"payment_on"`
	AmountValue         exact.Coefficient             `json:"amount_value"`
	AmountScale         int                           `json:"amount_scale"`
	EntitlementRule     string                        `json:"entitlement_rule"`
	SourceEvidenceJSON  string                        `json:"source_evidence_json"`
	RevisionID          *int64                        `json:"revision_id"`
	RevisionSeq         int                           `json:"revision_seq"`
	CausedByOperationID *int64                        `json:"caused_by_operation_id"`
	AuditEventID        int64                         `json:"audit_event_id"`
	CreatedAt           string                        `json:"created_at"`
	Effective           bool                          `json:"effective"`
	Effects             []capitalReturnEffectResponse `json:"effects"`
	ReductionValue      *exact.Coefficient            `json:"reduction_value"`
	ReductionScale      *int                          `json:"reduction_scale"`
	ExcessValue         *exact.Coefficient            `json:"excess_value"`
	ExcessScale         *int                          `json:"excess_scale"`
	UnknownLots         int                           `json:"unknown_lots"`
	HasExcess           bool                          `json:"has_excess"`
}

func toCapitalReturnReads(reads []app.CapitalReturnRead) []capitalReturnReadResponse {
	out := make([]capitalReturnReadResponse, 0, len(reads))
	for _, read := range reads {
		knowledge := "known"
		if read.UnknownLots > 0 {
			knowledge = "unknown"
		}
		effects := make([]app.CapitalReturnEffect, 0, len(read.Effects))
		for _, effect := range read.Effects {
			effects = append(effects, app.CapitalReturnEffect{BasisKnowledge: effect.BasisKnowledge, LotID: effect.LotID, EntitledQuantityValue: effect.EntitledQuantityValue, EntitledQuantityScale: effect.EntitledQuantityScale, AllocatedValue: effect.AllocatedValue, AllocatedScale: effect.AllocatedScale, ReductionValue: effect.ReductionValue, ReductionScale: effect.ReductionScale, ExcessValue: effect.ExcessValue, ExcessScale: effect.ExcessScale})
		}
		response := capitalReturnReadResponse{
			OperationID: read.OperationID, TransactionID: read.TransactionID, HoldingAccountID: read.AccountID,
			CommodityID: read.CommodityID, CurrencyID: read.CostCommodityID, HoldingName: read.HoldingName,
			InstrumentName: read.InstrumentName, CurrencyCode: read.CurrencyCode, CurrencyScale: read.CurrencyScale,
			EffectiveOn: read.EffectiveOn, PaymentOn: read.PaymentOn, AmountValue: read.AmountValue, AmountScale: read.AmountScale,
			EntitlementRule: read.EntitlementRule, SourceEvidenceJSON: read.SourceEvidenceJSON, RevisionSeq: read.RevisionSeq,
			AuditEventID: read.AuditEventID, CreatedAt: read.CreatedAt, Effective: read.Effective, Effects: toCapitalReturnEffectResponses(effects),
			ReductionValue: knownCoefficient(read.ReductionValue, knowledge), ReductionScale: projectedBasisScale(read.ReductionScale, knowledge),
			ExcessValue: knownCoefficient(read.ExcessValue, knowledge), ExcessScale: projectedBasisScale(read.ExcessScale, knowledge),
			UnknownLots: read.UnknownLots, HasExcess: read.HasExcess,
		}
		if read.RevisionID > 0 {
			id := read.RevisionID
			response.RevisionID = &id
		}
		if read.CausedByOperationID > 0 {
			id := read.CausedByOperationID
			response.CausedByOperationID = &id
		}
		out = append(out, response)
	}
	return out
}
