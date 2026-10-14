import type { components } from '#lib/api/schema.js';
import { APIClientError, apiClient, toAPIClientError, toNetworkError } from '#lib/api/client.ts';
export type InstrumentRequest = components['schemas']['InvestmentInstrumentRequest'];
export type Instrument = components['schemas']['InvestmentInstrumentResponse'];
export type InstrumentType = components['schemas']['InstrumentType'];
export async function saveInstrument(id: number | null, body: InstrumentRequest, csrfToken: string): Promise<Instrument> {
  try {
    const result = id === null
      ? await apiClient.POST('/api/v1/investments/instruments', { body, params: { header: { 'X-CSRF-Token': csrfToken } } })
      : await apiClient.PATCH('/api/v1/investments/instruments/{instrument_id}', { params: { path: { instrument_id: id }, header: { 'X-CSRF-Token': csrfToken } }, body });
    if (result.data) return result.data;
    throw toAPIClientError(result.response, result.error);
  } catch (error) { if (error instanceof APIClientError) throw error; throw toNetworkError(error); }
}
export async function createInstrumentHolding(body: components['schemas']['HoldingAccountRequest'], csrfToken: string) {
  try {
    const { data, error, response } = await apiClient.POST('/api/v1/investments/holding-accounts', { body, params: { header: { 'X-CSRF-Token': csrfToken } } });
    if (data) return data;
    throw toAPIClientError(response, error);
  } catch (error) { if (error instanceof APIClientError) throw error; throw toNetworkError(error); }
}
