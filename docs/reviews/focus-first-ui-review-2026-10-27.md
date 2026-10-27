# Focus-first Home: design review

Reviewed 2026-10-27 against `b2ca6ad3` and the owner's supplied
“Focus-First, Scroll-to-Discover Financial Interface” proposal.

Status: exploratory recommendation, not an accepted product contract or a
change to execution order. The owner's current request is to review and
challenge the idea. This review does not activate the pasted proposal's
implementation instructions. No application behavior changes in this commit.
Evidence is source and API-contract inspection; no rendered prototype,
performance measurement or usability study was performed.

## Recommendation

Proceed with a bounded Home experiment. The strongest idea is a useful,
quiet starting point that reveals more capability through ordinary scrolling.
The weakest parts are treating all planned features as widgets, introducing
four overlapping workspaces immediately, and allowing usage to rearrange a
carefully personalized surface automatically.

Home should help answer three questions: “Where do I stand?”, “Does anything
need attention?”, and “What can I do next?” Scrolling supplies breadth; stable
navigation supplies direct access. Detailed workflows retain their own pages.

## What the current application offers

- [Home](../../frontend/src/routes/app/+page.svelte) is a foundation-era
  overview of authentication, setup, routing, health and next steps. Replacing
  it has a relatively small financial-workflow footprint. That does not make
  replacing the shared shell low risk.
- The [authenticated layout](../../frontend/src/routes/app/+layout.svelte)
  owns setup/session gates, owner-local date context, recurring badges,
  navigation, logout and a common page header. Desktop uses a sidebar; mobile
  uses an overflowing horizontal navigation row. Preserve its responsibilities
  while changing its presentation.
- The [root layout](../../frontend/src/routes/+layout.svelte) also mounts a
  [sticky top bar](../../frontend/src/lib/components/top-bar.svelte). The
  search field currently binds local text without executing search. Do not
  design around it as an existing global search capability. A new Home header
  must account for this outer chrome; otherwise the phone gets multiple headers
  before useful financial content begins.
- Reuse semantic tokens and theme state, `Panel`, `StatePanel`, `StatusBadge`,
  `ModalDialog`, money formatting, owner-local dates, query options and URL
  parsers. Reuse small visual/content components where suitable, rather than
  mounting full report or investment screens inside cards: those screens own
  their own filters, reference queries and navigation.
- Reports, budgets, recurring, forecasting, returns and investment allocation
  already have real implementations. The proposal is primarily a presentation
  and read-composition project, not a reason to rebuild those domains.

The [feature ledger](../implemented.md) and [roadmap](../roadmap.md) provide
context, but individual statements still need code verification. For example,
the ledger's final “Not started” paragraph mentions saved return runs while
the current returns screen and API already support them. This review relies
on the inspected implementation for that capability; it does not repair
unrelated documentation.

## Requirements to challenge

| Proposed requirement | Problem | Recommended amendment |
|---|---|---|
| Focus occupies the initial viewport | A full-height hero can look like the end of the page and postpone useful content. Viewport height varies with browser chrome, text size and translation. | Use content-driven height. Keep an explicit link to Explore; let the next section begin naturally without blank spacer content or scroll snapping. |
| Every planned widget has a preview | An endless row of unavailable cards advertises a roadmap and makes the shipped app look unfinished. | Expose shipped capabilities with compact previews. For shipped-but-unconfigured features, show an honest setup action. Keep unbuilt features out of the normal Home. |
| All widgets are immediately visible | Discoverability does not require every chart to execute or every variant to have a card. | Show category headings and cheap semantic previews in document order; expand or fetch heavier detail on demand. Group variants such as spending by category/payee under one capability. |
| Any useful widget can enter Focus | Some tables and complex charts lose their meaning at glance size. | Require an intentionally designed Focus representation. Others offer a shortcut to the detailed page. |
| Highly configurable but always uncluttered | Unlimited favorites, arbitrary sizes and free placement cannot guarantee a calm result. | Start with one optional headline, four shortcuts and at most two compact favorites. These are proposed limits to test, not proven optimal values. |
| “Available cash” is a simple headline choice | Posted cash, cleared balance, projected cash and money left after commitments answer different questions. | Prefer a precisely named existing metric. Introduce available-to-spend only after specifying accounts, pending items, commitments, period and FX treatment. |
| Attention lives below Focus | A user who only glances can miss a failed import or draft needing review. | Reserve a compact attention summary near the top when actionable items exist, independent of favorites. Do not force all diagnostics into urgent alerts. |
| Personal / Investments / Planning / Books | These mix a broad home, domains, intentions and a storage/accounting term. “Books” can imply runtime book switching. | Test Home / Investments / Planning / Ledger as navigation labels, with one canonical destination per workflow. Do not imply multi-book runtime support. |

Discovery and customization need different success criteria. A person who
never customizes should still get a useful Home. Needing to curate a dashboard
before it becomes helpful would defeat the proposal.

## Desktop and mobile composition

Desktop Home, approximately 1280 × 900 for the first design check:

```text
Rekenraam    Home  Investments  Planning  Ledger       All pages  Settings

Net worth                              Spending this month   [menu]
<exact value or explicit unavailable>   <compact favorite>
As of <date> · <currency/scope>
<coverage notice when required>         Budget remaining      [menu]
                                       <compact favorite>
[Add transaction] [Transfer] [Import] [Accounts]   Customize

Needs attention: <actionable summary, only when relevant>

Explore your finances     Everyday · Wealth · Planning · Data & review
<the first previews begin here; ordinary document scrolling continues>
```

This is a layout proposal, not a screenshot or a claim that all named actions
already have a URL entry point. Use responsive columns with a stable reading
order, rather than masonry, absolute positioning or a freeform canvas.

For an initial 390 × 844 phone mockup, use this approximate vertical budget
in CSS pixels, excluding browser chrome:

| Region | Initial design budget |
|---|---:|
| One compact identity/navigation row | 56 |
| Headline, date, scope and ordinary coverage context | 144 |
| Up to four shortcuts in two rows | 104 |
| Optional compact attention summary | 56 |
| Up to two favorite rows | 112 |
| Section gaps, Customize and Explore link | 88 |
| Remaining room for discovery heading/first preview | About 184 |

These budgets are hypotheses, not fixed heights. At 360 × 640, discovery may
start below the fold; the visible Explore link must still jump to its heading.
At large text sizes or with longer translations, content grows vertically;
never crop coverage text or shrink controls to meet the budget. A headline of
“None” removes its block without preserving an empty hero. Favorites use the
same configured order on desktop and mobile, presented as rows on phones.

There is one vertical page scroll. No sticky hero, nested card scrollers,
horizontal discovery carousel or persistent customization toolbar. Start with
collapsed chart detail but real readable summaries. Category anchors accelerate
navigation; they do not hide other categories behind tabs.

### Header selector versus bottom navigation

| Choice | Benefit | Cost |
|---|---|---|
| Compact header section selector plus All pages menu | Leaves vertical space for financial content; accommodates more destinations and longer labels. | Changing sections takes an extra tap and is less thumb-accessible. |
| Bottom navigation | Stable, reachable destinations for frequent switching. | Uses persistent space, interacts with keyboards/safe areas and risks confusing section links with quick actions. Four long labels also compete at 360px. |

Start with the compact selector and an accessible full navigation menu. Label
it with the current section; use real navigation links, not a select control
that changes a hidden data context. Test bottom navigation if observed daily
tasks involve frequent section switching. If adopted, it should replace the
section selector, have a stable order and contain destinations rather than
financial commands. Do not ship both navigation systems together.

Check 320 CSS-pixel reflow and zoom as well as the requested 360–390px phones.
Use approximately 44px touch targets as a product goal; WCAG 2.2's AA minimum
is 24px with specified exceptions, not a blanket 44px requirement.
[Reflow](https://www.w3.org/WAI/WCAG22/Understanding/reflow.html) and
[target size](https://www.w3.org/WAI/WCAG22/Understanding/target-size-minimum.html).

## How the other pages work

The shared identity, navigation and financial display conventions carry
across the app. The large Focus composition belongs to Home. Detail pages
start with their task, selected object and controls; repeatedly scrolling
past a personal-finance hero to reach a register would be a regression.

| Page family | Recommended behavior |
|---|---|
| Transactions, account register, category activity, instrument activity | Compact heading, object breadcrumb, filters, table and existing detail/editor workflow. Keep filter/selection URLs, pagination, correction context and mobile entry. No discovery section. |
| Accounts and categories | Keep management hierarchy and lifecycle controls. Summaries may link to registers; do not turn editable trees into a dashboard. |
| Reports and investment returns | Start at the report title, applied period, scope, reporting currency and results. Home links pass supported filters to these canonical screens. Keep valuation reasons, drill-downs, exports and saved-report distinctions. |
| Investments | Retain its domain overview and operations. It can acquire a quieter summary later, with holdings, returns, income, gains and pricing as local navigation. No independent personalized layout in the first slice. |
| Planning | Initially a navigation group for the existing Budgets, Recurring and Forecast pages. Each retains its own financial semantics and controls. A combined landing page can follow evidence that it adds value. |
| Import and dividend review | Preserve batch/source identity, preview, review and commit stages. An attention card links into the relevant queue; it does not commit or repair financial records on Home. |
| Reconciliation | Preserve the account/statement task, working state, preview and explicit finish. Home can surface actionable state once its meaning is defined; elapsed time since last reconciliation alone is not proof of an issue. |
| Settings, pricing, backup and restore | Keep predictable administrative destinations. Foreground a specific failure when relevant, rather than promoting administration into permanent Focus content. |

Use “All pages” as a stable, grouped index containing every existing route.
Reports can have one canonical destination even if discoverable from more than
one context; do not create different copies for each workspace. A section label
does not silently change account scope, currency, book or filters.

In the first slice, preserve the detailed-page sidebar while trying the new
Home composition. Before replacing shared navigation, map every current
destination and migrate the root top bar and authenticated layout together.
Never add a third header on top of the existing two. Preserve authentication,
logout, date context and recurring query sharing during that extraction.

Back from a widget's detailed page should restore the Home scroll location
and expansion state. Explicit Home navigation may start at the top. Use stable
widget IDs and reserved loading geometry so late content does not shift the
return position. Preserve native link, open-in-new-tab and browser history
behavior; do not require Home to be visited before a direct detail link works.

## Widget inventory verified against current contracts

“Existing” describes the data/workflow, not a ready-made dashboard card.
Paths below are API paths under `/api/v1` unless stated otherwise.

| Capability | Existing basis | Work and semantic limits |
|---|---|---|
| Net worth | `/reports/net-worth`, exact commodity totals and valuation coverage | Focus adapter needed. Preserve trade-date/unsettled semantics and complete-only converted totals. No fabricated consolidated headline when FX/prices are missing. |
| Spending and income | `/reports/spending`, category/payee grouping and spending/income mode | Small summary and filtered report links. State period and scope; changing cash balance is not income. |
| Cashflow | `/reports/cashflow`, named liquid-cash default or selected scope | Compact adapter. Keep cashflow distinct from categorized spending and budget actuals. |
| Account balances | `/ledger/account-balances` and account metadata | A reusable account card should get composed labels/balances without per-account requests. Define a small preview limit and link to all accounts. |
| Budgets | `/budgets/month`, existing composed read | Preview exact monthly planned/actual/remaining figures per currency. Do not total incompatible currencies or count forecast assumptions as actual spending. |
| Recurring review | `/recurring/summary`, already queried by the shell | Reuse the shared cache for drafts/blocked counts. Future due amounts require the separate due/preview contracts; drafts and upcoming schedules must remain distinguishable. |
| Projected balances | `/forecasts/balances`, with separate event drill-down | Already shipped. Label horizon, selected accounts and assumptions, including opt-in learned spending. Lazy-load; never label an estimate as current cash. |
| Holdings | `/investments/positions` and `/investments/dated-holdings` | Select the contract appropriate to the card's question; retain position side, date, quantity and price/basis gaps. Do not query once per holding. |
| Portfolio returns | `/investments/returns`, TWR/MWR, chart, benchmark and optional instrument results | Already shipped, including saved-run workflow. Use one clearly named measure and period; unavailable reasons remain visible. A market-value change is not a return. |
| Allocation and currency breakdown | The same returns response includes account/asset-type/currency allocation | Reuse its scope and known-only/unknown disclosures. Currency denomination is not a full economic FX-risk analysis. Do not build a second client-side allocation calculator. |
| Gains | `/investments/gains` and existing statement/run reads | Preserve method, scope, known basis and live-versus-saved distinctions. Prefer a link until a useful compact representation is designed. |
| Dividend attribution coverage | `/investments/income-coverage`, including a summary-only `limit=0` read | Existing confirmed/unassigned information. A period-specific dividend card may need a new filtered summary; unattributed cash is not automatically known gross income. |
| Import review | Paginated `/imports`, individual batch reads and existing review screens | A complete actionable count needs a composed read. Do not count only the first batch page or issue one detail request per batch. |
| Reconciliation attention | Account-specific start/checkpoint and reconciliation reads | Define actionable states and add a bounded cross-account summary. Do not manufacture “overdue” status. |
| Provider/price failures | Existing pricing/settings composed data and background-work status | Need a small Home projection; distinguish an optional provider failure from an app-owned data/integrity failure. Opening Home must not trigger refresh jobs. |
| Goals and meaningful change insights | No goal-management or general insights contract identified in the inspected sources | Separate product/backend work. Exclude normal Home previews until usable; do not synthesize advice or trends from arbitrary differences. |
| Available-to-spend | No equivalent canonical contract identified | New financial definition and composed calculation, not a renamed balances widget. |

Relevant contracts: [reports](../../api/openapi/components/schemas/reports.yaml),
[returns](../../api/openapi/components/schemas/investment-returns.yaml),
[forecasts](../../api/openapi/paths/forecast-balances.yaml),
[budgets](../../api/openapi/paths/budget-month.yaml),
[income coverage](../../api/openapi/paths/investments-income-coverage.yaml).

## Customization and widget behavior

Use two independent operations with literal labels:

- **Pin to Focus / Remove from Focus** changes a widget's compact placement.
- **Move earlier / Move later in Explore** changes discovery order within its
  category. Category membership stays stable initially.

Keep the discovery representation after pinning; mark it “In Focus” and reuse
the same cached data and scope. This preserves findability without a second
request. Focus is a summary of the same configured question, not an independently
filtered copy with an identical label. Multiple saved configurations of a
widget are later scope.

At two favorites, pinning offers an explicit choice of which favorite to
replace, with cancel and undo. Do not silently evict one or grow the grid.
Customize opens a lightweight editor with explicit reorder/remove/configure
controls, Save and Cancel; changes persist only on Save. Direct pin actions
can save immediately and offer Undo. Reordering follows DOM/reading order,
returns keyboard focus to the moved item's control and announces the result.
Drag handles may supplement buttons. An expanded discovery chart stays in
normal document flow; the detailed feature opens at its own URL.

Rename only a personal shortcut or saved-view label, keeping its canonical
destination/metric visible where ambiguity matters. Do not let “Net worth”
become an unlabeled sum of selected cash accounts. Start with existing internal
destinations and explicit supported commands; avoid arbitrary external URLs
and user-created financial formulas.

Shared card chrome handles title, period/scope, status and menu; content stays
specific to the question. Model lifecycle (loading/error/ready) separately
from completeness and freshness: a ready response can be incomplete, and a
failed refresh can still have a last successful value. Zero, no records,
unconfigured, unavailable, partial, stale and failed are distinct states.
Never convert null to zero or show a success-colored complete total with a
generic warning elsewhere. Follow each report's policy: complete-only totals
remain unavailable; explicitly supported known-only allocation stays labeled.

## Graduation through actual use

Explore this as **usage-informed invitation**, with explicit adoption.
Someone can discover a useful workflow through use without having to discover
the Customize editor first. But repeated use can mean a temporary repair task;
infrequent use can mean a very important quarterly check. Frequency alone
should not control a financial home page.

Proposed interaction:

1. The user intentionally opens a widget's detail, expands useful content or
   completes its relevant workflow. Merely scrolling past or rendering a card
   does not count. Count at most one qualifying use per widget per owner-local
   day; count route use too so discovery clicks are not the only signal.
2. As an initial test hypothesis, three distinct use days within four weeks
   makes it eligible. This is a tunable heuristic, not an inferred preference
   or validated threshold. Monthly workflows can always be pinned directly.
3. At a later stable visit, its discovery preview can say “You use this often.
   Keep it in Focus?” with **Pin**, **Not now**, and **Don't suggest this**.
   No popover interrupts a task and nothing moves during the current visit.
4. Pin follows the normal capacity/replacement flow. Manual order always wins.
   Removed favorites stay suppressed from future suggestions until explicitly
   re-enabled; Not now has a cooldown. Offer disable/reset for suggestions.

Keep no more than one suggestion visible. Exclude features without a compact
Focus representation; for a complex workflow, offer an eligible shortcut
instead. Never demote a manually pinned widget because it is rarely opened,
and never use usage to remove actionable attention items. Explain the basis
in ordinary language without scores, streaks, unlocks or gamification.

For an experiment, retain only bounded local per-widget day buckets and
dismissals, with no amounts, transaction IDs or external telemetry. Protect
storage reads/writes, validate its version, tolerate storage being unavailable
and clear account-specific in-memory signals on logout. Explain that these
signals apply to this browser and provide reset. Do not make privacy controls
for optional usage learning a prerequisite to ordinary manual customization.

The recommendation to keep placement under user control is a design inference,
not a finding about Rekenraam users. A small historical menu study found static
menus faster than adaptive ones and a majority preference for user-adaptable
menus; its setting does not establish how financial widgets will behave.
[Findlater and McGrenere, CHI 2004](https://www.cs.ubc.ca/labs/imager/tr/2004/findlater04menus/).
Stable repeated navigation also supports predictable access under
[WCAG's consistent-navigation guidance](https://www.w3.org/WAI/WCAG22/Understanding/consistent-navigation.html).

If automatic placement is still worth exploring later, test an explicitly
opted-in, fixed “Suggested” slot. It must never consume an existing favorite,
reorder manual choices, or be required to access a feature. Compare it with
the invitation design before accepting the additional behavior.

## Practical delivery approach

These are proposed slices and acceptance boundaries, not a replacement for
the roadmap. Before implementation, record accepted scope in the product/design
sources and create ready GitHub issues; do not allocate new local work IDs.

1. **Validate the composition and journeys.** Prototype Home at desktop and
   phone widths plus one register, one report and import/reconciliation entry.
   Exercise empty books, dense real data and missing valuation inputs. Ask the
   owner to find spending, add a transaction, review drafts and return from a
   deep report. Decide navigation labels, attention semantics, Focus capacity
   and preference ownership. Do not approve a Home-only screenshot without
   checking the destinations it leads into.
2. **Ship one real Home slice.** Replace the existing overview, preserve the
   shell and existing detailed routes, and deliver one headline, a small action
   set, explicit recurring attention and roughly four real previews: spending,
   budgets, net worth and recurring. No phantom planned widgets, learning,
   freeform layout engine or new workspace dashboards. Quick actions use
   existing editors; any new URL command receives explicit validation and
   browser-history behavior rather than inventing an unsupported query flag.
3. **Add manual personalization.** Introduce typed widget/action IDs, explicit
   reorder controls, compact favorites, settings and validated preferences.
   A small registry describes translated title, category, component loaders,
   supported modes, default placement and destination construction. Do not
   build a general plugin system or give arbitrary JSON the power to execute
   requests. Keep query ownership outside static registry metadata.
4. **Migrate shared navigation deliberately.** Replace duplicated root/app
   chrome, retain all destinations and run detail workflows through it. Only
   this step should introduce the proposed cross-app section navigation.
   Keep one personalized Home; initially group existing routes under other
   sections without separate per-section layout persistence.
5. **Extend where useful.** Add forecast, holdings and investment views using
   lazy data/chart loading. Introduce the composed attention and summary reads
   justified by the inventory. Finally trial graduation suggestions against
   the manually personalized baseline.

### Data loading and persistence

Home must follow the repository's composed-read rule. Define a small
application-service-owned read model under `/api/v1` for the selected Home
summaries; its exact path/schema is new work, not an existing endpoint. Reuse
the existing report/domain services rather than reproduce financial arithmetic
in cards. Batch database reads, bound preview rows, supply display metadata
and retain coverage/provenance. Do not move N+1 work to the server unchanged.

For a static prototype, existing endpoints can demonstrate representative
cards, but that request pattern is not the shipped Home architecture. The
normal initial load should have one Home data request in addition to the
existing setup/session/shared-shell reads. It returns small ready summaries
and deferred capability descriptors; it does not compute every return series
or forecast. Expensive expanded views can have their own composed request,
enabled on expansion or near visibility. Merely keeping a Svelte component
offscreen does not prevent its query from running.

Reuse TanStack Query identities for a pinned and discovery representation;
include scope, currency and dates in keys. Wire new Home reads into existing
mutation invalidation and owner-date rollover. Avoid a forever-fresh Home
summary after transactions, budget changes, imports or price corrections.
Use per-summary error boundaries and typed failures for independent optional
summaries; authentication or a failed shared financial snapshot must not be
disguised as a valid empty book. A request/generation timestamp does not prove
all summaries share a financial snapshot. Either provide one defined snapshot
where cross-card consistency requires it or disclose independent as-of times.

Preference persistence has a real product tradeoff:

- Browser-local, versioned preferences are consistent with the existing
  appearance precedent and suitable for the experiment. Scope them to owner
  and book identity; allow reset and tolerate invalid/removed IDs. Explain that
  they do not follow the owner to another device or come from database restore.
- For a durable cross-device personal workspace, prefer a separate owner UI
  preference contract, with server persistence and backup/restore behavior
  designed intentionally. The existing settings preference schema only covers
  time zone; it does not already accept dashboard JSON. A small UI-preference
  migration is reasonable if this behavior is wanted. “No financial migrations”
  should not become a reason to choose the wrong persistence boundary.

Use one order across responsive layouts initially. Separate desktop/mobile
layouts, multiple instances of the same widget and per-workspace learning
would multiply state and migration cases without validating the core idea.

## Acceptance and validation

Keep financial correctness, navigation and usability acceptance explicit:

- Every Home value agrees with its canonical report for the same date, scope
  and currency. Test missing FX, missing prices, unknown basis, multiple
  currencies, empty data and known-only allocation with independent fixtures.
- A successful financial mutation refreshes the appropriate Home summaries;
  missing optional attribution remains incomplete without hiding known cash.
  Existing ADR 0017/0019 semantics govern; the redesign cannot invent stricter
  dependencies or silently relax complete-only reporting.
- A never-customized user can understand the headline, see required attention,
  discover more by scrolling and reach every shipped workflow through navigation.
  Ask the owner to explain what a displayed number includes; elegance alone
  does not establish comprehension.
- Pin, replace, remove, undo, Save/Cancel and reload behave deterministically.
  Invalid stored preferences recover to useful defaults without changing money.
  Keyboard and screen-reader order matches the visible order.
- Measure the actual Home request count and bounded query work. Opening Home
  should not execute offscreen returns/forecast charts or initiate online jobs.
  Establish timing/bundle baselines on representative hardware before accepting
  numeric performance budgets; inspect large-book behavior as well as empty data.
- Exercise 1280px desktop, 360/390px phones, a short phone viewport, 320 CSS-pixel
  reflow, zoom and long translations in both themes. No page-level horizontal
  overflow; no coverage notice hidden solely to fit the first screen.
- Preserve direct links, filters, deep selections, scroll restoration and new-tab
  behavior. Run actual transaction entry, import review, reconciliation preview
  and report drill-down through the migrated shell.
- For graduation, test day deduplication, bounded history, cooldown, suppression,
  full Focus capacity, logout and unavailable storage. Compare usefulness and
  dismissal with the manual baseline; repeated clicking is not success evidence.

Use existing unit/Playwright infrastructure rather than a new dashboard test
framework. Update `route-sweep.spec.ts`: it currently declares Overview as
`data: false`, since old Home only reads health. Otherwise the new Home's
data-error behavior could escape that sweep. Add customization subviews to
`subview-sweep.spec.ts`; extend URL, dialog-focus and contrast coverage where
behavior changes. Use named focused browser journeys before the relevant
shared frontend/backend/OpenAPI/build gates described in
[developer workflow](../developer-workflow.md).

For this review-only change, validate Markdown links and whitespace and inspect
the scoped diff. Runtime suites do not validate an unimplemented design, so
no runtime behavior or usability claim follows from those documentation checks.
