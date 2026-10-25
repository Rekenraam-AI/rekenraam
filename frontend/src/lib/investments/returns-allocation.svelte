<script lang="ts">
  import type { ReturnsAllocation, ReturnsAllocationRow } from '#lib/api/investments.ts';
  import type { BalanceQuantity } from '#lib/api/ledger.ts';
  import { formatQuantity } from '#lib/money/format.ts';
  import { m } from '#lib/paraglide/messages.js';
  import { allocationBarPercent, assetTypeText, formatWeight, returnReasonText, type AllocationDimension } from './returns-model';

  // R13 slice 4: the closing value split by account, asset type or currency.
  // Long and short legs of one group are separate rows; a short row says so
  // in words as well as in its signed weight.
  let {
    allocation,
    locale,
    formatDate,
    formatAmount,
    accountName,
    currencyCode,
    commodityLabel
  }: {
    allocation: ReturnsAllocation;
    locale: string;
    formatDate: (date: string) => string;
    formatAmount: (amount: BalanceQuantity) => string;
    accountName: (id: number) => string;
    currencyCode: (id: number) => string;
    commodityLabel: (id: number) => string;
  } = $props();
  // Unique per instance: a saved run can sit beside the live result.
  const titleID = $props.id();

  let dimension = $state<AllocationDimension>('account');
  const dimensions: { key: AllocationDimension; label: () => string }[] = [
    { key: 'account', label: m.returns_allocation_by_account },
    { key: 'asset_type', label: m.returns_allocation_by_asset_type },
    { key: 'currency', label: m.returns_allocation_by_currency }
  ];
  const rows = $derived(
    dimension === 'account' ? allocation.by_account
      : dimension === 'asset_type' ? allocation.by_asset_type
        : allocation.by_currency
  );

  function label(row: ReturnsAllocationRow): string {
    if (dimension === 'account') return row.account_id === null ? '—' : accountName(row.account_id);
    if (dimension === 'asset_type') return assetTypeText(row.asset_type);
    return row.currency_id === null ? m.returns_allocation_no_currency() : currencyCode(row.currency_id);
  }
</script>

<section aria-labelledby={titleID} class="space-y-3" data-testid="returns-allocation">
  <div>
    <h3 id={titleID} class="text-sm font-semibold text-foreground">{m.returns_allocation_title({ date: formatDate(allocation.date) })}</h3>
    <p class="mt-1 text-xs text-muted">{m.returns_allocation_help()}</p>
  </div>

  {#if allocation.reason}
    <p role="status" class="text-sm text-warning">{returnReasonText(allocation.reason, allocation.date, formatDate)}</p>
  {:else if allocation.by_account.length === 0 && allocation.unknown.length === 0}
    <p class="text-sm text-muted" data-testid="returns-allocation-empty">{m.returns_allocation_empty()}</p>
  {:else}
    <div role="group" aria-label={m.returns_allocation_group_by()} class="flex flex-wrap gap-2">
      {#each dimensions as option (option.key)}
        <button type="button" aria-pressed={dimension === option.key} onclick={() => { dimension = option.key; }}
          class="min-h-11 rounded-(--radius-control) border border-border px-3 text-sm text-foreground hover:bg-control-hover {dimension === option.key ? 'bg-surface-strong font-semibold' : 'bg-control'}">
          {option.label()}
        </button>
      {/each}
    </div>

    {#if allocation.basis === 'known_only'}
      <p role="status" class="text-sm text-warning">{m.returns_allocation_known_only()}</p>
    {/if}
    {#if allocation.weights_reason}
      <!-- On a known-only basis only the priced part is non-positive; the whole is unknown. -->
      <p role="status" class="text-sm text-warning" data-testid="returns-allocation-non-positive">
        {allocation.basis === 'known_only' ? m.returns_allocation_non_positive_known_only() : m.returns_allocation_non_positive()}
      </p>
    {/if}

    <ul class="space-y-3" data-testid="returns-allocation-rows">
      {#each rows as row (`${row.account_id}-${row.asset_type}-${row.currency_id}-${row.side}`)}
        <li class="space-y-1">
          <div class="flex flex-wrap items-baseline justify-between gap-x-3 text-sm">
            <span class="font-medium text-foreground">
              {label(row)}
              {#if row.side === 'short'}<span class="ml-1 text-xs font-semibold text-warning">{m.returns_allocation_short()}</span>{/if}
            </span>
            <span class="flex gap-3 tabular-nums text-foreground">
              <span>{formatAmount(row.value)}</span>
              {#if row.weight !== null}<span class="font-semibold">{formatWeight(row.weight, locale)}</span>{/if}
            </span>
          </div>
          {#if row.weight !== null}
            <div class="h-2 overflow-hidden rounded-full bg-surface-strong" aria-hidden="true">
              <div class="h-full rounded-full {row.side === 'short' ? 'bg-warning' : 'bg-accent'}" style:width={`${allocationBarPercent(row.weight)}%`}></div>
            </div>
          {/if}
        </li>
      {/each}
    </ul>

    {#if allocation.unknown.length > 0}
      <div class="space-y-1" data-testid="returns-allocation-unknown">
        <h4 class="text-xs font-semibold text-foreground">{m.returns_allocation_unknown_title()}</h4>
        <ul class="text-sm text-muted">
          {#each allocation.unknown as item (`${item.account_id}-${item.quantity.commodity_id}`)}
            <li>{accountName(item.account_id)} · {formatQuantity(item.quantity.quantity_value, item.quantity.quantity_scale, locale)} {commodityLabel(item.quantity.commodity_id)}</li>
          {/each}
        </ul>
      </div>
    {/if}
  {/if}
</section>
