# Catalog reliability and automation implementation plan

**Goal:** Reliable offline/online browsing, scriptable offering queries and working
archive updates, with reproducible verification.

**Architecture:** Catalog owns transport, cache and query logic. The CLI configures
a client and either starts the UI or prints query results. Update transport and
archive installation have independently testable boundaries.

**Tech stack:** Go 1.26.4, existing Charm/Cobra packages, standard library only.

**Spec:** [upgrade-design.md](upgrade-design.md)

1. Catalog client (`internal/catalog/client.go`, `ratelimit.go`, new cache tests):
   write regression tests for invalid payloads, validators without usable cache,
   failed disk persistence and bounded downloads; observe failure, implement,
   then run catalog tests and the full suite.
2. Query and CLI (`internal/catalog/query.go`, `cmd/modeltui`, `internal/ui`):
   test deterministic filters/order, CLI validation/JSON, shared client and offline
   UI behavior; implement reusable queries and injectable constructors; run suite.
3. Updater (`internal/update`): test release asset selection/version ordering and
   malformed archive preservation; implement bounded extraction and replacement;
   run suite.
4. Documentation and CI: document actual architecture, setup, flags, examples,
   cache migration, offline behavior and limitations. Add reproducible Makefile
   checks and branch/PR CI. Verify format, vet, race tests, build and CLI/TUI smoke.
5. Review final changes, commit the improvement branch, push without force and
   verify the remote commit SHA and CI results. Do not merge or release.

The user's delegated request authorizes implementing and pushing this plan.
Execution is inline; a fresh reviewer checks the final change before pushing.
