<script lang="ts">
  import type { ImportResolution, ImportStagedRow, NormalizedRow } from '#lib/api/imports.ts';
  import type { AccountResponse } from '#lib/api/accounts.ts';
  import type { CurrencyResponse } from '#lib/api/currencies.ts';
  import type { InvestmentInstrumentResponse } from '#lib/api/investments.ts';
  import InvestmentIncomeFields from './investment-income-fields.svelte';
  let { row, source, resolution, accounts, currencies, instruments, onchange }: {
    row: ImportStagedRow; source: NonNullable<NormalizedRow['investment']>; resolution: ImportResolution;
    accounts: AccountResponse[]; currencies: CurrencyResponse[]; instruments: InvestmentInstrumentResponse[];
    onchange: (patch: Partial<ImportResolution>) => void;
  } = $props();
  const disabled = $derived(row.dedupe_status === 'duplicate' || row.dedupe_status === 'excluded' || !!resolution.exclude || row.commit_status === 'committed');
</script>
<InvestmentIncomeFields rowIndex={row.row_index} {source} {resolution} {accounts} {currencies} {instruments} {disabled} {onchange} />
