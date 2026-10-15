<script lang="ts">
  import { createQuery } from '@tanstack/svelte-query';
  import Panel from '#lib/components/panel.svelte';
  import APIFormError from '#lib/components/api-form-error.svelte';
  import { investmentInstrumentsQueryOptions } from '#lib/api/investments.ts';
  import { parseNormalized, type ImportResolution, type ImportStagedRow } from '#lib/api/imports.ts';
  import type { CurrencyResponse } from '#lib/api/currencies.ts';
  import type { AccountResponse } from '#lib/api/accounts.ts';
  import CSVInvestmentRow from './csv-investment-row.svelte';
  import { m } from '#lib/paraglide/messages.js';
  let { rows, resolutions, accounts, currencies, onchange }: {
    rows: ImportStagedRow[]; resolutions: Map<number, ImportResolution>; accounts: AccountResponse[]; currencies: CurrencyResponse[];
    onchange: (rowID: number, patch: Partial<ImportResolution>) => void;
  } = $props();
  const query = createQuery(() => investmentInstrumentsQueryOptions());
</script>
<Panel>
  <h2 class="text-base font-semibold text-foreground">{m.import_investment_review_title()}</h2>
  <p class="mt-2 text-sm text-muted">{m.import_investment_review_help()}</p>
  {#if query.isPending}
    <p role="status" class="mt-3 text-sm text-muted">{m.import_investment_loading()}</p>
  {:else if query.isError}
    <APIFormError error={query.error} />
    <button type="button" onclick={() => query.refetch()} class="mt-3 min-h-11 rounded-(--radius-control) border border-border bg-control px-3 text-sm text-foreground">{m.install_gate_retry()}</button>
  {:else}
    {#if !query.data?.instruments.length}<p class="mt-3 text-sm text-warning">{m.import_investment_empty()}</p>{/if}
    <div class="mt-4 space-y-4">
      {#each rows as row (row.id)}
        {@const source = parseNormalized(row).investment}
        {#if source}
          <CSVInvestmentRow {row} {source} resolution={resolutions.get(row.id) ?? {}} {accounts} {currencies} instruments={query.data?.instruments ?? []} onchange={(patch) => onchange(row.id, patch)} />
        {/if}
      {/each}
    </div>
  {/if}
</Panel>
