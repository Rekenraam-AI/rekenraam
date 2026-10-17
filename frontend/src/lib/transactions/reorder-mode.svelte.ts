import type { QueryClient } from '@tanstack/svelte-query';
import { APIClientError } from '#lib/api/client.ts';
import { forecastQueryKey } from '#lib/api/forecast.ts';
import { accountRegisterQueryKey, transactionsQueryKey } from '#lib/api/transactions.ts';

/**
 * The reorder mode of one list screen (#185): off by default, so the arrows
 * stay out of everyday use. A move that would change a reconciled balance is
 * refused by the server with CONFLICT (the reorder endpoints have no other
 * conflict); the mode then asks before retrying with the override, which
 * invalidates that reconciliation. A reorder has no preview, so the refusal's
 * checkpoint ids are what the owner reviewed: the override names them and the
 * server refuses it if the set has changed since (#296).
 */
type ReorderAction = (reviewedCheckpointIDs?: number[]) => Promise<unknown>;

export class ReorderMode {
  active = $state(false);
  /** Account and category screens only: move in this account alone. */
  onlyThisAccount = $state(false);
  pending = $state(false);
  error = $state<unknown>(undefined);
  /** A refused move waiting for the user's decision on the override. */
  awaitingOverride = $state<ReorderAction | null>(null);
  /** The checkpoints the refused move would invalidate, as the server named them. */
  reviewedCheckpointIDs = $state<number[]>([]);
  /** A move to another posting day (#187) waiting for the user's confirmation. */
  dayMove = $state<{ date: string; action: ReorderAction } | null>(null);

  constructor(private readonly queryClient: QueryClient) {}

  start() {
    this.active = true;
    this.error = undefined;
  }

  finish() {
    this.active = false;
    this.onlyThisAccount = false;
    this.awaitingOverride = null;
    this.dayMove = null;
    this.error = undefined;
  }

  async move(action: ReorderAction) {
    this.pending = true;
    this.error = undefined;
    this.awaitingOverride = null;
    try {
      await action();
      await this.refresh();
    } catch (error) {
      if (error instanceof APIClientError && error.code === 'CONFLICT') {
        this.awaitingOverride = action;
        this.reviewedCheckpointIDs = error.checkpointIDs ?? [];
      } else {
        this.error = error;
      }
    } finally {
      this.pending = false;
    }
  }

  async confirmOverride() {
    const action = this.awaitingOverride;
    if (!action) return;
    this.pending = true;
    this.awaitingOverride = null;
    try {
      await action(this.reviewedCheckpointIDs);
      await this.refresh();
    } catch (error) {
      this.error = error;
      if (error instanceof APIClientError && error.code === 'RECONCILIATION_IMPACT_STALE') {
        // Another statement changed meanwhile: ask again about the set as it is now.
        this.awaitingOverride = action;
        this.reviewedCheckpointIDs = error.checkpointIDs ?? [];
      }
    } finally {
      this.pending = false;
    }
  }

  cancelOverride() {
    this.awaitingOverride = null;
    this.reviewedCheckpointIDs = [];
  }

  /** At a day's edge an arrow offers the adjacent day; the move waits for a yes. */
  proposeDayMove(date: string, action: ReorderAction) {
    this.error = undefined;
    this.awaitingOverride = null;
    this.dayMove = { date, action };
  }

  async confirmDayMove() {
    const proposal = this.dayMove;
    if (!proposal) return;
    this.dayMove = null;
    await this.move(proposal.action);
  }

  cancelDayMove() {
    this.dayMove = null;
  }

  // A move can change the journal, any register's running balance and the forecast.
  private async refresh() {
    await Promise.all([
      this.queryClient.invalidateQueries({ queryKey: transactionsQueryKey }),
      this.queryClient.invalidateQueries({ queryKey: accountRegisterQueryKey }),
      this.queryClient.invalidateQueries({ queryKey: forecastQueryKey })
    ]);
  }
}
