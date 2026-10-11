package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"slices"
)

// Same-day reordering everywhere (#185). Placing transaction T before or after
// an anchor transaction A states one fact: on that day T happened before (or
// after) A. It applies wherever both appear on the same day:
//   - in the journal order (transaction_day_sequence), when both share a
//     transaction date;
//   - in every account register that holds postings of both on the same entry
//     date (account_day_sequence).
//
// A scope where the relation already holds is left as it is, so an account
// reordered on its own ("only in this account", MovePosting) keeps that order
// unless this fact contradicts it. Each changed scope reuses its existing set
// of sequence values, so nothing outside the day moves. Account orders are the
// reconciliation boundary: a change that crosses an active same-day checkpoint
// needs the override and invalidates it, as MovePosting does. The journal order
// is presentation only and never guarded. Every changed transaction gets one
// new version under one audit event.

// ErrReorderNotSameDay means the two transactions share neither a transaction
// date nor an account entry date, so there is no same-day order to state.
var ErrReorderNotSameDay = errors.New("the transactions are not on the same day in any shared order")

// ErrInvalidReorder means the request does not name two different
// transactions and a position.
var ErrInvalidReorder = errors.New("invalid reorder request")

type PlaceTransactionParams struct {
	BookID                 int64
	TransactionID          int64
	AnchorTransactionID    int64
	Position               string // "before" | "after"
	ActorUserID            int64
	AuthSessionID          int64
	RequestID              string
	OriginType             string
	RecordedAt             string
	ReconciliationOverride bool
}

// reorderItem is one entry of a same-day scope: a transaction in the journal
// order, or a posting in an account register.
type reorderItem struct {
	transactionID int64
	postingLineID int64 // zero in the journal order
	commodityID   int64
	seq           int64
}

// placeInScope moves the subject's items next to the anchor's, keeping each
// group's internal order, when the relation does not already hold. It returns
// the items whose sequence changes, with their new sequence; the scope's set of
// sequence values is reused, so nothing outside it moves.
func placeInScope(items []reorderItem, subject, anchor int64, before bool) []reorderItem {
	var subjectItems, others []reorderItem
	for _, item := range items {
		if item.transactionID == subject {
			subjectItems = append(subjectItems, item)
		} else {
			others = append(others, item)
		}
	}
	anchorFirst, anchorLast := -1, -1
	for index, item := range others {
		if item.transactionID == anchor {
			if anchorFirst < 0 {
				anchorFirst = index
			}
			anchorLast = index
		}
	}
	if len(subjectItems) == 0 || anchorFirst < 0 {
		return nil
	}
	// The relation holds when every subject item already sits on the asked
	// side of every anchor item.
	holds := true
	for _, item := range subjectItems {
		for _, other := range others[anchorFirst : anchorLast+1] {
			if other.transactionID != anchor {
				continue
			}
			if (before && item.seq > other.seq) || (!before && item.seq < other.seq) {
				holds = false
			}
		}
	}
	if holds {
		return nil
	}
	insertAt := anchorLast + 1
	if before {
		insertAt = anchorFirst
	}
	ordered := slices.Concat(others[:insertAt], subjectItems, others[insertAt:])
	values := make([]int64, len(items))
	for index, item := range items {
		values[index] = item.seq
	}
	slices.Sort(values)
	var changed []reorderItem
	for index, item := range ordered {
		if item.seq != values[index] {
			item.seq = values[index]
			changed = append(changed, item)
		}
	}
	return changed
}

// PlaceTransaction places a transaction before or after an anchor transaction
// on their shared day, in the journal order and in every shared account.
func (r *TransactionRepository) PlaceTransaction(ctx context.Context, params PlaceTransactionParams) (TransactionRecord, error) {
	if params.TransactionID <= 0 || params.AnchorTransactionID <= 0 || params.TransactionID == params.AnchorTransactionID ||
		(params.Position != "before" && params.Position != "after") {
		return TransactionRecord{}, fmt.Errorf("%w: reorder needs two different transactions and a position", ErrInvalidReorder)
	}
	before := params.Position == "before"
	return withTransactionRecordTx(r, ctx, "reorder transaction", func(tx *sql.Tx) (TransactionRecord, error) {
		if _, err := readBookForUpdate(ctx, tx, params.BookID); err != nil {
			return TransactionRecord{}, err
		}
		subject, err := transactionByID(ctx, tx, params.BookID, params.TransactionID)
		if err != nil {
			return TransactionRecord{}, err
		}
		anchor, err := transactionByID(ctx, tx, params.BookID, params.AnchorTransactionID)
		if err != nil {
			return TransactionRecord{}, err
		}

		sameDay := false
		daySeq := map[int64]int64{}
		if subject.TransactionDate == anchor.TransactionDate {
			sameDay = true
			items, err := journalDayItemsTx(ctx, tx, params.BookID, subject.TransactionDate)
			if err != nil {
				return TransactionRecord{}, err
			}
			for _, item := range placeInScope(items, subject.ID, anchor.ID, before) {
				daySeq[item.transactionID] = item.seq
			}
		}

		shared, err := sharedAccountDaysTx(ctx, tx, params.BookID, subject.ID, anchor.ID)
		if err != nil {
			return TransactionRecord{}, err
		}
		postingSeq := map[int64]map[int64]int64{}
		var reached []CheckpointInvalidationRef
		seenCheckpoints := map[int64]bool{}
		crossesCheckpoint := false
		for _, scope := range shared {
			sameDay = true
			items, err := accountDayItemsTx(ctx, tx, params.BookID, scope.accountID, scope.entryDate)
			if err != nil {
				return TransactionRecord{}, err
			}
			changed := placeInScope(items, subject.ID, anchor.ID, before)
			if len(changed) == 0 {
				continue
			}
			// The changed postings lie between the lowest and highest sequence
			// they had or now have; a same-day checkpoint boundary inside that
			// range sees a different balance.
			lowSeq, highSeq := changed[0].seq, changed[0].seq
			commodities := map[int64]bool{}
			previous := make(map[int64]int64, len(items))
			for _, item := range items {
				previous[item.postingLineID] = item.seq
			}
			for _, item := range changed {
				lowSeq, highSeq = min(lowSeq, item.seq, previous[item.postingLineID]), max(highSeq, item.seq, previous[item.postingLineID])
				commodities[item.commodityID] = true
				if postingSeq[item.transactionID] == nil {
					postingSeq[item.transactionID] = map[int64]int64{}
				}
				postingSeq[item.transactionID][item.postingLineID] = item.seq
			}
			for commodityID := range commodities {
				refs, err := activeCheckpointRefsAtOrAfter(ctx, tx, params.BookID, PeriodScopedCheckpointRef{
					AccountID: scope.accountID, CommodityID: commodityID,
					EntryDate: scope.entryDate, AccountDaySequence: lowSeq,
				})
				if err != nil {
					return TransactionRecord{}, err
				}
				// Only an account whose boundary is crossed loses its
				// checkpoints: the crossed one and every later one in it.
				crossed := false
				for _, ref := range refs {
					if ref.StatementDate == scope.entryDate && ref.StatementAccountSequence < highSeq {
						crossed = true
					}
				}
				if !crossed {
					continue
				}
				crossesCheckpoint = true
				for _, ref := range refs {
					if !seenCheckpoints[ref.CheckpointID] {
						seenCheckpoints[ref.CheckpointID] = true
						reached = append(reached, ref)
					}
				}
			}
		}
		if !sameDay {
			return TransactionRecord{}, ErrReorderNotSameDay
		}
		if crossesCheckpoint && !params.ReconciliationOverride {
			return TransactionRecord{}, ErrReconciliationOverrideRequired
		}

		affected := map[int64]bool{}
		for id := range daySeq {
			affected[id] = true
		}
		for id := range postingSeq {
			affected[id] = true
		}
		if len(affected) == 0 {
			// The order already says this everywhere; nothing to record.
			records := []TransactionRecord{subject}
			if err := loadTransactionChildrenTx(ctx, tx, records); err != nil {
				return TransactionRecord{}, err
			}
			return records[0], nil
		}

		auditEventID, err := insertAuditEvent(ctx, tx, AuditEventParams{
			BookID: params.BookID, ActorUserID: params.ActorUserID, AuthSessionID: params.AuthSessionID,
			OccurredAt: params.RecordedAt, RequestID: params.RequestID, OriginType: params.OriginType,
			Operation: "transaction.reorder", Reason: "placed " + params.Position + " another transaction",
		})
		if err != nil {
			return TransactionRecord{}, err
		}
		ids := make([]int64, 0, len(affected))
		for id := range affected {
			ids = append(ids, id)
		}
		slices.Sort(ids)
		var result TransactionRecord
		subjectWritten := false
		for _, id := range ids {
			record, err := transactionByID(ctx, tx, params.BookID, id)
			if err != nil {
				return TransactionRecord{}, err
			}
			records := []TransactionRecord{record}
			if err := loadTransactionChildrenTx(ctx, tx, records); err != nil {
				return TransactionRecord{}, err
			}
			record = records[0]
			sequence := record.TransactionDaySequence
			if moved, ok := daySeq[id]; ok {
				sequence = moved
			}
			reason := "reordered beside a moved transaction"
			if id == subject.ID {
				reason = "placed " + params.Position + " another transaction"
			}
			written, err := r.insertTransactionVersion(ctx, tx, insertTransactionVersionParams{
				BookID: params.BookID, TransactionID: id, VersionSeq: record.VersionSeq + 1,
				SupersedesVersionID: sql.NullInt64{Int64: record.VersionID, Valid: true},
				Spec:                transactionSpecFromRecord(record), ReplaceTags: false,
				RecordedAt: params.RecordedAt, ChangedByUserID: params.ActorUserID,
				ChangeReason: reason, ChangeAuditEventID: auditEventID,
				RequestID: params.RequestID, TransactionDaySequence: sequence,
				PostingSeqOverrides: postingSeq[id],
			})
			if err != nil {
				return TransactionRecord{}, fmt.Errorf("insert reordered transaction version: %w", err)
			}
			if id == subject.ID {
				result, subjectWritten = written, true
			}
		}
		if !subjectWritten {
			// The subject itself kept its sequences; return its current record.
			records := []TransactionRecord{subject}
			if err := loadTransactionChildrenTx(ctx, tx, records); err != nil {
				return TransactionRecord{}, err
			}
			result = records[0]
		}
		if crossesCheckpoint {
			invalidated, err := invalidateReconciliationCheckpoints(ctx, tx, checkpointInvalidationParams{
				BookID: params.BookID, Refs: reached, ActorUserID: params.ActorUserID, AuditEventID: auditEventID,
				OccurredAt: params.RecordedAt, Reason: "transaction reordered across a reconciled checkpoint",
			})
			if err != nil {
				return TransactionRecord{}, err
			}
			result.InvalidatedCheckpointIDs = invalidated
		}
		return result, nil
	})
}

func journalDayItemsTx(ctx context.Context, tx *sql.Tx, bookID int64, date string) ([]reorderItem, error) {
	rows, err := tx.QueryContext(ctx, `SELECT ctv.transaction_id, ctv.transaction_day_sequence
		FROM current_transaction_versions ctv
		JOIN transactions t ON t.id = ctv.transaction_id
		WHERE ctv.book_id = ? AND ctv.transaction_date = ? AND t.deleted_at IS NULL
		ORDER BY ctv.transaction_day_sequence, ctv.transaction_id`, bookID, date)
	if err != nil {
		return nil, fmt.Errorf("read journal day order: %w", err)
	}
	defer rows.Close()
	var items []reorderItem
	for rows.Next() {
		var item reorderItem
		if err := rows.Scan(&item.transactionID, &item.seq); err != nil {
			return nil, fmt.Errorf("scan journal day order: %w", err)
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func accountDayItemsTx(ctx context.Context, tx *sql.Tx, bookID, accountID int64, date string) ([]reorderItem, error) {
	rows, err := tx.QueryContext(ctx, `SELECT t.id, pv.posting_line_id, pv.commodity_id, pv.account_day_sequence
		FROM posting_versions pv
		JOIN journal_entries je ON je.id = pv.journal_entry_id
		JOIN current_transaction_versions ctv ON ctv.id = pv.transaction_version_id
		JOIN transactions t ON t.id = ctv.transaction_id
		WHERE pv.book_id = ? AND pv.account_id = ? AND je.entry_date = ? AND t.deleted_at IS NULL
		ORDER BY pv.account_day_sequence, pv.id`, bookID, accountID, date)
	if err != nil {
		return nil, fmt.Errorf("read account day order: %w", err)
	}
	defer rows.Close()
	var items []reorderItem
	for rows.Next() {
		var item reorderItem
		if err := rows.Scan(&item.transactionID, &item.postingLineID, &item.commodityID, &item.seq); err != nil {
			return nil, fmt.Errorf("scan account day order: %w", err)
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

type accountDay struct {
	accountID int64
	entryDate string
}

// sharedAccountDaysTx lists the (account, entry date) registers in which both
// transactions currently have a posting.
func sharedAccountDaysTx(ctx context.Context, tx *sql.Tx, bookID, subjectID, anchorID int64) ([]accountDay, error) {
	rows, err := tx.QueryContext(ctx, `SELECT DISTINCT a_pv.account_id, a_je.entry_date
		FROM current_transaction_versions a
		JOIN posting_versions a_pv ON a_pv.transaction_version_id = a.id
		JOIN journal_entries a_je ON a_je.id = a_pv.journal_entry_id
		JOIN current_transaction_versions b ON b.transaction_id = ?
		JOIN posting_versions b_pv ON b_pv.transaction_version_id = b.id AND b_pv.account_id = a_pv.account_id
		JOIN journal_entries b_je ON b_je.id = b_pv.journal_entry_id AND b_je.entry_date = a_je.entry_date
		WHERE a.book_id = ? AND a.transaction_id = ?
		ORDER BY a_pv.account_id, a_je.entry_date`, anchorID, bookID, subjectID)
	if err != nil {
		return nil, fmt.Errorf("read shared account days: %w", err)
	}
	defer rows.Close()
	var days []accountDay
	for rows.Next() {
		var day accountDay
		if err := rows.Scan(&day.accountID, &day.entryDate); err != nil {
			return nil, fmt.Errorf("scan shared account day: %w", err)
		}
		days = append(days, day)
	}
	return days, rows.Err()
}
