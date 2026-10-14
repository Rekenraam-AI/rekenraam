# Two-process SQLite guard validation — 2026-10-14

Issue: [#105](https://github.com/sergeyfarin/rekenraam/issues/105).
Governing decisions: ADRs 0004, 0014, 0015 and 0016.

## Delivered coverage

`backend/cmd/rekenraam/two_process_test.go` re-executes the current Go test
binary, selecting only its subprocess helper. Each child has its own PID and
opens the shared fixture through production `db.Open`, preserving one writer
per pool, WAL/FULL, native SQLite and the gate's build tags. Race builds retain
Go instrumentation in the children. JSON pipes provide readiness and operation
barriers; both requests are sent before either result is received. Child
lifetimes are bounded at 30 seconds, with stdin closure, cancellation and
process reaping on cleanup. A health check confirms server readiness.

| Named test | Assertions |
|---|---|
| `TestTwoProcessWorkClaims` | Twelve separate pending items each have exactly one winning process, the returned item/owner/attempt agrees with durable state, and neither process steals a live lease. A held SQLite writer forces both children to return typed BUSY without a partial claim; after release, an explicit next operation claims the item once. At lease expiry, only one process reclaims the eligible item and advances its attempt count to two. |
| `TestTwoProcessScheduledBackupOccurrence` | Concurrent requests for one scheduled date yield one success and one `ErrBackupOccurrenceExists`, with exactly one run and one correctly linked work item. Both processes still report a duplicate after run and work completion; no pending orphan work appears. |
| `TestTwoProcessServerAndRestoreGuards` | A child runs the production `runServe` path. Another child's `runRestore` refuses with the holder's PID and leaves the database bytes intact, without a preservation directory. A second `runServe` also refuses. After the first server shuts down, restore succeeds, preserves the prior database and installs an integrity-checked database at the current schema version. |

The backup occurrence test drives the repository transaction that resolves
scheduler races, rather than relying on ticker timing. Ordinary fixtures use
the migrated test template; the restore fixture retains fresh migration and
online backup via the existing command-test helper.

## Validation

- Focused ordinary subprocess run passed.
- `./scripts/go-backend.sh test -race ./cmd/rekenraam -run '^TestTwoProcess' -count=3 -timeout=2m`
  passed all three repeats (25.510 seconds for the package).
- `./scripts/test-backend.sh` passed formatting, vet and the complete backend
  race suite with the normal two-package/five-minute gate. Command package:
  22.145 seconds; application: 141.341 seconds; database: 66.941 seconds.
- `git diff --check` passed.

Initial sandbox attempts lacked Go-cache writes or local socket permission;
the successful runs used the required access. No database durability, pool,
schema, API or application deployment policy changed.

Fault injection used isolated copies of the tracked backend and scripts plus
the new test file under `/tmp/rekenraam-105-faults`, leaving the working source
unchanged. Each defective variant compiled and failed behaviorally:

1. Replacing the backup occurrence UNIQUE constraint yielded two successful
   requests; `TestTwoProcessScheduledBackupOccurrence` failed its expected
   success/duplicate assertion.
2. Bypassing the restore lock check yielded a successful restore while the
   server ran; `TestTwoProcessServerAndRestoreGuards` failed exit-code,
   refusal-message, database-preservation and preservation-directory assertions.
3. Removing work eligibility from both claim SELECT and UPDATE allowed live
   work to be claimed again; `TestTwoProcessWorkClaims` failed item identity,
   attempt, durable-state and live-lease assertions.

## Limits and follow-up

The suite is Unix-only, matching the supported advisory-lock implementation.
The repository probes bypass the runtime lock solely to exercise independent
SQLite pools. Production still rejects a second server on the same database;
this delivery does not accept multi-instance deployment. The Go detector
continues to cover Go code, not the native SQLite C engine.

Removing only the claim UPDATE's status re-check still passed the subprocess
test in an isolated copy (0.602 seconds). Under the present deferred WAL
transaction, snapshot/write contention protects ownership too. These tests
therefore establish the observed combined claim behavior, not the necessity of
that individual predicate. The defensive predicate remains unchanged.

Found and logged [#227](https://github.com/sergeyfarin/rekenraam/issues/227):
restore currently releases its availability lock before replacing files, so a
server or another restore can enter during replacement. `Lock.Close` also
unlocks/closes before unlinking, permitting competing lock-file inodes during a
handoff. Those intervals require their own deterministic failing-first tests
and lock-lifetime/identity fix. The #105 test proves refusal while a server
already holds the lock; it does not prove these separate races safe.
