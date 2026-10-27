# Home and Navigation Design

Owner-approved design direction, 2026-10-27, for the new Home page and the
application shell around it. It records what was agreed after three mockup
rounds; it does not set delivery order. Order lives in the
[roadmap](../roadmap.md) and acceptance in GitHub issues, which still need to
be opened for the slices below.

Background and reasoning:

- [Design review](../reviews/focus-first-ui-review-2026-10-27.md): verified
  inventory of existing contracts, financial semantics, data-loading rules
  and acceptance checks. Those rules still apply.
- [Revised proposal](../reviews/focus-first-ui-proposal-revision-2026-10-27.md):
  options considered and the owner's first decisions. Where it differs from
  this document, this document wins.
- [Mockup](home-mockup/README.md): the interactive v3 canvas, with sample data.

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
  section opens a menu of its pages; a page that needs attention shows a
  short count in the menu (for example “Import · 1 in review”). A section is
  navigation only: it never changes account scope, currency or book.

  | Section | Pages |
  |---|---|
  | Ledger | Transactions, Accounts, Categories, Import, Reconcile |
  | Investments | Overview, Instruments, Returns, Income, Prices & FX |
  | Planning | Budgets, Recurring, Forecast |
  | Reports | Spending, Cashflow, Net worth, Export |

- **Right side:** hide amounts (eye), Customize (Home only), and a settings
  menu (Settings, Currencies, Backups and data, Language and appearance,
  Sign out).
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
      list with links; “All clear, nothing to review” when empty.

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

- Real buttons and links, menu and combobox semantics, visible focus, focus
  kept on a moved item with a live-region announcement, about 44px touch
  targets, reduced-motion support, both themes at WCAG AA contrast.
- One vertical page scroll; no nested card scrollers, sticky hero or
  horizontal carousels. Check 320px reflow and long translations in all six
  locales.

## Delivery slices (proposed)

Sequencing belongs in the roadmap. Proposed split for GitHub issues:

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
- Workspaces with separate layouts, bottom navigation on phones, usage-based
  automatic placement.
