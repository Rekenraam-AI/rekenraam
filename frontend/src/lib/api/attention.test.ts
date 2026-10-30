import { afterEach, expect, it, vi } from 'vitest';
import { apiClient, APIClientError } from './client.ts';
import { attentionQueryKey, attentionQueryOptions, getAttention } from './attention.ts';

afterEach(() => vi.restoreAllMocks());

it('uses one attention identity, a visible-page polling interval and focus refresh', () => {
  expect(attentionQueryOptions().queryKey).toBe(attentionQueryKey);
  expect(attentionQueryOptions()).toMatchObject({ refetchInterval: 30_000, refetchOnWindowFocus: true });
});

it('preserves partial read counts and sanitized API errors', async () => {
  const data = { checked_at: '2026-10-30T04:00:00Z', total_known: 3, complete: false, has_failures: true, sources: [] };
  vi.spyOn(apiClient, 'GET').mockResolvedValueOnce({ data, response: new Response() })
    .mockResolvedValueOnce({ error: { error: { code: 'INTERNAL_ERROR', message: 'Unavailable' } }, response: new Response(null, { status: 500 }) });
  expect(await getAttention()).toEqual(data);
  await expect(getAttention()).rejects.toMatchObject({ code: 'INTERNAL_ERROR', status: 500 });
});

it('reports a network failure instead of inventing an empty attention read', async () => {
  vi.spyOn(apiClient, 'GET').mockRejectedValueOnce(new Error('offline'));
  await expect(getAttention()).rejects.toBeInstanceOf(APIClientError);
});
