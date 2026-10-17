# Production proxy and shared-browser privacy audit — 2026-10-17

**Status:** targeted sub-audit
[#268](https://github.com/sergeyfarin/rekenraam/issues/268) of
[#260](https://github.com/sergeyfarin/rekenraam/issues/260). It is complete,
with one finding filed:
[#290](https://github.com/sergeyfarin/rekenraam/issues/290) (P3, fixed after this audit). The
deployment guidance is corrected in this change. The
[setup/auth/privacy audit](setup-auth-privacy-audit-2026-10-17.md) left two
things unproved. One was whether its in-process cookie, proxy-trust and header
evidence holds behind a real TLS reverse proxy. The other was shared-browser
privacy. This audit covers both. It does not cover remote GitHub repository
security settings or a human household dry run, and both remain **not
verified**.

## Scope and method

The audited SHA is `00fdbd8f`, with a clean working tree. The deployment is the
supplied production container (`deploy/docker/compose.yaml`, `APP_ENV=production`,
read-only root filesystem, non-root user) behind Caddy 2.11.4. Caddy uses
`tls internal`, which is its own local CA, on `https://rekenraam.test:8443`.
The app trusts only the proxy container's address (`172.31.68.10/32`). The
recipe is committed under
[`deploy/docker/proxy-audit/`](../../deploy/docker/proxy-audit/README.md).
[`run.sh`](../../deploy/docker/proxy-audit/run.sh) reproduces every row below
from an empty volume and removes everything again afterwards.

Evidence comes from three sources:

- **`curl`**, which verifies the TLS chain against the copied local root and
  records raw headers and cookies.
- **Full Chromium driven by Playwright**
  ([`e2e/proxy-audit/proxy-deployment.spec.ts`](../../e2e/proxy-audit/proxy-deployment.spec.ts),
  config `e2e/proxy-audit.config.ts`). Playwright normally disables the
  back/forward cache. That default is removed here, and the headless shell is
  avoided because it never restores pages from that cache. A probe showed the
  shell reporting a masked "not restored" reason for every page, including a
  static one.
- **Container logs.**

The expected results are derived independently from the deployment guide,
browser cookie-prefix rules, Caddy's documented header behavior and ADR 0002.
They are not taken from the code under test. The book is synthetic: one owner
and one account named `Proxy audit savings 4242`, used as a sentinel that only
a financial view renders.

## Findings and disposition

| Finding | Severity and evidence | Disposition |
|---|---|---|
| After a key rotation or with no key, a correct authenticator code is answered as a wrong one and spends the sign-in budget | P3, [#290](https://github.com/sergeyfarin/rekenraam/issues/290). `POST /auth/login/mfa` returned `401 "invalid authentication code"` and recorded `login_failed/mfa_invalid`, while the server logged `mfa secret could not be opened; only recovery codes can be used`. In one run, a fresh browser shared the username/IP budget with earlier failures from the same address, and the recovery code was then refused with `429` | **Confirmed defect**, filed. Recovery works once the budget allows, and an approved device has its own budget. Not fixed here, per the evidence contract |
| The guide's Caddy example adds three `header_up X-Forwarded-*` lines | Caddy 2.11.4 logs `Unnecessary header_up …` for each one. An echo upstream showed Caddy's defaults already replace client-supplied `X-Forwarded-For/Host/Proto`, with and without those lines | **Guidance corrected**: the example drops them and states what Caddy does by default |
| The guide does not say which client address the app sees behind a containerized proxy reached through a loopback-published port | Every recorded `client_ip` was the bridge gateway `172.31.68.1`, whether the request came through the proxy or went directly to the app. That is correct for this topology, but the per-IP budget is then shared by every client behind it | **Guidance corrected**: check the recorded addresses after deploying. Approved devices are the owner's lockout protection either way |
| The guide's key-loss text says "recover with a recovery code" without the steps that follow | Observed path: sign in with a recovery code, disable two-factor with the password, then enroll again. With no key, enrollment refuses with `503 CONFIG_REQUIRED`. `GET /auth/mfa` keeps reporting `active` after the key changes, and with no key it reports `configured:false` | **Guidance corrected** |

## Acceptance evidence by check

| Check | Expected (independent) | Observed | Disposition |
|---|---|---|---|
| HTTPS and the security headers through the proxy | HSTS on HTTPS responses; CSP with `frame-ancestors 'none'`; `X-Frame-Options: DENY`; `no-referrer`; `nosniff`; `no-store` and `no-cache` on `/api/v1/*` | All present on `/`, static assets, `/healthz` and API responses, with `via: 1.1 Caddy`. HSTS is present only through the proxy, not on direct HTTP | Verified |
| Setup-token onboarding | Owner creation refuses a wrong token and sets no cookie; the correct token works; the token is not kept in browser storage | Wrong token: alert shown and cookie jar empty. Correct token: currency step, then `/app`. The token was absent from `localStorage` and `sessionStorage`. `window.isSecureContext` was true | Verified |
| Cookie attributes as emitted | `__Host-` names, `Secure`, `HttpOnly`, `SameSite=Strict`, `Path=/`, no `Domain` | `__Host-rekenraam_session` (Max-Age 30 days), `__Host-rekenraam_device` (180 days) and `__Host-rekenraam_mfa` (300 s, cleared on completion). The browser stored all of them as host-only. No unprefixed session cookie is set over HTTPS | Verified |
| Forged client headers | The proxy replaces `X-Forwarded-*`. The app reads only `X-Forwarded-For/Host/Proto`, and only from the trusted peer | `X-Forwarded-For: 203.0.113.66`, `Forwarded`, `X-Real-IP`, `X-Forwarded-Host: evil.example` and `X-Forwarded-Proto: http` sent through the proxy gave `401`, and the recorded address was `172.31.68.1`. No `203.0.113.x` address appears in any event | Verified |
| Origin and CSRF with the proxied origin | Only `https://rekenraam.test:8443` passes | `https://evil.example`, `https://evil.example` with a forged `X-Forwarded-Host`, and `http://rekenraam.test:8443` each got `403 CSRF_INVALID`. Browser mutations with CSRF tokens succeeded | Verified |
| Unknown `Host` / SNI | The request does not reach the app | Unknown `Host`: an empty Caddy `200` with no app headers. Unknown SNI: TLS alert | Verified |
| Direct app access bypassing the proxy | Forged proxy headers are not trusted from an untrusted peer | On `127.0.0.1:16888`, a request with the proxied origin and forged `X-Forwarded-Host/Proto/For` got `403`, no HSTS and no cookie. With its own HTTP origin, the recorded address was the actual peer | Verified. Loopback-only publishing is the documented posture |
| MFA through the proxy | The password alone yields only a challenge, which grants nothing | `__Host-rekenraam_mfa` was set, `GET /accounts` returned `401` during the challenge, and the TOTP code then signed in | Verified |
| Sign-out in one tab | The other tab's reads fail. Once that tab is shown again, it leaves without fetching new data | The other tab's `GET /accounts` returned `401` immediately. Its already-rendered list stayed visible until the tab became visible again (`visibilitychange`; otherwise the 60 s session poll). It then showed sign-in, with the sentinel gone | Verified, with the accepted limit below |
| Back/forward after sign-out (same document) | A history pop must not render the financial route | Back, then forward, after sign-out: sign-in shown, sentinel absent | Verified |
| Back/forward cache after sign-out elsewhere | A restored document must not keep showing financial data | `pageshow.persisted` was true (actual restore, no "not restored" reasons). The sentinel was not visible at the first check after commit, and sign-in showed within 150 ms in every run (57 ms in the timed restore) | Verified, with the accepted limit below |
| HTTP cache after sign-out | Sensitive reads are never served from cache | `fetch(…, {cache:'force-cache'})` for `/accounts`, `/auth/events` and `/auth/session` returned fresh `401`/unauthenticated responses with `no-store`, without the sentinel | Verified |
| Restart, same key | Sessions and the second factor survive | An existing session read `/accounts` with `200`. TOTP sign-in worked on the approved device | Verified |
| Restart, rotated key | TOTP refused; recovery code works | Refused (`401`), recovery code signed in, `WARN` logged. Disable and re-enroll worked under the new key | Verified. Message and budget: #290 |
| Restart, missing key | As above; enrollment refuses | Refused, recovery code signed in, `/auth/mfa` reported `configured:false`, enrollment returned `503 CONFIG_REQUIRED` | Verified. Message and budget: #290 |
| Remote repository security settings; household dry run | Owner-side evidence | Not attempted | **Not verified** |

## Accepted limits

- **Rendered data stays in a hidden tab until it is looked at.** The shell
  re-checks the session when a tab becomes visible and on a 60-second poll.
  `refetchOnWindowFocus: 'always'` and `refetchInterval` are in
  `authSessionQueryOptions`, and TanStack Query maps `visibilitychange` to
  focus. A page restored from the back/forward cache also becomes visible, so
  it leaves the same way. The financial DOM is still in that tab's memory until
  then, and the restored snapshot may render it for the first frame. The audit
  observed no frame with the sentinel, but at millisecond resolution it cannot
  exclude a single frame. Closing the browser remains the advice for a shared
  computer.
- **Topology.** All traffic came from the host through Docker's loopback port
  publishing. LAN clients reaching a published port through DNAT, other
  proxies, and other browsers were not exercised. The browser accepted the
  local CA's leaf through `ignoreHTTPSErrors`, while `curl` verified the chain.

## Maintainability notes

- **Proxy trust is interpreted in three helpers that disagree on parsing.**
  `requestUsesHTTPS` takes the first comma-separated `X-Forwarded-Proto` value.
  `requestOriginForHost` uses the whole header as the scheme.
  `loginClientIP`/`trustedForwardedClientIP` parses `X-Forwarded-For`
  separately (`backend/internal/api/auth.go`). A multi-value
  `X-Forwarded-Proto` therefore yields HTTPS cookies but a non-matching origin
  (fails closed). The divergence was observed in code, not triggered: Caddy
  sends one value.
- **`clearCookie(w, name, secure)` ignores `secure`** (`_ = secure`) and always
  clears with `Secure`. Its callers (session and MFA challenge clearing) compute a value that has no effect.
- **The shared-browser exit is a side effect of query options.** Nothing named
  enforces "leave when the session ends". It follows from
  `authSessionQueryOptions` (`frontend/src/lib/api/auth.ts`) and from the
  layout's redirect, so changing those defaults would silently remove it. Only
  this audit's spec covers it, outside the main suite.

## Validation

- `deploy/docker/proxy-audit/run.sh` passed end to end from an empty volume:
  build, headers, 8 browser checks, proxy probes, restart, rotated-key and
  missing-key phases, then teardown.
- No application code changed, so the backend and main browser suites were not
  rerun. `csp.spec.ts` and `auth.spec.ts` are unaffected.
- `./scripts/check-post-merge.sh HEAD` for the documentation change.
