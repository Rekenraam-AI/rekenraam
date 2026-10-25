<script lang="ts">
  import { m } from '#lib/paraglide/messages.js';
  let { status }: { status: string | undefined } = $props();
</script>

{#if status && status !== 'usable'}
  <span class="block text-xs text-muted">
    {#if status === 'stale'}{m.valuation_price_stale()}
    {:else if status === 'future_only'}{m.valuation_price_future()}
    {:else if status === 'incompatible_quote_basis'}{m.valuation_price_incompatible()}
    {:else if status === 'conflicting_splits'}{m.valuation_price_conflicting_splits()}
    {:else if status === 'not_known_at_cutoff'}{m.valuation_price_unknown()}
    {:else if status === 'no_observation_in_window'}{m.valuation_price_window()}
    {:else}{m.valuation_price_missing()}{/if}
  </span>
{/if}
