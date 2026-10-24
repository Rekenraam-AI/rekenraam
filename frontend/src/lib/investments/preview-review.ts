import type { GainImpact } from '#lib/api/investments.ts';
import {
  checkpointOverride,
  gainAcknowledgement,
  impactNeedsReview,
  isStaleImpactRefusal,
  type ReviewedCheckpoints
} from '#lib/investments/gain-impact.ts';

/**
 * The preview-then-record review protocol shared by the investment forms whose
 * preview names both affected checkpoints and changed gains (#400): split,
 * capital return, spin-off, share exchange, cash-and-stock merger, internal
 * transfer and external transfer out. Correction, resolution and provider
 * completion flows review at commit instead and keep their own protocols.
 *
 * The server recomputes everything at commit. A stale refusal previews the
 * same payload again and re-opens the review with the current figures; it
 * never retries the write or reuses an acknowledgement. The host keeps its own
 * reactive state: this only moves it.
 */
export type ReviewablePreview<P> = {
  impacts: ReviewedCheckpoints;
  gainImpact: GainImpact | null;
  payload: P;
};

export type ReviewModal = { gainRefreshed: boolean };

/** What the write sends: exactly the reviewed checkpoints and gain set. */
export type ReviewGuards = {
  reconciliation_override?: boolean;
  reconciliation_override_checkpoint_ids?: number[];
  gain_impact_acknowledgement?: string;
};

export type PreviewReviewHost<P, T extends ReviewablePreview<P>> = {
  readonly preview: T | null;
  /** Previews the payload again, replacing the host's preview. */
  refresh(payload: P): Promise<void>;
  /** Records the reviewed preview once; may invalidate reads and close. */
  write(preview: T, guards: ReviewGuards): Promise<void>;
  discardPreview(): void;
  setPending(pending: boolean): void;
  setError(error: unknown): void;
  setModal(modal: ReviewModal | null): void;
};

export function reviewGuards(reviewed: ReviewedCheckpoints, gainImpact: GainImpact | null): ReviewGuards {
  const acknowledgement = gainAcknowledgement(gainImpact);
  return {
    ...checkpointOverride(reviewed),
    ...(acknowledgement ? { gain_impact_acknowledgement: acknowledgement } : {})
  };
}

function needsReview<P>(preview: ReviewablePreview<P>): boolean {
  return impactNeedsReview({ affected_checkpoints: preview.impacts, gain_impact: preview.gainImpact });
}

export function previewReview<P, T extends ReviewablePreview<P>>(host: PreviewReviewHost<P, T>) {
  async function commit(reviewed: ReviewedCheckpoints) {
    const current = host.preview;
    if (!current) return;
    host.setPending(true);
    host.setError(undefined);
    try {
      await host.write(current, reviewGuards(reviewed, current.gainImpact));
    } catch (error) {
      if (isStaleImpactRefusal(error) && host.preview) {
        try {
          await host.refresh(host.preview.payload);
          const refreshed = host.preview;
          if (refreshed && needsReview(refreshed)) host.setModal({ gainRefreshed: !!refreshed.gainImpact });
        } catch (previewError) {
          host.discardPreview();
          host.setError(previewError);
        }
      } else {
        host.setError(error);
      }
    } finally {
      host.setPending(false);
    }
  }

  return {
    /** Opens the review when the preview needs a decision; otherwise records. */
    record() {
      const current = host.preview;
      if (!current) return;
      if (needsReview(current)) {
        host.setModal({ gainRefreshed: false });
        return;
      }
      void commit([]);
    },
    /** Records with exactly the checkpoints and gain set the review showed. */
    confirm() {
      const current = host.preview;
      if (!current) return;
      const reviewed = current.impacts;
      host.setModal(null);
      void commit(reviewed);
    },
    cancel() {
      host.setModal(null);
    }
  };
}
