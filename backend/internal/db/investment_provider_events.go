package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"rekenraam/backend/internal/exact"
)

var (
	// ErrEventLinkTarget means the transaction is not an effective posted
	// cash dividend of the event's instrument in its window, or another
	// provider event already admitted it.
	ErrEventLinkTarget = errors.New("transaction cannot be linked to this provider event")
)

// DividendEventLinkWindowDays bounds how long after an ex-date an owner-recorded
// payment is offered as the same dividend. Payment usually follows within weeks;
// the window is a discovery aid only, the owner still chooses the link.
const DividendEventLinkWindowDays = 90

// EntitlementHoldings is the dated position of one instrument at the close of
// the day before an ex-date, per holding account (#166 slot semantics).
type EntitlementHoldings struct {
	Long  []DatedHolding
	Short []DatedHolding
}

// DividendEntitlementHoldings replays every position of commodityID up to
// asOf in one rolled-back read. It is evidence for a suggestion only; the
// dividend writer never consumes it.
func (r *PricingRepository) DividendEntitlementHoldings(ctx context.Context, bookID, commodityID int64, asOf string) (EntitlementHoldings, error) {
	if !isDisposalCalendarDate(asOf) {
		return EntitlementHoldings{}, fmt.Errorf("%w: entitlement needs a calendar date", ErrInvalidDisposalParams)
	}
	tx, err := r.database.BeginTx(ctx, nil)
	if err != nil {
		return EntitlementHoldings{}, fmt.Errorf("begin entitlement read: %w", err)
	}
	defer rollbackTx(ctx, tx)
	rows, err := tx.QueryContext(ctx, `
		SELECT DISTINCT account_id, cost_commodity_id, position_side
		FROM investment_lots
		WHERE book_id = ? AND commodity_id = ? AND opened_on <= ?
		ORDER BY account_id, cost_commodity_id, position_side`, bookID, commodityID, asOf)
	if err != nil {
		return EntitlementHoldings{}, fmt.Errorf("read entitlement positions: %w", err)
	}
	type position struct {
		key  investmentReplayPositionKey
		side string
	}
	var positions []position
	for rows.Next() {
		p := position{key: investmentReplayPositionKey{commodityID: commodityID}}
		if err := rows.Scan(&p.key.accountID, &p.key.costCommodityID, &p.side); err != nil {
			rows.Close()
			return EntitlementHoldings{}, fmt.Errorf("scan entitlement position: %w", err)
		}
		positions = append(positions, p)
	}
	if err := errors.Join(rows.Err(), rows.Close()); err != nil {
		return EntitlementHoldings{}, fmt.Errorf("iterate entitlement positions: %w", err)
	}
	var result EntitlementHoldings
	for _, p := range positions {
		holding, ok, err := datedHoldingTx(ctx, tx, bookID, p.key, p.side, asOf)
		if err != nil {
			return EntitlementHoldings{}, err
		}
		if !ok {
			continue
		}
		if p.side == PositionSideShort {
			result.Short = append(result.Short, holding)
		} else {
			result.Long = append(result.Long, holding)
		}
	}
	return result, nil
}

type StageProviderEventParams struct {
	BookID          int64
	ActorUserID     int64
	SourceID        int64
	ProviderEventID string
	InstrumentID    int64
	EventFamily     string
	EventDate       string
	NormalizedJSON  string
	RawJSON         string
	ConfidenceBPS   int
	ProposedJSON    string
	RecordedAt      string
}

// StageProviderEvent writes one immutable provider event and its pending
// suggestion under one audit event. A repeated fetch of the same source
// identity changes nothing and reports staged=false: the first observation,
// and every owner decision recorded against it, stay authoritative.
func (r *PricingRepository) StageProviderEvent(ctx context.Context, params StageProviderEventParams) (bool, error) {
	tx, err := r.database.BeginTx(ctx, nil)
	if err != nil {
		return false, fmt.Errorf("begin stage provider event: %w", err)
	}
	defer rollbackTx(ctx, tx)
	var existing int64
	err = tx.QueryRowContext(ctx, `SELECT id FROM investment_provider_events WHERE source_id = ? AND provider_event_id = ?`,
		params.SourceID, params.ProviderEventID).Scan(&existing)
	if err == nil {
		return false, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return false, fmt.Errorf("read provider event identity: %w", err)
	}
	auditEventID, err := insertAuditEvent(ctx, tx, AuditEventParams{BookID: params.BookID, ActorUserID: params.ActorUserID, OccurredAt: params.RecordedAt,
		OriginType: "internal", Operation: "investment.provider_event.stage", Reason: "staged provider event for review"})
	if err != nil {
		return false, err
	}
	result, err := tx.ExecContext(ctx, `
		INSERT INTO investment_provider_events (book_id, source_id, provider_event_id, instrument_id, event_family, event_date,
			status, normalized_json, raw_json, created_at, created_audit_event_id)
		VALUES (?, ?, ?, ?, ?, ?, 'new', ?, ?, ?, ?)`,
		params.BookID, params.SourceID, params.ProviderEventID, params.InstrumentID, params.EventFamily, params.EventDate,
		params.NormalizedJSON, params.RawJSON, params.RecordedAt, auditEventID)
	if err != nil {
		return false, fmt.Errorf("insert provider event: %w", err)
	}
	eventID, err := result.LastInsertId()
	if err != nil {
		return false, fmt.Errorf("read provider event id: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO investment_event_suggestions (book_id, provider_event_id, instrument_id, confidence_bps, status,
			proposed_transaction_json, created_at, updated_at, created_audit_event_id, updated_audit_event_id)
		VALUES (?, ?, ?, ?, 'suggested', ?, ?, ?, ?, ?)`,
		params.BookID, eventID, params.InstrumentID, params.ConfidenceBPS, params.ProposedJSON,
		params.RecordedAt, params.RecordedAt, auditEventID, auditEventID); err != nil {
		return false, fmt.Errorf("insert provider event suggestion: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return false, fmt.Errorf("commit stage provider event: %w", err)
	}
	return true, nil
}

// DividendLinkCandidate is an effective posted cash dividend an owner may link
// to a pending provider dividend suggestion instead of posting it again.
type DividendLinkCandidate struct {
	SuggestionID    int64
	TransactionID   int64
	TransactionDate string
	CashCode        string
	Gross           *exact.ScaledInt
}

const dividendLinkCandidatesSQL = `
	SELECT s.id, tv.transaction_id, o.event_date, cash.code, c.amount_value, c.amount_scale
	FROM investment_event_suggestions s
	JOIN investment_provider_events e ON e.id = s.provider_event_id AND e.book_id = s.book_id
	JOIN investment_instruments ii ON ii.id = s.instrument_id
	JOIN investment_dividend_facts facts ON facts.commodity_id = ii.commodity_id AND facts.book_id = s.book_id AND facts.withholding_only = 0
	JOIN effective_investment_operations o ON o.id = facts.operation_id AND o.operation_kind = 'dividend'
	JOIN investment_operation_journal_links link ON link.operation_id = o.id AND link.book_id = o.book_id AND link.role = 'primary'
	JOIN transaction_versions linked ON linked.id = link.transaction_version_id
	JOIN transactions t ON t.id = linked.transaction_id AND t.deleted_at IS NULL
	JOIN current_transaction_versions tv ON tv.transaction_id = t.id AND tv.status = 'posted'
	JOIN investment_operation_components c ON c.operation_id = o.id AND c.component_kind = 'dividend_gross'
	JOIN commodities cash ON cash.id = c.commodity_id
	WHERE s.book_id = ? AND s.status = 'suggested' AND e.event_family = 'dividend'
		AND o.event_date >= e.event_date AND o.event_date <= date(e.event_date, '+' || ? || ' days')
		AND NOT EXISTS (SELECT 1 FROM investment_event_suggestions other
			WHERE other.generated_transaction_id = t.id OR other.linked_transaction_id = t.id)`

// DividendLinkCandidates composes every pending dividend suggestion's
// candidates in one read so the review screen needs one request.
func (r *InvestmentRepository) DividendLinkCandidates(ctx context.Context, bookID int64) ([]DividendLinkCandidate, error) {
	rows, err := r.database.QueryContext(ctx, dividendLinkCandidatesSQL+` ORDER BY s.id, o.event_date, t.id, c.component_seq`, bookID, DividendEventLinkWindowDays)
	if err != nil {
		return nil, fmt.Errorf("read dividend link candidates: %w", err)
	}
	var result []DividendLinkCandidate
	for rows.Next() {
		var candidate DividendLinkCandidate
		var value exact.Coefficient
		var scale int
		if err := rows.Scan(&candidate.SuggestionID, &candidate.TransactionID, &candidate.TransactionDate, &candidate.CashCode, &value, &scale); err != nil {
			rows.Close()
			return nil, fmt.Errorf("scan dividend link candidate: %w", err)
		}
		last := len(result) - 1
		if last < 0 || result[last].SuggestionID != candidate.SuggestionID || result[last].TransactionID != candidate.TransactionID {
			candidate.Gross = exact.NewScaledInt()
			result = append(result, candidate)
			last++
		}
		result[last].Gross.AddCoefficient(value, scale)
	}
	if err := errors.Join(rows.Err(), rows.Close()); err != nil {
		return nil, fmt.Errorf("iterate dividend link candidates: %w", err)
	}
	return result, nil
}

type LinkSuggestionParams struct {
	BookID        int64
	SuggestionID  int64
	TransactionID int64
	ActorUserID   int64
	AuthSessionID int64
	RequestID     string
	RecordedAt    string
}

// LinkSuggestion records that an owner-recorded dividend is this provider
// event. The candidate check, the suggestion transition and the event status
// share one transaction, so a concurrent accept, ignore or second link cannot
// admit the event or the payment twice.
func (r *InvestmentRepository) LinkSuggestion(ctx context.Context, params LinkSuggestionParams) (EventSuggestionRecord, error) {
	tx, err := r.database.BeginTx(ctx, nil)
	if err != nil {
		return EventSuggestionRecord{}, fmt.Errorf("begin link event suggestion: %w", err)
	}
	defer rollbackTx(ctx, tx)
	current, err := eventSuggestionByIDTx(ctx, tx, params.BookID, params.SuggestionID)
	if err != nil {
		return EventSuggestionRecord{}, err
	}
	if current.Status != "suggested" {
		return EventSuggestionRecord{}, ErrEventSuggestionNotPending
	}
	var found int64
	err = tx.QueryRowContext(ctx, dividendLinkCandidatesSQL+` AND s.id = ? AND t.id = ? LIMIT 1`,
		params.BookID, DividendEventLinkWindowDays, params.SuggestionID, params.TransactionID).Scan(&found, new(int64), new(string), new(string), new(exact.Coefficient), new(int))
	if errors.Is(err, sql.ErrNoRows) {
		return EventSuggestionRecord{}, ErrEventLinkTarget
	}
	if err != nil {
		return EventSuggestionRecord{}, fmt.Errorf("check dividend link target: %w", err)
	}
	auditEventID, err := insertAuditEvent(ctx, tx, AuditEventParams{BookID: params.BookID, ActorUserID: params.ActorUserID, AuthSessionID: params.AuthSessionID,
		OccurredAt: params.RecordedAt, RequestID: params.RequestID, OriginType: "browser_api", Operation: "investment.event_suggestion.link",
		Reason: "linked provider event to recorded dividend"})
	if err != nil {
		return EventSuggestionRecord{}, err
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE investment_event_suggestions
		SET status = 'linked', linked_transaction_id = ?, updated_at = ?, updated_by_user_id = ?, updated_audit_event_id = ?
		WHERE book_id = ? AND id = ? AND status = 'suggested'`,
		params.TransactionID, params.RecordedAt, params.ActorUserID, auditEventID, params.BookID, params.SuggestionID); err != nil {
		return EventSuggestionRecord{}, fmt.Errorf("link event suggestion: %w", err)
	}
	if err := setProviderEventStatusTx(ctx, tx, params.BookID, current.ProviderEventID, "matched"); err != nil {
		return EventSuggestionRecord{}, err
	}
	record, err := eventSuggestionByIDTx(ctx, tx, params.BookID, params.SuggestionID)
	if err != nil {
		return EventSuggestionRecord{}, err
	}
	if err := tx.Commit(); err != nil {
		return EventSuggestionRecord{}, fmt.Errorf("commit link event suggestion: %w", err)
	}
	return record, nil
}

// setProviderEventStatusTx moves a new event to the owner's decision. The raw
// and normalized evidence never change.
func setProviderEventStatusTx(ctx context.Context, tx *sql.Tx, bookID, eventID int64, status string) error {
	result, err := tx.ExecContext(ctx, `UPDATE investment_provider_events SET status = ? WHERE book_id = ? AND id = ? AND status = 'new'`, status, bookID, eventID)
	if err != nil {
		return fmt.Errorf("update provider event status: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read provider event status rows: %w", err)
	}
	if rows != 1 {
		return ErrEventSuggestionNotPending
	}
	return nil
}
