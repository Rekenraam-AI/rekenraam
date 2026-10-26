package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"rekenraam/backend/internal/db"
)

// Saved return runs (R13 slice 6, docs/plans/investment-returns-plan.md):
// an explicitly saved, immutable returns_v1 document computed on one
// fingerprinted image of the book, after the R18 gains-run pattern.

var ErrReturnsRunNotFound = errors.New("investment returns run not found")
var ErrReturnsRunIntegrity = errors.New("investment returns run integrity check failed")

// ReturnsRunRequest is the live read's inputs as a saved run records them,
// with defaults resolved. Nullable fields are pointers (null means none);
// max_staleness_days is not nullable, so it records presence itself and an
// explicit value, null included, is never mistaken for omission.
type ReturnsRunRequest struct {
	From                 string              `json:"from"`
	To                   string              `json:"to"`
	ReportingCurrencyID  int64               `json:"reporting_currency_id"`
	AccountIDs           []int64             `json:"account_ids"`
	MaxStalenessDays     ReturnsRunStaleness `json:"max_staleness_days,omitzero"`
	BenchmarkCommodityID *int64              `json:"benchmark_commodity_id"`
	BenchmarkBasis       *string             `json:"benchmark_basis"`
	// Instruments adds the per-instrument read (instrument_returns_v1). It
	// is recorded only when true, so a run without it keeps the slice 6
	// request shape, descriptor and digests.
	Instruments bool `json:"instruments,omitzero"`
}

// ReturnsRunStaleness is max_staleness_days as sent: Present once the field
// appears in the body, Null when it appears as null. Omitted, it is the zero
// value and is left out when written.
type ReturnsRunStaleness struct {
	Days    int
	Present bool
	Null    bool
}

// StalenessDays is a staleness sent as a number.
func StalenessDays(days int) ReturnsRunStaleness {
	return ReturnsRunStaleness{Days: days, Present: true}
}

func (s ReturnsRunStaleness) IsZero() bool { return !s.Present }

func (s *ReturnsRunStaleness) UnmarshalJSON(data []byte) error {
	*s = ReturnsRunStaleness{Present: true, Null: string(data) == "null"}
	if s.Null {
		return nil
	}
	return json.Unmarshal(data, &s.Days)
}

func (s ReturnsRunStaleness) MarshalJSON() ([]byte, error) {
	if s.Null {
		return []byte("null"), nil
	}
	return json.Marshal(s.Days)
}

type ReturnsRunInput struct {
	Request                    ReturnsRunRequest
	OwnerUserID, AuthSessionID int64
	RequestID                  string
}

// ReturnsRunFormatter is the versioned wire representation of a read; a
// caller cannot submit its own output.
type ReturnsRunFormatter func(ReturnsResult) (json.RawMessage, error)

func (s *InvestmentService) CreateReturnsRun(ctx context.Context, input ReturnsRunInput, format ReturnsRunFormatter) (db.ReturnsRunRecord, error) {
	request := input.Request
	read := ReturnsInput{From: request.From, To: request.To, ReportingCommodityID: request.ReportingCurrencyID,
		AccountIDs: request.AccountIDs, Instruments: request.Instruments}
	if request.MaxStalenessDays.Present {
		// Only omission takes the default: zero is the live read's
		// "omitted", so an explicit zero, like null, is out of range.
		if request.MaxStalenessDays.Null || request.MaxStalenessDays.Days < 1 {
			return db.ReturnsRunRecord{}, ValidationError{Message: fmt.Sprintf("max_staleness_days must be between 1 and %d", returnsMaxStalenessDays)}
		}
		read.MaxStalenessDays = request.MaxStalenessDays.Days
	}
	if request.BenchmarkCommodityID != nil {
		if *request.BenchmarkCommodityID <= 0 {
			return db.ReturnsRunRecord{}, ValidationError{Message: "benchmark_commodity_id must be a positive integer"}
		}
		read.BenchmarkCommodityID = *request.BenchmarkCommodityID
	}
	if request.BenchmarkBasis != nil {
		read.BenchmarkBasis = *request.BenchmarkBasis
	}
	for _, id := range read.AccountIDs {
		if id <= 0 {
			return db.ReturnsRunRecord{}, ValidationError{Message: "account_ids must be positive integers"}
		}
	}
	read, err := validateReturnsInput(read)
	if err != nil {
		return db.ReturnsRunRecord{}, err
	}
	snapshot, fingerprint, cleanup, err := s.repository.GainsSnapshot(ctx)
	if err != nil {
		return db.ReturnsRunRecord{}, fmt.Errorf("capture returns snapshot: %w", err)
	}
	defer cleanup()
	captured := s.now().UTC()
	result, err := readReturnsOn(ctx, snapshot, captured, read)
	if err != nil {
		return db.ReturnsRunRecord{}, err
	}
	output, err := format(result)
	if err != nil {
		return db.ReturnsRunRecord{}, err
	}
	// Compact once before hashing so HTTP's RawMessage encoding and portable
	// exports retain the same document bytes.
	if !json.Valid(output) {
		return db.ReturnsRunRecord{}, ErrReturnsRunIntegrity
	}
	compact, err := json.Marshal(output)
	if err != nil {
		return db.ReturnsRunRecord{}, err
	}
	resolved := ReturnsRunRequest{From: read.From, To: read.To, ReportingCurrencyID: read.ReportingCommodityID,
		AccountIDs: read.AccountIDs, MaxStalenessDays: StalenessDays(read.MaxStalenessDays), Instruments: read.Instruments}
	if resolved.AccountIDs == nil {
		resolved.AccountIDs = []int64{}
	}
	if read.BenchmarkCommodityID != 0 {
		resolved.BenchmarkCommodityID, resolved.BenchmarkBasis = &read.BenchmarkCommodityID, &read.BenchmarkBasis
	}
	var descriptor db.ReturnsRunDescriptor
	if err := json.Unmarshal(compact, &descriptor); err != nil {
		return db.ReturnsRunRecord{}, err
	}
	inputs, err := json.Marshal(struct {
		Request        ReturnsRunRequest       `json:"request"`
		SnapshotSHA256 string                  `json:"snapshot_sha256"`
		Descriptor     db.ReturnsRunDescriptor `json:"descriptor"`
	}{resolved, fingerprint, descriptor})
	if err != nil {
		return db.ReturnsRunRecord{}, err
	}
	run := db.ReturnsRunRecord{BookID: BookID, SchemaVersion: 1,
		CapturedAt: captured.Format(time.RFC3339Nano), CreatedAt: s.now().UTC().Format(time.RFC3339),
		CreatedByUserID: input.OwnerUserID, SnapshotSHA256: fingerprint,
		InputsJSON: inputs, OutputJSON: compact, InputsSHA256: db.GainsRunDigest(inputs), OutputSHA256: db.GainsRunDigest(compact)}
	if !db.ValidReturnsRunDocument(run) {
		return db.ReturnsRunRecord{}, ErrReturnsRunIntegrity
	}
	return s.repository.CreateReturnsRun(ctx, run, db.AuditEventParams{BookID: BookID, ActorUserID: input.OwnerUserID,
		AuthSessionID: input.AuthSessionID, RequestID: input.RequestID, OriginType: "browser_api", OccurredAt: run.CreatedAt})
}

func (s *InvestmentService) ReturnsRun(ctx context.Context, id int64) (db.ReturnsRunRecord, error) {
	run, err := s.repository.ReturnsRun(ctx, BookID, id)
	if errors.Is(err, db.ErrNotFound) {
		return run, ErrReturnsRunNotFound
	}
	if err == nil && !db.ValidReturnsRunDocument(run) {
		return db.ReturnsRunRecord{}, ErrReturnsRunIntegrity
	}
	return run, err
}

func (s *InvestmentService) ListReturnsRuns(ctx context.Context, before int64) ([]db.ReturnsRunRecord, int64, error) {
	if before < 0 {
		return nil, 0, ValidationError{Message: "invalid returns run cursor"}
	}
	runs, err := s.repository.ListReturnsRuns(ctx, BookID, before, 21)
	if err != nil {
		return nil, 0, err
	}
	// The 21st row only says whether a next page exists; it belongs to that
	// page, so it is trimmed before the checks.
	var next int64
	if len(runs) > 20 {
		runs = runs[:20]
		next = runs[19].ID
	}
	// A summary is listed only for a document a single read would return:
	// each run on this page is verified whole, then its output is left for
	// selection.
	for i := range runs {
		if !db.ValidReturnsRunDocument(runs[i]) {
			return nil, 0, ErrReturnsRunIntegrity
		}
		runs[i].OutputJSON = nil
	}
	return runs, next, nil
}
