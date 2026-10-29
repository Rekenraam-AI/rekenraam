<script lang="ts">
  import { parseISO } from 'date-fns';
  import type { PriceObservation, PricingWorkspace } from '#lib/api/price-management.ts';
  import StatusBadge from '#lib/components/status-badge.svelte';
  import { formatQuantity } from '#lib/money/format.ts';
  import { m } from '#lib/paraglide/messages.js';
  import { getLocale } from '#lib/paraglide/runtime.js';
  let { price, workspace, detailed = false }: { price: PriceObservation; workspace: PricingWorkspace; detailed?: boolean } = $props();
  const locale = $derived(getLocale());
  const sources = $derived(new Map(workspace.sources.map(source => [source.id, source])));
  const codes = $derived(new Map(workspace.commodities.map(commodity => [commodity.id, commodity.code])));
  const source = $derived(price.is_manual ? m.pricing_manual() : price.quote_type === 'trade_implied' ? m.pricing_trade() : (sources.get(price.source_id ?? 0)?.name ?? m.pricing_unknown_source()));
  const quoteLabels = $derived<Record<string, string>>({ manual: m.pricing_manual(), trade_implied: m.pricing_trade(), close: m.pricing_quote_close(), adjusted_close: m.pricing_quote_adjusted_close(), nav: m.pricing_quote_nav(), official_fixing: m.pricing_quote_fixing(), spot_mid: m.pricing_quote_mid(), bid: m.pricing_quote_bid(), ask: m.pricing_quote_ask(), valuation_override: m.pricing_quote_override() });
  const adjustmentLabels = $derived<Record<string, string>>({ raw: m.pricing_basis_raw(), split_adjusted: m.pricing_basis_split(), dividend_adjusted: m.pricing_basis_dividend(), total_return: m.pricing_basis_total(), not_applicable: m.pricing_basis_na() });
  function date(value: string) { return new Intl.DateTimeFormat(locale, { dateStyle: 'medium' }).format(parseISO(value)); }
  function timestamp(value?: string) { return value ? new Intl.DateTimeFormat(locale, { dateStyle: 'medium', timeStyle: 'long', timeZone: 'UTC' }).format(parseISO(value)) : m.pricing_not_recorded(); }
</script>

<div class="min-w-0 space-y-2 wrap-anywhere">
  <p class="font-semibold">{m.pricing_amount_pair({ price: formatQuantity(price.price_value, price.price_scale, locale), quote: codes.get(price.quote_commodity_id) ?? '', base: formatQuantity(price.base_quantity_value, price.base_quantity_scale, locale), instrument: codes.get(price.base_commodity_id) ?? '' })}</p>
  <p class="text-sm text-muted">{m.pricing_observation_identity({ id: new Intl.NumberFormat(locale).format(price.id), date: date(price.valuation_date), source })}</p>
  <div class="flex flex-wrap gap-2">
    {#if price.voided_at}<StatusBadge tone="warning">{m.pricing_retired()}</StatusBadge>{/if}
    {#if price.is_manual}<StatusBadge>{m.pricing_owner_assertion()}</StatusBadge>{/if}
    {#if price.is_derived}<StatusBadge tone="warning">{m.pricing_derived_warning()}</StatusBadge>{/if}
    {#if price.is_approximate}<StatusBadge tone="warning">{m.pricing_approximate()}</StatusBadge>{/if}
    {#if price.adjustment_basis === 'total_return'}<StatusBadge>{m.pricing_basis_total()} · {m.pricing_benchmark_only()}</StatusBadge>{/if}
  </div>
  {#if detailed}
    <dl class="grid gap-2 text-sm sm:grid-cols-2 [&_dt]:text-muted">
      <div><dt>{m.pricing_quote_type()}</dt><dd>{quoteLabels[price.quote_type] ?? price.quote_type}</dd></div>
      <div><dt>{m.pricing_adjustment_basis()}</dt><dd>{adjustmentLabels[price.adjustment_basis] ?? price.adjustment_basis}</dd></div>
      <div><dt>{m.pricing_observed_at()}</dt><dd>{timestamp(price.observed_at)}</dd></div>
      <div><dt>{m.pricing_published_at()}</dt><dd>{timestamp(price.source_published_at)}</dd></div>
      <div><dt>{m.pricing_recorded_at()}</dt><dd>{timestamp(price.recorded_at)}</dd></div>
      <div><dt>{m.pricing_provider_id()}</dt><dd>{price.provider_observation_id || m.pricing_not_recorded()}</dd></div>
      {#if price.source_transaction_version_id}<div><dt>{m.pricing_trade_version()}</dt><dd>{new Intl.NumberFormat(locale).format(price.source_transaction_version_id)}</dd></div>{/if}
      {#if price.supersedes_observation_id}<div><dt>{m.pricing_replaces()}</dt><dd>{new Intl.NumberFormat(locale).format(price.supersedes_observation_id)}</dd></div>{/if}
      {#if price.voided_at}<div><dt>{m.pricing_retired()}</dt><dd>{timestamp(price.voided_at)} · {price.void_reason}</dd></div>{/if}
    </dl>
    <details><summary class="cursor-pointer text-sm font-semibold">{m.pricing_source_evidence()}</summary>
      <p class="mt-2 text-sm text-muted">{m.pricing_metadata()}</p><pre class="whitespace-pre-wrap break-all text-xs">{JSON.stringify(price.metadata, null, 2)}</pre>
      {#if price.is_derived}<p class="mt-2 text-sm text-muted">{m.pricing_derivation()}</p><pre class="whitespace-pre-wrap break-all text-xs">{price.derivation_text}</pre>{/if}
    </details>
  {/if}
</div>
