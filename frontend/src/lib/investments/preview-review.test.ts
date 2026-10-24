import { describe, expect, it } from 'vitest';
import { APIClientError } from '#lib/api/client.ts';
import type { GainImpact } from '#lib/api/investments.ts';
import { previewReview, reviewGuards, type ReviewGuards, type ReviewModal } from './preview-review';

type Preview = { impacts: { checkpoint_id: number }[]; gainImpact: GainImpact | null; payload: { n: number } };

function gains(acknowledgement: string): GainImpact {
  return { acknowledgement, changes: [{} as GainImpact['changes'][number]] } as GainImpact;
}

const stale = () => new APIClientError({ status: 409, code: 'INVESTMENT_GAIN_IMPACT_ACKNOWLEDGEMENT_STALE' });
const staleCheckpoints = () => new APIClientError({ status: 409, code: 'RECONCILIATION_IMPACT_STALE' });

/** A host that records every transition, refreshing to the queued previews. */
function harness(initial: Preview | null, options: {
  writes?: (() => Promise<void>)[];
  refreshes?: (() => Promise<Preview>)[];
} = {}) {
  const log: string[] = [];
  const state = {
    preview: initial,
    pending: false,
    error: undefined as unknown,
    modal: null as ReviewModal | null,
    writes: [] as { preview: Preview; guards: ReviewGuards }[],
    refreshed: [] as { n: number }[]
  };
  const writes = [...(options.writes ?? [])];
  const refreshes = [...(options.refreshes ?? [])];
  const review = previewReview<{ n: number }, Preview>({
    get preview() { return state.preview; },
    async refresh(payload) {
      state.refreshed.push(payload);
      const next = refreshes.shift();
      if (!next) throw new Error('unexpected refresh');
      state.preview = await next();
    },
    async write(preview, guards) {
      state.writes.push({ preview, guards });
      const next = writes.shift();
      if (next) await next();
      log.push('saved');
    },
    discardPreview() { state.preview = null; },
    setPending(pending) { state.pending = pending; log.push(`pending:${pending}`); },
    setError(error) { state.error = error; },
    setModal(modal) { state.modal = modal; }
  });
  return { state, review, log };
}

const settle = () => new Promise((resolve) => setTimeout(resolve, 0));

describe('previewReview', () => {
  it('records at once with no guards when nothing needs review', async () => {
    const { state, review, log } = harness({ impacts: [], gainImpact: null, payload: { n: 1 } });
    review.record();
    await settle();
    expect(state.modal).toBeNull();
    expect(state.writes).toEqual([{ preview: expect.anything(), guards: {} }]);
    expect(log).toEqual(['pending:true', 'saved', 'pending:false']);
  });

  it('opens a fresh review and records exactly the reviewed set once on confirm', async () => {
    const preview = { impacts: [{ checkpoint_id: 4 }], gainImpact: gains('ack-1'), payload: { n: 1 } };
    const { state, review } = harness(preview);
    review.record();
    expect(state.modal).toEqual({ gainRefreshed: false });
    expect(state.writes).toHaveLength(0);
    review.confirm();
    expect(state.modal).toBeNull();
    await settle();
    expect(state.writes).toHaveLength(1);
    expect(state.writes[0].guards).toEqual({
      reconciliation_override: true, reconciliation_override_checkpoint_ids: [4], gain_impact_acknowledgement: 'ack-1'
    });
    expect(state.error).toBeUndefined();
  });

  it('cancel closes the review without writing', () => {
    const { state, review } = harness({ impacts: [{ checkpoint_id: 4 }], gainImpact: null, payload: { n: 1 } });
    review.record();
    review.cancel();
    expect(state.modal).toBeNull();
    expect(state.writes).toHaveLength(0);
    expect(state.preview).not.toBeNull();
  });

  it('does nothing without a preview', async () => {
    const { state, review, log } = harness(null);
    review.record();
    review.confirm();
    await settle();
    expect(state.writes).toHaveLength(0);
    expect(log).toEqual([]);
  });

  it('a stale refusal previews the same payload again and re-opens the review without retrying the write', async () => {
    const refreshed = { impacts: [{ checkpoint_id: 9 }], gainImpact: gains('ack-2'), payload: { n: 1 } };
    const { state, review } = harness({ impacts: [{ checkpoint_id: 4 }], gainImpact: gains('ack-1'), payload: { n: 1 } }, {
      writes: [() => Promise.reject(stale())],
      refreshes: [async () => refreshed]
    });
    review.record();
    review.confirm();
    await settle();
    expect(state.writes).toHaveLength(1);
    expect(state.refreshed).toEqual([{ n: 1 }]);
    expect(state.preview).toBe(refreshed);
    expect(state.modal).toEqual({ gainRefreshed: true });
    expect(state.error).toBeUndefined();
    expect(state.pending).toBe(false);

    // The next confirmation sends the refreshed set, never the stale one.
    review.confirm();
    await settle();
    expect(state.writes).toHaveLength(2);
    expect(state.writes[1].guards).toEqual({
      reconciliation_override: true, reconciliation_override_checkpoint_ids: [9], gain_impact_acknowledgement: 'ack-2'
    });
  });

  it('a stale checkpoint refusal re-opens a checkpoint-only review as not gain-refreshed', async () => {
    const { state, review } = harness({ impacts: [], gainImpact: null, payload: { n: 2 } }, {
      writes: [() => Promise.reject(staleCheckpoints())],
      refreshes: [async () => ({ impacts: [{ checkpoint_id: 5 }], gainImpact: null, payload: { n: 2 } })]
    });
    review.record();
    await settle();
    expect(state.modal).toEqual({ gainRefreshed: false });
    expect(state.writes).toHaveLength(1);
  });

  it('a stale refusal whose refreshed preview needs no review leaves it for the user to record', async () => {
    const { state, review } = harness({ impacts: [{ checkpoint_id: 4 }], gainImpact: null, payload: { n: 3 } }, {
      writes: [() => Promise.reject(stale())],
      refreshes: [async () => ({ impacts: [], gainImpact: null, payload: { n: 3 } })]
    });
    review.record();
    review.confirm();
    await settle();
    expect(state.modal).toBeNull();
    expect(state.writes).toHaveLength(1);
    expect(state.preview?.impacts).toEqual([]);
  });

  it('a failed refresh discards the preview and shows the refresh error', async () => {
    const failure = new Error('preview failed');
    const { state, review } = harness({ impacts: [], gainImpact: null, payload: { n: 1 } }, {
      writes: [() => Promise.reject(stale())],
      refreshes: [() => Promise.reject(failure)]
    });
    review.record();
    await settle();
    expect(state.preview).toBeNull();
    expect(state.error).toBe(failure);
    expect(state.modal).toBeNull();
    expect(state.pending).toBe(false);
  });

  it('any other write refusal keeps the preview and shows the error', async () => {
    const refusal = new APIClientError({ status: 422, code: 'VALIDATION_FAILED' });
    const { state, review } = harness({ impacts: [], gainImpact: null, payload: { n: 1 } }, {
      writes: [() => Promise.reject(refusal)]
    });
    review.record();
    await settle();
    expect(state.error).toBe(refusal);
    expect(state.preview).not.toBeNull();
    expect(state.refreshed).toEqual([]);
    expect(state.pending).toBe(false);
  });

  it('clears an earlier error when it commits', async () => {
    const { state, review } = harness({ impacts: [], gainImpact: null, payload: { n: 1 } });
    state.error = new Error('earlier');
    review.record();
    await settle();
    expect(state.error).toBeUndefined();
  });
});

describe('reviewGuards', () => {
  it('sends no override or acknowledgement when nothing was reviewed', () => {
    expect(reviewGuards([], null)).toEqual({});
    expect(reviewGuards([], { acknowledgement: 'x', changes: [] } as unknown as GainImpact)).toEqual({});
  });
});
