<script lang="ts">
  import { createQuery } from '@tanstack/svelte-query';
  import { goto } from '$app/navigation';
  import { page } from '$app/state';
  import PageHeader from '#lib/components/page-header.svelte';
  import Panel from '#lib/components/panel.svelte';
  import StatePanel from '#lib/components/state-panel.svelte';
  import APIFormError from '#lib/components/api-form-error.svelte';
  import { accountsQueryOptions, type AccountResponse } from '#lib/api/accounts.ts';
  import { currentBookQueryOptions } from '#lib/api/books.ts';
  import { currenciesQueryOptions } from '#lib/api/currencies.ts';
  import { investmentInstrumentsQueryOptions, investmentReturnsQueryOptions, type ReturnFigure, type ReturnsAmount } from '#lib/api/investments.ts';
  import { useOwnerToday } from '#lib/dates/owner-date-context.ts';
  import { formatMoney, joinCommodityAmount } from '#lib/money/format.ts';
  import { m } from '#lib/paraglide/messages.js';
  import { getLocale } from '#lib/paraglide/runtime.js';
  import {
    fallbackReasonText, formatReturn, parseReturnsSelection, presetRange, returnDirection, returnReasonText,
    returnsCoverage, returnsSelectionParams, unavailableAccountIDs, validRange, type ReturnsPreset
  } from './returns-model';
  import ReturnsChart from './returns-chart.svelte';

  // R13 portfolio returns (#119, slice 2): a read-only view of the returns_v1
  // live read. Every missing figure says why; nothing here posts or moves money.
  const ownerToday = useOwnerToday();
  const today = ownerToday();
  const locale = $derived(getLocale());
  const dates = $derived(new Intl.DateTimeFormat(locale, { dateStyle: 'medium', timeZone: 'UTC' }));
  const instants = $derived(new Intl.DateTimeFormat(locale, { dateStyle: 'medium', timeStyle: 'short' }));

  // Archived accounts stay selectable: a past period can need them.
  const accounts = createQuery(() => accountsQueryOptions(true));
  const currencies = createQuery(() => currenciesQueryOptions());
  const book = createQuery(() => currentBookQueryOptions());
  const instruments = createQuery(() => investmentInstrumentsQueryOptions());

  const selection = $derived(parseReturnsSelection(page.url.searchParams, today));
  const activeCurrencies = $derived((currencies.data?.currencies ?? []).filter((currency) => currency.status === 'active'));
  // The owner's choice wins; otherwise propose the book's default currency for
  // new UI, then the first active currency. The response names whichever is used.
  const reportingCurrencyID = $derived(
    selection.reportingCurrencyID
      ?? book.data?.default_currency_commodity_id
      ?? activeCurrencies[0]?.id
      ?? null
  );
  const portfolioAccounts = $derived(
    (accounts.data?.accounts ?? [])
      .filter((account) => !account.is_system && (account.account_class === 'asset' || account.account_class === 'liability'))
      .sort((a, b) => accountLabel(a).localeCompare(accountLabel(b), locale))
  );

  // Draft controls, applied to the URL on submit; reset when the URL moves
  // (back/forward, a link), so the form always describes what is shown.
  let fromDraft = $state('');
  let toDraft = $state('');
  let currencyDraft = $state('');
  let accountDraft = $state<number[]>([]);
  $effect(() => {
    fromDraft = selection.from;
    toDraft = selection.to;
    accountDraft = [...selection.accountIDs];
  });
  $effect(() => {
    currencyDraft = reportingCurrencyID === null ? '' : String(reportingCurrencyID);
  });
  const rangeInvalid = $derived(!validRange(fromDraft, toDraft, today));
  const unavailableDraft = $derived(unavailableAccountIDs(accountDraft, portfolioAccounts.map((account) => account.id)));

  const returns = createQuery(() => ({
    ...investmentReturnsQueryOptions({
      from: selection.from,
      to: selection.to,
      reportingCurrencyID: reportingCurrencyID ?? 0,
      accountIDs: selection.accountIDs
    }),
    enabled: reportingCurrencyID !== null
  }));
  const result = $derived(returns.data);
  const referencesLoading = $derived(accounts.isPending || currencies.isPending || book.isPending);
  const referenceError = $derived(accounts.error ?? currencies.error ?? book.error);
  const coverage = $derived(returnsCoverage(result?.valuation));
  const seriesCoverage = $derived(returnsCoverage(result?.time_weighted_series.valuation));

  function apply(range?: { from: string; to: string }) {
    const from = range?.from ?? fromDraft;
    const to = range?.to ?? toDraft;
    if (!validRange(from, to, today) || currencyDraft === '') return;
    // An id that is not a selectable account here would fail every request;
    // it is listed as unavailable above the button and left out.
    const selectable = accountDraft.filter((id) => !unavailableDraft.includes(id));
    const params = returnsSelectionParams({
      from, to, reportingCurrencyID: Number(currencyDraft), accountIDs: [...selectable].sort((a, b) => a - b)
    });
    void goto(`/app/investments/returns?${params.toString()}`, { reset: false });
  }

  function choosePreset(preset: ReturnsPreset) {
    apply(presetRange(preset, today));
  }

  function toggleAccount(id: number, checked: boolean) {
    accountDraft = checked ? [...accountDraft, id] : accountDraft.filter((existing) => existing !== id);
  }

  function accountLabel(account: AccountResponse): string {
    const name = account.name || account.code || `#${account.id}`;
    if (account.status === 'archived') return m.returns_account_archived({ name });
    if (account.status === 'closed') return m.returns_account_closed({ name });
    return name;
  }

  function accountName(id: number): string {
    const account = accounts.data?.accounts.find((candidate) => candidate.id === id);
    return account ? accountLabel(account) : `#${id}`;
  }

  function currencyCode(id: number): string {
    return currencies.data?.currencies.find((currency) => currency.id === id)?.code ?? `#${id}`;
  }

  // A coverage gap names a currency or a priced instrument.
  function currencyOrCommodity(id: number): string {
    const currency = currencies.data?.currencies.find((candidate) => candidate.id === id);
    if (currency) return currency.code;
    const instrument = instruments.data?.instruments.find((candidate) => candidate.commodity_id === id);
    return instrument ? instrument.symbol || instrument.commodity_code : `#${id}`;
  }

  function day(date: string): string {
    return dates.format(new Date(`${date}T00:00:00Z`));
  }

  function amountText(amount: ReturnsAmount): string {
    if (!amount.amount) return returnReasonText(amount.reason, amount.date, day);
    const currency = currencies.data?.currencies.find((candidate) => candidate.id === amount.amount?.commodity_id);
    const formatted = formatMoney(amount.amount.quantity_value, amount.amount.quantity_scale, currency?.standard_scale ?? amount.amount.quantity_scale, locale);
    return joinCommodityAmount(currency?.code ?? `#${amount.amount.commodity_id}`, formatted);
  }

  function directionClass(figure: ReturnFigure): string {
    if (figure.value === null) return 'text-muted';
    const direction = returnDirection(figure.value);
    return direction === 'up' ? 'text-positive' : direction === 'down' ? 'text-danger' : 'text-foreground';
  }
</script>

<PageHeader title={m.returns_title()} copy={m.returns_help()} />
<div class="mt-6 max-w-5xl space-y-5 px-4 sm:px-6 lg:px-8" data-testid="returns-screen">
  <a href="/app/investments" class="inline-flex min-h-11 items-center text-sm font-semibold text-foreground underline underline-offset-2">{m.investments_instrument_view_back()}</a>

  {#if referencesLoading}
    <Panel><p role="status" class="text-sm text-muted">{m.returns_loading()}</p></Panel>
  {:else if referenceError}
    <StatePanel title={m.returns_error_title()} copy={m.returns_error_copy()} alert>
      <APIFormError error={referenceError} />
      <button type="button" class="min-h-11 text-sm text-foreground underline" onclick={() => { accounts.refetch(); currencies.refetch(); book.refetch(); }}>{m.install_gate_retry()}</button>
    </StatePanel>
  {:else if activeCurrencies.length === 0}
    <StatePanel title={m.returns_no_currency_title()} copy={m.returns_no_currency_copy()}>
      <a href="/app/settings" class="inline-flex min-h-11 items-center text-sm font-semibold text-foreground underline underline-offset-2">{m.returns_no_currency_action()}</a>
    </StatePanel>
  {:else}
    <Panel>
      <form
        class="space-y-4"
        aria-labelledby="returns-controls-title"
        onsubmit={(event) => { event.preventDefault(); apply(); }}
      >
        <h2 id="returns-controls-title" class="text-sm font-semibold text-foreground">{m.returns_controls_title()}</h2>
        <div class="flex flex-wrap gap-2">
          <button type="button" class="min-h-11 rounded-(--radius-control) border border-border bg-control px-3 text-sm text-foreground hover:bg-control-hover" onclick={() => choosePreset('ytd')}>{m.returns_preset_ytd()}</button>
          <button type="button" class="min-h-11 rounded-(--radius-control) border border-border bg-control px-3 text-sm text-foreground hover:bg-control-hover" onclick={() => choosePreset('last_year')}>{m.returns_preset_last_year()}</button>
        </div>
        <div class="grid grid-cols-1 gap-3 sm:grid-cols-3">
          <label class="flex flex-col gap-1 text-sm text-muted">
            {m.returns_from()}
            <input type="date" bind:value={fromDraft} max={today} aria-invalid={rangeInvalid}
              aria-describedby={rangeInvalid ? 'returns-range-error' : undefined}
              class="min-h-11 rounded border border-border bg-control px-2 text-foreground focus:outline-none focus:ring-1 focus:ring-foreground/30" />
          </label>
          <label class="flex flex-col gap-1 text-sm text-muted">
            {m.returns_to()}
            <input type="date" bind:value={toDraft} max={today} aria-invalid={rangeInvalid}
              aria-describedby={rangeInvalid ? 'returns-range-error' : undefined}
              class="min-h-11 rounded border border-border bg-control px-2 text-foreground focus:outline-none focus:ring-1 focus:ring-foreground/30" />
          </label>
          <label class="flex flex-col gap-1 text-sm text-muted">
            {m.returns_reporting_currency()}
            <select bind:value={currencyDraft}
              class="min-h-11 rounded border border-border bg-control px-2 text-foreground focus:outline-none focus:ring-1 focus:ring-foreground/30">
              {#each activeCurrencies as currency (currency.id)}
                <option value={String(currency.id)}>{currency.code}</option>
              {/each}
            </select>
          </label>
        </div>
        {#if rangeInvalid}
          <p id="returns-range-error" class="text-sm text-danger">{m.returns_range_invalid()}</p>
        {/if}

        <fieldset class="space-y-2">
          <legend class="text-sm font-medium text-foreground">{m.returns_accounts_title()}</legend>
          <p class="text-xs text-muted">{m.returns_accounts_help()}</p>
          {#if portfolioAccounts.length === 0}
            <p class="text-sm text-muted">{m.returns_accounts_none()}</p>
          {:else}
            <ul class="grid grid-cols-1 gap-1 sm:grid-cols-2">
              {#each portfolioAccounts as account (account.id)}
                <li>
                  <label class="flex min-h-11 items-center gap-2 text-sm text-foreground">
                    <input type="checkbox" checked={accountDraft.includes(account.id)}
                      onchange={(event) => toggleAccount(account.id, event.currentTarget.checked)} />
                    {accountLabel(account)}
                  </label>
                </li>
              {/each}
            </ul>
          {/if}
          {#if unavailableDraft.length > 0}
            <div role="status" class="space-y-1 text-sm text-warning">
              <p>{m.returns_accounts_unavailable()}</p>
              <ul class="flex flex-wrap gap-2">
                {#each unavailableDraft as id (id)}
                  <li>
                    <button type="button" class="min-h-11 rounded-(--radius-control) border border-border bg-control px-3 text-foreground hover:bg-control-hover"
                      onclick={() => toggleAccount(id, false)}>{m.returns_accounts_remove_unavailable({ account: `#${id}` })}</button>
                  </li>
                {/each}
              </ul>
            </div>
          {/if}
          {#if accountDraft.length > 0}
            <button type="button" class="min-h-11 text-sm text-foreground underline underline-offset-2" onclick={() => { accountDraft = []; }}>{m.returns_accounts_reset()}</button>
          {/if}
        </fieldset>

        <button type="submit" disabled={rangeInvalid || currencyDraft === ''}
          class="min-h-11 rounded-(--radius-control) border border-border bg-control px-4 text-sm font-medium text-foreground hover:bg-control-hover disabled:opacity-50">
          {m.returns_apply()}
        </button>
      </form>
    </Panel>

    {#if returns.isPending}
      <Panel><p role="status" class="text-sm text-muted">{m.returns_loading()}</p></Panel>
    {:else if returns.isError}
      <StatePanel title={m.returns_error_title()} copy={m.returns_error_copy()} alert>
        <APIFormError error={returns.error} />
        <button type="button" class="min-h-11 text-sm text-foreground underline" onclick={() => returns.refetch()}>{m.install_gate_retry()}</button>
      </StatePanel>
    {:else if result && result.resolved_account_ids.length === 0}
      <StatePanel title={m.returns_empty_title()} copy={m.returns_empty_copy()}>
        <a href="/app/accounts" class="inline-flex min-h-11 items-center text-sm font-semibold text-foreground underline underline-offset-2">{m.returns_empty_action()}</a>
      </StatePanel>
    {:else if result}
      <section aria-labelledby="returns-summary-title" class="space-y-3" data-testid="returns-summary" aria-busy={returns.isFetching}>
        {#if returns.isFetching}
          <p role="status" class="text-sm text-muted">{m.returns_updating()}</p>
        {/if}
        <h2 id="returns-summary-title" class="text-base font-semibold text-foreground">
          {m.returns_period({ from: day(result.from), to: day(result.to), currency: currencyCode(reportingCurrencyID ?? 0), days: result.days })}
        </h2>
        <div class="grid grid-cols-1 gap-3 md:grid-cols-2">
          {#each [
            { key: 'twr', title: m.returns_twr_title(), help: m.returns_twr_help(), period: result.time_weighted, annual: result.time_weighted_annualized },
            { key: 'mwr', title: m.returns_mwr_title(), help: m.returns_mwr_help(), period: result.money_weighted_period, annual: result.money_weighted }
          ] as card (card.key)}
            <Panel>
              <h3 class="text-sm font-semibold text-foreground">{card.title}</h3>
              <p class="mt-1 text-xs text-muted">{card.help}</p>
              <dl class="mt-3 grid grid-cols-2 gap-3">
                <div>
                  <dt class="text-xs text-muted">{m.returns_figure_period()}</dt>
                  <dd class="text-xl font-semibold tabular-nums {directionClass(card.period)}" data-testid="returns-{card.key}-period">
                    {card.period.value !== null ? formatReturn(card.period.value, locale) : '—'}
                  </dd>
                  {#if card.period.value === null}<dd class="text-xs text-muted">{returnReasonText(card.period.reason, card.period.date, day)}</dd>{/if}
                </div>
                <div>
                  <dt class="text-xs text-muted">{m.returns_figure_annual()}</dt>
                  <dd class="text-xl font-semibold tabular-nums {directionClass(card.annual)}" data-testid="returns-{card.key}-annual">
                    {card.annual.value !== null ? formatReturn(card.annual.value, locale) : '—'}
                  </dd>
                  {#if card.annual.value === null}<dd class="text-xs text-muted">{returnReasonText(card.annual.reason, card.annual.date, day)}</dd>{/if}
                </div>
              </dl>
            </Panel>
          {/each}
        </div>

        <Panel>
          <ReturnsChart series={result.time_weighted_series} {locale} formatDate={day}
            missingCommodities={seriesCoverage.missing.map(currencyOrCommodity)} staleDates={seriesCoverage.staleDates} />
        </Panel>

        <Panel>
          <h3 class="text-sm font-semibold text-foreground">{m.returns_values_title()}</h3>
          <dl class="mt-3 grid grid-cols-1 gap-3 text-sm sm:grid-cols-2 lg:grid-cols-5">
            {#each [
              { key: 'opening', label: m.returns_opening_value({ date: day(result.opening_date) }), amount: result.opening_value },
              { key: 'contributions', label: m.returns_contributions(), amount: result.contributions },
              { key: 'withdrawals', label: m.returns_withdrawals(), amount: result.withdrawals },
              { key: 'gain', label: m.returns_gain(), amount: result.gain },
              { key: 'closing', label: m.returns_closing_value({ date: day(result.to) }), amount: result.closing_value }
            ] as line (line.key)}
              <div>
                <dt class="text-xs text-muted">{line.label}</dt>
                <dd class="font-medium tabular-nums {line.amount.amount ? 'text-foreground' : 'text-muted'}" data-testid="returns-{line.key}">{amountText(line.amount)}</dd>
              </div>
            {/each}
          </dl>
        </Panel>

        {#if coverage.missing.length > 0}
          <p role="status" class="text-sm text-warning">{m.returns_coverage_incomplete({ commodities: coverage.missing.map(currencyOrCommodity).join(', ') })}</p>
        {/if}
        {#if coverage.staleDates.length > 0}
          <p class="text-sm text-muted">{m.returns_coverage_stale({ dates: coverage.staleDates.map(day).join(', ') })}</p>
        {/if}

        {#if result.attribution_fallbacks.length > 0}
          <Panel>
            <h3 class="text-sm font-semibold text-foreground">{m.returns_fallbacks_title({ count: result.attribution_fallbacks.length })}</h3>
            <p class="mt-1 text-xs text-muted">{m.returns_fallbacks_help()}</p>
            <ul class="mt-3 space-y-2 text-sm">
              {#each result.attribution_fallbacks as fallback (`${fallback.transaction_id}-${fallback.date}`)}
                <li class="text-foreground">
                  <a href={`/app/transactions?transaction_id=${fallback.transaction_id}`} class="inline-flex min-h-11 items-center font-semibold underline underline-offset-2">{day(fallback.date)}</a>
                  · {fallbackReasonText(fallback.reason)}
                  · {fallback.counted ? m.returns_fallback_counted() : m.returns_fallback_not_counted()}
                </li>
              {/each}
            </ul>
          </Panel>
        {/if}

        <Panel>
          <h3 class="text-sm font-semibold text-foreground">{m.returns_scope_title()}</h3>
          <p class="mt-1 text-sm text-muted">
            {result.account_scope === 'default_investment_accounts' ? m.returns_scope_default() : m.returns_scope_selected()}
          </p>
          <p class="mt-2 text-sm text-foreground">{result.resolved_account_ids.map(accountName).join(', ')}</p>
          <p class="mt-3 text-xs text-muted">{m.returns_read_only({ cutoff: instants.format(new Date(result.knowledge_cutoff)) })}</p>
        </Panel>
      </section>
    {/if}
  {/if}
</div>
