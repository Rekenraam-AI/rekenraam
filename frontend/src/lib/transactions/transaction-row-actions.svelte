<script lang="ts">
  import { m } from '#lib/paraglide/messages.js';
  import type { RowActionTransaction, TransactionRowActionHandler } from './row-action.ts';

  let {
    transaction,
    onAction,
    disabled = false
  }: {
    transaction: RowActionTransaction;
    onAction: TransactionRowActionHandler;
    disabled?: boolean;
  } = $props();

  const isPosted = $derived(transaction.status === 'posted' && transaction.transaction_kind !== 'investment');
  const isVoided = $derived(transaction.status === 'voided');
  const needsReview = $derived(transaction.needs_review);

  const btnClass =
    'rounded-(--radius-control) px-2.5 py-1 text-xs font-medium text-muted ring-1 ring-border transition hover:bg-surface-strong hover:text-foreground focus:outline-none focus:ring-accent disabled:cursor-not-allowed disabled:opacity-60';
</script>

<!-- The row retains its row role; each action handles its own click/key. -->
<div class="flex flex-wrap items-center justify-end gap-2" role="group" aria-label={m.transactions_detail_actions()}>
  {#if isPosted}
    <button type="button" {disabled} class={btnClass}
      onclick={(e) => { e.stopPropagation(); onAction('edit', transaction.id); }}>
      {m.transactions_action_edit()}
    </button>
    <button type="button" {disabled} class={btnClass}
      onclick={(e) => { e.stopPropagation(); onAction('correct', transaction.id); }}>
      {m.transactions_action_correct()}
    </button>
  {/if}
  {#if isVoided}
    <button type="button" {disabled} class={btnClass}
      onclick={(e) => { e.stopPropagation(); onAction('view', transaction.id); }}>
      {m.transactions_action_view()}
    </button>
    {#if transaction.transaction_kind !== 'investment'}
      <button type="button" {disabled} class={btnClass}
        onclick={(e) => { e.stopPropagation(); onAction('unvoid', transaction.id); }}>
        {m.transactions_action_unvoid()}
      </button>
    {/if}
  {/if}
  {#if needsReview}
    <button type="button" {disabled}
      class={`${btnClass} text-accent ring-accent/30`}
      onclick={(e) => { e.stopPropagation(); onAction('approve', transaction.id); }}>
      {m.transactions_action_approve()}
    </button>
  {/if}
</div>
