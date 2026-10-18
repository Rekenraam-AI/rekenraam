/**
 * In-page API access for e2e runs.
 *
 * Requests go through `page.evaluate` rather than Playwright's request context
 * so they carry the browser's session cookie — the same one the UI under test
 * uses — instead of a second, separately authenticated session.
 */

import type { Page } from '@playwright/test';

/**
 * Calls the API from inside the page and returns the decoded JSON body.
 *
 * `csrfTokenOrExpected` is overloaded: pass the CSRF token for mutations, or
 * the accepted status codes when no token is needed. Setup endpoints that are
 * idempotent-by-conflict are called as `apiJSON(page, 'POST', path, token,
 * undefined, [201, 409])`.
 */
export async function apiJSON<T = unknown>(
  page: Page,
  method: 'GET' | 'POST' | 'PUT' | 'PATCH' | 'DELETE',
  path: string,
  csrfTokenOrExpected?: string | number[],
  body?: unknown,
  expectedStatuses: number[] = method === 'POST' ? [200, 201] : [200]
): Promise<T> {
  const csrfToken = typeof csrfTokenOrExpected === 'string' ? csrfTokenOrExpected : undefined;
  const statuses = Array.isArray(csrfTokenOrExpected) ? csrfTokenOrExpected : expectedStatuses;
  const response = await page.evaluate(
    async ({ method, path, csrfToken, body }) => {
      const headers: Record<string, string> = {};
      if (csrfToken) headers['X-CSRF-Token'] = csrfToken;
      if (body !== undefined) headers['Content-Type'] = 'application/json';
      const res = await fetch(path, { method, headers, body: body === undefined ? undefined : JSON.stringify(body) });
      return { status: res.status, text: await res.text() };
    },
    { method, path, csrfToken, body }
  );
  if (!statuses.includes(response.status)) throw new Error(`${method} ${path} failed with ${response.status}: ${response.text}`);
  return response.status === 204 || response.text === '' ? (undefined as T) : (JSON.parse(response.text) as T);
}

/**
 * The current session's CSRF token.
 *
 * The token rotates on privilege-changing requests, so re-read it after each
 * mutation rather than caching one for a whole test.
 */
export async function csrfTokenFor(page: Page): Promise<string> {
  const session = await apiJSON<{ authenticated: boolean; csrf_token?: string }>(page, 'GET', '/api/v1/auth/session');
  if (!session.authenticated || !session.csrf_token) throw new Error('expected authenticated session with CSRF token');
  return session.csrf_token;
}

/**
 * Sends a raw JSON body and returns the response text, both unparsed.
 *
 * Exact-evidence tests (#311) need number tokens such as 9007199254740993
 * that `JSON.stringify`/`JSON.parse` would round, so neither side of this
 * call goes through a JavaScript number.
 */
export async function apiExactText(
  page: Page,
  method: 'GET' | 'POST',
  path: string,
  csrfToken?: string,
  rawBody?: string
): Promise<string> {
  const response = await page.evaluate(
    async ({ method, path, csrfToken, rawBody }) => {
      const headers: Record<string, string> = {};
      if (csrfToken) headers['X-CSRF-Token'] = csrfToken;
      if (rawBody !== undefined) headers['Content-Type'] = 'application/json';
      const res = await fetch(path, { method, headers, body: rawBody });
      return { status: res.status, text: await res.text() };
    },
    { method, path, csrfToken, rawBody }
  );
  if (response.status < 200 || response.status > 299) throw new Error(`${method} ${path} failed with ${response.status}: ${response.text}`);
  return response.text;
}

/** JSON text of `body` with each `"EXACT:<token>"` string replaced by its bare number token. */
export function exactJSON(body: unknown): string {
  return JSON.stringify(body).replace(/"EXACT:(-?[0-9.eE+-]+)"/g, '$1');
}
