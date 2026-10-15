<script lang="ts">
  import { createQuery } from '@tanstack/svelte-query';
  import Panel from '#lib/components/panel.svelte';
  import { currenciesQueryOptions } from '#lib/api/currencies.ts';
  import { instrumentIncomeCoverageQueryOptions } from '#lib/api/investments.ts';
  import { m } from '#lib/paraglide/messages.js';
  import { getLocale } from '#lib/paraglide/runtime.js';
  import { coverageMoney, type CoverageAmount } from './income-coverage';

  // An instrument's owner-confirmed dividend income (#244), with the book's
  // unassigned scope beside it so an empty or partial figure never reads as
  // complete. One summary request; payments are listed in the activity below.
  let { commodityID }: { commodityID: number } = $props();
  const locale = $derived(getLocale());
  const coverage = createQuery(() => instrumentIncomeCoverageQueryOptions(commodityID));
  const currencies = createQuery(() => currenciesQueryOptions());
  const rows = $derived(coverage.data?.instruments ?? []);
  const totals = $derived(coverage.data?.unassigned_totals ?? []);

  function money(amount: CoverageAmount, currencyID: number, code: string): string {
    return coverageMoney(amount, currencyID, code, currencies.data?.currencies ?? [], locale);
  }
</script>

<Panel>
  <h3 class="text-sm font-semibold text-foreground">{m.income_coverage_instrument_title()}</h3>
  {#if coverage.isPending}
    <p role="status" class="mt-2 text-sm text-muted">{m.income_coverage_loading()}</p>
  {:else if coverage.isError}
    <p role="alert" class="mt-2 text-sm text-danger">{m.income_coverage_error()}</p>
    <button type="button" class="min-h-11 text-sm text-foreground underline" onclick={() => coverage.refetch()}>{m.install_gate_retry()}</button>
  {:else}
    {#if rows.length === 0}
      <p class="mt-2 text-sm text-muted">{m.income_coverage_instrument_none()}</p>
    {/if}
    <ul class="mt-2 space-y-2 text-sm">
      {#each rows as row (row.cash_commodity_id)}
        <li class="text-foreground">
          {#if row.income_state === 'known' && row.gross}
            <span>{m.income_coverage_gross()}: <span class="font-semibold tabular-nums">{money(row.gross, row.cash_commodity_id, row.currency_code)}</span></span>
          {:else}
            <span>{m.income_coverage_gross_none()} ({row.currency_code})</span>
          {/if}
          {#if row.withholding_state === 'known' && row.withholding}
            <span class="text-muted"> · {m.income_coverage_withholding()}: <span class="tabular-nums">{money(row.withholding, row.cash_commodity_id, row.currency_code)}</span></span>
          {:else if row.withholding_state === 'known_zero'}
            <span class="text-muted"> · {m.income_coverage_withholding_zero()}</span>
          {/if}
          {#if row.imported_unclassified_count > 0 && row.imported_unclassified_cash}
            <span class="block text-warning">{m.income_coverage_instrument_pending({ count: row.imported_unclassified_count, cash: money(row.imported_unclassified_cash, row.cash_commodity_id, row.currency_code) })}</span>
          {/if}
        </li>
      {/each}
    </ul>
    {#if totals.length > 0}
      <p class="mt-2 text-sm text-muted">
        {m.income_coverage_instrument_excluded({ amounts: totals.map((total) => money(total.cash, total.cash_commodity_id, total.currency_code)).join(', '), count: totals.reduce((sum, total) => sum + total.item_count, 0) })}
      </p>
    {/if}
    <a href="/app/investments/income" class="inline-flex min-h-11 items-center text-sm font-semibold text-foreground underline underline-offset-2">{m.income_coverage_open()}</a>
  {/if}
</Panel>
