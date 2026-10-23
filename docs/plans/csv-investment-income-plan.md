# CSV dividend and withholding mapping (#241)

Governed by product requirements, the import plan and ADRs 0011–0013/0019.

CSV profiles opt into an `investment` mapping; profiles without it keep their
bank-cash contract. A mapping chooses dividend, separate withholding, or an
explicit row-type column with two distinct owner-chosen source values. Unknown
values stay unresolved. The optional instrument column contains an exact existing
commodity code, never a guessed ticker or a description. Missing columns or blank
cells mean unassigned payer (#393); a supplied nonempty unresolved code still
requires review rather than being silently discarded. One bulk instrument
read resolves codes for staging; the owner may choose another existing instrument
in review. No instrument is created implicitly.

The existing amount/debit/credit columns always mean signed **cash movement**.
Dividend mappings additionally require an explicit gross column, with an optional
withholding column (omission means zero withholding). Separate tax rows require
negative cash and post its positive magnitude as withholding, without gross
income. All amount columns share the file's selected decimal separator; automatic
selection considers them together. Dividends require positive gross, nonnegative
withholding, and exact scale-aware gross minus tax equal to cash. An inconsistent
or unreadable source must be reviewed, never posted through the bank fallback.

Profiles may save cash, income and tax account defaults. Staging snapshots these
choices; row review can change cash/currency, income, tax, instrument, kind, gross
and withholding. Raw and normalized source evidence stay immutable; reviewed
choices live in the existing resolution JSON. Bank categorization rules do not
reinterpret investment rows. Transfers and tags are not supported on this slice's
investment rows and cannot be silently discarded at commit.

Preview and commit use the shared named dividend/tax application writer and its
account-role, date, precision and reconciliation checks. Invalid mappings refuse
before any row is committed, leaving the batch reviewable. A reconciliation
refusal writes no journal or operation facts. Each accepted row's journal,
operation facts, dedupe identity and committed row marker share one transaction.
Identity is scoped to the CSV profile and source reference (or complete normalized
investment evidence plus occurrence when no reference exists); changed evidence
under an already committed identity remains duplicate and uses native correction.

Instrument activity, correction/reversal and bundle export reuse #229's facts.
The #393 refinement stores payment shape independently of nullable payer; native
payer correction preserves on omission, clears on explicit null and assigns on
a value. Unassigned standalone tax is known tax without gross income, not an
unknown-economics cash fallback. Raw/normalized source and commit identity stay
unchanged through native corrections and reimport. Bundle schema 17 retains
withholding-only shape with an empty payer field. See the
[delivery evidence](../reviews/dividend-attribution-repair-2026-10-23.md).
This slice adds no provider adapter, corporate-action importer, automatic tax
classification, guessed gross amounts or currency conversion.
