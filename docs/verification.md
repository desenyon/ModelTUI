# Upgrade verification record

Base: `3b24fae36488bc7ca8f0636074b645dcc5348e79`.
Branch: `codex/catalog-reliability-cli`.

## Execution decisions

- Work is confined to a fresh clone in the delegated task workspace. No existing
  user changes were present. No repository AGENTS.md or local .agents skills exist.
- Initial network/cache/localhost failures were environment restrictions, not test
  failures. Authorized network access and task-specific /tmp Go caches enabled
  baseline verification. Builds run with GOMAXPROCS=2 and GOFLAGS=-p=2.
- Baseline full Go suite passed before implementation (three catalog tests).
- Catalog regression tests first failed for invalid payloads, missing-cache ETags,
  disk-failure throttling, stale validators, oversized bodies, Retry-After overflow
  and hidden cancellation. They passed after implementation.
- New query, CLI and UI tests initially failed because the required interfaces did
  not exist; implemented interfaces passed the full suite. A separate UI form
  regression reproduced lost capability selection and passed after fixing ownership.
- Updater tests cover the release archive naming contract, version ordering,
  extraction, gzip checksum/truncation failures, symlinks, traversal, cancellation
  and preservation of the prior executable. No real executable was replaced.

Final command results and remote CI evidence are reported with the delivery.

## Final independent review

A fresh reviewer found one P2 issue: UI startup's additional timeout hid offline
fallback on stalled requests. TestBootstrapHTTPTimeoutFallsBack reproduced both
cache and snapshot failures; removing the redundant context timeout restored both
fallbacks while retaining explicit caller-cancellation coverage. No other
important findings or deferred minor findings were reported.

A live smoke request returned source `live models.dev`. The Linux amd64 release
configuration cross-compiled successfully. GitHub's latest v0.1.1 assets use the
same tar.gz filenames selected by the repaired updater.

## Final local checks

- `GOMAXPROCS=2 GOFLAGS=-p=2 make check`: passed after the review fix.
  Includes gofmt, `go vet ./...`, `go test -race ./...`, native build and
  Python offline command/PTY smoke checks. The suite contains 29 top-level tests
  plus table-driven subtests; `internal/format` has no package-specific tests.
- `CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -p 2 ./cmd/modeltui`:
  cross-build passed (output was placed in a task-specific temporary path).
- `git diff --check`: passed.
- No release installation, merge, deployment or release publication was performed.
