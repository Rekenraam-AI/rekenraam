# Unresolved non-sale investment results (#169)

Delivered 2026-10-13. Extends the [unknown-basis contract](investment-unknown-basis-plan.md)
under ADRs 0012 and 0013. Basis knowledge describes what can be calculated;
unknown never means zero. No jurisdiction's tax treatment is selected.

## Write-off

A worthless holding can be written off before its opening basis is sourced.
The journal removes the exact elected security quantity, with zero proceeds
and no cash leg. FIFO, LIFO and specific-lot allocations retain each lot's
knowledge; a decision touching unknown basis has NULL total basis and loss.
Independently known allocations remain numeric. Average cost consumes an
unknown pool without a definitive rate, exactly as an unresolved sale does.
Closing the quantity does not relabel historical basis or loss as known zero.

## Cash in lieu

A fractional settlement can consume unknown basis. Its pinned split, exchange,
merger or spin-off, exact quantity, disposal/payment dates and exact proceeds
remain known. Security and cash journals retain their existing dates. Lot
allocation, average pooling and decision knowledge follow the same rules as
write-off and sale. Entry and replacement previews expose `basis_knowledge`
and paired nullable disposed-basis and realized-result amounts. The settled
operation remains pinned and rechecked inside the writer.

## Return of capital

Receipt, entitlement and per-lot cash allocation remain exact. Each effect
stores `basis_knowledge`. With known eligible basis, reduction is the lesser
of allocation and eligible basis; the remaining allocation is the recorded
excess. With unknown basis, both reduction and excess amount pairs are NULL.
The lot's remaining basis stays unknown; the receipt is never substituted for
a reduction, and unknown excess is never recorded as zero or income. Its
quantity-zero `basis_reduction` lot event carries the same knowledge.

An explicit entitlement to an independently known lot stays calculable beside
unrelated unknown lots. `explicit_lots` takes the whole remaining quantity;
`explicit_quantities` caps eligible basis to the named units' proportional
share. Unentitled units cannot absorb excess. Allocation precision considers
known position amounts only for range admission; it does not derive a basis
rate from their subtotal.

An established operational average pool (the position's `average_cost` method
lock) containing any unknown lot has no definitive rate. Every entitled effect
is unknown, including an explicitly named known lot acquired after an earlier
pooling disposal. Applying that effect leaves its remaining basis NULL.
Unentitled lots are unchanged. Before pooling establishes a lock, immutable
lot bases remain independent; changing an account's default alone does not
redistribute those bases or reinterpret an already committed return.
Replay reconstructs the pool at the action's own dated slot. A later average
disposal still treats the whole unresolved pool as unknown.

Numeric excess on a known effect remains outside gains and income, as in the
existing return-of-capital contract. Its recorded inspection workflow remains
[#171](https://github.com/sergeyfarin/rekenraam/issues/171).

## Resolution, correction and trust boundaries

The sourced resolution supplies the original inbound's full basis in its cost
currency, including units already depleted. Dated replay recomputes write-offs,
cash in lieu and returns of capital, appending calculation revisions while
leaving original facts, NULL amounts, quantities and journals unchanged.
Downstream gains use the resulting reduced basis. Correction or withdrawal of
a resolution can return these action results to unknown; the existing refusal
for known outbound/onward transfer links remains in force.

Each family's native replacement/reversal retains its existing lineage,
root ordering and source guards. Preview runs the actual writer and rolls back.
Changed gains, including unknown/known transitions and removed decisions, bind
the shared gain acknowledgement. Checkpoint review guards every actual dated
balance change. An unknown result alone does not weaken a quantity, provenance,
reconciliation, range or late-failure guard.

## Persistence and readers

Forward migration `0003_unresolved_capital_return.sql` rebuilds the two leaf
effect tables, preserves existing known original/revised figures and adds
explicit knowledge with NULL-pair CHECKs. Original effect insertion must agree
with its lot event's knowledge. Downgrade refuses to discard unknown evidence;
operators use the documented backup/restore path rather than production schema
downgrades. Existing baseline migrations are unchanged.

Effective lot movements include each revised effect's knowledge. Self-check
conserves receipt allocations for every original and revision, even superseded
ones; it compares reduction/excess and event amounts only when known, and
validates unknown pairs and event knowledge independently. Replay equivalence,
quantity conservation, disposal snapshots and proceeds attribution remain
active. Unknown information is not damaged financial evidence.

Bundle schema 11 appends knowledge to original and revised capital-return
effect files, with blank unresolved reduction/excess amounts. Existing disposal
exports retain NULL knowledge through blank basis amounts. SQLite backup and
restore retain both original evidence and effective revisions. Six-locale
previews label unresolved results and keep exact cash/entitlement visible;
write-off gains use the existing six-locale gains display. The separate
write-off entry surface remains [#172](https://github.com/sergeyfarin/rekenraam/issues/172).

## Validation

Named regressions in `investment_unknown_non_sale_test.go` cover resolution
after closed write-off/cash-in-lieu quantities and a closed sale after return
of capital; mixed known/unknown lots; split settlement and split entitlement;
an explicitly known entitlement beside independent unknown lots and an
established unknown average pool; writer-preview parity; each native correction,
resolution withdrawal and reversal; missing/stale acknowledgements and forced
late rollback; checkpoint override and late invalidation rollback for each family;
original/revised export and restore; and superseded knowledge
damage in self-check. `TestUnknownNonSaleAPIKeepsExactFactsAndNullResults`
checks the real authenticated API, NULL fields, exact facts and resolution
transitions. Mobile browser coverage runs cash-in-lieu and return-of-capital
entry and accessibility in all six locales in
`investments-unknown-non-sale.spec.ts`. Populated known original/revised history survives migration 2→3 unchanged,
while downgrade and invalid NULL pairs refuse unknown evidence. Existing family
reconciliation suites remain part of the full backend race gate.
