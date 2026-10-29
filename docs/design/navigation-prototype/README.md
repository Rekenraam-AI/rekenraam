# Navigation prototype — 2026-10-29

Browser-openable companion to the approved [Home and navigation design](../home-and-navigation.md).
It extends the [v3 Home canvas](../home-mockup/README.md) with destinations so
the header, scoped object search and everyday journeys can be reviewed together.
This is implementation preparation for #436/#437/#442, not delivery of those slices.

Open [index.html](index.html) directly in a browser. No server, canvas runtime,
credentials, network access or build is needed. All data is synthetic. Local
browser storage remembers only this prototype's theme and amount masking; entry,
import and reconciliation controls never write financial data. The original
approved canvas remains the reference for Home styling and personalization.

## What to review

- All approved section/settings menu entries lead to distinct destinations.
  Ledger includes Payees; Gains has one canonical Investments destination and
  a Reports link; Prices and currency/FX settings remain separate.
- Checking and Savings registers retain applicable URL filters when switched
  using the title's scoped search. Removing the scope chip changes the same
  search into global navigation. Closed accounts are one checkbox away;
  holding accounts appear when searched. Pins are sample choices, not usage-ranked.
- Spending links to category details; Home review links to an import batch
  and recurring drafts; Reconcile has explicit preview and sample finish.
- Add is available on registers, Transactions and category pages, including
  phones. Escape with unsaved sample input asks whether to discard it; no
  search action saves an entry.
- The phone menu opens, local page tabs scroll, and scoped search becomes
  full screen. Header menus use disclosures; search keeps input focus and
  exposes the selected option to assistive technology.
- The sample-book selector exposes fresh-book setup, unavailable valuation,
  and failed attention checks. A failed check never says All clear.
- Legacy Reports/Investments URL selections and dividend-review links retain
  other filters. Native Back, forward, copied links and new tabs remain usable.
  Returning to Home restores its review/explore expansion and scroll.

## Bounded walkthrough

1. From Home, open Ledger → Checking. Apply March and a search filter.
2. Open Checking ▾, type Savings and press Enter. Check that the applicable
   filters remain. Remove the scope chip and observe that results become global.
3. Open Add; check the selected account, type a payee, then press Escape.
   Retain the input or discard deliberately. Sample Save confirms the interaction
   without adding a journal or changing the displayed fixture balances.
4. Follow Reconcile this account, preview, then finish the sample. Switch the
   account before previewing to check context.
5. Return Home, expand review and continue the import batch. Choose a category
   and review the sample changes; the batch remains uncommitted.
6. Open Spending, drill into Groceries, then use Back twice. Check Home's
   scroll and expanded review. Repeat on a 390px phone and with only a keyboard.
7. Try both themes, hide amounts, the three alternate sample books and
   320px reflow. Open a pin or report link in a new tab.

Record task time, wrong turns, context loss and whether each figure's scope
was understood against today's app for the same tasks. No owner task timings,
real-book counts or usability acceptance have been recorded by building this
artifact. D11's real-book inventory remains #442's prerequisite for final grouping.

## Validation

Use the already installed repository browser dependencies; no new packages:

```sh
pnpm exec node docs/design/navigation-prototype/check.mjs
# Optional screenshots outside the source tree:
pnpm exec node docs/design/navigation-prototype/check.mjs /tmp/rekenraam-navigation-prototype
```

The focused check exercises links and context retention, editor cancellation,
review and reconciliation, legacy URLs, Back restoration, amount masking,
30 axe/reflow cases over five representative pages at 1440/390/320px in both
themes, open switchers, browser errors and absence of remote requests.
This checks the design artifact only. It is not production integration,
financial-correctness validation or native terminology sign-off.

Passed locally on 2026-10-29: 30 menu/pin destinations, the walkthrough's
automated journeys, all 30 page axe/reflow cases and three open switchers,
masking and zero remote requests. JavaScript syntax, local Markdown targets
and `git diff --check` also passed. Acceptance was tightened in
[#436](https://github.com/sergeyfarin/rekenraam/issues/436) and
[#437](https://github.com/sergeyfarin/rekenraam/issues/437) for retained route
context, companion object navigation, attention failure/counting contracts,
masking, composed Home loading/refresh and unsaved editor snapshots. Both
issues remain open for application implementation and its acceptance evidence.

## Deliberate limits

Home has representative summaries and three discovery destinations rather
than the complete widget catalog. Most other destinations identify placement
and link to relevant objects rather than reproduce their full screens.
Transfer, posting recurring drafts, uploads, actual exports, security/session
gates, personalization, pin persistence, payee actions and live API/cache
behavior are not implemented here. The sample editor is not the production
editor and does not parse or calculate money. English sample copy and simplified
account grouping are for this artifact only; production retains the six-locale
translation boundary, canonical exact arithmetic and all existing workflow guards.

The prototype uses `#/app/...` links to work from a local file. Production
uses actual `/app/...` routes served by the existing Go/static SvelteKit app.
The frontend route registry, dialog primitives, composed reads and cache
invalidation must be implemented in their slices, not copied from this artifact.
