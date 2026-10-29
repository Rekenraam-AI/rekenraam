# Dividend entry enabled-state contrast (#416)

Reviewed against `9687e111` on 2026-10-29. This is a local delivery record;
publication and hosted CI confirmation remain pending.

## Issue and recent-fix triage

[#416](https://github.com/sergeyfarin/rekenraam/issues/416) and
[#414](https://github.com/sergeyfarin/rekenraam/issues/414) remain open.
They have different observed mechanisms: #416 reports a contrast finding on
a control carrying `disabled:opacity-50`; #414 reports a capital-return
reversal click stalled inside one browser page. No evidence links that stall
to the opacity transition identified here. #414's light failure traces were
already delivered in `ff08ffc6`; its unconfirmed cause remains separate.

Recent merged fixes do not cover this submit button:

- [#428](https://github.com/sergeyfarin/rekenraam/pull/428) and
  [#431](https://github.com/sergeyfarin/rekenraam/pull/431) repair and test
  investment returns reference-query loading/error/retry behavior.
- [#426](https://github.com/sergeyfarin/rekenraam/pull/426) bounds Chromium
  installation and repairs the Go cache fingerprint in CI setup.

## Identified element and state

The offending control is the `button[type="submit"]` in
`frontend/src/lib/investments/dividend-form.svelte`, shared by cash dividends,
separate withholding and reinvested dividends. Its native disabled condition
is `!canSubmit || pending`. The button combined `disabled:opacity-50` with
Tailwind's bare `transition`, which includes opacity with a 150 ms duration.
Completing the last required field removes `disabled` immediately but starts
an opacity transition from 0.5 to 1. Thus the control becomes operable before
its full-contrast appearance returns. axe's disabled-control exemption no
longer applies during that interval.

The new browser regression fills the real cash-dividend form in Russian at
390 px in both light and dark themes. It changes the last required amount
and reads the submit button after Svelte's microtask flush in the same browser
task. This avoids a Playwright round trip that could outlast the transition.
It measures the real rendered control rather than checking class names or
waiting until the defect disappears.

On a freshly built unchanged app, all six attempts (two themes, three repeats,
three workers) failed with exactly:

```json
{"disabledOpacity":"0.5","disabled":false,"opacity":"0.5"}
```

The unchanged binary SHA-256 was
`83693039af639a2f0bf35732cd8cadedd4cc6300e99d80aabd4927f8223eee7d`.
This reproduces the invalid enabled/faded state; it does not claim six
reproductions of the original timing-dependent axe finding.

## Change and validation

The submit button now uses `transition-colors`, matching the dividend
correction form and the existing validate-and-ship guidance. Enabling it
restores opacity immediately. Native disabling, hover feedback and the
existing semantic color tokens remain in use. The regression expects
`disabled: false` and opacity `1` immediately, then runs the ordinary axe
check. No rule, element, locale or existing journey is excluded.

Reproduction commands, from the repository root:

```sh
pnpm build
E2E_SKIP_BUILD=1 pnpm --dir e2e exec playwright test \
  investments-dividend-attribution.spec.ts --grep 'fully opaque' \
  --repeat-each=3 --workers=3
E2E_SKIP_BUILD=1 pnpm --dir e2e exec playwright test \
  investments-dividend-attribution.spec.ts --repeat-each=3 --workers=3
./scripts/test-frontend.sh
E2E_SKIP_BUILD=1 E2E_WORKERS=3 ./scripts/test-e2e-smoke.sh
```

To reproduce the negative control, restore only the submit button's
`transition-colors` class to `transition`, rebuild, and run the first browser
command. The test must remain in place.

- `pnpm build`: passed, including the embedded frontend serving test.
- Unchanged-build negative control: 6/6 failed on the enabled opacity assertion.
- Fixed-build focused run: 60/60 passed (2.1 min, three workers). This includes
  all 18 existing payer journeys (three shapes × six locales), plus the two
  theme regressions, each repeated three times.
- `./scripts/test-frontend.sh`: passed; zero Svelte errors/warnings and 781
  unit tests.
- Full smoke: 458/458 passed in 19.0 min at three workers, including all route/sub-view
  audit matrices and all 20 dividend-attribution cases on the shared worker
  books. The wrapper's 12 provider-stub tests, 8 Node harness controls and 4
  browser readiness controls also passed.
- All six capital-return reversal locale cases from #414 passed in the full
  run. This supplies one more non-recurrence observation, not a diagnosis or
  closure of that separate rare stall.

The fixed binary SHA-256 is
`b52be53e87fbbd83a771f62c91fde5e9f98bc50745f74d6b595f852e8f7ba0c4`.
Both binaries were built locally from the same checkout, with only the
submit transition changed between them. Playwright 1.64.1 used its installed
pinned Chromium headless shell (1248), with three workers and fresh disposable
worker books. Timing records and command logs for this investigation live
under `/tmp/rekenraam-416-*`; they are local reproduction artifacts, not hosted
CI evidence. The GitHub issues remain open; #414 has no new diagnosis here.
