import type { components } from '#lib/api/schema.js';
import { APIClientError, apiClient, toAPIClientError, toNetworkError } from './client.ts';
export type SearchCatalogItem = components['schemas']['SearchCatalogItem'];
export const searchCatalogQueryKey = ['api', 'search-catalog'] as const;
export async function getSearchCatalog(cursor?: string, signal?: AbortSignal) {
  try {
    const { data, error, response } = await apiClient.GET('/api/v1/search/catalog', {
      params: { query: { cursor } }, signal
    });
    if (data !== undefined) return data;
    throw toAPIClientError(response, error);
  } catch (error) {
    if (error instanceof APIClientError) throw error;
    throw toNetworkError(error);
  }
}
