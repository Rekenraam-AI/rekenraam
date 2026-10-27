# Focus-first Home: revised proposal

Written 2026-10-27 as a companion to the
[design review](focus-first-ui-review-2026-10-27.md). It responds to the
owner's original proposal, the owner's “potential problem / recommendation”
table and the supplied concept mockup (“Focus first, discover by scrolling”).

Status: exploratory proposal for owner decision. Not an accepted product
contract and not a change to roadmap order. No application behavior changes.
All figures in wireframes and card examples are illustrative, not data.
Endpoint references were checked by name against `api/openapi/paths`; card
facts marked *composed* still need the Home read model described below.

The design review's verified inventory, financial semantics, data-loading
rules and acceptance checks still apply. This document changes the
interaction model on top of them and records where it deliberately differs.

## The short version

1. **Home is glance + status + map, not a widget dashboard.** The lower part
   is a live map of what Rekenraam can do: one card per shipped capability,
   each answering one question with one live fact. Charts appear only when a
   card is expanded. That keeps “show everything” cheap and calm.
2. **Three tiers, two verbs.** *Focus* (at most two, glance-sized, beneath the
   headline), *Shortlist* (your pinned cards, in your order, at the top of
   Explore) and the *Map* (every capability, fixed order). The verbs are
   “Add to Focus” and “Add to Shortlist”. Only Focus and Shortlist can be
   reordered; the Map never moves.
3. **Attention is system-owned.** One status line under the headline:
   “All clear · data through 25 Oct”, or “2 drafts to review · 3 prices
   missing ›”. It replaces the mockup's *Review* action and *Activity* card,
   is never a pinnable widget and cannot be hidden when it reports a failure.
4. **An honest headline change.** Replace “+1.8% this month” with an absolute
   change and an as-of date. A percentage change in net worth mixes savings,
   contributions, market movement and FX; it is not a return.
5. **One customization entry point.** “Edit Home” switches the Focus area into
   an in-place edit state. The mockup has three (sliders icon, *Customize*
   quick action, *Customize* link); a quick action slot is too valuable to
   spend on configuration.
6. **New: a “Go to…” command palette.** Turn the currently inert top-bar
   search into a palette over pages, commands, accounts, categories and
   instruments. Scrolling teaches what exists; the palette gets you there.
   Both read from the same capability registry.
7. **Sections, not workspaces.** Personal / Investments / Planning / Books are
   navigation sections over one data set. Calling them workspaces promises
   separate layouts that the first slices should not build.
8. **New: hide amounts.** A one-tap privacy toggle for a page whose purpose is
   to show large balances on a phone.

## Mockup critique

| Mockup element | Issue | Change |
|---|---|---|
| Sliders icon, *Customize* quick action, *Customize* link | Three entry points to one function; one consumes a quarter of the action row. | Single “Edit Home” text button after the Focus area. Card menus offer “Add to Focus”. |
| “Explore all your finances ⌄” and “Scroll for investments … ⌄” | Two hints for one fact. Arrows are a crutch when content can peek. | Let the first Explore card visibly peek above the fold. Keep one real anchor link, “Explore ↓”, for keyboard and screen-reader users. |
| Search icon | The current top-bar search binds text and never searches. | Make it the “Go to…” palette, or remove it until it works. |
| Personal · Investments · Planning · Books | No active state; “Books” suggests switchable books in a single-book runtime. | Sections with a visible current state. Use “Ledger” as in the design review. |
| “€1,124,000 · +1.8% this month” in green | No as-of date, no completeness signal, and a percentage that reads like a return. | “Net worth · 27 Oct” / exact amount / “+€19,880 since 1 Oct”. Neutral colour for change; coverage notice when prices or FX are missing. |
| *Review* with a check mark | A check mark reads as “done”; the action carries no count and is permanently present. | Moves into the status line, which shows counts or “All clear”. |
| *Activity: 2 to review · 1 import · 1 recurring* | This is attention, not activity. “1 import” can hide 40 rows. | Status line, with batch and row counts from the composed read. |
| *Spending €3,720 · 68% of budget* bar | 68% on day 27 of 31 is under pace, but nothing says so. Multi-currency budgets cannot be one bar. | Show a pace marker and “on pace / ahead of pace”, per currency, from `/budgets/month`. |
| Pin icon on each card | Always-visible, small target, adds noise to every card. | One ⋯ menu per card with a target of about 44px. |
| Centred hero with wide empty margins | Calm on a phone, wasted on a 1280px screen. | Phone: stacked. Desktop: headline left, Focus favourites right. |

Kept from the mockup: the quiet header, one dominant number, a short action
row, restrained cards, and the idea that the page continues downward.

## Page anatomy

Home only. Other pages keep their task-first layouts, as the design review
describes.

1. **Shell header.** Identity, sections, Go to…, hide amounts, settings.
   One header in total: this replaces the current root top bar plus app
   header, not a third bar on top of them.
2. **Glance.** Headline metric, as-of date, change, coverage notice.
3. **Status line.** Always present; “All clear” is useful information too.
4. **Shortcuts.** Up to four; the first is the primary *Add transaction*.
5. **Focus.** Zero to two compact favourites.
6. **Explore.** Shortlist (if any), then the Map by category.

### The three tiers

| Tier | Purpose | Size | Order | Data cost |
|---|---|---|---|---|
| Focus | What I check every time | 0–2 | User-ordered | Part of the Home read |
| Shortlist | What I use often | Small, suggested ceiling of 6 | User-ordered | Part of the Home read |
| Map | Everything the app can do | Every shipped capability | Fixed, by category | Cheap fact in the Home read; detail lazy |

This answers “promote versus reorder” without a second move vocabulary.
Promoting is always an *Add to …* action. Reordering exists only inside the
two small personal tiers, so explicit ↑/↓ buttons stay practical on a phone.
The Map's fixed order keeps it predictable, makes every feature findable in
the same place on every device, and removes a whole class of preference state.
A card that is in Focus or the Shortlist remains on the Map, marked as such.

This deliberately differs from the design review's “Move earlier / later in
Explore within its category”, and from the owner's table where all pinned
widgets go to the top of discovery: that table's idea is kept as the
Shortlist, while the Map stays stable.

### Explore card contract: one question, one fact

Every Map card has the same frame: capability title, one live fact, a
scope/as-of line, a state, a ⋯ menu and an *Open* link to the canonical
page. *Show more* expands the card in normal document flow and only then
loads a chart or table. Rows use a fixed card height so late data does not
shift scroll position.

Illustrative facts (wording and values are examples, not output):

| Category | Card | Fact | Source |
|---|---|---|---|
| Everyday | Spending | “€3,720 this month · top: Groceries €612” | `/reports/spending` |
| Everyday | Budgets | “€1,580 left of €5,300 · on pace” (per currency) | `/budgets/month` |
| Everyday | Cashflow | “+€2,140 net this month · liquid accounts” | `/reports/cashflow` |
| Everyday | Accounts | Three largest balances, “+3 more” | `/ledger/account-balances` |
| Wealth | Net worth | Shown when it is not the headline | `/reports/net-worth` |
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

That is about fifteen cards, not thirty. Variants (spending by payee,
gains by method) live behind *Show more* or on the canonical page.

Card states, per the design review: loading, ready, zero, no data,
**not set up**, partial, stale, failed. *Not set up* is a feature, not an
empty state: “No budgets yet — Set up budgets” turns the Map into onboarding
for a new book. Unbuilt capabilities (goals, insights, available-to-spend)
do not appear on Home.

### Status line

- Sources, by severity: integrity or self-check failures; failed price/FX
  refresh or backup; items awaiting review (recurring drafts, import batches,
  unassigned dividends); then informational staleness.
- Collapsed, it shows up to two items plus “+N more”. Tapping discloses the
  full list inline, each item linking to its queue. Home never commits,
  repairs or dismisses financial records itself.
- “All clear · data through 25 Oct” names what the date means (latest
  posted transaction or latest import) rather than claiming the books are
  “up to date”.
- Failures cannot be hidden. Informational items can be muted per type.
- Elapsed time since reconciliation is not a failure (design review).

### Headline

Choices: Net worth (default), Liquid cash (the cashflow report's named liquid
scope), Spending this month, Portfolio value, None. Every choice shows its
as-of date and scope. When the converted total is unavailable under
complete-only rules, the headline says so and links to the known breakdown;
it never shows a partial sum as the number.

Later idea, high value and specific to Rekenraam: **“What changed?”** under
the headline — the month's net worth change split into net savings, market
movement and currency effect. It needs a defined backend decomposition and
must reconcile exactly to the two net worth figures; until then, show only
the absolute change.

## Customization

- **Edit Home** puts the Focus area into an in-place edit state: headline
  picker; shortcuts with remove, ↑/↓ and *Add* (a grouped sheet of
  supported actions and pages); favourites with remove, ↑/↓ and replace.
  Done and Cancel; Cancel restores the previous layout. Editing in place is
  its own preview, so there is no separate settings page to learn.
- **Card ⋯ menu:** Add to / Remove from Focus; Add to / Remove from
  Shortlist; Move up / down (Focus and Shortlist only); Open; card settings
  where they exist (period, accounts). Changes from the menu save at once
  and offer Undo.
- **Focus full:** “Replace which favourite?” with Cancel. Never silent
  eviction or a third slot.
- **Rename:** shortcuts only. Financial metrics keep their canonical names.
- **Reset Home** restores defaults. No drag-and-drop in the first slice;
  buttons work for touch, keyboard and screen readers alike, and focus
  returns to the moved item's control with a live-region announcement.
- Defaults for a never-customized owner: Net worth headline; Add
  transaction, Transfer, Import, Accounts; Focus = Spending + Budgets when
  budgets exist, otherwise Spending + Recurring; empty Shortlist.

Usage-informed suggestions (“You open this often — add to Focus?”) stay
deferred, as in the design review.

## “Go to…” command palette

Why it belongs in this redesign:

- The app has some 20 routes and will gain more. Shortcuts and sections
  cannot scale with that; a palette can, without adding visible chrome.
- It fixes an existing defect: the top-bar search field does nothing.
- It complements scroll discovery. Explore answers “what can it do?”; the
  palette answers “take me to X now”.

Scope for a first version:

- **Pages and commands** from the same capability registry as Explore, with
  translated titles plus per-locale synonyms (“budget”, “begroting”).
- **Accounts, categories, instruments** from list reads the app already
  caches; selecting one opens its register or detail page.
- Opens with ⌘K / Ctrl K or `/` on desktop, the search icon on phones.
  Implemented as an accessible dialog with a combobox and listbox.
- Transaction full-text search is a later backend feature, not a palette
  prerequisite.

## Navigation

Sections: **Home · Ledger · Investments · Planning**, with every existing
route reachable from a grouped “All pages” menu and from the palette. A
section is navigation only; it never changes account scope, currency or
book.

The phone decision is genuinely open (see below). The two candidates:

- **Header only:** compact header with section menu, Go to… and settings.
  No fixed chrome; fits the brief's “avoid sticky controls”. Section
  changes take an extra tap at the top of the screen.
- **Bottom bar:** Home · Ledger · Investments · More, four destinations.
  One-handed and familiar from banking apps, but permanent height and long
  labels in German and Russian (“Transaktionen”, “Инвестиции”). *Add
  transaction* stays a Home shortcut, never a destination slot.

Recommendation: header only for the first slices, with the palette covering
reach; revisit if the phone becomes the daily entry device. Do not ship
both.

## Layouts

Desktop, about 1280 × 900:

```text
Rekenraam   Home  Ledger  Investments  Planning      [Go to…  ⌘K]  ◐  ⚙

Net worth · 27 Oct 2026                ┌ Spending this month      ⋯ ┐
€1,124,000                             │ €3,720 of €5,300 · on pace  │
+€19,880 since 1 Oct                   │ ▓▓▓▓▓▓▓▓▓░░│░░             │
                                       └─────────────────────────────┘
2 drafts to review · 3 prices missing ›┌ Next recurring           ⋯ ┐
                                       │ Rent €1,450 · 1 Nov         │
[+ Add transaction] [Transfer] [Import] [Accounts]          Edit Home

Explore   Shortlist · Everyday · Wealth · Planning · Ledger & data
Everyday
┌ Spending ┐ ┌ Budgets ┐ ┌ Cashflow ┐ ┌ Accounts ┐
…
```

Phone, 375 × 812 (CSS pixels, excluding browser chrome):

```text
Rekenraam                 🔍  ◐  ☰      56
Net worth · 27 Oct
€1,124,000
+€19,880 since 1 Oct                   112
[2 to review · 3 prices missing  ›]     48
[+ Add] [⇄ Transfer] [↓ Import] [▦ Accts] 88
┌ Spending this month            ⋯ ┐
│ €3,720 of €5,300 · on pace      │   80
└─────────────────────────────────┘
┌ Next recurring                 ⋯ ┐   80
Edit Home              Explore ↓       48
Everyday                               …
┌ Budgets …   ← peeks above the fold
```

The numbers are a design budget (about 510px before Explore), not fixed
heights. At 360 × 640 Explore starts below the fold and the anchor still
works; at large text sizes content grows downward and nothing is cropped.
Shortcuts are a 4-column row with labels wrapping to two lines; with long
translations they fall back to 2 × 2. Explore is a single column on phones,
two columns only for cards whose fact fits.

## Delivery slices (replaces the design review's slice order)

0. **Decide** the open questions below; record the accepted model in the
   product requirements or a design document and open ready GitHub issues.
1. **Map first.** Capability registry, one composed `/api/v1` Home read
   (headline, status line, one fact per cheap capability), new Home with
   Glance, Status, default shortcuts and the fixed Map. No personalization.
   This alone delivers the discovery goal and replaces the foundation-era
   overview.
2. **Personalization.** Focus, Shortlist, Edit Home, card menus, persistence,
   reset, hide amounts.
3. **Go to… palette** on the same registry, replacing the inert search field.
4. **Shell migration.** One header, sections, All pages, and the phone
   navigation decision.
5. **Depth.** Lazy expanded charts for returns, allocation and forecast;
   “What changed?” decomposition; “Since your last visit” summary (counts of
   new and changed records since a stored timestamp, not a transaction feed).

Slice 1 is the cheapest test of the core idea: if a fixed, well-written Map
already makes Home useful, personalization can stay small.

## Open decisions for the owner

| Question | Options | Recommendation |
|---|---|---|
| Where should Home preferences live? | Browser-local (no migration, per device) or server-side owner UI preferences (follows you, included in backups; small non-financial migration). | Server-side if you use Rekenraam on both phone and desktop. |
| Phone navigation | Header only, or bottom bar. | Header only plus palette, revisit after use. |
| Default headline | Net worth, or liquid cash. | Net worth, with an honest coverage state. |
| Explore order | Fixed Map plus reorderable Shortlist, or fully reorderable Map. | Fixed Map plus Shortlist. |
| Palette in scope | Include in this initiative, or later. | Include as slice 3; it fixes the inert search. |
