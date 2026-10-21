import { expect, type Page, type Request } from '@playwright/test';

type Activity = { pending: Set<Request>; revision: number };
const activity = new WeakMap<Page, Activity>();

/** Start before navigation so even requests already pending at the wait are counted. */
export function observePageRequests(page: Page): void {
  if (activity.has(page)) return;
  const state: Activity = { pending: new Set(), revision: 0 };
  const started = (request: Request) => { state.pending.add(request); state.revision++; };
  const finished = (request: Request) => {
    if (state.pending.delete(request)) state.revision++;
  };
  page.on('request', started);
  page.on('requestfinished', finished);
  page.on('requestfailed', finished);
  page.once('close', () => {
    page.off('request', started);
    page.off('requestfinished', finished);
    page.off('requestfailed', finished);
  });
  activity.set(page, state);
}

/** Await current/chained requests, fonts and two rendered frames, without a fixed quiet delay. */
export async function waitForPageReady(page: Page): Promise<void> {
  const state = activity.get(page);
  if (!state) throw new Error('observePageRequests must run before navigation');
  await expect.poll(async () => {
    const revision = state.revision;
    const fontsReady = await page.evaluate(async () => {
      await new Promise<void>(resolve => requestAnimationFrame(() => resolve()));
      await new Promise<void>(resolve => requestAnimationFrame(() => resolve()));
      return document.fonts.status === 'loaded';
    });
    return fontsReady && state.pending.size === 0 && state.revision === revision;
  }, { timeout: 10_000, intervals: [0], message: 'requests, fonts and rendered state settle' }).toBe(true);
}
