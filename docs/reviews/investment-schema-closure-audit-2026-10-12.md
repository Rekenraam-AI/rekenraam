# Investment schema issue closure audit (2026-10-12)

Reviewed the 2026-10-11 and 2026-10-12 commits through `d0032efb`, the
[#194](https://github.com/sergeyfarin/rekenraam/issues/194) delivery, and the
acceptance and closure comments of #188–#193. The scope is the bounded work
from the [investment schema design review](investment-schema-design-review-2026-10-11.md).
This is dated evidence, not a work-order document. GitHub owns acceptance and
state; the roadmap owns order.

## Findings

| Issue / commit | Evidence checked | Closure assessment |
| --- | --- | --- |
| [#188](https://github.com/sergeyfarin/rekenraam/issues/188) / `a3d37258` | Revised-cover quantity, unexplained-negative warning, withheld net-worth total, repeated revisions, correction/reversal and both side guards in `investment_short_named_quantity_test.go`; current short reader uses the #189 owner. | Complete. The activity-date guard's separate original-event read is justified: revisions change amounts, not dates. |
| [#189](https://github.com/sergeyfarin/rekenraam/issues/189) / `22711ffe` | Movement owner, three migrated consumers, named merger/pooled-transfer/spin-off cases, scanner allow-list and governing plan. | Partial. Reader consolidation ships, but promised ordering and provenance metadata are absent; reopen the original issue. |
| [#190](https://github.com/sergeyfarin/rekenraam/issues/190) / `6bfda276` | Three independent expectations in `investment_conservation_oracle_test.go`, five named multi-family sequences, shared self-check helpers and recorded mutation evidence. | Complete within its documented test-suite scope. The short sequence checks the oracle before replacing the cover, while its revision is still effective. |
| [#191](https://github.com/sergeyfarin/rekenraam/issues/191) / `b89a24e5` | Writer kind validation, matrix completeness checks, separate replay-intent axis, per-kind export contents and self-check scenarios. | Complete as a matrix delivery. The discovered cash-in-lieu test gap was explicitly filed as #194, as required. #194 now supplies its replay cell. |
| [#192](https://github.com/sergeyfarin/rekenraam/issues/192) / `d0032efb` | All 48 discovered coefficient columns have legality rules; installed CHECK expressions, zero/NULL exceptions, checksum pin, fresh/seeded equivalence and reset documentation. An extra embedded-NUL probe fails through the repo's SQLite driver. | Partial. Ordinary malformed text is refused, but `1\0abc` passes canonical representation checks; reopen the original issue. The unchanged candidate seed data remains valid. |
| [#193](https://github.com/sergeyfarin/rekenraam/issues/193) / `16ea7035` | Requirements name the supported replay commands and blocking refusals; replay-equivalence comment and capability ledger name long and short sides. | Complete. Historical ADR wording remains dated context, with subsequent refinements governing shipped replay. |

### Unfinished #189 contract

The issue's delivery scope explicitly requires **event date, correction-root
slot and effect sequence**, and provenance identifying the **operation,
decision or link, and revision**.

`EffectiveLotMovement` carries event date, operation/decision IDs, transaction,
movement family and an origin event. It has no correction-root slot, effect
sequence, transfer-link identity or exact revision identity. The implementation
sorts by date and movement-family rank, preserving each query's order within a
family. A source tag such as `transfer_link_revision` identifies the family,
not which link revision contributed the movement.

The current consumers fold dated totals; this audit found no incorrect total
caused by that ordering. It nevertheless leaves the original delivery contract
unfinished. `requireEffectiveMovementsConsistent` checks dates and filter
counts, not the missing metadata. This is an unmet original requirement, so
it belongs on reopened #189 rather than a new issue or a silently narrowed
acceptance. The justified activity-date guard exception is separate from this
gap and remains documented in the plan.

The capability ledger also omitted the #189–#192 deliveries, and its original
operation-foundation row still called named shorts future work. This audit
updates those entries and records the remaining limits in #189 and #192.

### Unfinished #192 malformed-text admission

An embedded NUL terminates SQLite's text `length` and `GLOB` interpretation.
The digit/length predicates therefore admit a coefficient such as the Go
string `"1\x00abc"`: its visible prefix is a valid positive digit, while the
stored string contains additional nonnumeric bytes. `exact.Coefficient` does
not write this value, but the point of #192 is to reject malformed storage
even when an application bug bypasses that boundary.

Reproduced against the installed checks through `modernc.org/sqlite`, using
the existing `TestExactNumberColumnChecks`: temporarily add
`"1\x00abc": false` to its malformed-value map and run
`go test ./internal/db -run '^TestExactNumberColumnChecks$' -count=1`.
The probe fails with `expected: false, actual: true` in signed, positive and
nonnegative columns, including `investment_disposal_allocations.quantity_value`.
The temporary probe was removed after reproduction; #194 changes no schema.

The original canonical-representation and malformed-text acceptance remains
unmet. #192 needs a durable regression covering embedded/trailing NULs and a
constraint that checks the whole stored value, while preserving legitimate
signed boundaries, zero and paired NULLs. Its existing pre-release baseline,
checksum and reset policy still applies. Passing the existing race gate does
not establish this missing case.

## CI evidence

Checked the GitHub Actions runs against the exact commit SHAs:

- #188: [CI passed](https://github.com/sergeyfarin/rekenraam/actions/runs/38167864722).
- #189: [its CI run was cancelled](https://github.com/sergeyfarin/rekenraam/actions/runs/38170322040),
  not failed; the descendant #193 run includes the change and passed.
- #193: [CI passed](https://github.com/sergeyfarin/rekenraam/actions/runs/38170782091).
- #190: [CI passed](https://github.com/sergeyfarin/rekenraam/actions/runs/38173259072).
- #191: [CI passed](https://github.com/sergeyfarin/rekenraam/actions/runs/38207308699).
- #192: [CI passed](https://github.com/sergeyfarin/rekenraam/actions/runs/38210673231).

All six closure notes include commit and local validation evidence and disclose
pending CI. The completed runs above resolve that pending status; cancellation
of #189's individual run is not the reason to reopen it.

## #194 validation

The new tests pin a 3-for-2 split of three shares bought for 30.00, followed by
a recorded FIFO sale of four, then entry of the earlier 0.5-share cash in lieu.
At the fixture currency's allocation scale of six, the fraction disposes
3.333333 and the later sale takes the remainder, 26.666667. Its original basis
26.666666 remains immutable evidence. The preview discloses the sale revision;
the unacknowledged commit leaves all snapshotted durable rows unchanged.

The full-sale case already disposed all 4.5 units. Both preview and commit
refuse the earlier fraction with that sale's operation ID and preserve the
durable snapshot. Both cases run the investment self-checks, movement contract
checks and independent conservation oracle.

Issue-scoped validation:
`go test ./internal/app/ -run 'CashInLieu|CapabilityMatrix|EveryOperationKind'`.
Full `./scripts/test-backend.sh` passed: formatting, `go vet` and serial
package race tests with the 20-minute per-package deadline (application
953.627 s; database 168.599 s). This includes fresh/seeded migration and
checksum checks. Documentation links resolve and `git diff --check` passes.
The embedded-NUL reproduction above is retained evidence of a separate
unfinished acceptance case, despite the existing suite passing.
