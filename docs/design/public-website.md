# Public project website

Owner-approved direction, 2026-10-28; related adoption work is
[#335](https://github.com/sergeyfarin/rekenraam/issues/335). This is the
website slice, not acceptance of #335's demo and migration walkthrough.

## Presentation and copy

Use the approved Paper & pine direction: warm neutral surfaces, forest-green
accents, system fonts, restrained spacing and actual application screenshots.
Support light and dark appearances and mobile browsing. Do not add slogans,
testimonials, activity counters or claims of financial correctness or security
that the project cannot establish.

Describe the concrete differences: self-hosted data storage, no required
external account for core bookkeeping, double-entry accounting with exact
decimals, and detailed investment records. Explain optional provider requests,
operator responsibilities and the single-user boundary. Keep the pre-release
notice beside the introduction and retain development/upgrade limits, the
AI-use disclosure and the translation-review status. Source capability claims
from `docs/implemented.md`; the README can lag behind recent delivery.

The retirement planner at `retirement.rekenraam.com` is a separate tool. Link
to it without implying that it shares a login, database or hosting lifecycle.

## Static distribution

`website/` is a separate, dependency-free static website in the app repository.
A Node build renders complete HTML from `website/locales/*.json`, with a shared
template and semantic CSS tokens. This small content site does not need a
second SvelteKit application or a running Go/API server. It does not change the
app's Go/SvelteKit/SQLite production shape or its Docker image.

The app's `frontend/project.inlang/settings.json` owns the locale set. Website
catalogs must match it in locales, keys, nonempty strings and placeholders.
English is `/`; Dutch, German, French, Spanish and Russian are `/nl/`, `/de/`,
`/fr/`, `/es/` and `/ru/`. All page copy, metadata, captions, alternative text,
notices and controls come from these catalogs. Language selection uses ordinary
links and works without JavaScript. Native review is still pending, as for the
app catalogs; this work does not sign off #102.

Load all website assets from the site's own origin. Do not add analytics,
tracking scripts, remote fonts, forms for financial data or app API calls.
The sole runtime script switches appearance, with an optional preference in
local browser storage. Hosting-provider access logs remain outside this claim.
Screenshots are English and explicitly labeled as fictional sample data.

The site has no interactive app demo yet. A future demo must distinguish
supported sample-book interactions from the real app and preserve the
provider-independent/privacy requirements in #335. Preparing this website does
not satisfy that issue or authorize a public launch.

## Hosting boundary

The initial target is GitHub Pages at `rekenraam.com`. Build and publication
are separate: normal CI checks/builds the site; the Website workflow defaults
to a review artifact without deployment. Its explicit `publish` input is for
an owner-authorized publication after Pages settings and DNS are configured.
Preserve the retirement subdomain's DNS and hosting. See
`docs/developer-workflow.md` → Public website for commands and setup.
