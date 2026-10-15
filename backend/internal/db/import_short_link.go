package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// The owner may assert that a pending unsupported provider row is the source
// evidence of a short sale or cover they already recorded natively (#245).
// Like a split link (T-122), the row's dedupe identity then names that
// operation and no journal, lot or disposal is written. Automatic provider
// short admission (#176) is a separate, evidence-gated path.

var (
	// ErrShortLinkUnavailable means the row can no longer be linked (committed,
	// discarded, or its identity already admitted) or the operation is not an
	// effective short sale or cover without source evidence.
	ErrShortLinkUnavailable = errors.New("staged row cannot be linked to this short operation")
	// ErrShortLinkChanged means the operation's facts changed after the owner
	// reviewed them, for example by a correction.
	ErrShortLinkChanged = errors.New("short operation changed since it was reviewed")
)

// ShortLinkCandidate is an effective short sale or cover with the facts a
// source row is compared against. Linked is true when this operation, or any
// operation it corrects, already has source evidence.
type ShortLinkCandidate struct {
	OperationID       int64
	OperationKind     string
	TransactionID     int64
	HoldingAccountID  int64
	CommodityID       int64
	EventDate         string
	QuantityValue     string
	QuantityScale     int
	CashAccountID     int64
	CashCommodityID   int64
	CashCommodityCode string
	NetValue          string
	NetScale          int
	Linked            bool
}

const shortLinkCandidateSelect = `
	SELECT o.id, o.operation_kind, COALESCE(v.transaction_id, 0),
		COALESCE(l.account_id, d.account_id), COALESCE(l.commodity_id, d.commodity_id), o.event_date,
		COALESCE(l.quantity_value, d.quantity_value), COALESCE(l.quantity_scale, d.quantity_scale),
		COALESCE(net.cash_account_id, 0), COALESCE(net.commodity_id, 0), COALESCE(cash.code, ''),
		COALESCE(net.amount_value, '0'), COALESCE(net.amount_scale, 0)
	FROM effective_investment_operations o
	LEFT JOIN investment_lots l ON l.operation_id = o.id AND o.operation_kind = 'short_sale' AND l.position_side = 'short'
	LEFT JOIN investment_disposal_decisions d ON d.operation_id = o.id AND o.operation_kind = 'short_cover' AND d.position_side = 'short'
	LEFT JOIN investment_operation_journal_links link ON link.operation_id = o.id AND link.book_id = o.book_id AND link.role = 'primary'
	LEFT JOIN transaction_versions v ON v.id = link.transaction_version_id
	LEFT JOIN investment_operation_components net ON net.operation_id = o.id AND net.component_kind = 'net_settlement'
		AND net.component_seq = (SELECT MIN(component_seq) FROM investment_operation_components
			WHERE operation_id = o.id AND component_kind = 'net_settlement')
	LEFT JOIN commodities cash ON cash.id = net.commodity_id
	WHERE o.book_id = ? AND o.operation_kind IN ('short_sale', 'short_cover')
		AND COALESCE(l.id, d.id) IS NOT NULL
		AND (SELECT count(*) FROM investment_lots WHERE operation_id = o.id) <= 1
		AND (SELECT count(*) FROM investment_disposal_decisions WHERE operation_id = o.id) <= 1`

type shortLinkScanner interface {
	Scan(dest ...any) error
}

func scanShortLinkCandidate(scanner shortLinkScanner) (ShortLinkCandidate, error) {
	var candidate ShortLinkCandidate
	err := scanner.Scan(&candidate.OperationID, &candidate.OperationKind, &candidate.TransactionID,
		&candidate.HoldingAccountID, &candidate.CommodityID, &candidate.EventDate,
		&candidate.QuantityValue, &candidate.QuantityScale,
		&candidate.CashAccountID, &candidate.CashCommodityID, &candidate.CashCommodityCode,
		&candidate.NetValue, &candidate.NetScale)
	return candidate, err
}

// ListShortLinkCandidates returns the effective operations of one kind
// (short_sale or short_cover) for a security, newest first.
func (r *ImportRepository) ListShortLinkCandidates(ctx context.Context, bookID, commodityID int64, operationKind string) ([]ShortLinkCandidate, error) {
	rows, err := r.database.QueryContext(ctx, shortLinkCandidateSelect+`
		AND o.operation_kind = ? AND COALESCE(l.commodity_id, d.commodity_id) = ?
		ORDER BY o.event_date DESC, o.id DESC`, bookID, operationKind, commodityID)
	if err != nil {
		return nil, fmt.Errorf("read short link candidates: %w", err)
	}
	var candidates []ShortLinkCandidate
	for rows.Next() {
		candidate, err := scanShortLinkCandidate(rows)
		if err != nil {
			rows.Close()
			return nil, fmt.Errorf("scan short link candidate: %w", err)
		}
		candidates = append(candidates, candidate)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, fmt.Errorf("iterate short link candidates: %w", err)
	}
	if err := rows.Close(); err != nil {
		return nil, fmt.Errorf("close short link candidates: %w", err)
	}
	for i := range candidates {
		if candidates[i].Linked, err = investmentOperationHasImportedLineageQuery(ctx, r.database, bookID, candidates[i].OperationID); err != nil {
			return nil, err
		}
	}
	return candidates, nil
}

// ShortLinkCandidateByOperation reads one effective short sale or cover.
func (r *ImportRepository) ShortLinkCandidateByOperation(ctx context.Context, bookID, operationID int64) (ShortLinkCandidate, error) {
	return shortLinkCandidateQuery(ctx, r.database, bookID, operationID)
}

func shortLinkCandidateQuery(ctx context.Context, reader saleOperationReader, bookID, operationID int64) (ShortLinkCandidate, error) {
	candidate, err := scanShortLinkCandidate(reader.QueryRowContext(ctx, shortLinkCandidateSelect+` AND o.id = ?`, bookID, operationID))
	if errors.Is(err, sql.ErrNoRows) {
		return ShortLinkCandidate{}, ErrNotFound
	}
	if err != nil {
		return ShortLinkCandidate{}, fmt.Errorf("read short link candidate: %w", err)
	}
	if candidate.Linked, err = investmentOperationHasImportedLineageQuery(ctx, reader, bookID, operationID); err != nil {
		return ShortLinkCandidate{}, err
	}
	return candidate, nil
}

// LinkStagedRowToShortParams names the staged provider row and the short
// operation as the owner reviewed it.
type LinkStagedRowToShortParams struct {
	BookID            int64
	BatchID           int64
	RowID             int64
	DedupeFingerprint string
	SourceKind        string
	Expected          ShortLinkCandidate
	ActorUserID       int64
	AuthSessionID     int64
	RequestID         string
	Now               string
}

// LinkStagedRowToShort admits the row's identity with the reviewed short
// operation as its only effect. Row admissibility and every reviewed
// operation fact are re-read inside the write transaction: a correction,
// a discard or another row's link in between refuses the whole link.
func (r *ImportRepository) LinkStagedRowToShort(ctx context.Context, params LinkStagedRowToShortParams) (int64, error) {
	if params.Expected.Linked {
		return 0, ErrShortLinkUnavailable
	}
	tx, err := r.database.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("begin short link: %w", err)
	}
	defer rollbackTx(ctx, tx)
	row := stagedRowLink{BookID: params.BookID, BatchID: params.BatchID, RowID: params.RowID,
		DedupeFingerprint: params.DedupeFingerprint, SourceKind: params.SourceKind}
	if err := checkLinkableStagedRowTx(ctx, tx, row); err != nil {
		if errors.Is(err, errStagedRowNotLinkable) {
			return 0, ErrShortLinkUnavailable
		}
		return 0, err
	}
	current, err := shortLinkCandidateQuery(ctx, tx, params.BookID, params.Expected.OperationID)
	if errors.Is(err, ErrNotFound) {
		// Corrected or reversed after review: no longer effective.
		return 0, ErrShortLinkChanged
	}
	if err != nil {
		return 0, err
	}
	if current.Linked {
		return 0, ErrShortLinkUnavailable
	}
	if current != params.Expected {
		return 0, ErrShortLinkChanged
	}
	identityID, err := r.admitStagedRowAsOperationTx(ctx, tx, row, operationLinkAdmission{
		OperationID: current.OperationID, TransactionID: current.TransactionID, AccountID: current.HoldingAccountID,
		AuditOperation: "import.row.link_" + current.OperationKind, ActorUserID: params.ActorUserID,
		AuthSessionID: params.AuthSessionID, RequestID: params.RequestID, Now: params.Now,
	})
	if errors.Is(err, errStagedRowNotLinkable) {
		return 0, ErrShortLinkUnavailable
	}
	if err != nil {
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit short link: %w", err)
	}
	return identityID, nil
}
