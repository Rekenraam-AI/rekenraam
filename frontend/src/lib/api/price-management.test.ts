import { createRequire } from 'node:module';
import type { QueryClient as SvelteQueryClient } from '@tanstack/svelte-query';
import { describe, expect, it } from 'vitest';
import { investmentReturnsQueryKey, investmentReturnsQueryOptions } from '#lib/api/investments.ts';
import { invalidatePriceReads } from '#lib/api/price-management.ts';

// The Svelte adapter's entry imports .svelte files vitest cannot load here;
// its QueryClient is query-core's.
const localRequire = createRequire(import.meta.url);
const { QueryClient } = createRequire(localRequire.resolve('@tanstack/svelte-query/package.json'))('@tanstack/query-core') as { QueryClient: typeof SvelteQueryClient };

// Returns value every holding at its price, so a quote correction restates them.
describe('price corrections and cached returns', () => {
  const input = { from: '2026-01-01', to: '2026-10-24', reportingCurrencyID: 1, accountIDs: [], benchmarkCommodityID: null, benchmarkBasis: 'price_return' as const };

  it('invalidates a cached return', async () => {
    const client = new QueryClient();
    const key = [...investmentReturnsQueryKey, input];
    client.setQueryData(key, { time_weighted: { value: '0.100000', reason: null, date: null } });
    await invalidatePriceReads(client);
    expect(client.getQueryState(key)?.isInvalidated).toBe(true);
    client.clear();
  });

  it('refetches on every visit, since ledger writers have no shared invalidation hook', () => {
    expect(investmentReturnsQueryOptions(input).staleTime).toBe(0);
  });
});
