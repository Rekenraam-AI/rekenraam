# Native localization review packet — 2026-10-13

[#102](https://github.com/sergeyfarin/rekenraam/issues/102) owns review
assignments, findings and acceptance. Catalog parity is technical evidence;
it does not establish that terminology is native, clear or safe to rely on.
No native approval is recorded by this delivery. The five non-English
catalogs and language-page notice remain explicitly drafts.

## Material and technical evidence

Read [the glossary](../localization-glossary.md) first. The current catalogs
contain 2,342 messages per locale: 663 app, 1,573 settings, 24 budgets and
82 pricing. This change drafts the original 36 `import_csv_*` and 29
MFA/security/investment-validation keys missing in each of es/fr/nl/de/ru;
it removes `categories_field_opened_on`. Review the complete catalogs as
well as these additions; old translations have no native sign-off either.

- [App catalogs](../../frontend/messages/app/): CSV column mapping, profiles,
  date/number layout, cancellation and retained import history.
- [Settings catalogs](../../frontend/messages/settings/): authenticator
  enrollment, recovery codes, password confirmation, sign-in challenge and
  positive investment quantities/amounts.
- [Budget catalogs](../../frontend/messages/budgets/) and
  [pricing catalogs](../../frontend/messages/pricing/): existing financial
  language is also subject to terminology review.

`pnpm --dir frontend run catalogs:check` checks every configured locale and
domain, using `project.inlang/settings.json` as its source. It rejects
key/placeholder drift, empty messages and shared-namespace collisions;
it cannot decide whether a translation is accurate. English fallback is
not translation evidence.

`catalog-localization.spec.ts` exercises real saved CSV mapping creation,
reuse, editing, delete cancellation/confirmation and real MFA enrollment,
one-use recovery sign-in and disabling at 390 px in all five languages.
It also verifies the draft notice. These rendering checks do not substitute
for native review.

## Reviewer brief

The owner must nominate a native reviewer for each of Spanish, French,
Dutch, German and Russian and record their identity or agreed attribution
on #102. No reviewer has been assigned by this packet, and no outreach
has been sent. A reviewer should understand personal banking and the
distinction between a transaction and its ledger postings.

Use disposable development data (`pnpm dev`) and select the language on
`/app/settings/language`. Inspect at 390 px and a desktop width, with
keyboard navigation, in light and dark themes. Keep names and financial
source data distinct from translated UI labels.

Walk `/app/import` with a small synthetic semicolon CSV containing date,
description and signed amount columns. Switch between signed amounts and
separate debit/credit columns; check the sign-inversion wording and column
vs number separators. Save/reopen/edit a mapping and cancel its deletion
before confirming. The wording must say that removing a reusable profile
preserves prior imports, and distinguish the reusable mapping from the
statement itself.

Walk `/app/settings/security`: enroll a disposable authenticator, read the
setup key/link instructions, activate, save the ten recovery codes, then
sign in with a recovery code. Check that one code is consumed. Review code
regeneration and disabling, and restore MFA to its original test state.
The wording must distinguish the setup secret, current authenticator code,
single-use recovery code and password. Recovery codes are displayed once;
security advice and `REKENRAAM_SECRET_KEY` must retain their exact meaning.

Check investment refusal copy: negative input must direct the owner to the
matching sale, write-off or correction, without implying that losses or
negative ledger balances are forbidden. Review the established pairs
transaction/posting and cleared/reconciled across their actual screens.
Prioritize Spanish *Punteado*, Russian *партия* for a tax lot, and Dutch
*Instelling* for an institution. Check German/Dutch long labels and Russian
1/2–4/5+ counts (including zero); counts use neutral phrasing rather than
assuming English plural rules.

## Recording review

Record findings on #102 with locale, catalog/key, current/proposed wording,
screen context and reason. Record an explicit reviewed revision, reviewer
attribution, scope and date for each locale. Fix confirmed findings,
rerun the parity and relevant browser gates, and obtain sign-off for the
updated wording. #102 remains open until its original native-review
acceptance is met; preparing this packet does not close that acceptance.
