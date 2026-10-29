# Home and Navigation Design

Owner-approved design direction, 2026-10-27, for the new Home page and the
application shell around it. It records what was agreed after three mockup
rounds; it does not set delivery order. Order lives in the
[roadmap](../roadmap.md) and acceptance in GitHub issues: tracking #441,
slices #436–#440, object navigation #442 and the Payees page #443. The
navigation decisions of 2026-10-29 (#435) are recorded in
[Navigation beyond the header](#navigation-beyond-the-header); their options
and reasoning are in the
[navigation and pages review](../reviews/navigation-and-pages-options-2026-10-29.md).

Background and reasoning:

- [Design review](../reviews/focus-first-ui-review-2026-10-27.md): verified
  inventory of existing contracts, financial semantics, data-loading rules
  and acceptance checks. Those rules still apply.
- [Revised proposal](../reviews/focus-first-ui-proposal-revision-2026-10-27.md):
  options considered and the owner's first decisions. Where it differs from
  this document, this document wins.
- [Mockup](home-mockup/README.md): the interactive v3 canvas, with sample data.
- [Navigation prototype](navigation-prototype/README.md): browser-openable
  implementation-preparation companion (2026-10-29), with registers, report
  drill-down, import review and reconciliation. Synthetic data only; it does
  not deliver the shell, Home or object-navigation slices.

## Principles

- Home answers three questions: where do I stand, does anything need me,
  and what do I do next. Breadth comes from scrolling; direct access comes
  from search and navigation.
- Calm first screen: two lines of text, a search field and a row of
  shortcuts, with generous empty space. Widgets start below.
- Desktop is the priority. Phones (360–390px) must stay fully usable.
- Every figure states its scope and completeness. A missing price or rate
  makes a total unavailable, never silently partial.
- Customization never changes financial data.

## Application shell (whole app)

The left sidebar and the separate dark top bar are replaced by one light
header on every page.

- **Surface:** the header uses the page background, separated by space or a
  hairline, not a coloured band. Colour is reserved for content and the
  primary shortcut.
- **Sections:** Home · Ledger · Investments · Planning · Reports. Each
  section opens a menu of its pages, built as a disclosure (button and list
  of links, opened by click or Enter, not hover); a page that needs attention shows a
  short count in the menu (for example “Import · 1 in review”). A section is
  navigation only: it never changes account scope, currency or book.

  | Section | Pages |
  |---|---|
  | Ledger | Transactions, Accounts, Categories, Payees, Import, Reconcile |
  | Investments | Overview, Instruments, Returns, Income, Gains, Prices |
  | Planning | Budgets, Recurring, Forecast |
  | Reports | Spending, Cashflow, Net worth, Gains, Export |

  The Ledger menu also shows up to six pinned accounts with balances and
  *All accounts*. Gains is one page under Investments, also listed in
  Reports.

- **Right side:** hide amounts (eye), Customize (Home only), and a settings
  menu (Settings, Security, Backups and data, Language and appearance,
  Trash, Sign out) with an inline Light / Dark / System theme choice.
- **Search:** on every page except Home the header has a compact search
  field with the same behaviour as Home's command bar (below). On Home the
  large central bar replaces it, so there is never more than one search
  field. Keyboard: `/` and Ctrl/⌘ K focus it.
- **Phone:** sections collapse into a menu button; search becomes an icon.
  No bottom navigation bar.
- Every existing route stays reachable. The shell keeps the current
  layout's responsibilities: setup and session gates, owner-local date
  context, shared recurring summary, logout and page titles.

## Home layout

Top to bottom, centred in a narrow column (about 720px) above a full-width
widget area:

1. **Briefing**: two lines, same type size, regular weight with amounts in
   medium weight.
   1. Headline: net worth (default), one account, or a named set of
      accounts, with absolute change since the start of the month. ▾ switches
      the headline.
   2. To review: “2 drafts and 1 import to review”, expanding in place to a
      list with links; “All clear, nothing to review” only after a successful
      check with no actionable items. An unchecked or failed source has its
      own visible state; informational notes remain expandable when there
      are no actionable items.

   Below them, one small muted line states what the number covers: “All 14
   accounts in EUR · 3 prices from 24 Oct · data through 25 Oct”. It belongs
   to the number, so it is not a widget and not a briefing line.
2. **Command bar**: large, centred search and command field.
3. **Shortcuts**: a centred row of tiles.
4. **My Home**: the owner's widgets, full width.
5. **More to explore**: every other widget, full width, by category.

### What belongs in the briefing

A briefing line is one sentence, chosen by the app, and either a position
or something to act on. Anything needing a chart, a list, a period or
settings is a widget. Notable moves (a holding's value change, a budget
overrun) were tried as a third line and rejected: they belong in the
Holdings and Budgets widgets. Notes such as stale prices or unassigned
dividends appear only inside the expanded review list; failures (backup,
self-check, price refresh errors) always count as items to review.

## Command bar

One component for Home and the compact header field.

- Empty: suggested commands and pages.
- Typing matches, in groups:
  1. **Payee**, when the text matches one: *Add expense at Albert Heijn*
     (opens the transaction editor prefilled with the payee and its
     last-used category) and *Transactions with Albert Heijn* (Transactions
     page filtered by payee).
  2. **Pages, accounts, categories, instruments and commands.**
  3. **Everything**: *Search all transactions for “albert”*, covering
     payees, descriptions, references and notes.
- Quick entry never posts. Any typed amount or date only prefills the
  editor, which the owner confirms; parsing must respect locale decimal
  separators and dates.

Existing support: the `transaction_search` FTS5 index (payee, description,
external reference, notes), the transactions API `q` and `payee_id`
parameters with matching Transactions URL filters, and MiniSearch-based
payee matching in the frontend. New work: the shared editor dialog with
prefill, a last-used-category read per payee, and the result grouping.
Searching account, category or instrument notes is later work.

## Shortcuts

- Tiles with an icon and a label of up to two lines, Firefox New Tab style.
- **Add transaction** is the first tile by default, in the accent colour.
  It can be moved but not removed. On other pages it is reachable through
  search (“add”), the keyboard shortcut N and the Transactions page.
- Other tiles are actions (Transfer, Import file, Reconcile an account) or
  links to a page or saved view (an account register, a filtered report).
  Defaults: Add transaction, Transfer, Import file, Transactions, Spending by
  category, Checking.
- Each tile has a ⋯ menu (Move left/right, Rename, Remove); a final
  “+ Add shortcut” tile lists the remaining options. Phones show the first
  three tiles plus More.

## Widgets

- A widget is in exactly one place: **My Home** or **More to explore**.
  Adding moves it up; removing returns it to More to explore, so nothing
  disappears and every feature stays discoverable by scrolling.
- More to explore shows widgets at **real size**, exactly as they would look
  on My Home, with a dashed outline and *Add to Home*. The section can be
  collapsed, not removed. Categories: Everyday, Wealth & investments,
  Planning, Ledger & data.
- My Home is fully reorderable. Sizes: standard, or wide (two columns) where
  a widget supports it. Grid: three columns at desktop width in reading
  order, two on tablets, one on phones. No masonry or free placement.
- Each widget has a ⋯ menu: Move up/down, size, Settings… (period, accounts
  and similar where relevant), Remove from My Home, Open the full page. In
  More to explore: Add to My Home, Add to top, Settings…, Open.
- Default My Home: Spending this month, Budgets, Accounts, Recurring, Net
  worth trend (wide).
- States: loading, ready, zero, no data, not set up (doubles as onboarding,
  for example “Set up backups”), partial, stale, failed. Estimates (forecast)
  are labelled; market-value changes are never called returns.
- Unbuilt capabilities (goals, insights, available-to-spend) do not appear.

The capability inventory, which widget uses which existing contract, and the
composed Home read model are specified in the design review.

## Customization

- One Customize panel (sliders icon on Home): headline choice including an
  account or a named account set; whether review notes are mentioned;
  shortcut list with reorder and remove; My Home order with reorder and
  remove; More to explore expanded or collapsed; Reset Home.
- Item-level changes from ⋯ menus save immediately and offer Undo.
- Reordering works with buttons and keyboard everywhere; drag-and-drop is a
  desktop supplement. Nothing depends on hover.

## Persistence

Server-side, per owner and book, in its own table by migration (not the
`user_preferences` time-zone row): one versioned JSON document with headline
choice and account set, shortcuts, My Home order and sizes, widget settings
and section state. It is written whole with a revision number so two tabs
cannot silently overwrite each other, validated on the server, and tolerant
of unknown widgets, archived accounts or removed routes, falling back to
defaults. Because it lives in the database it is included in backups.

## Accessibility and responsiveness

- Real buttons and links, disclosure navigation for section menus and
  combobox semantics for search and the switcher, visible focus, focus
  kept on a moved item with a live-region announcement, about 44px touch
  targets, reduced-motion support, both themes at WCAG AA contrast.
- One vertical page scroll; no nested card scrollers, sticky hero or
  horizontal carousels. Check 320px reflow and long translations in all six
  locales.

## Navigation beyond the header

Accepted 2026-10-29 (#435). Options and the path map with estimated entry
counts are in the
[navigation and pages review](../reviews/navigation-and-pages-options-2026-10-29.md).

### Three levels

| Level | Examples | Tool |
|---|---|---|
| Section | Ledger, Investments, Planning, Reports, settings | Header menus |
| Page | Transactions, Accounts, Returns, Budgets, Spending | A tab row of the section's fixed pages, under the header |
| Object | Accounts, categories, instruments, payees, import batches | List page, scoped search (switcher), pins, breadcrumbs. Never tabs. |

Views and filters inside a page (Due / Templates, budget month, spending by
category or payee, active / closed) stay in the page as segmented controls,
steppers or filters. On phones the tab row scrolls horizontally.

### Pages and routes

- Every tab is a real route (`/app/reports/spending`,
  `/app/investments/instruments`, `/app/investments/gains`…); old `?view=` and
  `?tab=` links redirect.
- One typed route registry (route, section, title key, icon, search keywords,
  attention source, width) drives menus, tabs, document titles, breadcrumbs
  and the search's page results. A unit test fails when a `+page.svelte`
  route is missing from it.
- Placement of today's sub-views: dividend review is a view inside Import;
  split history is a section on the instrument page; investment suggestions
  are review items (menu count and Home's review list), reached from
  Overview; ledger CSV and QIF export move to *Reports → Export*, while
  *Settings → Data* keeps backup and restore and links there; FX rates stay
  in *Settings → Currencies*. Settings keeps its own side list.
- Menu counts and Home's review line come from one attention read, replacing
  the shell's recurring-summary query.

### Page header

Breadcrumb on detail pages, the title or the object switcher, a scope line
where the page has one, and a slot on the right for the page's main action.
The shell's description paragraph is dropped; help text moves into empty
states. Transactions, account registers and category pages put *Add* in the
action slot, prefilled for that page, so entry works on phones without a
keyboard shortcut. Widths: narrow (about 720px) for Home's top and forms,
standard (about 1200px) for reports, budgets and investments, fluid (up to
about 1600px) for registers, transactions and import review. `?` opens
keyboard help.

### Objects

- **Switcher = scoped search.** On a detail page the object's name is a
  button (*Checking ▾*) that opens the shared search with a scope chip
  (*Accounts ×*). Scoped, it lists one kind, grouped (accounts by class and
  institution with balances, categories as a tree, instruments held first),
  and Enter reopens the current page for the chosen object, keeping period,
  filters, view and sort where they apply; a filter that cannot apply is
  dropped and shown in the scope line. Removing the chip makes it the global
  search, whose results open an object's default page. The global search's
  empty state offers the same chips. Reconcile uses it to pick the account.
- **Pins.** Up to six accounts, chosen by the owner, shown in the Ledger menu
  with balances. On first use the app offers once to pin the three to five
  accounts with the most postings in the last 90 days; afterwards only the
  owner changes the list. The switcher may show a labelled *Recent* group
  below the pins.
- **Accounts page** is the complete index, grouped by class and institution,
  with a filter and active / closed / all. Holdings collapse under their
  broker with a count.
- **Holdings** are left out of the account switcher and pins by default and
  reached through Instruments and their broker; typing still finds them.
- **Closed and archived** objects are hidden by default and one control away.
- **Breadcrumbs** follow the hierarchy (*Accounts › DEGIRO › VWRL*,
  *Categories › Housing › Energy*). A holding's register and its instrument
  page link to each other.
- **Payees** get a *Ledger → Payees* page (#443).
- Before #442 fixes grouping and the pin limit, count a real book (accounts
  by kind, holdings per broker, closed accounts, categories by depth, payees,
  postings per account over 90 days) and record the result in the review.

### Other shell details

- Hide amounts is applied in `$lib/money/format.ts`, covers chart labels,
  leaves exports, edit fields and stored data unchanged, and is remembered
  per browser.
- Sign-in, setup and recovery lose the root top bar and get a minimal header
  with name, language and theme.
- Back to Home restores its scroll position and expanded state.

## Delivery slices (proposed)

Sequencing belongs in the roadmap. Slice issues: #436, #437, #438, #439, #440 in
the order below.

1. **Shell and search:** light header, section menus, settings menu, hide
   amounts, compact search over pages, accounts, categories, instruments and
   commands; retire the sidebar and the duplicate top bar.
2. **Home:** briefing, central command bar, shortcuts with the shared Add
   transaction dialog, default My Home and More to explore, composed Home
   read; replaces the foundation-era overview.
3. **Personalization:** layout migration and API, headline choices, Customize
   panel, ⋯ actions with Undo, widget sizes and settings.
4. **Search depth:** payee actions with prefilled editor and last-used
   category, payee filter, full-text transaction search.
5. **Widget depth:** lazy expanded charts for returns, allocation and
   forecast; further widgets.

## Considered and dropped

- A slot for widgets beside the headline (Focus): added noise; widgets live
  in My Home.
- A third briefing line for notable moves or spending pace.
- *Add transaction* as a header button: it is the first shortcut instead.
- The dark green top bar and the left sidebar.
- A fixed-order map with a separate shortlist: replaced by My Home plus More
  to explore.
- Compact previews in More to explore: real-size widgets instead.
- Hover-opened section menus and `role="menu"` for site navigation.
- A separate object picker beside the search: the switcher is the search
  with a scope chip.
- Tabs for objects such as accounts or instruments.
- Usage-ordered pins that rearrange themselves.
- Workspaces with separate layouts, bottom navigation on phones, usage-based
  automatic placement.
