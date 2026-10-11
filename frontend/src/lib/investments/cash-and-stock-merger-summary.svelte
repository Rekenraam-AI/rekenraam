<script lang="ts">
  import { parseISO } from 'date-fns';
  import type { CurrencyResponse } from '#lib/api/currencies.ts';
  import type { CashStockMergerPlan } from '#lib/api/investments.ts';
  import { formatExactMoney, formatQuantity } from '#lib/money/format.ts';
  import { m } from '#lib/paraglide/messages.js';
  import { getLocale } from '#lib/paraglide/runtime.js';
  import { basisFractionPercent } from './split-ratio';

  // One cash-and-stock merger's lots and per-currency totals (#184), shared by
  // the entry preview and the transaction detail: the units, each lot's
  // realized and carried basis and cash share, and the cash result. Every
  // amount is the exact coefficient the server returned; unknown basis is a
  // word, never a zero, and an unknown result is unresolved.
  let { plan, oldName, newName, currenciesByID }: {
    plan: CashStockMergerPlan;
    oldName: string;
    newName: string;
    currenciesByID: Map<number, CurrencyResponse>;
  } = $props();

  const locale = $derived(getLocale());
  const dateFormatter = $derived(new Intl.DateTimeFormat(locale, { year: 'numeric', month: 'short', day: 'numeric' }));
  const percent = $derived.by(() => {
    const exact = basisFractionPercent(plan.cash_basis_fraction_value, plan.cash_basis_fraction_scale);
    return formatQuantity(exact.value, exact.scale, locale);
  });

  function moneyText(value: string | null, scale: number | null, currencyID: number): string {
    if (value === null || scale === null) return m.investments_basis_unknown();
    const currency = currenciesByID.get(currencyID);
    return `${formatExactMoney(value, scale, currency?.standard_scale ?? 2, locale)} ${currency?.code ?? ''}`.trim();
  }
</script>

<div class="space-y-3">
  <p class="text-sm font-medium text-foreground">{m.investments_merger_total({
    old: formatQuantity(plan.source_quantity_value, plan.source_quantity_scale, locale), oldName,
    new: formatQuantity(plan.destination_quantity_value, plan.destination_quantity_scale, locale), newName,
    ratioNew: String(plan.ratio_numerator), ratioOld: String(plan.ratio_denominator), percent
  })}</p>
  <ul class="space-y-2 text-sm text-foreground" aria-label={m.investments_exchange_lots_label()}>
    {#each plan.links as link (link.source_lot_id)}
      <li class="rounded-(--radius-control) border border-border px-3 py-2">
        <span class="block break-words font-mono">{m.investments_exchange_lot_units({
          old: formatQuantity(link.source_quantity_value, link.source_quantity_scale, locale), oldName,
          new: formatQuantity(link.destination_quantity_value, link.destination_quantity_scale, locale), newName
        })}</span>
        <span class="block text-xs text-muted">
          {link.original_acquired_on
            ? m.investments_exchange_lot_acquired({ date: dateFormatter.format(parseISO(link.original_acquired_on)) })
            : m.investments_exchange_lot_acquired_unknown()}
          · {m.investments_merger_lot_cash({ cash: moneyText(link.cash_value, link.cash_scale, link.cost_commodity_id) })}
          · {m.investments_merger_lot_realized({ basis: moneyText(link.realized_basis_value, link.realized_basis_scale, link.cost_commodity_id) })}
          · {m.investments_exchange_lot_basis({ basis: moneyText(link.carried_basis_value, link.carried_basis_scale, link.cost_commodity_id) })}
        </span>
      </li>
    {/each}
  </ul>
  <dl class="space-y-1 text-sm">
    {#each plan.totals as total (total.cost_commodity_id)}
      {@const code = currenciesByID.get(total.cost_commodity_id)?.code ?? ''}
      <div class="flex flex-wrap justify-between gap-x-3">
        <dt class="text-muted">{m.investments_merger_total_cash({ currency: code })}</dt>
        <dd class="font-mono text-foreground">{moneyText(total.cash_value, total.cash_scale, total.cost_commodity_id)}</dd>
      </div>
      <div class="flex flex-wrap justify-between gap-x-3">
        <dt class="text-muted">{m.investments_merger_total_realized({ currency: code })}</dt>
        <dd class="font-mono text-foreground">{moneyText(total.realized_basis_value, total.realized_basis_scale, total.cost_commodity_id)}</dd>
      </div>
      <div class="flex flex-wrap justify-between gap-x-3">
        <dt class="text-muted">{m.investments_merger_total_result({ currency: code })}</dt>
        <dd class="font-mono font-semibold text-foreground">
          {total.result_value === null || total.result_scale === null
            ? m.investments_merger_result_unresolved()
            : moneyText(total.result_value, total.result_scale, total.cost_commodity_id)}
        </dd>
      </div>
      <div class="flex flex-wrap justify-between gap-x-3">
        <dt class="text-muted">{m.investments_exchange_total_basis({ currency: code })}</dt>
        <dd class="font-mono text-foreground">
          {moneyText(total.carried_basis_value, total.carried_basis_scale, total.cost_commodity_id)}
          {#if total.unknown_lots > 0}
            <span class="block text-xs font-sans text-muted">{m.investments_exchange_unknown_lots({ count: String(total.unknown_lots) })}</span>
          {/if}
        </dd>
      </div>
    {/each}
  </dl>
  <p class="text-xs text-muted">{m.investments_merger_realized_note()}</p>
</div>
