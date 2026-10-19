# Shared-book sub-view regressions — 2026-10-19

Delivery for [#344](https://github.com/sergeyfarin/rekenraam/issues/344),
following the [architecture audit](architecture-quality-audit-2026-10-19.md).
Baseline: `7e1ffa6b466eb90401e0806d54156c5fb56dc2bd`.

## Causes and repairs

- The sweep read only the first 200 shared investment transactions and grouped
  them by date. Its own older transactions disappeared from that page; repeated
  reference books could also supply another fixture's same-date operation.
  `seedReferenceBook` now returns the transaction IDs from its own write
  responses (including the direct dividend response and wrapped trade, split,
  transfer and capital-return responses). The sweep uses those IDs for both
  ordinary and investment openers, retaining the shared-worker execution model.
- Import preview's bulk-assignment selects sized their flex containers from
  the longest account option. On a failed worker book their 385 px width
  overflowed a 316 px container, making the page 32 px too wide at 390 px.
  Both select and flex item now respect the available width. The SVG paths in
  the former diagnostic were incidental, rather than the source of the width.
- The desktop lot card split its width equally between labels and values.
  A 24-place crypto quantity overflowed the value cell and ultimately the
  384 px panel by 2 px. The card now allows wrapping, retaining every digit.

The sweep owns a precise crypto position instead of opening the first shared
position, and seeds a long account option explicitly. Its existing named rows
cover `lot panel`, `staged CSV preview with payee resolution`, `staged CSV
preview with a rejected row`, `failed-row review`, `provider preview: short
link` and `provider preview: split link`. No fit threshold or assertion was
weakened. The financial reference register and production financial rules are
unchanged.

## Before/after evidence

Copies of the failed full-run worker books at `25a24f4e` reproduce the original
32 px import overflow and the lot panel's 2 px overflow. With the new explicit
fixtures and the unchanged baseline binary, the named five import states fail
by 63 px at `de 390`, and the owned precise lot fails by 2 px at `ru desktop`.
This makes both defects reproducible without relying on a different spec's
choice of fixture data.

After rebuilding, the same saved book is checked in those configurations with
all correction openers, unknown-basis resolution, investment detail and the six
geometry rows selected. Result: **2 passed (2.4 minutes), 21 reachable views
per configuration, no failed axe/fit/close cells**.

Shared frontend validation passes (zero diagnostics, 691 unit cases, 66 catalog
cases), backend race validation passes, and the structural audit against
`ee23e455` passes. Integrated production build passes. The complete browser gate
and remote `E2E Smoke` remain pending; #344 stays open until both pass.

## Limits

These are synthetic, stubbed-provider browser checks. The nested-dialog
screenshot contrast override remains independently unsound under
[#348](https://github.com/sergeyfarin/rekenraam/issues/348); fixing fixture
ownership and geometry does not establish the affected text-contrast evidence.
No new independent issue or additional audit was identified in this bounded fix.
