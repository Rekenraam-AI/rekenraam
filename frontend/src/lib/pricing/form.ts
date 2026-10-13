import { parseDecimalAmount } from '#lib/money/amount.ts';

// The stored pricing model is int64, distinct from the ledger's 38-digit
// quantities. Preserve every supported digit without going through Number.
export function parsePriceAmount(input: string) {
  const parsed = parseDecimalAmount(input, { maxScale: 18 });
  if (!parsed || BigInt(parsed.value) <= 0n || BigInt(parsed.value) > 9223372036854775807n) return null;
  return parsed;
}
