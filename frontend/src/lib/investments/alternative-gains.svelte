<script lang="ts">
  import PositionSideBadge from './position-side-badge.svelte';
  import Panel from '#lib/components/panel.svelte';
  import type { InvestmentGainsResponse } from '#lib/api/investments.ts';
  import { m } from '#lib/paraglide/messages.js';

  let { data, instrumentName, formatGain }: {
    data: InvestmentGainsResponse;
    instrumentName: (id: number) => string;
    formatGain: (value: string, scale: number, currencyID: number) => string;
  } = $props();
  const alternative = $derived(data.alternative);
  const personPooled = $derived(alternative?.profile.basis_method === 'person_pooled_average');
  const method = $derived(personPooled ? m.valuation_person_pool_method() : alternative?.profile.basis_method === 'fifo'
    ? m.transactions_investment_replace_method_fifo()
    : alternative?.profile.basis_method === 'lifo'
      ? m.transactions_investment_replace_method_lifo()
      : m.transactions_investment_replace_method_average());
</script>

{#if alternative}
  <h2 class="text-sm font-semibold text-foreground">{m.valuation_alternative_title({ method })}</h2>
  <p class="text-sm text-muted">{personPooled ? m.valuation_person_pool_policy({ date: alternative.profile.valuation_as_of }) : m.valuation_alternative_policy({ date: alternative.profile.valuation_as_of })}</p>
  {#if alternative.profile.jurisdiction_note}
    <p class="text-sm text-muted">{m.valuation_person_pool_context({ note: alternative.profile.jurisdiction_note })}</p>
  {/if}
  <Panel padding="none">
    <!-- svelte-ignore a11y_no_noninteractive_tabindex (keyboard users must be able to scroll the comparison) -->
    <div class="overflow-x-auto" role="region" aria-label={m.valuation_alternative_title({ method })} tabindex="0">
      <table class="min-w-full text-sm">
        <thead>
          <tr class="border-b border-border">
            <th class="px-4 py-3 text-left text-foreground">{m.investments_gains_col_gain()}</th>
            <th class="px-4 py-3 text-right text-foreground">{m.valuation_alternative_operational()}</th>
            <th class="px-4 py-3 text-right text-foreground">{method}</th>
          </tr>
        </thead>
        <tbody class="divide-y divide-border">
          {#each alternative.realized_totals as total (total.cost_commodity_id)}
            {@const operational = data.realized_totals.find((row) => row.cost_commodity_id === total.cost_commodity_id)}
            <tr>
              <th class="px-4 py-3 text-left font-medium text-foreground">{m.investments_gains_realized_title()}</th>
              <td class="px-4 py-3 text-right font-mono text-foreground">{operational?.total_gain_value != null && operational.total_gain_scale != null ? formatGain(operational.total_gain_value, operational.total_gain_scale, total.cost_commodity_id) : m.investments_basis_unknown()}</td>
              <td class="px-4 py-3 text-right font-mono text-foreground">{total.total_gain_value !== null && total.total_gain_scale !== null ? formatGain(total.total_gain_value, total.total_gain_scale, total.cost_commodity_id) : m.investments_basis_unknown()}</td>
            </tr>
          {/each}
          {#each alternative.unrealized as position (position.account_id + '_' + position.commodity_id + '_' + position.cost_commodity_id + '_' + position.position_side)}
            {@const operational = data.unrealized.find((row) => row.account_id === position.account_id && row.commodity_id === position.commodity_id && row.cost_commodity_id === position.cost_commodity_id && row.position_side === position.position_side)}
            <tr>
              <th class="px-4 py-3 text-left font-medium text-foreground">{instrumentName(position.commodity_id)}<PositionSideBadge side={position.position_side} /><span class="block text-xs text-muted">{m.valuation_runs_position({ instrument: position.commodity_id, account: position.account_id })}</span><span class="block text-xs text-muted">{m.investments_gains_unrealized_title()}</span></th>
              <td class="px-4 py-3 text-right font-mono text-foreground">{operational?.unrealized_gain_value != null && operational.unrealized_gain_scale != null ? formatGain(operational.unrealized_gain_value, operational.unrealized_gain_scale, position.cost_commodity_id) : m.valuation_alternative_unavailable()}</td>
              <td class="px-4 py-3 text-right font-mono text-foreground">{position.unrealized_gain_value != null && position.unrealized_gain_scale != null ? formatGain(position.unrealized_gain_value, position.unrealized_gain_scale, position.cost_commodity_id) : m.valuation_alternative_unavailable()}</td>
            </tr>
          {/each}
          {#if alternative.realized_totals.length === 0 && alternative.unrealized.length === 0}
            <tr><td colspan="3" class="px-4 py-3 text-muted">{m.valuation_alternative_empty()}</td></tr>
          {/if}
        </tbody>
      </table>
    </div>
  </Panel>
{/if}
