package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

type QuoteRefreshTarget struct {
	AssignmentID, BaseCommodityID, QuoteCommodityID, SourceID, InstrumentID int64
	InstrumentType, IdentifiersJSON, MIC, ProviderKey, BaseCode, QuoteCode  string
}

// Explicit dated assignments opt in to network fetching. There is no ticker
// guessing or fallback from a currency-wide FX default into equities/crypto.
func (r *PricingRepository) QuoteRefreshTargets(ctx context.Context, bookID int64, date string, sourceID int64) ([]QuoteRefreshTarget, error) {
	rows, err := r.database.QueryContext(ctx, `SELECT a.id,a.base_commodity_id,a.quote_commodity_id,a.source_id,ii.id,
 iiv.instrument_type,iiv.identifiers_json,COALESCE(iiv.mic,''),COALESCE(s.provider_key,s.code),b.code,q.code
 FROM pricing_source_assignments a
 JOIN investment_instruments ii ON ii.book_id=a.book_id AND ii.commodity_id=a.base_commodity_id
 JOIN `+CurrentReferenceVersions(ctx, "investment_instrument")+` iiv ON iiv.instrument_id=ii.id AND iiv.status='active'
 JOIN commodities b ON b.id=a.base_commodity_id
 JOIN commodities q ON q.id=a.quote_commodity_id AND q.kind='currency'
 JOIN `+CurrentReferenceVersions(ctx, "commodity")+` qv ON qv.commodity_id=q.id AND qv.status='active'
 JOIN market_data_sources s ON s.id=a.source_id AND s.status='active'
 WHERE a.book_id=? AND a.status='active' AND a.effective_from<=? AND (a.effective_to IS NULL OR a.effective_to>=?)
 AND (?=0 OR a.source_id=?)
 ORDER BY a.priority,a.id`, bookID, date, date, sourceID, sourceID)
	if err != nil {
		return nil, fmt.Errorf("list assigned quote targets: %w", err)
	}
	defer rows.Close()
	var targets []QuoteRefreshTarget
	for rows.Next() {
		var target QuoteRefreshTarget
		if err := rows.Scan(&target.AssignmentID, &target.BaseCommodityID, &target.QuoteCommodityID, &target.SourceID, &target.InstrumentID, &target.InstrumentType, &target.IdentifiersJSON, &target.MIC, &target.ProviderKey, &target.BaseCode, &target.QuoteCode); err != nil {
			return nil, err
		}
		targets = append(targets, target)
	}
	return targets, rows.Err()
}

// StoreProviderQuote keeps retry identity, any same-timestamp correction,
// observation and audit atomic. The caller has already fetched/validated the
// remote fact outside this write transaction.
func (r *PricingRepository) StoreProviderQuote(ctx context.Context, params CreatePriceObservationParams, originalProviderID string) (PriceObservationRecord, bool, error) {
	tx, err := r.database.BeginTx(ctx, nil)
	if err != nil {
		return PriceObservationRecord{}, false, err
	}
	defer tx.Rollback()
	existing, err := scanPriceObservationRow(tx.QueryRowContext(ctx, priceObservationSelect(` WHERE source_id=? AND provider_observation_id=?`), params.Spec.SourceID.Int64, params.Spec.ProviderObservationID))
	if err == nil {
		return existing, true, nil
	}
	if !errors.Is(err, ErrNotFound) {
		return PriceObservationRecord{}, false, err
	}
	var previousID int64
	err = tx.QueryRowContext(ctx, `SELECT id FROM price_observations WHERE book_id=? AND source_id=? AND base_commodity_id=? AND quote_commodity_id=? AND json_extract(metadata_json,'$.provider_observation_id')=? ORDER BY id DESC LIMIT 1`, params.BookID, params.Spec.SourceID.Int64, params.Spec.BaseCommodityID, params.Spec.QuoteCommodityID, originalProviderID).Scan(&previousID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return PriceObservationRecord{}, false, err
	}
	if err == nil {
		params.Spec.SupersedesObservationID = sql.NullInt64{Int64: previousID, Valid: true}
	}
	auditID, err := insertAuditEvent(ctx, tx, AuditEventParams{BookID: params.BookID, ActorUserID: params.ActorUserID, OriginType: params.OriginType, Operation: params.Operation, OccurredAt: params.CreatedAt, Reason: params.ChangeReason})
	if err != nil {
		return PriceObservationRecord{}, false, err
	}
	record, err := insertPriceObservationTx(ctx, tx, params, auditID)
	if err != nil {
		return PriceObservationRecord{}, false, err
	}
	if err := tx.Commit(); err != nil {
		return PriceObservationRecord{}, false, err
	}
	return record, false, nil
}
