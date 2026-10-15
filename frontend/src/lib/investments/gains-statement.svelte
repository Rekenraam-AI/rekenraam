<script lang="ts">
  import GainsFXView from './gains-fx-view.svelte';
  import type { GainsFXInput } from '#lib/api/investments.ts';
  let { fx }: { fx?: GainsFXInput } = $props();
  import { createQuery } from '@tanstack/svelte-query';
  import Panel from '#lib/components/panel.svelte';
  import StatePanel from '#lib/components/state-panel.svelte';
  import { useOwnerToday } from '#lib/dates/owner-date-context.ts';
  import {
    investmentGainsStatementQueryOptions,
    type GainsStatementRow,
    type GainsStatementValuation
  } from '#lib/api/investments.ts';
  import { formatMoney, joinCommodityAmount } from '#lib/money/format.ts';
  import { coefficientSign } from '#lib/money/amount.ts';
  import { formatScaledValue } from './investment-labels';
  import { m } from '#lib/paraglide/messages.js';
  import { getLocale } from '#lib/paraglide/runtime.js';

  // R18 period statement (#231): per currency, opening unrealized at the day
  // before the period, realized and income in it, closing unrealized at its
  // end. An absent figure is shown with its reason, never as zero.
  const ownerToday = useOwnerToday();
  const today = ownerToday();
  const locale = $derived(getLocale());
  const dates = $derived(new Intl.DateTimeFormat(locale, { dateStyle: 'medium', timeZone: 'UTC' }));

  let fromDate = $state(`${today.slice(0, 4)}-01-01`);
  let toDate = $state(today);
  let applied = $state({ from: `${today.slice(0, 4)}-01-01`, to: today });
  const rangeInvalid = $derived(fromDate === '' || toDate === '' || fromDate > toDate || toDate > today);

  function apply() {
    if (rangeInvalid) return;
    applied = { from: fromDate, to: toDate };
  }

  const statementQuery = createQuery(() => investmentGainsStatementQueryOptions(applied.from, applied.to, fx));
  const statement = $derived(statementQuery.data);

  function day(date: string): string {
    return dates.format(new Date(`${date}T00:00:00Z`));
  }

  function amount(value: string, scale: number, currencyID: number): string {
    const currency = statement?.currencies.find((c) => c.id === currencyID);
    const formatted = currency
      ? formatMoney(value, scale, currency.standard_scale, locale)
      : formatScaledValue(value, scale, locale);
    return joinCommodityAmount(currency?.code ?? `#${currencyID}`, formatted);
  }

  function currencyCode(currencyID: number): string {
    return statement?.currencies.find((c) => c.id === currencyID)?.code ?? `#${currencyID}`;
  }

  function gainClass(value: string): string {
    const sign = coefficientSign(value);
    if (sign > 0) return 'text-green-700 dark:text-green-400';
    if (sign < 0) return 'text-destructive';
    return 'text-foreground';
  }

  function valuationReason(valuation: GainsStatementValuation): string {
    switch (valuation.unavailable_reason) {
      case 'unpriced':
        return m.valuation_statement_reason_unpriced({ count: valuation.unpriced_count });
      case 'unresolved_basis':
        return m.valuation_statement_reason_unresolved({ count: valuation.unresolved_count });
      default:
        return m.valuation_statement_reason_both({
          unpriced: valuation.unpriced_count,
          unresolved: valuation.unresolved_count
        });
    }
  }

  function priceDates(valuation: GainsStatementValuation): string | null {
    if (valuation.earliest_price_date === null || valuation.latest_price_date === null) return null;
    if (valuation.earliest_price_date === valuation.latest_price_date) {
      return m.valuation_gains_price_date({ date: day(valuation.latest_price_date) });
    }
    return m.valuation_statement_price_dates({
      earliest: day(valuation.earliest_price_date),
      latest: day(valuation.latest_price_date)
    });
  }

  type Line = {
    key: string;
    label: string;
    value: string | null;
    scale: number | null;
    absent: string;
    notes: string[];
    emphasis?: boolean;
  };

  function valuationLine(key: string, label: string, valuation: GainsStatementValuation): Line {
    const notes: string[] = [m.valuation_statement_positions({ count: valuation.position_count })];
    const dated = priceDates(valuation);
    if (dated) notes.push(dated);
    if (valuation.stale_count > 0) notes.push(m.valuation_statement_stale({ count: valuation.stale_count }));
    return {
      key,
      label,
      value: valuation.unrealized_gain_value,
      scale: valuation.unrealized_gain_scale,
      absent: valuation.unavailable_reason ? valuationReason(valuation) : '',
      notes
    };
  }

  function lines(row: GainsStatementRow): Line[] {
    return [
      valuationLine('opening', m.valuation_statement_opening({ date: day(row.opening.valuation_as_of) }), row.opening),
      {
        key: 'realized',
        label: m.valuation_statement_realized(),
        value: row.realized_gain_value,
        scale: row.realized_gain_scale,
        absent: m.valuation_statement_reason_realized({ count: row.realized_unresolved_count }),
        notes: [m.valuation_statement_disposals({ count: row.disposal_count })]
      },
      {
        key: 'income',
        label: m.valuation_statement_income(),
        value: row.income_value,
        scale: row.income_scale,
        absent: '',
        notes: [m.valuation_statement_dividends({ count: row.income_count })]
      },
      valuationLine('closing', m.valuation_statement_closing({ date: day(row.closing.valuation_as_of) }), row.closing),
      {
        key: 'change',
        label: m.valuation_statement_change(),
        value: row.unrealized_change_value,
        scale: row.unrealized_change_scale,
        absent: m.valuation_statement_needs_both(),
        notes: []
      },
      {
        key: 'total',
        label: m.valuation_statement_total(),
        value: row.total_value,
        scale: row.total_scale,
        absent: m.valuation_statement_needs_all(),
        notes: [],
        emphasis: true
      }
    ];
  }
</script>

<section class="space-y-3" aria-labelledby="gains-statement-title" data-testid="gains-statement">
  <div class="flex flex-wrap items-center gap-3">
    <h2 id="gains-statement-title" class="text-sm font-semibold text-foreground">{m.valuation_statement_title()}</h2>
    <form
      class="flex flex-wrap items-center gap-2 text-sm"
      onsubmit={(event) => {
        event.preventDefault();
        apply();
      }}
    >
      <label class="flex items-center gap-1 text-muted">
        {m.valuation_statement_from()}
        <input
          type="date"
          bind:value={fromDate}
          max={today}
          aria-invalid={rangeInvalid}
          aria-describedby={rangeInvalid ? 'gains-statement-range-error' : undefined}
          class="rounded border border-border bg-control px-2 py-1 text-xs text-foreground focus:outline-none focus:ring-1 focus:ring-foreground/30"
        />
      </label>
      <label class="flex items-center gap-1 text-muted">
        {m.valuation_statement_to()}
        <input
          type="date"
          bind:value={toDate}
          max={today}
          aria-invalid={rangeInvalid}
          aria-describedby={rangeInvalid ? 'gains-statement-range-error' : undefined}
          class="rounded border border-border bg-control px-2 py-1 text-xs text-foreground focus:outline-none focus:ring-1 focus:ring-foreground/30"
        />
      </label>
      <button
        type="submit"
        disabled={rangeInvalid}
        class="rounded-(--radius-control) border border-border bg-control px-3 py-1 text-xs font-medium text-foreground hover:bg-control-hover disabled:opacity-50"
      >
        {m.valuation_statement_apply()}
      </button>
    </form>
  </div>
  {#if rangeInvalid}
    <p id="gains-statement-range-error" class="text-sm text-destructive">{m.valuation_statement_range_invalid()}</p>
  {/if}
  <p class="text-xs text-muted">
    {m.valuation_statement_note({ from: day(applied.from), to: day(applied.to) })}
  </p>

  {#if statementQuery.isPending}
    <Panel><p class="text-sm text-muted" role="status">{m.valuation_statement_loading()}</p></Panel>
  {:else if statementQuery.isError}
    <Panel><p class="text-sm text-destructive" role="alert">{m.valuation_statement_error()}</p></Panel>
  {:else if !statement || statement.rows.length === 0}
    <StatePanel title={m.valuation_statement_empty()} copy="" />
  {:else}
    {#if statement.reporting}
      {@const reporting = statement.reporting}
      <GainsFXView profile={reporting.profile} formatGain={amount} figures={[
        { label: m.valuation_statement_opening({ date: day(statement.rows[0].opening.valuation_as_of) }), figure: reporting.opening },
        { label: m.valuation_statement_realized(), figure: reporting.realized },
        { label: m.valuation_statement_income(), figure: reporting.income },
        { label: m.valuation_statement_closing({ date: day(statement.to) }), figure: reporting.closing },
        { label: m.valuation_statement_change(), figure: reporting.unrealized_change },
        { label: m.valuation_statement_total(), figure: reporting.total }
      ]} />
    {/if}
    <div class="grid gap-3 md:grid-cols-2">
      {#each statement.rows as row (row.commodity_id)}
        <Panel>
          <article aria-label={m.valuation_statement_currency({ currency: currencyCode(row.commodity_id) })} data-testid="gains-statement-row">
            <h3 class="mb-2 text-sm font-semibold text-foreground">{currencyCode(row.commodity_id)}</h3>
            <dl class="divide-y divide-border text-sm">
              {#each lines(row) as line (line.key)}
                <div class="flex flex-wrap items-baseline justify-between gap-x-3 gap-y-1 py-2" data-line={line.key}>
                  <dt class={line.emphasis ? 'font-semibold text-foreground' : 'text-muted'}>
                    {line.label}
                    {#each line.notes as note}
                      <span class="block text-xs text-muted">{note}</span>
                    {/each}
                  </dt>
                  <dd class="text-right font-mono {line.emphasis ? 'font-semibold' : ''}">
                    {#if line.value !== null && line.scale !== null}
                      <span class={gainClass(line.value)}>{amount(line.value, line.scale, row.commodity_id)}</span>
                    {:else}
                      <span class="font-sans text-muted">{m.valuation_statement_absent()}</span>
                      <span class="block font-sans text-xs text-muted">{line.absent}</span>
                    {/if}
                  </dd>
                </div>
              {/each}
            </dl>
          </article>
        </Panel>
      {/each}
    </div>
  {/if}
</section>
