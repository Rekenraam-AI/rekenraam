# Setup, identity and privacy audit — 2026-10-17

**Status:** first bounded audit in [#260](https://github.com/sergeyfarin/rekenraam/issues/260)
completed with findings. #261, #263 and #264 are corrected in this change;
[#262](https://github.com/sergeyfarin/rekenraam/issues/262) was retained here and
corrected afterwards (see the resolution note below).
This evaluates the first checklist area, not the whole app or an internet-facing
deployment. The roadmap owns remaining audit order.

## Scope and baseline

Audited baseline `0c79fbb7` and the fixes in this change. The working tree was
clean initially. Reviewed setup/auth/MFA/session/preferences/recovery services,
repositories and handlers; their routing and middleware; configuration/runtime
wiring; secretbox/TOTP primitives; install gate, app-shell sign-out and security
settings; OpenAPI; deployment guidance; and their actual tests. All fixtures
use synthetic owners and isolated temporary databases. No owner data or real
credentials were inspected, and no external provider is needed.

Existing focused backend acceptance passed at the baseline. Nine browser
journeys also passed against a fresh integrated build before fixes. These
normal journeys did not expose the subsequently reproduced MFA interleavings
or the HTTP cookie policy mismatches. New regressions were run against the
unfixed code and failed by assertion, not by compilation; the fixes then make
those regressions pass. Final validation is recorded below.

## Findings and disposition

| Finding | Severity and evidence | Disposition |
|---|---|---|
| An enrollment request that read pending state can overwrite an intervening activation and restore password-only login | P1, #261. A deterministic interleaving through `BeginTOTPEnrollment`/`ActivateTOTP` left status pending and the next password login returned a session without MFA | Fixed: the upsert itself cannot replace active state; the service returns its existing active-enrollment conflict |
| Activation can verify secret A but activate a concurrent replacement secret B | P2, also #261. A second enrollment at the existing clock seam replaced pending evidence after the activation read; a code for A activated B and issued ten recovery codes | Fixed: the transactional pending-to-active update pins the exact sealed secret that was verified; no matching row means conflict, with current enrollment/codes unchanged |
| MFA-settings password confirmations have no failure budget | P2, #262. Six wrong recovery-regeneration passwords followed by a wrong disable password all reached verification and returned invalid credentials, never a rate-limit result | Open: specify a shared bounded reauthentication/activation policy that preserves owner-safe recovery and approved-device lockout protection. A valid session, origin and CSRF are prerequisites; this is not an unauthenticated MFA bypass |
| Session Max-Age always used 30 days despite configured server expiry; reused device approval slid only in SQLite | P2, #263. Real HTTP cases observed 2-hour/60-day rows with a 30-day cookie; successful reuse emitted no renewed device cookie | Fixed: password/completed-MFA logins share cookie issuance with the persisted lifetime and renew the presented valid device token without another identity. Setup keeps its documented default; the database expiry remains authoritative |
| Recovery-required UI assumes a source checkout and dev database | P2, #264. The displayed `go run ./backend/cmd/rekenraam` is not a root-module command; the root has no Go module, native build tags are omitted, and production need not include Go | Fixed: show `rekenraam recover-owner --password-stdin`, using existing operator configuration and the hidden terminal prompt, with the translated backup note |
| Guidance drift | Documentation finding: deployment text still called two-process guards unimplemented; backend agent guidance retained WAL/NORMAL, raw SQLite URI concatenation and app TLS configuration | Corrected to #105/#227, ADR 0015 FULL, template fixtures/escaped URIs and reverse-proxy TLS. No new architecture decision |

Both MFA races require overlapping authenticated setup operations. Beginning
an enrollment additionally requires the password; ordinary unauthenticated
requests cannot enroll or activate MFA. The regression tests introduce no new
production scheduling hook: they use the service's existing injected clock to
place a second real service call at the vulnerable read/write boundary.

## Acceptance evidence by workflow

| Workflow | Verified through | Limit of the evidence |
|---|---|---|
| Fresh/resumable setup | Setup status, first-owner/token/origin/JSON/password validation, duplicate-owner rejection, book/currency/system/category setup tests; bootstrap browser journey | Existing inconsistent owner/step states deliberately require operator recovery; no remote repair endpoint |
| Owner preferences and financial date | Saved IANA-zone validation, audited preference repository, owner-date service/API tests and opposite-midnight/explicit-date browser journeys | A server-computed owner date is a calendar default, not a trusted browser clock or a reporting-policy change |
| Password login | Argon2id PHC parameters/unique salt, dummy-hash unknown-user path, rehash, durable username/IP throttles, Retry-After and approved-device scope tests | Sequential lockout cases pass; concurrency and all failure boundaries need the supplementary audit below |
| Sessions, cookies and sign-out | Unauthenticated/expired/revoked API reads, logout with valid CSRF/origin, cookie attribute tests; new configured-policy HTTP matrix for password and MFA, and existing-device renewal; browser sign-out/sign-in | MFA enrollment does not by itself revoke all older sessions; no new session-management policy is selected here |
| MFA | RFC TOTP vectors, skew/malformed input, sealed-secret checks, challenge expiry/single use, code replay, recovery-code single use/normalization/regeneration and lost-key fallback; atomic activation rollback; new state-interleaving cases; browser enrollment/activation/challenge/recovery | Pending activation guessing and settings password confirmations need #262. Complete identity mutation atomicity is not proved by the successful paths |
| Origin, CSRF, proxy trust and HTTP privacy | Existing authenticated mutation wrappers, login/logout/setup negative API cases, right-to-left XFF tests with allowed/untrusted peers, HSTS/cache/frame/CSP middleware tests, production config validation | This is in-process request/config evidence, not a real Caddy/container/TLS deployment exercise |
| Operator recovery | Real `recover-owner` command tests: verified backup before migration, backup failure refusal, explicit no-backup override, session revocation, and application MFA-clearing test | Not a whole-book disaster-recovery audit; concurrent recovery versus a running server/in-flight authentication is supplementary work |
| Secrets and logging | AES-GCM nonce/tamper/wrong-key tests, TOTP vectors, missing-key refusal and recovery-code fallback, auth-event material/retention checks; request logging and frontend source inspection | Generated setup token is intentionally in private operator logs; attempted usernames/IPs are intentionally retained in auth-event rows. No whole-repository secret-history scan or remote settings check |
| Browser privacy boundary | Credentialed same-origin API client, no-store API headers, transient setup/password/MFA component state, no third-party calls found in the reviewed identity flow | Non-loopback HTTP cannot retain Secure auth cookies; use reverse-proxy HTTPS. No complete browser-memory, multi-tab logout, extension or full-app outbound-traffic audit |

The public setup-status response exposes progress, not passwords or MFA secrets.
Sessions and approved-device/challenge tokens are stored hashed; TOTP secrets
are sealed with the operator key; recovery codes are hashed and remain usable
when that key is lost. Native SQLite data encryption remains deliberately
out of scope. The API-only authentication event/device-management surfaces
remain API-only, as recorded in the feature ledger.

## Reproducing the retained #262 finding

The original probe was run against the real application service and temporary
SQLite fixture. To reproduce, place this test in `backend/internal/app/`, run
it through `scripts/go-backend.sh`, then remove it while #262 is unresolved.
It intentionally asserts desired behavior and currently fails; it is not a
passing regression that would enshrine the defect:

```go
func TestMFASettingsReauthenticationBudget(t *testing.T) {
    t.Parallel()
    ctx := context.Background()
    now := time.Date(2026, 10, 17, 12, 0, 0, 0, time.UTC)
    _, service := newMFATestService(t, now)
    enrollAndActivate(t, service, now)
    for range 6 {
        _, _ = service.RegenerateRecoveryCodes(ctx, MFAPasswordInput{
            UserID: 1, Password: "wrong-password", ClientIP: "198.51.100.4",
        })
    }
    err := service.DisableMFA(ctx, MFAPasswordInput{
        UserID: 1, Password: "wrong-password", ClientIP: "198.51.100.4",
    })
    assert.ErrorIs(t, err, ErrRateLimited)
}
```

The reviewed HTTP routes add authentication, origin and CSRF checks but no
other password-attempt limiter. Thus the service result is reachable by a
session holder rather than being a private helper behavior hidden by HTTP.
The risk interpretation follows OWASP's
[sensitive-feature reauthentication and automated-attack guidance](https://cheatsheetseries.owasp.org/cheatsheets/Authentication_Cheat_Sheet.html).
A suitable budget must account for this app's approved-device recovery model;
this review does not adopt a generic lockout policy as a product decision.

**Resolution (#262).** A follow-up change added a per-session reauthentication
budget shared by enrollment, activation, disable and recovery-code regeneration
(policy in `docs/conventions.md` and `docs/deployment-security.md`). The probe's
sequence (wrong passwords switching from regeneration to disable) is covered by
`backend/internal/app/auth_mfa_reauth_test.go` and
`backend/internal/api/auth_mfa_reauth_test.go`. Approved-device and sign-in
budgets are untouched.

## Additional targeted audits to record in #260

1. **Identity concurrency and failure atomicity.** Deterministically overlap
   password failure counters, enrollment/disable/regeneration/login, password
   rehash and owner recovery. Force failures between factor consumption,
   challenge consumption, session insertion, device approval and event recording.
   Establish the revocation cutoff for in-flight authentication and whether
   rejected requests leave spendable factors, orphan sessions or misleading
   success records. Review attribution of successful MFA management/recovery
   changes as well: the current authentication-event schema records sign-in
   attempts and logout, rather than those management transitions. #261 fixes
   two demonstrated state transitions, not all
   of these paths. Go's race detector alone cannot detect logical SQL races.
2. **Production browser/proxy and shared-browser privacy.** In the existing
   deployment/frontend audit areas, verify setup-token onboarding, HTTPS/host
   cookies, proxy header overwrites, MFA and logout across tabs/back navigation,
   cached sensitive views and restart/recovery with a missing key. Use a
   disposable deployment. Remote repository scanning settings and a human
   household dry run remain separate evidence.

Item 1 was audited as #267: see the
[identity concurrency audit](identity-concurrency-audit-2026-10-17.md)
(#285/#286/#287 fixed, #288 open).

These scopes are logged as further audits, not asserted vulnerabilities or a
new provider prerequisite. They do not reopen a successful normal-flow test.
The first checklist area can be evaluated with #262 open, provided the
completion record names that risk and does not certify release readiness.

## Validation

- `GOCACHE=/tmp/rekenraam-go-build-cache ./scripts/test-backend.sh`: passed;
  shared formatting, vet and all package race gates. API and application suites
  executed the new regressions; unaffected passing packages may use Go's cache.
- `./scripts/test-frontend.sh`: passed; Svelte reported zero errors/warnings,
  all 676 unit cases passed, and the separate 66-case catalog gate passed.
- `pnpm --dir e2e test auth.spec.ts mfa.spec.ts owner-local-dates.spec.ts owner-local-date-contract.spec.ts`:
  passed all nine journeys on a rebuilt integrated binary (2.8 minutes for build
  plus tests). The build includes frontend catalog/API generation, embedding and
  the static-serving test.
- Focused new regressions first failed on the unfixed baseline, then passed:
  `TestMFAEnrollmentCannotOverwriteAnActivation`,
  `TestMFAActivationMustProveCurrentPendingSecret`,
  `TestLoginCookieLifetimeMatchesPersistedPolicy` and
  `TestSuccessfulLoginRenewsExistingApprovedDeviceCookie`.
- `./scripts/check-post-merge.sh HEAD`: passed, including OpenAPI regeneration
  without drift. `git diff --check` and relative-link/anchor validation passed.

The initial sandboxed full/backend browser attempts could not bind localhost
servers. Final runs used authorized localhost access with the same shared
commands and assertions. The read-only default Go cache also required a writable
`GOCACHE`; no test/resource limit was weakened. #262's desired-behavior probe
still fails as documented and is not installed as an executable test in the
passing suite. No real production proxy, owner data or secret-history audit was
performed.

Evidence correction: the initial #264 issue description incorrectly called the
example `new-password` too short. It is exactly 12 characters and meets the
minimum; that claim is withdrawn. The demonstrated command/module/deployment
assumptions remain the finding, and the issue description is corrected.
