<script lang="ts">
  import { m } from '#lib/paraglide/messages.js';
  import { attributionReason } from './attribution-reason.ts';

  let { diagnostic }: { diagnostic: { status: string; reasons: string[]; unavailable: boolean } } = $props();
</script>

{#if diagnostic.status !== 'passed'}
  <div class="rounded-(--radius-control) border border-warning/40 bg-warning-soft p-3 text-sm" data-testid="investment-attribution-diagnostic">
    <p class="font-medium text-foreground">{m.investments_attribution_review()}</p>
    {#each diagnostic.reasons as reason (reason)}
      <p class="mt-1 text-foreground">{attributionReason(reason)}</p>
    {/each}
    {#if diagnostic.unavailable}
      <p class="mt-1 text-foreground">{m.investments_attribution_unavailable()}</p>
    {/if}
    <p class="mt-1 text-muted">{m.settings_data_selfcheck_attribution_next()}</p>
  </div>
{/if}
