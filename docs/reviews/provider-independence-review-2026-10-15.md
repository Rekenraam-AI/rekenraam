# Provider Independence Review — 2026-10-15

The owner clarified that absent Trading 212 or other provider data must not hold
back app features. Manual correction and deliberately incomplete attribution
are valid outcomes. The accepted rule is
[ADR 0017](../adrs/0017-provider-independent-features-and-incomplete-data.md),
with requirements in `product-requirements.md`; this review records the audit,
not a second work queue or a claim of new runtime delivery.

## Findings

| Area | Current evidence | Consequence |
| --- | --- | --- |
| Core investment lifecycle | Native manual entry/correction and dated replay ship independently of a connection; #173–#175 cover shorts, #146/#196 splits. | Keep developing and validating domain features with deterministic local evidence. |
| Missing basis, prices or FX | Unknown basis and sourced resolution ship; manual observation entry and report-specific null/reason coverage ship. | Reuse these patterns. Missing provider evidence does not justify zero basis or a fabricated return. |
| CSV income | #241 maps exact existing instruments, independent gross/tax and signed cash; review uses native writers. | An owner-supplied file is a valid input route without an online provider. Missing required money evidence stays in review. |
| Trading 212 dividend fallback | `commitTrading212InvestmentRow` resolves a payer; `commitTrading212Dividend` falls back when a local income default is missing. #242 reproduces the loss of that known link. | This is an app review/repair gap, not an upstream-data prerequisite. Account choices should be supplied in app review; partial attribution must be explicit. |
| Dividend economics | The local Trading 212 dividend DTO carries one `Amount`; its writer passes that amount to the gross-dividend input and carries no separate withholding value. | The existing adapter shape cannot establish a general rule that an absent tax field means zero or that cash is gross. Require verified semantics for automation; offer owner declarations or explicit incomplete classification. No claim about a newer external API is made here. |
| Unsupported import events | `import_trading212_split.go` provides manual split linking with atomic identity and no second journal. Import UI links only split-like fills. | General manual recovery is narrower than native command coverage. Extend linking in bounded slices rather than wait for provider inference. |
| Provider suggestions | #111 was labelled blocked on R15, although the durable ingestion, BYO-key and normalized event/suggestion foundations already exist. | R15 placement is sequencing, not a hard technical dependency. Audit the chosen adapter when that slice starts; incomplete events can be informational. |
| Split/short/cancellation mappings | #145/#176/#136 wait for verified wire semantics. | Retain the automation evidence gate. It cannot block manual features, correction, unrelated providers or reports. |
| Return analytics | R18 retains exact original-currency data, manual prices, missing coverage and immutable saved reports; #119 is next. | R13's core scope must not wait for Trading 212 history, broker comparisons or a benchmark feed. Decide each calculation's minimum inputs and display unavailable values honestly. |

## Product And Delivery Disposition

Missing optional attribution may stay incomplete indefinitely. Missing required
financial effects keep only the affected command/row unposted until the owner
supplies them or links a correct manual operation. Avoid silent generic cash
fallbacks that pretend an unsupported investment operation completed: known
cash may be retained through an explicitly partial workflow without claiming
complete gross income, tax or instrument attribution.

A provider mapping's lack of live evidence is a reason to defer its automation,
not a reason to defer the manual domain workflow. A local missing default,
unclear incomplete-state display or unavailable repair path is app-owned work.
Keep those gaps actionable without waiting on an API change. GitHub issues own
the bounded follow-ups; `roadmap.md` alone owns their order.

This change records policy, reviews issue dependencies and updates planning.
It does not implement provider fallback repair, a new completeness read model,
or new import-to-operation links. Existing runtime limits remain in
`implemented.md` and the linked issues.

## Bounded Follow-ups

- #242: app-owned dividend defaults/review, retention and audited repair of a
  known payer; no new upstream fields required.
- #244: visible dividend/withholding cash attribution and economics coverage,
  with original-currency unknown/unassigned figures and manual guidance.
- #245: explicit links from unresolved rows to already recorded manual short
  sale/cover operations, following the split-link precedent without guessing
  provider semantics or posting twice.
- #111: removed the R15-order blocker; implement a bounded verified producer
  when scheduled, retaining incomplete events as informational.
- #136/#145/#176: keep evidence gates confined to automatic mappings. Existing
  manual correction, splits and shorts remain usable and independently complete.
- #119: manual/file data and manual prices/FX can support core returns;
  provider comparisons and benchmark adapters do not gate the feature.

## Review Basis

Read active product/convention/architecture sources, ADRs 0008/0012/0013,
connection/import/reporting plans, open provider issues and the current Go/Svelte
paths named above. This is a code and dependency review, not a refresh of API
terms, entitlements or undocumented payload semantics. Those still require
primary-source verification when implementing an adapter.

## Validation Of This Review Change

Checked 121 relative document links across the 13 changed documentation/skill
files and ran `git diff --check`. Reviewed the new rules against exact-money,
ledger/subledger, source-identity, correction, reconciliation and report-coverage
contracts. No application code, API, schema or generated artifacts change, so no
backend/frontend runtime test rerun is required for this documentation slice.
