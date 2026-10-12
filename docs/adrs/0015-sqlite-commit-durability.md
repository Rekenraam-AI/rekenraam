# ADR 0015: SQLite Commit Durability

## Status

Accepted (2026-10-12). Supersedes ADR 0004's WAL/NORMAL runtime policy and
ADR 0014's requirement to preserve that synchronous value during cutover.

[ADR 0016](0016-native-backend-test-resources.md) subsequently supersedes
decision 5's test schedule and deadline after the owner's clarification and
native scheduling measurements. Other decisions remain in force.

## Context

The completed #195 driver migration deliberately kept the former writer's
WAL/NORMAL policy for a comparable performance measurement. The owner then
selected FULL consistently for the financial application.

In WAL mode, NORMAL preserves consistency and committed data across application
crashes, but acknowledged commits can disappear after a power loss or operating
system crash. FULL adds a WAL synchronization for every commit, making those
commits durable when the filesystem and storage honor synchronization requests.
Manual entries, reconciliation decisions and corrections deserve that guarantee.
Backups alone do not protect changes made since the last durable copy.

## Decision

1. Use WAL with `synchronous=FULL` for the application writer, migrations and
   ordinary financial tests. No faster test-only or import-only durability mode.
2. Install FULL in both the normalized DSN and the registered native driver's
   connection hook. Direct driver opens and replacement physical connections
   receive FULL too. Auxiliary backup, inspection and maintenance connections
   continue using FULL explicitly. Read-only pools use the same setting for
   consistency; it provides no extra durability for reads.
3. Startup validates synchronous mode as `2` (FULL). An older URL supplying
   NORMAL/OFF through supported former `_pragma` options or native aliases
   cannot weaken the required policy. This is a connection-policy change;
   schema, seed data and migration checksums do not change.
4. Retain one writer, read-only pool maximum four/idle two, foreign keys on,
   busy timeout 5000 ms, deferred BEGIN, mutex protection and autocheckpoint
   1000 pages. Four read connections are a bounded concurrency choice, not four
   nodes or a CPU requirement. Application pool waits use Go contexts;
   `busy_timeout` handles SQLite lock contention, not Go pool queueing.
5. Keep the shared race gate at `-p 1 -timeout=20m`, without a fixed four-CPU
   cap. Four-CPU affinity/GOMAXPROCS settings describe controlled benchmarks.
   Prior modernc scaling results do not establish an optimal native schedule.
   Retain all assertions and Go race instrumentation; the C engine remains
   outside the Go detector's coverage.
6. Continue the admitted bundled native driver, CGO and FTS5/netgo/osusergo
   build tags. Cross-compilation and fully static linking remain optional;
   the Debian runtime's libc dependency is explicit. No system libsqlite3 or
   second application driver is introduced.
7. Keep statement caching off for this change. Optimize measured write costs
   with appropriate atomic transactions and batching rather than weakening
   durability. FULL does not checkpoint or rewrite the entire database on
   every commit. The 1000-page checkpoint threshold controls WAL maintenance;
   it is not a time-based durability guarantee.

## Consequences and validation

FULL can increase commit latency, especially with many small transactions or
slow storage. This cost is accepted for financial records. Keep the original
#195 matched driver comparison as historical WAL/NORMAL evidence and record
FULL performance separately; do not reuse its percentages as FULL results.

Named tests cover initial/replacement writer and read connections, direct
driver opens with a weakening DSN, and auxiliary connections. Existing
financial, migration, backup/restore, concurrency and embedded/container checks
remain required. The [FULL review](../reviews/sqlite-full-durability-2026-10-12.md)
records the settings audit, timings and validation.

## References

- [SQLite synchronous guarantees](https://sqlite.org/pragma.html#pragma_synchronous).
- [SQLite WAL performance and checkpoints](https://sqlite.org/wal.html).
- [ADR 0004](0004-sqlite-connection-migrations-and-backup.md).
- [ADR 0014](0014-native-sqlite-driver.md).
- [Original driver migration evidence](../reviews/native-sqlite-driver-2026-10-12.md).
