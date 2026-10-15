import { parseDecimalAmount, formatEditableAmount, formatLedgerAmount } from '#lib/money/amount.ts';

// Source decimals are canonical; working copies use the active locale.
export function editableCSVInvestmentAmount(value: string, locale?: Parameters<typeof formatEditableAmount>[2]): string {
  const parsed = parseDecimalAmount(value, { locale: 'en' });
  return parsed ? formatEditableAmount(parsed.value, parsed.scale, locale) : value;
}
export function canonicalCSVInvestmentAmount(value: string, locale?: Parameters<typeof formatEditableAmount>[2]): string | null {
  const parsed = parseDecimalAmount(value, { locale });
  return parsed ? formatLedgerAmount(parsed.value, parsed.scale) : null;
}
