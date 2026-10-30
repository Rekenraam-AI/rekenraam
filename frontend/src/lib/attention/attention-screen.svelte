<script lang="ts">
  import { useAttention } from './context.ts';
  import { attentionSummary, sourceDetails, sourceStatus } from './model.ts';
  import { m } from '#lib/paraglide/messages.js';
  import { getLocale } from '#lib/paraglide/runtime.js';
  import Panel from '#lib/components/panel.svelte';

  const attention = useAttention();
  const query = $derived(attention());
  const summary = $derived(attentionSummary(query, getLocale()));
  const checkedAt = $derived(query.data ? new Intl.DateTimeFormat(getLocale(), { dateStyle: 'medium', timeStyle: 'short' }).format(new Date(query.data.checked_at)) : '');
</script>

<Panel>
  <div class="flex flex-wrap items-start justify-between gap-3">
    <div aria-live="polite">
      <h2 class="text-lg font-semibold">{summary.label}</h2>
      {#if checkedAt}<p class="mt-1 text-sm text-muted">{m.attention_checked_at({ when: checkedAt })}</p>{/if}
      {#if query.data?.complete && !query.isError && !query.data.has_failures && query.data.total_known === 0}
        <p class="mt-2 text-sm text-muted">{m.attention_clear()}</p>
      {/if}
    </div>
    <button type="button" disabled={query.isFetching} onclick={() => query.refetch()} class="min-h-11 rounded-(--radius-control) border border-border px-4 text-sm font-semibold text-foreground hover:bg-control-hover disabled:opacity-50">{m.attention_retry()}</button>
  </div>
  {#if query.isError}<p role="alert" class="mt-3 text-sm text-danger">{m.attention_source_error()}</p>{/if}
  {#if query.data}
    <ul class="mt-4 divide-y divide-border">
      {#each query.data.sources as source (source.id)}
        <li class="flex flex-wrap items-center justify-between gap-x-4 gap-y-1 py-3">
          <a href={sourceDetails[source.id].href} class="min-h-11 content-center text-sm font-medium text-foreground underline decoration-border underline-offset-4 hover:decoration-foreground">{sourceDetails[source.id].title()}</a>
          <span class="text-sm text-muted">{sourceStatus(source, getLocale())}</span>
        </li>
      {/each}
    </ul>
  {/if}
</Panel>
