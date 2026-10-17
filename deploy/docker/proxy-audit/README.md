# Disposable reverse-proxy audit deployment

A reproducible recipe for the [#268](https://github.com/sergeyfarin/rekenraam/issues/268)
production-proxy and shared-browser privacy audit. The findings are in
[`docs/reviews/proxy-browser-privacy-audit-2026-10-17.md`](../../../docs/reviews/proxy-browser-privacy-audit-2026-10-17.md).

This is an audit fixture, not a supported deployment template. It layers a
Caddy TLS proxy over the supplied [`compose.yaml`](../compose.yaml), so the app
container is exactly the documented one. The only differences from the
[deployment guide](../../../docs/deployment-security.md#reverse-proxy-and-tls)
are these:

- `tls internal`: Caddy's local CA issues the certificate for `rekenraam.test`,
  so no real domain or ACME account is needed.
- The proxy publishes `127.0.0.1:8443` only.
- `TRUSTED_PROXY_CIDRS` is the proxy container's fixed address
  (`172.31.68.10/32`), never the network.

The app's own `127.0.0.1:16888` mapping is kept on purpose. It shows that a
client reaching the app directly cannot use forged proxy headers.

## Requirements

- Docker with Compose v2, `curl`, `openssl` and Node.
- The e2e workspace installed (`pnpm --dir e2e install`) with Playwright's
  full Chromium. The headless shell never restores pages from the
  back/forward cache, so it cannot show what that cache keeps.

## Run

```sh
deploy/docker/proxy-audit/run.sh
```

The script does the following:

1. Generates a throwaway `SETUP_TOKEN` and `REKENRAAM_SECRET_KEY` into a
   temporary directory.
2. Builds and starts the stack from a fresh volume.
3. Copies Caddy's root certificate out, so every `curl` verifies the TLS chain
   against it.
4. Runs these phases in order:

| Phase | What it shows |
|---|---|
| headers | HSTS, CSP, frame, referrer and `no-store` headers as emitted through the proxy |
| `deploy` (browser) | Setup-token onboarding, `__Host-` cookie attributes, MFA enrollment and sign-in, sign-out across tabs, back/forward navigation and the back/forward cache, HTTP-cache reads after sign-out |
| proxy headers | Forged `X-Forwarded-*`, `Forwarded` and `X-Real-IP` headers; foreign and `http` origins; unknown `Host`; direct access to the app port; the client address recorded in auth events |
| `restart` | Same key: existing session and TOTP sign-in survive |
| `key-changed` (twice) | Rotated key, then no key: TOTP is refused and a recovery code signs in |

At exit it runs `docker compose down -v` and deletes the temporary directory.
Set `KEEP=1` to leave both for inspection, and `PROXY_AUDIT_WORKDIR` to choose
the directory. The browser phases are `e2e/proxy-audit.config.ts`. They are not
part of the main e2e suite, because they need this deployment, a fresh volume
and restarts between phases.

To browse the deployment by hand, start it with `KEEP=1`. Then map
`rekenraam.test` to `127.0.0.1` and trust the copied `ca.crt` in a disposable
browser profile.

## Limits

- All traffic comes from the host through Docker's loopback port publishing.
  The proxy therefore sees every client as the bridge gateway (`172.31.68.1`),
  and so does the app. That is the correct result for this topology, but it
  means a per-IP budget is shared by every local client. LAN clients going
  through Docker's DNAT were not exercised.
- Only Chromium was used.
- The browser accepts the local CA's leaf with `ignoreHTTPSErrors`. `curl`
  checks the chain explicitly.
