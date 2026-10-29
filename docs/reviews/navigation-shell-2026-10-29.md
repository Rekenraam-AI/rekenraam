# Navigation shell delivery review — 2026-10-29

Reviewed the combined change against `988c6b61` for #455, the first delivery
slice of #436. The original shell implementation and subsequent completion
changes form one feature slice. No backend, API, migration, dependency or
financial calculation changes are included.

## Scope and contracts

- The sidebar and duplicate top bar become one sticky header, section links,
  section page routes and phone disclosures. Settings keeps its side list.
- The typed route registry owns page identity, section, title key, icon,
  localized keyword keys, width metadata, detail parents and tab membership.
  Registry and browser-sweep coverage checks reject missing app pages. Icons
  render from a separate module so the registry remains usable in Node tests.
- Reports, Instruments, Gains and Prices use real routes. Legacy paths and
  query selectors redirect while retaining other parameters and fragments.
  Report tabs retain filters; Overview views read the URL reactively.
- Setup/session gates, owner-local dates, CSRF-aware logout and document titles
  retain their existing responsibilities. Print hides the shell.
- Existing financial assertions remain: exact figures, report scope, saved
  results, correction history and unknown/stale coverage are not weakened.

## Findings resolved

- Old navigation-button selectors, the retired theme toggle, desktop-only
  Home links and the old Reports landing-screen expectation were corrected.
  The Home helper opens the phone disclosure when needed.
- Dutch and German instrument activity labels now include their visible
  action as one phrase in the accessible name. The newly routed Instruments
  page is included in the locale/accessibility sweep.
- The active tab is kept visible after navigation, viewport resize and font
  changes. Scroll padding matches the overflow fade so a middle tab's label
  cannot disappear beneath it.
- Header disclosures fit short phone viewports and scroll to Sign out.
  Escape restores focus; theme selection includes a persistent system choice.
- Tab labels, page headings and content share the shell's left edge; extra
  outer padding on Recurring was removed.
- Width metadata uses the approved narrow/standard/fluid vocabulary. It has
  no rendering consumer in this slice; applying widths remains #459.

## Validation

- `./scripts/test-frontend.sh`: 802 tests passed; Svelte check reported zero
  errors and zero warnings. Catalog parity and generated API types passed.
- `pnpm build`: static frontend, embedded-web check and Go binary passed.
- `E2E_SKIP_BUILD=1 E2E_WORKERS=3 ./scripts/test-e2e-smoke.sh`: all 491 browser
  tests passed (29.1 minutes), including route/subview sweeps, all configured
  locales, both themes, error states and existing financial journeys. The
  provider-stub and browser-harness checks in the wrapper also passed.
- Final `shell-navigation.spec.ts` run: all 12 tests passed against the final
  production binary, including active-tab fade/resize and short phone menus.
- Full staged diff review and whitespace checks passed. No unresolved finding
  remains in the reviewed scope.

The clean full browser run used an isolated source snapshot and production
binary, avoiding other sessions' databases, ports and trace directories.
The final review's changes to unused width metadata and tab scroll padding
were covered by the final frontend gate/build and shell checks respectively.
No unrelated source changes were incorporated during validation.

## Retained scope

#455 does not complete #436. Hide amounts (#456), shared search (#457), the
attention read and Review count (#458), and page actions, widths and keyboard
help (#459) remain separate. The cross-page journey with #442 is still an
acceptance item on #436. Ledger, Planning and Reports overviews are link-list
placeholders until #448/#450. The Accounts tab remains until #448.

The proposed roadmap reorder in the handoff was not applied. Native
terminology review remains #102. Hosted CI and publication are not asserted by
local validation. No independent large follow-up was found in this review.
