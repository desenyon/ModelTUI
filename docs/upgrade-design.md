# Catalog reliability and automation design

ModelTUI remains a Charm terminal browser for models.dev. This upgrade makes its
catalog usable predictably in offline terminals and scripts, and repairs updates
to match the archives already produced by the release workflow.

## Constraints and decisions

- Preserve the four TUI tabs, existing shortcuts, embedded snapshot and cache
  fallback. No new dependencies or external services.
- Keep Go 1.26.4. Limit local build parallelism while other tasks share the Mac.
- Share one configured catalog client between startup, refresh and scheduling.
- Keep the existing catalog.json/rate.json files. Associate validators with a
  content digest and endpoint; old metadata remains readable but unsafe ETags
  are discarded. Atomic temporary files prevent partially written JSON.
- Retain 45-second request spacing, 15-minute refresh and capped Retry-After.
  Keep rate state in memory when persistence fails. Bound response sizes and
  reject payloads that do not contain catalog maps. Never replace usable data
  with malformed JSON. A missing cache must prevent conditional requests.
- Add global offline, cache-directory and timeout options. Offline means zero
  catalog network calls, including from manual and automatic TUI refresh.
- Add a headless offering list using shared catalog query functions: substring
  search, exact provider IDs, intersected capabilities, minimum context, stable
  name/context/price ordering and table or JSON output. Unknown pricing sorts
  last; empty JSON results are arrays, and invalid flags fail before fetching.
- Download published tar.gz update assets into a bounded temporary file, extract
  only the expected regular binary, then replace the executable atomically.
  Reject malformed/oversized archives without touching the installed binary.
  Treat development builds explicitly and never downgrade newer versions.
- Add branch/PR CI for formatting, vet, race tests, build and offline smoke tests.
  Preserve release automation and do not publish or merge anything in this task.

## Alternatives considered

A UI-only patch would leave scripts and reliability failures unresolved. A new
database/cache service would add migration and dependency costs without helping
this read-only catalog. Focused package boundaries and standard-library storage
keep the project small and make failure cases testable with local HTTP fixtures.

## Verification focus

Exercise corrupt/missing cache with an ETag, disk write failures, cancellation,
concurrent refresh, Retry-After overflow, oversized/malformed HTTP bodies,
deterministic equal-name ordering, absent pricing, invalid CLI flags, offline
startup/refresh and malformed archive replacement. Run the entire suite with the
race detector, vet, formatting and build checks, then CLI and terminal smoke tests.

## Known boundaries

Cache metadata coordinates sequential invocations; it is not a cross-process
locking protocol. Provider aliases are catalog data, not authoritative canonical
model identity. Pricing is descriptive metadata, not a billing calculator.
