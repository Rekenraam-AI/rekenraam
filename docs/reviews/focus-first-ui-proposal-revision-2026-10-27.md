# Focus-first Home: revised proposal

Written 2026-10-27 as a companion to the
[design review](focus-first-ui-review-2026-10-27.md). It responds to the
owner's original proposal, the owner's “potential problem / recommendation”
table, the supplied concept mockup (“Focus first, discover by scrolling”),
the owner's answers to the first revision's open questions and the owner's
Firefox New Tab reference.

Status: exploratory proposal with owner decisions recorded below. Not yet an
accepted product contract and not a change to roadmap order. No application
behavior changes. All figures in wireframes and card examples are
illustrative, not data. Endpoint references were checked by name against
`api/openapi`; card facts marked *composed* still need the Home read model.

The design review's verified inventory, financial semantics, data-loading
rules and acceptance checks still apply. This document changes the
interaction model on top of them and records where it deliberately differs.

## Owner decisions (2026-10-27)

| Question | Decision |
|---|---|
| Where Home preferences live | **Server-side**, per owner and book, so they follow the owner across devices and are included in backups. |
| Platform priority | **Desktop first.** The phone must stay fully usable, but layout and interaction trade-offs favor desktop. Phone navigation: header only plus the Go to palette. |
| Headline | Selectable: **net worth** (default), **one account**, or **a chosen set of accounts** (for example liquid accounts). |
| Explore order | Owner wants real reordering, as on the Microsoft Money home page; some people have different priorities. Resolved by the My Home / More to explore model below. |
| Go to palette | **In scope** for this initiative. |
| Add transaction | Required and always available, but **not a widget**; other buttons may be needed too. Follow the Firefox New Tab shortcut pattern. |

## The short version

1. **Two zones: My Home and More to explore.** My Home holds the widgets you
   chose, in any order you like, Microsoft Money style. More to explore,
   further down the same page, shows every other capability as a live
   compact preview with an *Add to Home* button. Removing a widget from My
   Home returns it to More to explore, so nothing ever disappears and
   discovery still works by scrolling. A new owner starts with a curated
   My Home, so the page is useful before any customization.
2. **Shortcuts are not widgets** (the Firefox New Tab lesson). A row of
   tiles under the headline, each with a ⋯ menu (Edit, Move, Remove) and a
   final “+” tile. Tiles are either actions (Transfer, Import file, Start
   reconciliation) or links to any page or saved view (an account register,
   a filtered spending report).
3. **Add transaction lives in the shell header**, as a primary
   “+ Add transaction” button with a ▾ menu for other create actions. It is
   on every page including Home, never removable, never a widget, and also
   reachable by keyboard and from the palette.
4. **Attention is system-owned.** One status line under the headline:
   “All clear · data through 25 Oct”, or “2 drafts to review · 3 prices
   missing ›”. It replaces the mockup's *Review* action and *Activity* card
   and cannot be hidden when it reports a failure.
5. **An honest headline.** Net worth, one account or a named set of
   accounts, always with as-of date and scope. Replace “+1.8% this month”
   with an absolute change: a percentage change in net worth mixes savings,
   contributions, market movement and FX, and is not a return.
6. **One Customize panel plus in-place controls.** A Customize side panel
   (Firefox's gear) toggles and configures whole sections; each widget and
   tile has its own ⋯ menu; desktop adds drag-and-drop with button and
   keyboard equivalents.
7. **“Go to…” command palette.** Turns the currently inert top-bar search
   into a palette over pages, commands, accounts, categories and
   instruments, sharing one capability registry with Home.
8. **Hide amounts.** A one-tap privacy toggle in the header.

## What Firefox New Tab teaches

Based on the owner's screenshot and Mozilla's New Tab help pages. Mozilla's
widget set (weather, clocks, timer, lists) is still being rolled out by
market, so details differ between versions.

| Firefox New Tab | Lesson for Rekenraam |
|---|---|
| The search bar is the single dominant control. | One dominant action per page. For Rekenraam that is Add transaction, in the header, not a tile among equals. |
| Shortcut tiles: icon, short label, ⋯ menu to pin, edit or remove; a “+” to add; a configurable number of rows. | Shortcuts are a separate primitive from widgets, with a small fixed vocabulary and a row limit set in Customize. |
| Weather sits in its own corner with its own ⋯ menu. | A glance slot beside the headline (Focus) works on desktop without stealing space from the main column. |
| The Widgets section has a heading, a collapse chevron, a section ⋯ menu and a “Show more widgets” button. | Sections are first-class: each has a heading, collapse state and a menu. Progressive disclosure is a button in the page, not a separate catalogue screen. |
| The Personalize panel switches whole sections on or off. | A Customize panel for section-level choices; per-item choices stay on the item. |
| Translucent cards over a photo wallpaper. | Not adopted. The brief and `docs/conventions.md` ask for calm, token-based surfaces; glass over imagery hurts contrast and number legibility. |

Sources: [Firefox New Tab page](https://support.mozilla.org/kb/new-tab),
[Customize items on Firefox New Tab](https://support.mozilla.org/en-US/kb/customize-items-on-firefox-new-tab-page/revision/282035),
[Mozilla widgets feedback thread](https://connect.mozilla.org/t5/discussions/feedback-on-new-tab-experimentation-with-widgets/m-p/121842).

## Mockup critique

| Mockup element | Issue | Change |
|---|---|---|
| Sliders icon, *Customize* quick action, *Customize* link | Three entry points; one consumes a quarter of the action row. | One Customize panel (header gear on Home) plus ⋯ menus on items. |
| *Add* among Transfer / Review / Customize | The most frequent action looks equal to a settings button. | Primary “+ Add transaction” in the header on every page. |
| “Explore all your finances ⌄” and “Scroll for investments … ⌄” | Two hints for one fact. | Let the next section visibly peek; keep one anchor link for keyboard and screen-reader users. |
| Search icon | The current top-bar search binds text and never searches. | The “Go to…” palette; on desktop a visible field, not just an icon. |
| Personal · Investments · Planning · Books | No active state; “Books” suggests switchable books in a single-book runtime. | Sections with a visible current state: Home · Ledger · Investments · Planning. |
| “€1,124,000 · +1.8% this month” in green | No as-of date, no completeness signal, a percentage that reads like a return. | “Net worth · 27 Oct” / exact amount / “+€19,880 since 1 Oct”, neutral colour, coverage notice when prices or FX are missing. |
| *Review* with a check mark; *Activity: 2 to review* | Attention spread over an action and a widget; a check mark reads as “done”. | The status line. |
| *Spending · 68% of budget* | 68% on day 27 of 31 is under pace, but nothing says so; multi-currency budgets cannot be one bar. | Pace marker and “on pace / ahead of pace”, per currency. |
| Pin icon on each card | Always-visible, small target, noisy. | One ⋯ menu per card, about 44px target. |
| Centred hero with wide empty margins | Wasted on a 1280px screen. | Desktop: headline left, Focus right, as Firefox's weather corner. |

Kept: the quiet header, one dominant number, restrained cards, and a page
that continues downward.

## Page anatomy

Home only. Other pages keep their task-first layouts, as the design review
describes.

1. **Shell header** (every page): identity, sections, “Go to…”, hide
   amounts, “+ Add transaction ▾”, settings. On Home also the Customize
   gear. One header in total: it replaces today's root top bar plus app
   header, not a third bar on top of them.
2. **Glance**: headline, as-of date, change, coverage notice; Focus slot
   (0–2 compact widgets) to its right on desktop.
3. **Status line**: always present; “All clear” is useful information too.
4. **Shortcuts**: tile row, one or two rows.
5. **My Home**: chosen widgets, user order.
6. **More to explore**: every other shipped capability, by category.

### My Home and More to explore

| Zone | Contents | Order | Size | Data cost |
|---|---|---|---|---|
| Focus | 0–2 compact widgets beside the headline | User | Compact only | Home read |
| My Home | Any number of chosen widgets | User: drag, ⋯ Move, keyboard | Standard or wide where the widget supports it | Home read; heavy detail lazy |
| More to explore | Every capability not in My Home | Fixed by category | Compact preview only | Cheap fact in the Home read |

Verbs: **Add to Home**, **Remove from Home**, **Show beside headline**
(Focus). A widget is in exactly one zone, so there is no duplicate card and
no “pinned copy” to reason about.

This replaces the first revision's fixed Map plus Shortlist. The owner's
point stands: people prioritize differently, so the part of the page you
actually use must be fully orderable. Discovery is still protected, because
More to explore is always the complete remainder, in a stable order, on the
same page.

Defaults for a never-customized owner: headline net worth; Focus empty;
My Home = Spending, Budgets (or Recurring when no budgets exist), Accounts,
Net worth trend if the headline is not net worth; everything else in More to
explore. More to explore starts expanded and can be collapsed; the choice is
remembered.

Desktop grid: three columns at about 1280px in reading order; a *wide*
widget spans two. No masonry, free placement or arbitrary resizing. Phones
are one column in the same order.

### Shortcuts

- Tiles, Firefox style: icon, label of up to two lines, ⋯ menu (Edit label,
  Move left/right, Remove), and a final “+ Add shortcut” tile.
- **Action** tiles start a supported workflow: Transfer, Import file,
  Reconcile an account, Record an investment trade. Only commands the app
  can open by URL or shared dialog are offered.
- **Link** tiles open a page or saved view: an account register, a report
  with its period and filters, an instrument. Pages get an “Add to Home
  shortcuts” item in their own menu, so the owner can pin the view they are
  looking at, like pinning a site in Firefox.
- Labels can be renamed; the destination stays visible in the edit dialog.
- Customize sets one or two rows (desktop up to 8 per row). On phones the
  first four show, plus “More”.
- Defaults: Transfer, Import, Transactions, Reports.

Add transaction is deliberately not a tile: it is the header's primary
button, so its position never depends on configuration. The existing
`transaction-editor.svelte` is already reused by the transactions workspace
and the recurring template editor; opening it as a shared dialog from the
shell is the work this needs, together with a URL entry that survives
reload and back navigation.

### Explore card contract: one question, one fact

Every compact preview has the same frame: capability title, one live fact,
a scope/as-of line, a state, a ⋯ menu and an *Open* link to the canonical
page. In My Home a widget may show its standard view; in More to explore
only the compact fact, with *Add to Home*. Charts load only in a standard
or expanded view, and when near the viewport. Fixed row heights prevent
late data from moving the scroll position.

Illustrative facts (wording and values are examples, not output):

| Category | Card | Fact | Source |
|---|---|---|---|
| Everyday | Spending | “€3,720 this month · top: Groceries €612” | `/reports/spending` |
| Everyday | Budgets | “€1,580 left of €5,300 · on pace” (per currency) | `/budgets/month` |
| Everyday | Cashflow | “+€2,140 net this month · liquid accounts” | `/reports/cashflow` |
| Everyday | Accounts | Three largest balances, “+3 more” | `/ledger/account-balances` |
| Wealth | Net worth | Trend; shown when it is not the headline | `/reports/net-worth` |
| Wealth | Holdings | “12 positions · largest 34% of known value” | `/investments/positions` |
| Wealth | Returns | YTD time-weighted return, loaded near viewport | `/investments/returns` (lazy) |
| Wealth | Allocation | Top three asset types, known-only labelled | returns response (lazy) |
| Wealth | Income | “3 dividends not yet assigned” | `/investments/income-coverage?limit=0` |
| Planning | Recurring | “Next: Rent €1,450 on 1 Nov” | recurring summary / due reads |
| Planning | Forecast | “Liquid cash on 31 Dec: €18,400 (estimate)” | `/forecasts/balances` (lazy) |
| Ledger & data | Imports | “Last import 2 days ago · 1 batch in review” | *composed* |
| Ledger & data | Reconcile | “Checking last reconciled 12 Sep” | *composed* |
| Ledger & data | Prices & FX | “All prices current” / “3 instruments missing prices” | pricing workspace, *composed* |
| Ledger & data | Backups | “Last backup 3 days ago” | `/maintenance/backups`, *composed* |

About fifteen capabilities, not thirty. Variants (spending by payee, gains
by method) are widget settings or live on the canonical page.

Card states, per the design review: loading, ready, zero, no data,
**not set up**, partial, stale, failed. *Not set up* doubles as onboarding:
“No budgets yet — Set up budgets”. Unbuilt capabilities (goals, insights,
available-to-spend) do not appear on Home.

### Status line

- Sources, by severity: integrity or self-check failures; failed price/FX
  refresh or backup; items awaiting review (recurring drafts, import
  batches, unassigned dividends); informational staleness.
- Collapsed: up to two items plus “+N more”. Activating it discloses the
  full list inline, each item linking to its queue. Home never commits,
  repairs or dismisses financial records itself.
- “All clear · data through 25 Oct” names what the date means (latest
  posted transaction or latest import).
- Failures cannot be hidden; informational types can be muted in Customize.
- Elapsed time since reconciliation is not a failure (design review).

### Headline

- **Net worth** (default), from the net worth report.
- **One account**: its balance in its own currency, with account name.
- **Account set**: a named set of accounts the owner picks (for example
  “Liquid accounts”), summed under the same complete-only conversion rules
  as reports. The sub-label always shows “N accounts” and the set is
  editable from the headline's ⋯ menu. The cashflow report's liquid-cash
  scope is offered as a ready-made set.

Every choice shows as-of date and scope. When a converted total is
unavailable, the headline says so and links to the known breakdown; it
never shows a partial sum as the number. Change line: absolute, since the
start of the month.

Later idea, specific to Rekenraam: **“What changed?”** — the month's change
split into net savings, market movement and currency effect. It needs a
defined backend decomposition that reconciles exactly to both net worth
figures.

## Customization

- **Customize panel** (gear on Home; side panel on desktop, sheet on phone):
  headline choice; shortcut rows; Focus on/off; My Home density; More to
  explore expanded/collapsed; muted informational status types; Reset Home.
  Section-level only, like Firefox's Personalize panel.
- **Item ⋯ menus**: Add to / Remove from Home; Show beside headline;
  Move up/down (Move left/right for tiles); Size (standard/wide, where
  supported); widget settings such as period or accounts; Open.
- **Desktop drag-and-drop** in My Home and Shortcuts, as a supplement.
  Buttons and keyboard work everywhere; nothing depends on drag or hover.
  Reuse the reorder-mode lessons from #185/#221: arrows stay
  `aria-disabled` rather than `disabled` while a move saves, so keyboard
  focus stays on the moved item, and a live region announces the result.
- Item-level changes save immediately with Undo. Panel changes apply live
  and persist on close.
- **Focus full**: “Replace which?” with Cancel; never silent eviction.
- Financial metrics keep canonical names; only shortcuts and account sets
  can be renamed.

Usage-informed suggestions stay deferred, as in the design review.

### Persistence

- New per-owner, per-book Home layout resource under `/api/v1/settings`,
  stored in its own table by migration. It is not financial data and
  stays separate from the time-zone row in `user_preferences`.
- One versioned JSON document: headline choice, account set, shortcuts,
  Focus, My Home order and sizes, section states. Read once with the Home
  data; written whole with a revision number so two open tabs cannot
  silently overwrite each other.
- The server validates shape and limits. Unknown widget IDs, archived
  accounts or removed routes are dropped or marked unavailable on read,
  never a load failure; Home falls back to defaults if the document is
  unusable.
- Included in backups and restore because it lives in the database.

## “Go to…” command palette

- Pages and commands from the same capability registry as Home, with
  translated titles plus per-locale synonyms (“budget”, “begroting”).
- Accounts, categories and instruments from list reads the app already
  caches; selecting one opens its register or detail page.
- Desktop: a visible field in the header, ⌘K / Ctrl K and `/`. Phone: the
  search icon. Accessible dialog with combobox and listbox.
- Commands include “Add transaction”, so the palette and the header button
  share one entry point.
- Transaction full-text search is later backend work, not a prerequisite.

## Navigation

Sections **Home · Ledger · Investments · Planning** in the header, with
every existing route reachable from a grouped “All pages” menu and from the
palette. A section never changes account scope, currency or book. On phones
the sections collapse into the header menu; no bottom bar.

## Layouts

Desktop, about 1280 × 900:

```text
Rekenraam  Home Ledger Investments Planning  [Go to… ⌘K]  ◐  [+ Add transaction ▾]  ⚙

Net worth · 27 Oct 2026                       ┌ Spending this month     ⋯ ┐
€1,124,000                                    │ €3,720 of €5,300 · on pace │
+€19,880 since 1 Oct                          └────────────────────────────┘
2 drafts to review · 3 prices missing ›       ┌ Next recurring          ⋯ ┐
                                              │ Rent €1,450 · 1 Nov        │
                                              └────────────────────────────┘
[⇄ Transfer] [↓ Import] [≡ Transactions] [▤ Reports] [▦ Checking] [ + ]

My Home                                                          ⋯
┌ Budgets ───────┐ ┌ Accounts ──────┐ ┌ Spending by category ──────────────┐
│                │ │                │ │ (wide)                             │
└────────────────┘ └────────────────┘ └────────────────────────────────────┘

More to explore ⌄       Everyday · Wealth · Planning · Ledger & data
Wealth
┌ Holdings  + ┐ ┌ Returns  + ┐ ┌ Allocation  + ┐ ┌ Income  + ┐
…
```

Phone, 375 × 812 (CSS pixels, excluding browser chrome):

```text
Rekenraam          🔍  ◐  [+]  ☰
Net worth · 27 Oct
€1,124,000
+€19,880 since 1 Oct
[2 to review · 3 prices missing  ›]
[⇄ Transfer] [↓ Import] [≡ Trans.] [⋯ More]
My Home
┌ Spending this month          ⋯ ┐
┌ Budgets                      ⋯ ┐   ← peeks above the fold
```

On phones Focus widgets render as the first rows of the page. Content grows
downward at large text sizes; nothing is cropped. The header's “+” is the
same Add transaction button, icon-only with an accessible name.

## Delivery slices

0. **Decide and record.** Accept this model in the product requirements or
   a design document; open ready GitHub issues per slice.
1. **Home and Explore, no personalization.** Capability registry; one
   composed `/api/v1` Home read (headline, status line, one fact per cheap
   capability); new Home with Glance, Status, default shortcuts, default My
   Home and More to explore; header Add transaction dialog. Replaces the
   foundation-era overview.
2. **Personalization.** Layout migration and resource; headline choices
   including account sets; Add to/Remove from Home; reorder; Focus;
   shortcuts editing; Customize panel; reset; hide amounts.
3. **Go to palette** on the same registry, replacing the inert search
   field; “Add to Home shortcuts” from pages.
4. **Shell migration.** One header, sections, All pages.
5. **Depth.** Wide and expanded views for returns, allocation and forecast;
   “What changed?”; “Since your last visit” summary (counts of new and
   changed records since a stored timestamp, not a transaction feed).

## Still open

| Question | Recommendation |
|---|---|
| Should More to explore be removable entirely, as Firefox allows for widgets? | Collapsible, not removable: it is the discovery guarantee. |
| Should Focus exist separately from the top of My Home? | Yes on desktop, where the corner beside the headline is otherwise empty; on phones they merge visually. |
