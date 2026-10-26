<script lang="ts">
  import { m } from '#lib/paraglide/messages.js';
  import { getLocale } from '#lib/paraglide/runtime.js';
  import type { InvestmentCorrectionDividendTerms } from '#lib/api/investments.ts';

  // Discloses an owner correction against an imported payment's source claim
  // (#395). It never blocks a correction; the imported source stays unchanged.
  let { differences }: { differences: InvestmentCorrectionDividendTerms['source_differences'] } = $props();

  const labels = {
    date: m.investments_dividend_source_field_date,
    cash_account: m.investments_dividend_source_field_cash_account,
    currency: m.investments_dividend_source_field_currency,
    amount: m.investments_dividend_source_field_amount,
    withholding: m.investments_dividend_source_field_withholding
  } as const;
  const fields = $derived(new Intl.ListFormat(getLocale(), { type: 'conjunction' })
    .format(differences.map((field) => labels[field]())));
</script>

{#if differences.length > 0}
  <p role="note" class="rounded-(--radius-panel) border border-warning/50 bg-warning/10 p-3 text-sm text-foreground">
    {m.investments_dividend_source_differs({ fields })}
  </p>
{/if}
