import type { components } from '#lib/api/schema.js';
import { APIClientError, apiClient, toAPIClientError, toNetworkError } from '#lib/api/client.ts';

export type Attention = components['schemas']['AttentionResponse'];
export type AttentionSource = components['schemas']['AttentionSource'];
export type AttentionSourceID = AttentionSource['id'];
export const attentionQueryKey = ['api', 'attention'] as const;

/** One shell observer; Home and Review consume its context, without extra polls. */
export function attentionQueryOptions() {
  return {
    queryKey: attentionQueryKey,
    queryFn: getAttention,
    staleTime: 10_000,
    refetchInterval: 30_000,
    refetchOnWindowFocus: true
  };
}

export async function getAttention(): Promise<Attention> {
  try {
    const { data, error, response } = await apiClient.GET('/api/v1/attention');
    if (data !== undefined) return data;
    throw toAPIClientError(response, error);
  } catch (error) {
    if (error instanceof APIClientError) throw error;
    throw toNetworkError(error);
  }
}
