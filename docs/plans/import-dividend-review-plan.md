# Imported dividend review and repair (#242)

Governed by product requirements and ADRs 0011–0013 and 0017. This slice repairs
Trading 212's known-payer cash fallback; #244 adds wider completeness reporting ([coverage contract](dividend-income-coverage-plan.md)).

New Trading 212 dividend rows never enter the ordinary cash fallback. A reviewer
can supply cash/currency/income/tax accounts and choose an existing payer; absent
choices resolve existing connection and dated dividend defaults. Preserve the
existing source-identity instrument resolution/creation path at commit only.
Known payers cannot disappear because an income default is absent.

The staged amount is signed source cash. An owner must explicitly declare gross
and withholding (including known zero), with exact gross minus tax equal to cash.
The UI may offer a deliberate "cash is gross; no withholding" declaration; it
must not silently initialize it from an absent tax field. Raw/normalized evidence
stays immutable; account/economic declarations use reviewed resolution and source
component evidence. No new upstream fields or successful refresh are required.
Missing required declarations/accounts keep that row pending for later review;
other valid rows can commit and the batch remains partially committed. A source
without a resolvable payer may be recorded as a named unassigned dividend when
its owner-confirmed financial effects are complete; it is not assigned a guessed
instrument. Existing domain/account/reconciliation guards still apply.

A paginated composed review read exposes pending rows and committed historical
ordinary-cash fallbacks, with source identity, current defaults, and current
transaction/version for repair. The dedicated phone review workflow is reachable
from import preview/results and can be opened independently after an old import.
It loads catalogs once, never once per row. Owner-triggered row review has the
same named dividend validation and a read-only reconciliation preview.

Historical repair appends a new version of the **same transaction**, classified
as an investment dividend, plus its named operation, payer/components and audited
source link in one database transaction. Require a current posted ordinary source
with the committed import identity, same date/cash account/currency and exact net
cash as the source payment. Preserve owner approval, descriptions, notes, payee,
tags and source evidence. Match existing posting line identities where possible so unchanged
reconciliation state and ordering survive. Gross/tax or account reallocations
use the normal per-posting reconciliation guard, with explicit override where
needed. The old journal version stays evidence; no second payment is posted.

The existing immutable import identity and original transaction effect remain.
Append an operation-only effect for the newly classified operation, whose primary
journal is the new version of that same transaction. Reimport still deduplicates;
native audited dividend replacement/reversal follows its committed source. Pin
transaction version and source/row eligibility inside the writer; stale review,
discard, duplicate ownership and late failure roll back classification, audit,
operation facts, version and identity effects together. Future owner correction
is never overwritten by provider refresh. No backfill or guessed payer migration.

Reuse existing tables and exported version/fact/effect shapes; no schema migration
is needed. Validation covers review without defaults, unknown economics staying
pending beside valid cash rows, new-instrument cleanup/retry, historical same-ID
repair, exact cash and metadata preservation, stale/duplicate/discard and injected
late rollback, reconciliation refusal/override, native correction/export, reimport,
real API boundaries and the 390px review/repair journey.

The open repair form captures `expected_transaction_version_id` alongside its
editable resolution (#433). Background review-list refetch preserves both;
it cannot authorize stale fields with a newer transaction version. Concurrent
edits use the existing stale-version refusal and explicit reload recovery.
