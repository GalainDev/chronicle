---
type: decision
area: work
created: 2026-07-26T18:29:52Z
---

# Use pgbouncer to fix Postgres connection pool exhaustion instead of raising max_connections

## Problem

Our Postgres connection pool kept maxing out under load. Root cause took
about an hour to track down: a retry path leaked a connection whenever a
query hit a timeout — the retry logic re-issued the query on a new
connection without releasing the original one back to the pool. Under
sustained load, timeouts became frequent enough that leaked connections
accumulated faster than they could be reaped, eventually exhausting the
pool.

## Decision

Fix the leak in the retry path (release the connection before/regardless of
retry), and put pgbouncer in front of Postgres rather than simply raising
`max_connections`.

## Rationale

- The leak was a real bug that needed fixing regardless — a raised
  `max_connections` doesn't address it.
- Bumping `max_connections` only buys headroom; the leak rate scales with
  load, so the same exhaustion would recur at a higher ceiling once traffic
  grew further. It treats the symptom, not the cause.
- pgbouncer adds connection pooling/multiplexing in front of Postgres,
  giving much better headroom against future leaks or connection-heavy
  spikes without requiring Postgres itself to hold open as many backend
  connections (each of which has real memory/resource cost on the DB
  server). It's a more robust structural fix than a config bump.

## Follow-up

- Retry-path connection handling is worth an audit for other leak-prone
  patterns (see if there are similar retry-then-reconnect code paths
  elsewhere).

