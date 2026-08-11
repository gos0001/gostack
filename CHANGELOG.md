# Changelog

All notable changes to this project are documented here.

The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and
this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).
While the major version is 0, the CLI surface may still change between minor
releases.

## [Unreleased]

## [0.1.0] - 2026-08-11

First public release.

### Added

- `gostack new` — renders a project from feature-selected template layers: a
  base layer always, plus optional frontend, Postgres, Redis and Docker. An
  interactive `huh` wizard runs when no feature flag and no `-y` is given. The
  chosen feature set is recorded in a committed `gostack.json` that later
  commands read back.
- `gostack g uc` — a use case package: `usecase.go`, `dto.go`, `wire.go`. It
  deliberately leaves `cmd/wire.go` alone, because wire rejects a provider set
  nothing consumes.
- `gostack g api` — the same plus an `http_v1.go` handler, registered in
  `cmd/wire.go` and in the HTTP controller in the same run, so the handler is
  never unreachable.
- `gostack g page` — a server-rendered page with Next.js-style dynamic segments
  (`posts/[post_id]`), its HTML template, and its route in the generated SSR
  router.
- `gostack g crud` — a domain type, five grouped use cases, a migration pair,
  sqlc queries and all five REST routes, in one command.
- `gostack g uc --orchestrator bootstrap|workers` — use cases with callers that
  are not the network. `internal/orchestrator/bootstrap` runs tasks once at
  startup before the listener opens; `internal/orchestrator/workers` runs
  background goroutines with graceful shutdown. Each use case gets a handler
  file named after its caller, the same convention `http_v1.go` follows.
- `gostack dev` — runs `air` with the project's `.env.development`, falling back
  to `go run ./cmd`.
- Generated projects ship their own Claude skill under `.claude/skills/gostack/`,
  so an agent opening the project works from the same architecture contract the
  generator does.
- `install.sh` — a POSIX `sh` installer that checks the Go toolchain version,
  installs the CLI and puts the Go bin directory on `PATH`.

[Unreleased]: https://github.com/gos0001/gostack/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/gos0001/gostack/releases/tag/v0.1.0
