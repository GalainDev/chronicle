# Postgres connection pool exhaustion under load

Date: 2026-07-26
Status: Resolved

## Problem

Under load, the Postgres connection pool kept maxing out. Took about an hour to
track down.

## Root cause

A connection leak in a retry path: when a query timed out, the retry logic
re-acquired a new connection but never released the original one back to the
pool. Under normal conditions this path rarely fired, so the leak was slow and
easy to miss — but under load, timeouts became frequent enough that leaked
connections accumulated quickly and exhausted the pool.

## Fix

Two options were on the table:

1. **Bump `max_connections`.** Rejected. This doesn't fix the leak, it just
   raises the ceiling — the same leak would eventually exhaust the larger pool
   under sufficiently high load. It would have delayed the problem, not solved
   it, and made the eventual recurrence harder to diagnose (larger blast
   radius, same root cause buried further away from the last change).
2. **Switch to pgbouncer.** Chosen. Pgbouncer sits in front of Postgres and
   manages connection pooling/reuse independently of the application's
   connection handling, so a leak in application-level retry logic is far less
   likely to exhaust the real database connections. It addresses the actual
   failure mode (connections not being reliably returned) rather than just
   giving the leak more room to run before failing again.

The leaked-connection bug in the retry path was also fixed directly (the
retry path now releases the original connection before acquiring a new one),
but pgbouncer was adopted regardless, as defense in depth against similar
leaks in the future.

## Reasoning to remember

- When a resource limit is hit, first ask "is this a leak or genuine demand?"
  before raising the limit. Raising limits on a leak just changes the time
  constant, not the outcome.
- Timeout/retry paths are a common place for resource leaks to hide, because
  they're exercised rarely in normal operation and heavily under load/failure
  conditions — exactly when you can least afford them.
- Prefer fixes that remove a class of failure (pgbouncer isolating the app
  from raw connection management) over point fixes that only address the
  specific bug found today.
