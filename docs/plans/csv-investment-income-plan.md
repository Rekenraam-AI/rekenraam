# CSV dividend and withholding mapping (#241)

Governed by product requirements, the import plan and ADRs 0011–0013.

CSV profiles opt into an `investment` mapping; profiles without it keep their
bank-cash contract. A mapping chooses dividend, separate withholding, or an
explicit row-type column with two distinct owner-chosen source values. Unknown
values stay unresolved. The instrument column contains the exact existing
commodity code, never a guessed ticker or a description. One bulk instrument
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
This slice adds no provider adapter, corporate-action importer, automatic tax
classification, guessed gross amounts or currency conversion.
