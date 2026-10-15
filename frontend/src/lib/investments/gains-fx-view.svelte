<script lang="ts">
  import type { components } from '#lib/api/schema.js';
  import type { GainsFXFigure, GainsFXComponent } from '#lib/api/investments.ts';
  import Panel from '#lib/components/panel.svelte';
  import { m } from '#lib/paraglide/messages.js';
  import { getLocale } from '#lib/paraglide/runtime.js';
  import { formatQuantity } from '#lib/money/format.ts';

  let { profile, figures, formatGain }: {
    profile: components['schemas']['GainsProfile'];
    figures: { label: string; figure: GainsFXFigure }[];
    formatGain: (value: string, scale: number, currency: number) => string;
  } = $props();
  const locale = $derived(getLocale());
  function role(part: GainsFXComponent): string {
    switch (part.role) {
      case 'basis': return m.investments_gains_col_disposed_basis();
      case 'proceeds': return m.investments_gains_col_proceeds();
      case 'market_value': return m.investments_gains_col_market_value();
      case 'income': return m.valuation_statement_income();
    }
  }
  function reason(part: GainsFXComponent): string {
    switch (part.unavailable_reason) {
      case 'unknown_acquisition_date': return m.valuation_fx_unknown_date();
      case 'unresolved_basis': return m.investments_basis_unknown();
      case 'unpriced': return m.investments_gains_no_price();
      case 'future_only': return m.valuation_price_future();
      case 'incompatible_quote_basis': return m.valuation_price_incompatible();
      case 'not_known_at_cutoff': return m.valuation_price_unknown();
      default: return m.valuation_fx_rate_gap({ date: part.requested_date, nearest: part.nearest_observation_date ?? m.valuation_fx_no_rate() });
    }
  }
</script>

<Panel>
  <div class="space-y-3" data-testid="gains-fx-results">
    <h3 class="text-sm font-semibold text-foreground">{m.valuation_fx_title()}</h3>
    <p class="break-words text-xs text-muted">{m.valuation_fx_profile({ method: profile.fx_method, date: profile.valuation_as_of, days: profile.fx_max_age_days ?? 7 })}</p>
    <dl class="divide-y divide-border text-sm">
      {#each figures as { label, figure }}
        <div class="grid gap-x-3 gap-y-1 py-2 sm:grid-cols-[minmax(0,1fr)_auto]" data-fx-label={label}>
          <dt class="min-w-0 text-muted">{label}</dt>
          <dd class="min-w-0 break-words font-mono text-foreground sm:text-right">
            {#if figure.amount}
              {formatGain(figure.amount.value, figure.amount.scale, profile.reporting_currency_id!)}
            {:else}
              <span class="font-sans text-muted">{m.valuation_fx_incomplete()}</span>
            {/if}
          </dd>
          {#if figure.components.length}
            <dd class="min-w-0 sm:col-span-2">
              <details class="text-xs text-muted">
                <summary class="cursor-pointer">{m.valuation_fx_evidence()}</summary>
                <ul class="mt-2 space-y-2">
                  {#each figure.components as part}
                    <li class="break-words">
                      <span>{role(part)}: {part.source ? formatGain(part.source.value, part.source.scale, part.commodity_id) : m.investments_basis_unknown()}</span>
                      {#if part.unavailable_reason}
                        <span class="block">{reason(part)}</span>
                      {:else if part.rate && part.base_quantity}
                        <span class="block">{m.valuation_fx_rate({ rate: formatQuantity(part.rate.value, part.rate.scale, locale), base: formatQuantity(part.base_quantity.value, part.base_quantity.scale, locale), observed: part.observation_date ?? '', requested: part.requested_date, id: part.observation_id ?? 0 })}</span>
                        {#if part.approximate}<span class="block">{m.investments_trade_price_approximate()}</span>{/if}
                      {:else}
                        <span class="block">{m.valuation_fx_noop()}</span>
                      {/if}
                    </li>
                  {/each}
                </ul>
              </details>
            </dd>
          {/if}
        </div>
      {/each}
    </dl>
  </div>
</Panel>
