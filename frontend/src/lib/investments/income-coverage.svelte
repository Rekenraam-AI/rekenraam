<script lang="ts">
  import { createInfiniteQuery, createQuery } from '@tanstack/svelte-query';
  import PageHeader from '#lib/components/page-header.svelte';
  import Panel from '#lib/components/panel.svelte';
  import APIFormError from '#lib/components/api-form-error.svelte';
  import { currenciesQueryOptions } from '#lib/api/currencies.ts';
  import { getIncomeCoverage, incomeCoverageQueryKey, investmentInstrumentsQueryOptions, type IncomeCoverageItem } from '#lib/api/investments.ts';
  import { m } from '#lib/paraglide/messages.js';
  import { getLocale } from '#lib/paraglide/runtime.js';
  import { coverageMoney, coverageRepairHref, type CoverageAmount } from './income-coverage';

  // Income attribution coverage (#244): owner-confirmed income per paying
  // instrument beside the book's identifiable unassigned cash. Unknown facts
  // stay labelled unknown; nothing here is required before using the app.
  const locale = $derived(getLocale());
  const dates = $derived(new Intl.DateTimeFormat(locale, { dateStyle: 'medium', timeZone: 'UTC' }));
  const instruments = createQuery(() => investmentInstrumentsQueryOptions());
  const currencies = createQuery(() => currenciesQueryOptions());
  const coverage = createInfiniteQuery(() => ({
    queryKey: [...incomeCoverageQueryKey, 'book'],
    initialPageParam: undefined as string | undefined,
    queryFn: ({ pageParam }) => getIncomeCoverage({ cursor: pageParam }),
    getNextPageParam: (last) => last.next_cursor ?? undefined
  }));
  const first = $derived(coverage.data?.pages[0]);
  const items = $derived(coverage.data?.pages.flatMap((page) => page.unassigned) ?? []);
  const loading = $derived(coverage.isPending || instruments.isPending || currencies.isPending);
  const loadError = $derived(coverage.error ?? instruments.error ?? currencies.error);

  function money(amount: CoverageAmount, currencyID: number, code: string): string {
    return coverageMoney(amount, currencyID, code, currencies.data?.currencies ?? [], locale);
  }
  function instrumentName(commodityID: number): string {
    const instrument = instruments.data?.instruments.find((item) => item.commodity_id === commodityID);
    return instrument ? `${instrument.display_name} (${instrument.symbol || instrument.commodity_code})` : `#${commodityID}`;
  }
  function day(date: string): string {
    return dates.format(new Date(`${date}T00:00:00Z`));
  }
  function grossText(item: IncomeCoverageItem): string {
    if (item.gross_state === 'unresolved') return m.income_coverage_gross_unresolved();
    if (item.gross_state === 'none' || !item.gross) return m.income_coverage_gross_none();
    return money(item.gross, item.cash_commodity_id, item.currency_code);
  }
  function withholdingText(item: IncomeCoverageItem): string {
    if (item.withholding_state === 'unresolved') return m.income_coverage_withholding_unresolved();
    if (item.withholding_state === 'known_zero' || !item.withholding) return m.income_coverage_withholding_zero();
    return money(item.withholding, item.cash_commodity_id, item.currency_code);
  }
  function payerText(item: IncomeCoverageItem): string {
    if (item.payer_state === 'source_identified' && item.source_payer_commodity_id) {
      return m.income_coverage_payer_source({ instrument: instrumentName(item.source_payer_commodity_id) });
    }
    return m.income_coverage_payer_unassigned();
  }
</script>

<PageHeader title={m.income_coverage_title()} copy={m.income_coverage_help()} />
<div class="mt-6 max-w-4xl space-y-5">
  <a href="/app/investments" class="inline-flex min-h-11 items-center text-sm font-semibold text-foreground underline underline-offset-2">{m.investments_instrument_view_back()}</a>

  {#if loading}
    <Panel><p role="status" class="text-sm text-muted">{m.income_coverage_loading()}</p></Panel>
  {:else if loadError}
    <APIFormError error={loadError} />
    <button type="button" class="min-h-11 text-sm text-foreground underline" onclick={() => { coverage.refetch(); instruments.refetch(); currencies.refetch(); }}>{m.install_gate_retry()}</button>
  {:else if first && first.instruments.length === 0 && first.unassigned_totals.length === 0}
    <Panel><p role="status" class="text-sm text-muted">{m.income_coverage_empty()}</p></Panel>
  {:else if first}
    <section aria-labelledby="income-coverage-attributed" class="space-y-3">
      <h2 id="income-coverage-attributed" class="text-base font-semibold text-foreground">{m.income_coverage_attributed_title()}</h2>
      {#if first.instruments.length === 0}
        <Panel><p class="text-sm text-muted">{m.income_coverage_attributed_empty()}</p></Panel>
      {:else}
        <ul class="space-y-3">
          {#each first.instruments as row (`${row.commodity_id}-${row.cash_commodity_id}`)}
            <li>
              <Panel>
                <a href={`/app/investments/instruments/${row.commodity_id}`} class="inline-flex min-h-11 items-center font-semibold text-foreground underline underline-offset-4">{instrumentName(row.commodity_id)}</a>
                <dl class="mt-2 grid grid-cols-1 gap-2 text-sm sm:grid-cols-3">
                  <div>
                    <dt class="text-xs text-muted">{m.income_coverage_gross()} · {row.currency_code}</dt>
                    <dd class="font-medium tabular-nums text-foreground">{row.income_state === 'known' && row.gross ? money(row.gross, row.cash_commodity_id, row.currency_code) : m.income_coverage_gross_none()}</dd>
                  </div>
                  <div>
                    <dt class="text-xs text-muted">{m.income_coverage_withholding()}</dt>
                    <dd class="font-medium tabular-nums text-foreground">
                      {#if row.withholding_state === 'known' && row.withholding}{money(row.withholding, row.cash_commodity_id, row.currency_code)}
                      {:else if row.withholding_state === 'known_zero'}{m.income_coverage_withholding_zero()}
                      {:else}{m.income_coverage_withholding_none()}{/if}
                    </dd>
                  </div>
                  <div>
                    <dt class="text-xs text-muted">{m.income_coverage_net_cash()}</dt>
                    <dd class="font-medium tabular-nums text-foreground">{row.net_cash ? money(row.net_cash, row.cash_commodity_id, row.currency_code) : '—'}</dd>
                  </div>
                </dl>
                <p class="mt-2 text-xs text-muted">{m.income_coverage_counts({ dividends: row.dividend_count, taxes: row.withholding_payment_count })}</p>
                {#if row.imported_unclassified_count > 0 && row.imported_unclassified_cash}
                  <p class="mt-2 text-sm text-warning">{m.income_coverage_instrument_pending({ count: row.imported_unclassified_count, cash: money(row.imported_unclassified_cash, row.cash_commodity_id, row.currency_code) })}</p>
                {/if}
              </Panel>
            </li>
          {/each}
        </ul>
      {/if}
    </section>

    <section aria-labelledby="income-coverage-unassigned" class="space-y-3">
      <h2 id="income-coverage-unassigned" class="text-base font-semibold text-foreground">{m.income_coverage_unassigned_title()}</h2>
      {#if first.unassigned_totals.length === 0}
        <Panel><p role="status" class="text-sm text-muted">{m.income_coverage_unassigned_empty()}</p></Panel>
      {:else}
        <Panel>
          <p class="text-sm text-muted">{m.income_coverage_unassigned_help()}</p>
          <ul class="mt-3 space-y-1 text-sm">
            {#each first.unassigned_totals as total (total.cash_commodity_id)}
              <li class="text-foreground">
                <span class="font-semibold tabular-nums">{money(total.cash, total.cash_commodity_id, total.currency_code)}</span>
                · {m.income_coverage_total_counts({ count: total.item_count, payer: total.payer_unassigned_count, economics: total.economics_unresolved_count })}
              </li>
            {/each}
          </ul>
        </Panel>
        <ul class="space-y-3">
          {#each items as item (`${item.transaction_id}-${item.cash_commodity_id}`)}
            <li>
              <Panel>
                <p class="font-semibold text-foreground">
                  <span class="tabular-nums">{money(item.cash, item.cash_commodity_id, item.currency_code)}</span>
                  <span class="text-sm font-normal text-muted"> · {day(item.date)} · {item.kind === 'imported_cash' ? m.income_coverage_kind_imported() : m.income_coverage_kind_named()}</span>
                </p>
                <dl class="mt-2 grid grid-cols-1 gap-2 text-sm sm:grid-cols-3">
                  <div><dt class="text-xs text-muted">{m.income_coverage_payer()}</dt><dd class="text-foreground">{payerText(item)}</dd></div>
                  <div><dt class="text-xs text-muted">{m.income_coverage_gross()}</dt><dd class="text-foreground">{grossText(item)}</dd></div>
                  <div><dt class="text-xs text-muted">{m.income_coverage_withholding()}</dt><dd class="text-foreground">{withholdingText(item)}</dd></div>
                </dl>
                {#if item.source}
                  <p class="mt-2 text-xs text-muted">{m.income_coverage_source({ instrument: item.source.instrument || '—', amount: item.source.cash_amount, currency: item.source.currency, date: item.source.date })}</p>
                {/if}
                <a href={coverageRepairHref(item)} class="mt-2 inline-flex min-h-11 items-center text-sm font-semibold text-foreground underline underline-offset-2">
                  {item.kind === 'imported_cash' ? m.income_coverage_repair_imported() : m.income_coverage_repair_named()}
                </a>
              </Panel>
            </li>
          {/each}
        </ul>
        <p class="text-xs text-muted">{m.income_coverage_optional()}</p>
        {#if coverage.hasNextPage}
          <button type="button" disabled={coverage.isFetchingNextPage} onclick={() => coverage.fetchNextPage()}
            class="min-h-11 rounded-(--radius-control) border border-border bg-control px-4 py-2 text-sm text-foreground transition-colors">{m.income_coverage_more()}</button>
        {/if}
      {/if}
    </section>
  {/if}
</div>
