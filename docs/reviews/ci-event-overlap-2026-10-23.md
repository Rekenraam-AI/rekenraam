# CI branch-push / pull-request overlap — 2026-10-23

Delivery record for [#383](https://github.com/sergeyfarin/rekenraam/issues/383).
The policy itself lives in `docs/developer-workflow.md` (CI section) and is
pinned by `scripts/test-ci-events.py`.

## Before: event, ref and SHA evidence

`ci.yml` triggered on `push` to `"**"` and on every `pull_request`, with
concurrency keyed on `github.ref`. A branch ref (`refs/heads/…`) and its PR's
merge ref (`refs/pull/N/merge`) never shared a group, so neither cancelled the
other. Run metadata read from the Actions API:

| Run | Event | Branch | Head SHA | Context |
| --- | --- | --- | --- | --- |
| [39602471378](https://github.com/sergeyfarin/rekenraam/actions/runs/39602471378) | `push` | `dependabot/npm_and_yarn/svelte-check-4.7.7` | `11ba8354` | branch ref, open PR #353 |
| [39602477579](https://github.com/sergeyfarin/rekenraam/actions/runs/39602477579) | `pull_request` | same | `11ba8354` | PR #353 merge ref, 2 s later |
| [39024873645](https://github.com/sergeyfarin/rekenraam/actions/runs/39024873645) | `push` | `fix/262-mfa-settings-reauth-budget` | `83c94c7b` | branch ref, no PR yet |
| [39026832266](https://github.com/sergeyfarin/rekenraam/actions/runs/39026832266) | `pull_request` | same | `83c94c7b` | PR #265 merge ref, opened 31 min later |

The #353 pair is the duplicate: the same SHA, the same moment, one branch
context and one merge context. Only the merge context is needed — it is what a
pull request merges and what reports the required checks on the PR. The #265
pair shows the branch-without-PR case: the push run validated a branch that had
no PR yet, and the PR's merge-context run was still required afterwards.

Branch protection on `main` (inspected 2026-10-23): required status checks
`Backend Tests`, `Frontend Check`, `Integrated Build`, `strict: true`. The one
ruleset (`main: pull requests except sergeyfarin`) requires a pull request with
an owner bypass and adds no status checks. `pull_request` runs report on the PR
head SHA, so the required checks keep reporting without the branch-push run.

## Runner minutes, separate from wall time

Sum of job durations from the jobs API (all jobs ran in parallel, so removing a
run does not shorten any other run's wall time, and no suite gets faster):

| Run | Runner minutes | Workflow wall time |
| --- | --- | --- |
| 39602471378 (`push`, removed by this change) | 34m 23s | 18m 35s |
| 39602477579 (`pull_request`, kept) | 29m 37s | 15m 37s |
| 39024873645 (`push`, branch without PR, no longer automatic) | 18m 59s | 6m 37s |

Each avoided branch-push run saves one full run's runner minutes — about
19–34 runner-minutes at the sampled sizes, more now that browser smoke is a
two-job matrix. Wall time per validation context is unchanged.

## After: intended validation per context

| Context | Trigger | Concurrency group | Newer run cancels |
| --- | --- | --- | --- |
| Push to `main` | `push` (branches `[main]`) | `CI-push-refs/heads/main` | previous main push run |
| Pull request opened/updated | `pull_request` | `CI-pull_request-refs/pull/N/merge` | previous run of that PR |
| Manual run on any branch | `workflow_dispatch` | `CI-workflow_dispatch-refs/heads/<branch>` | previous dispatch on that branch |
| Push to a branch with an open PR | `pull_request` only | as above | — |
| Push to a branch without a PR | none | — | — |

Branch without a PR: nothing runs automatically. Open a pull request (draft is
fine) or run `gh workflow run ci.yml --ref <branch>`. Until v0.1.0 work is
committed straight to `main`, so in practice this affects Dependabot branches,
which always have a PR.

`scripts/test-ci-events.py` evaluates these cases against the committed workflow
(positive: main push, PR, dispatch; negative: Dependabot and fix branch pushes,
tag pushes) and asserts that the required job names exist, no job carries an
`if:`, and neither event has a path or branch-ignore filter. Against the
previous workflow it fails the negative-push and concurrency cases.

Live after-runs: recorded on the issue once observed.
