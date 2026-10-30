package app

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"rekenraam/backend/internal/db"
)

type attentionTestReader struct {
	records map[string]db.AttentionRecord
	errors  map[string]error
}

func (r attentionTestReader) Read(_ context.Context, _ int64, source string) (db.AttentionRecord, error) {
	return r.records[source], r.errors[source]
}

type attentionTestPricing struct {
	workspace PricingWorkspace
	err       error
}

func (p attentionTestPricing) Workspace(context.Context, string) (PricingWorkspace, error) {
	return p.workspace, p.err
}

func TestAttentionPreservesUnknownSourcesAndKnownCounts(t *testing.T) {
	t.Parallel()
	s := NewAttentionService(attentionTestReader{
		records: map[string]db.AttentionRecord{"drafts": {Count: 3}, "self_check": {Status: "running"}},
		errors:  map[string]error{"imports": errors.New("unavailable")},
	}, attentionTestPricing{})
	result := s.Read(context.Background())
	assert.Equal(t, 3, result.TotalKnown)
	assert.False(t, result.Complete)
	assert.True(t, result.HasFailures)
	for _, source := range result.Sources {
		switch source.ID {
		case "imports":
			assert.Equal(t, "error", source.State)
			assert.Nil(t, source.Count)
		case "self_check":
			assert.Equal(t, "loading", source.State)
			assert.Nil(t, source.Count)
		default:
			require.NotNil(t, source.Count)
		}
	}
}

func TestAttentionLatestOperationalFailuresClearOnlyAfterRecovery(t *testing.T) {
	t.Parallel()
	for _, status := range []string{"never", "pending", "running", "failed", "errored", "incomplete", "warning", "passed", "completed"} {
		t.Run(status, func(t *testing.T) {
			s := NewAttentionService(attentionTestReader{records: map[string]db.AttentionRecord{"self_check": {Status: status}}}, attentionTestPricing{})
			result := s.Read(context.Background())
			assert.Equal(t, status != "pending" && status != "running", result.Complete)
			assert.Equal(t, status == "failed" || status == "errored", result.HasFailures)
			if status == "failed" || status == "errored" || status == "incomplete" || status == "warning" {
				assert.Equal(t, 1, result.TotalKnown)
			} else {
				assert.Zero(t, result.TotalKnown)
			}
		})
	}
}

func TestAttentionRunningDownloadsAndRefreshesCannotLookAllClear(t *testing.T) {
	t.Parallel()
	for _, id := range []string{"connections", "refresh"} {
		for _, count := range []int{0, 2} {
			s := NewAttentionService(attentionTestReader{records: map[string]db.AttentionRecord{id: {Status: "running", Count: count}}}, attentionTestPricing{})
			result := s.Read(context.Background())
			assert.False(t, result.Complete, id)
			assert.Equal(t, count, result.TotalKnown, "known failures survive another source running")
			assert.Equal(t, count > 0, result.HasFailures)
			for _, source := range result.Sources {
				if source.ID == id {
					assert.Equal(t, "loading", source.State)
				}
			}
		}
	}
}

func TestAttentionReusesPriceCoverageAndDeduplicatesPairs(t *testing.T) {
	t.Parallel()
	s := NewAttentionService(attentionTestReader{}, attentionTestPricing{workspace: PricingWorkspace{
		Commodities: []PricingCommodity{{ID: 1, Kind: "security"}, {ID: 2, Kind: "currency"}, {ID: 3, Kind: "currency"}},
		Coverage: []PricingCoverage{
			{BaseCommodityID: 1, QuoteCommodityID: 2, Status: "missing"},
			{BaseCommodityID: 1, QuoteCommodityID: 2, Status: "stale"},
			{BaseCommodityID: 1, QuoteCommodityID: 3, Status: "usable"},
			{BaseCommodityID: 2, QuoteCommodityID: 3, Status: "stale"},
		},
	}})
	result := s.Read(context.Background())
	assert.Equal(t, 2, result.TotalKnown)
	assert.True(t, result.Complete)
	assert.False(t, result.HasFailures)
	s.pricing = attentionTestPricing{err: errors.New("coverage failed")}
	result = s.Read(context.Background())
	assert.False(t, result.Complete)
	assert.True(t, result.HasFailures)
	for _, source := range result.Sources {
		if source.ID == "prices" || source.ID == "rates" {
			assert.Nil(t, source.Count)
			assert.Equal(t, "error", source.State)
		}
	}
}
