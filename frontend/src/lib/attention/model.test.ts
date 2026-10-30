import { describe, expect, it } from 'vitest';
import { attentionSummary, sourceBadge, sourceStatus, type AttentionState } from './model.ts';

function state(overrides: Partial<AttentionState> = {}): AttentionState {
  return {
    isPending: false, isError: false, isFetching: false, refetch: () => {},
    data: { checked_at: '2026-10-30T04:00:00Z', total_known: 1234, complete: true, has_failures: false, sources: [
      { id: 'imports', unit: 'batch', state: 'ready', status: '', count: 1234, failed: false }
    ] }, ...overrides
  };
}

describe('attention badges', () => {
  it('formats successful counts in the app locale', () => {
    expect(attentionSummary(state(), 'de').text).toBe('1.234');
    expect(sourceBadge(state(), ['imports'], 'fr').text).toBe(new Intl.NumberFormat('fr').format(1234));
  });
  it('never presents pending, missing, or errored sources as successful zeroes', () => {
    expect(attentionSummary(state({ data: undefined, isPending: true }), 'en').text).toBe('…');
    expect(sourceBadge(state(), ['rates'], 'en').text).toBe('…');
    const query = state();
    query.data!.sources[0] = { id: 'imports', unit: 'batch', state: 'error', status: '', count: null, failed: false, error_code: 'INTERNAL_ERROR' };
    query.data!.complete = false;
    query.data!.has_failures = true;
    expect(attentionSummary(query, 'en').text).toBe('!');
    expect(sourceBadge(query, ['imports'], 'en').text).toBe('!');
  });
  it('shows a transport failure even when the cached response was healthy', () => {
    const query = state({ isError: true });
    expect(attentionSummary(query, 'en').text).toBe('!');
    expect(sourceBadge(query, ['imports'], 'en').text).toBe('!');
  });
  it('retains a known partial failure count while other work runs', () => {
    const query = state();
    query.data!.sources[0] = { id: 'connections', unit: 'connection', state: 'loading', status: 'running', count: 2, failed: true };
    const badge = sourceBadge(query, ['connections'], 'en');
    expect(badge.count).toBe(2);
    expect(badge.text).toBe('!');
    expect(sourceStatus(query.data!.sources[0], 'en')).toContain('2');
  });
  it('shows durable failures and hides only a confirmed zero tab badge', () => {
    const query = state();
    query.data!.sources[0].failed = true;
    expect(sourceBadge(query, ['imports'], 'en').text).toBe('!');
    query.data!.sources[0].failed = false;
    query.data!.sources[0].count = 0;
    expect(sourceBadge(query, ['imports'], 'en').text).toBe('');
    query.data!.sources[0].count = null;
    query.data!.sources[0].state = 'loading';
    expect(sourceBadge(query, ['imports'], 'en').text).toBe('…');
  });
});
