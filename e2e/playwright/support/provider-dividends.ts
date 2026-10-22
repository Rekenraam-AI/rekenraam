import process from 'node:process';

/** Register the spec's calendar date; the provider stub never chooses one. */
export async function setProviderDividendDate(symbol: string, exDate: string): Promise<void> {
  const stubURL = process.env.E2E_TRADING212_STUB_URL ?? `http://127.0.0.1:${process.env.TRADING212_STUB_PORT ?? '16890'}`;
  const response = await fetch(`${stubURL}/__control/dividends/${encodeURIComponent(symbol)}`, {
    method: 'PUT', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ ex_date: exDate })
  });
  if (response.status !== 204) throw new Error(`provider dividend stub control failed with ${response.status}: ${await response.text()}`);
}
