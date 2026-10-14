import { parseDecimalAmount } from '#lib/money/amount.ts';
import { formatQuantity } from '#lib/money/format.ts';
import { m } from '#lib/paraglide/messages.js';
export function cryptoCSVSummary(rawJSON: string, locale: string): string | null {
  try {
    const raw = JSON.parse(rawJSON) as Record<string, unknown>;
    if (raw.kind !== 'crypto_csv' || typeof raw.quantity !== 'string' || (raw.side !== 'BUY' && raw.side !== 'SELL')) return null;
    const quantity = parseDecimalAmount(raw.quantity, { locale: 'en', maxScale: 24 });
    if (!quantity) return null;
    const side = raw.side === 'BUY' ? m.investments_record_buy() : m.investments_record_sell();
    return m.quotes_csv_trade({ side, quantity: formatQuantity(quantity.value, quantity.scale, locale) });
  } catch { return null; }
}
