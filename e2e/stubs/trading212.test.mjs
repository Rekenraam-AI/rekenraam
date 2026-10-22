import assert from 'node:assert/strict';
import { spawn } from 'node:child_process';
import { once } from 'node:events';
import { createInterface } from 'node:readline';
import test from 'node:test';
import { fileURLToPath } from 'node:url';

async function startStub(t, control) {
  const stubPath = fileURLToPath(new URL('./trading212.mjs', import.meta.url));
  const args = control ? ['--input-type=module', '--eval', `
    import { mock } from 'node:test';
    import { daysFromTodayISO, todayISO } from ${JSON.stringify(new URL('../playwright/support/dates.ts', import.meta.url).href)};
    mock.timers.enable({ apis: ['Date'], now: new Date(${JSON.stringify(control.instant)}) });
    const reference = new Date();
    console.log(JSON.stringify({ today: todayISO(reference), exDate: daysFromTodayISO(-10, reference), paymentDate: daysFromTodayISO(-3, reference) }));
    await import(${JSON.stringify(new URL('./trading212.mjs', import.meta.url).href)});
  `] : [stubPath];
  const child = spawn(process.execPath, args, {
    env: { ...process.env, ...(control ? { TZ: control.zone } : {}), TRADING212_STUB_PORT: '0' },
    stdio: ['ignore', 'pipe', 'inherit']
  });
  const lines = createInterface({ input: child.stdout, signal: AbortSignal.timeout(10_000) });
  t.after(async () => {
    lines.close();
    if (child.exitCode === null && child.signalCode === null) {
      const closed = once(child, 'close');
      child.kill();
      await closed;
    }
  });
  let dates;
  for await (const line of lines) {
    if (line.startsWith('{')) dates = JSON.parse(line);
    const address = line.match(/http:\/\/\S+/);
    if (address) return { baseURL: new URL(address[0]).origin, dates };
  }
  throw new Error('provider stub exited before readiness');
}

test('provider stub keeps exception diagnostics out of HTTP responses', { timeout: 20_000 }, async (t) => {
  const { baseURL } = await startStub(t);
  const controlURL = `${baseURL}/__control/accounts/synthetic-key`;

  async function expectGenericFailure(response) {
    assert.equal(response.status, 500);
    assert.equal(response.headers.get('content-type'), 'application/json');
    assert.deepEqual(await response.json(), { error: 'internal server error' });
  }

  await t.test('malformed JSON does not echo parser diagnostics or input', async () => {
    await expectGenericFailure(await fetch(controlURL, {
      method: 'PUT', body: 'synthetic-private-marker-not-json'
    }));
  });

  await t.test('malformed account encoding does not expose URI errors', async () => {
    await expectGenericFailure(await fetch(`${baseURL}/__control/accounts/%FF`, {
      method: 'PUT', body: '{}'
    }));
  });

  await t.test('provider transformation failures do not expose type errors', async () => {
    const seed = await fetch(controlURL, {
      method: 'PUT', body: JSON.stringify({ orders: [{ filledAt: null }, { filledAt: null }] })
    });
    assert.equal(seed.status, 204);
    await expectGenericFailure(await fetch(`${baseURL}/api/v0/equity/history/orders`, {
      headers: { Authorization: 'synthetic-key' }
    }));
  });

  await t.test('health, authorization and valid provider requests survive failures', async () => {
    assert.deepEqual(await (await fetch(`${baseURL}/healthz`)).json(), { ok: true });
    const unauthorized = await fetch(`${baseURL}/api/v0/equity/account/summary`);
    assert.equal(unauthorized.status, 401);
    assert.deepEqual(await unauthorized.json(), { error: 'unauthorized' });
    const seed = await fetch(controlURL, {
      method: 'PUT', body: JSON.stringify({ dividends: [{ paidOn: '2026-01-02' }, { paidOn: '2026-01-03' }] })
    });
    assert.equal(seed.status, 204);
    assert.equal(await seed.text(), '');
    const history = await fetch(`${baseURL}/api/v0/equity/history/dividends`, {
      headers: { Authorization: 'synthetic-key' }
    });
    assert.equal(history.status, 200);
    assert.deepEqual(await history.json(), {
      items: [{ paidOn: '2026-01-03' }, { paidOn: '2026-01-02' }], nextPagePath: null
    });
  });
});

// Independent calendar oracles at the same instant on either side of UTC,
// then across both DST transitions. The real child stub has a frozen clock;
// fixtures use calendar offsets, never elapsed 24-hour periods (#386).
for (const control of [
  { zone: 'UTC', instant: '2026-10-21T22:30:00Z', today: '2026-10-21', exDate: '2026-10-11', paymentDate: '2026-10-18' },
  { zone: 'Europe/Amsterdam', instant: '2026-10-21T22:30:00Z', today: '2026-10-22', exDate: '2026-10-12', paymentDate: '2026-10-19' },
  { zone: 'America/Los_Angeles', instant: '2026-10-21T22:30:00Z', today: '2026-10-21', exDate: '2026-10-11', paymentDate: '2026-10-18' },
  { zone: 'America/Los_Angeles', instant: '2026-10-22T00:30:00Z', today: '2026-10-21', exDate: '2026-10-11', paymentDate: '2026-10-18' },
  { zone: 'Europe/Amsterdam', instant: '2026-03-30T22:30:00Z', today: '2026-03-31', exDate: '2026-03-21', paymentDate: '2026-03-28' },
  { zone: 'Europe/Amsterdam', instant: '2026-10-26T23:30:00Z', today: '2026-10-27', exDate: '2026-10-17', paymentDate: '2026-10-24' },
  { zone: 'America/Los_Angeles', instant: '2026-03-09T07:30:00Z', today: '2026-03-09', exDate: '2026-02-27', paymentDate: '2026-03-06' }
]) {
  test(`explicit provider dividend dates in ${control.zone} at ${control.instant}`, { timeout: 20_000 }, async (t) => {
    const { baseURL, dates } = await startStub(t, control);
    assert.deepEqual(dates, { today: control.today, exDate: control.exDate, paymentDate: control.paymentDate });
    async function seed(symbol, exDate) {
      return fetch(`${baseURL}/__control/dividends/${symbol}`, {
        method: 'PUT', body: JSON.stringify({ ex_date: exDate })
      });
    }
    async function announcement(symbol, adjust = 'false') {
      return fetch(`${baseURL}/market-data/dividends?symbol=${symbol}&mic_code=XNYS&adjust=${adjust}`);
    }
    assert.equal((await announcement('E2EDIV-unregistered')).status, 404);
    assert.equal((await seed('E2EDIV-invalid', '2026-02-30')).status, 400);
    for (const symbol of ['E2EDIV-normal', 'E2EDIVREV-revised', 'E2EDIVPIL-short']) {
      assert.equal((await seed(symbol, dates.exDate)).status, 204);
      for (const fetchNumber of [1, 2]) {
        const response = await announcement(symbol);
        assert.equal(response.status, 200);
        assert.deepEqual(await response.json(), {
          meta: { symbol, currency: 'USD', mic_code: 'XNYS', exchange_timezone: 'America/New_York' },
          dividends: [{ ex_date: control.exDate, amount: symbol.startsWith('E2EDIVREV') && fetchNumber === 2 ? 0.55 : 0.5 }]
        });
      }
    }
    assert.equal((await announcement('E2EDIV-normal', 'true')).status, 404);
    // A second symbol and re-registration cannot change another symbol's fact;
    // re-registration resets only that symbol's revision sequence.
    assert.equal((await seed('E2EDIVREV-revised', dates.paymentDate)).status, 204);
    assert.deepEqual((await (await announcement('E2EDIVREV-revised')).json()).dividends,
      [{ ex_date: control.paymentDate, amount: 0.5 }]);
    assert.deepEqual((await (await announcement('E2EDIVPIL-short')).json()).dividends,
      [{ ex_date: control.exDate, amount: 0.5 }]);
  });
}
