<script lang="ts">
  import ChevronUp from '@lucide/svelte/icons/chevron-up';
  import ChevronDown from '@lucide/svelte/icons/chevron-down';
  import { m } from '#lib/paraglide/messages.js';

  // One row's arrows in reorder mode (#185). Lists show the newest first, so
  // up is later in the day and down is earlier. An arrow with no same-day
  // neighbour in its direction is disabled: moves never leave the day. While a
  // move is pending the arrows are only aria-disabled: a disabled button drops
  // keyboard focus, and the moved row should keep it across the refresh (#221).
  let { canMoveLater, canMoveEarlier, pending, onLater, onEarlier }: {
    canMoveLater: boolean;
    canMoveEarlier: boolean;
    pending: boolean;
    onLater: () => void;
    onEarlier: () => void;
  } = $props();

  const buttonClass = 'inline-flex h-9 w-9 items-center justify-center rounded-(--radius-control) border border-border bg-control text-foreground transition hover:bg-control-hover disabled:opacity-30 aria-disabled:opacity-30';
</script>

<div class="flex items-center gap-1">
  <button type="button" class={buttonClass} title={m.transactions_reorder_later()} aria-label={m.transactions_reorder_later()}
    disabled={!canMoveLater} aria-disabled={pending} onclick={(event) => { event.stopPropagation(); if (!pending) onLater(); }}>
    <ChevronUp size={16} aria-hidden="true" />
  </button>
  <button type="button" class={buttonClass} title={m.transactions_reorder_earlier()} aria-label={m.transactions_reorder_earlier()}
    disabled={!canMoveEarlier} aria-disabled={pending} onclick={(event) => { event.stopPropagation(); if (!pending) onEarlier(); }}>
    <ChevronDown size={16} aria-hidden="true" />
  </button>
</div>
