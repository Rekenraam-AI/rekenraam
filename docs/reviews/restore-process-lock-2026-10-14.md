# Restore process-lock ownership — 2026-10-14

Issue: [#227](https://github.com/sergeyfarin/rekenraam/issues/227), found during
[#105](https://github.com/sergeyfarin/rekenraam/issues/105).
Governing contract: ADR 0004 and the restore section of
`docs/plans/data-portability-plan.md`.

## Change

`runRestore` now acquires the same database lock as server startup and retains
it through backup verification, checkpointing, preservation, replacement and
command completion. Deferred cleanup releases it on every outcome; a release
error produces a nonzero exit and a specific diagnostic. A competing command
reports that the database is in use, without assuming the holder is a server
rather than another restore. Existing recovery output still distinguishes a
refusal before replacement, a failed install after preservation, and a failure
after the restored file is already installed.

`lockfile.Lock.Close` releases ownership without unlinking the file. A process
that opened the lock file before release therefore still locks the same inode
as the next contender. The persistent file is private (`0600`); its existence
or saved PID is diagnostic metadata, not ownership. `CheckAvailable` remains
an instantaneous probe and documents why restore must retain `Acquire` instead.
The database restore function also documents its caller's ownership obligation.

This retains the existing stopped-app restore, one server per database,
WAL/FULL, backup verification, WAL-set preservation and atomic durable install.
No schema, financial facts, migrations or API contract changed.

## Regression evidence

The Unix command subprocess harness retains production native SQLite settings
and Go race instrumentation. JSON pipe barriers define the interleavings;
30-second child deadlines and cleanup bound process lifetimes.

- `TestTwoProcessRestoreOwnsLockThroughoutReplacement` pauses at entry to the
  restore dependency and again after actual replacement but before command
  return. In both cases, a separate process's actual server and restore
  commands refuse, name the restoring PID and leave the database bytes and
  preservation directories unchanged. The original restore completes and
  verifies its database; server startup then succeeds.
- `TestTwoProcessLockHandoffsKeepOneFileIdentity` repeats eight handoffs. A
  child opens the original lock file before the owner releases it, then takes
  the real OS flock. A second child's production `Acquire` must refuse and
  the pathname must still identify the original inode. After the waiter
  unlocks, the next process acquires and releases normally. The pre-opened
  file models the interval between `Acquire`'s `OpenFile` and `flock` without
  modifying production locking or relying on scheduler timing.
- `TestRestoreCommandReleasesLockOnFailure` covers an unusable backup,
  source==destination, a newer schema, cancellation, a real install failure
  after preservation and an injected failure after a real successful install.
  Every restore dependency invocation verifies lock ownership. Every error
  releases ownership and retains the correct data location, intact preserved
  database and operator recovery instructions.

The first two tests were added before the fixes, with only the restore
dependency seam in place. They compiled and failed behaviorally (4.795 seconds):
startup and a second restore entered the paused interval, changed the database
or preservation set, and every handoff allowed two processes to own different
inodes under the same pathname. After the fixes, the focused restore/subprocess
suite passed (5.332 seconds).

Independent defect variants in `/tmp/rekenraam-227-faults` confirmed each
regression against the final tests: releasing the acquired lock just before
calling the restore dependency failed both paused-restore phases; putting
unlink back after unlock/close failed all eight lock-file handoffs. Both
variants compiled; the working source was unchanged by those checks.

## Validation

- Three race repeats of the new ownership/handoff/failure tests passed:
  `./scripts/go-backend.sh test -race ./cmd/rekenraam -run 'TestTwoProcess(RestoreOwns|LockHandoffs)|TestRestoreCommandReleasesLockOnFailure' -count=3 -timeout=3m`
  (96.500 seconds for the package).
- `./scripts/test-backend.sh` passed formatting, vet and all backend race
  tests. Command package: 50.438 seconds; application: 125.614 seconds;
  database: 66.266 seconds. The normal two-package/five-minute gate is unchanged.
- `git diff --check` passed.

The focused developer command for three repeats of all subprocess tests now
allows three minutes because #227 adds sixteen handoff children per repeat.
The Go detector covers application/driver Go code, not the native C engine.
Advisory-lock subprocess tests remain Unix-only. Operators must keep the lock
file in place; manually unlinking it defeats inode-based exclusion.
