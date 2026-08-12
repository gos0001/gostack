# Changelog

All notable changes to this project are documented here.

The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and
this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).
While the major version is 0, the CLI surface may still change between minor
releases.

## [Unreleased]

### Added

- Unit tests for the pure functions in `internal/scaffold` — name derivation,
  the four page path forms, validation, and the splice helpers that inject code
  into an existing project. Each `Append*` is now checked for idempotence in a
  second rather than only by the CI generation suite. `go test ./... -race` runs
  in CI.

### Fixed

- The CRUD singulariser stripped a trailing `es` unconditionally, so
  `gostack g crud notes` produced `domain.Not` and `ErrNotNotFound`, and
  `images` produced `imag`. `es` is now only stripped after `s`, `x`, `z`, `ch`
  or `sh`, which keeps `boxes` → `box` and `categories` → `category` while
  fixing `notes` → `note`. Irregular plurals are still not handled and still
  need a look before committing.

## [0.2.0] - 2026-08-12

### Changed

- **The `workers` orchestrator is now `cron`.** Every worker generated in
  practice was a ticker loop, so the general "long-lived goroutine" slot was a
  name promising something nothing used. One orchestrator now covers all periodic
  background work.
- **The scheduling loop moved from the handler into the orchestrator.** A job's
  `CronRun(ctx)` does the work exactly once, so a failed run costs one tick
  rather than the schedule, and it is a plain function to test. Each job gets its
  own goroutine and ticker, so a run outliving its interval delays the next tick
  instead of overlapping with itself.
- **A cron job has one config knob instead of two.** The interval is the
  on-switch: zero is off, and zero is also the value `time.NewTicker` panics on,
  so "enabled but unscheduled" can no longer be configured. `bootstrap` keeps its
  `<NAME>_ENABLED` bool, having no interval to overload.
- `Cron.Start` logs every job it scheduled and every one it skipped, so a job
  that is not running can be diagnosed from the boot log.

### Removed

- `--orchestrator workers`. A process that must genuinely run forever rather than
  on a schedule — a queue consumer, a websocket hub — is now a hand-written
  orchestrator; `@sections/orchestrators.md` in the generated skill covers how.

### Migration from 0.1.0

For each existing project:

1. `git mv internal/orchestrator/workers internal/orchestrator/cron` and rename
   `workers.go` to `cron.go`; inside it `Workers` → `Cron`, `Worker` → `Job`,
   `WorkersRun` → `CronRun`, the `workers` field → `jobs`, and add
   `Interval() time.Duration` to the interface.
2. `WORKERS_SHUTDOWN_TIMEOUT` → `CRON_SHUTDOWN_TIMEOUT`.
3. In each use case, rename `workers.go` to `cron.go`, `WorkersHandler` →
   `CronHandler`, `NewWorkers` → `NewCron`. Delete the ticker loop from the
   handler and leave only one pass of the work in `CronRun`; add
   `Interval() time.Duration { return h.cfg.Interval }`.
4. Drop `Enabled` from each job's `Config`; a zero `Interval` now means off.
   Replace `<NAME>_ENABLED=true` with `<NAME>_INTERVAL=<duration>` in your env.
5. In `cmd/app.go` and `cmd/wire.go`, swap the import and the `*workers.Workers`
   parameter for `*cron.Cron`, then re-run `wire ./cmd/`.

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
  (`workers` became `cron` in 0.2.0.)
- `gostack dev` — runs `air` with the project's `.env.development`, falling back
  to `go run ./cmd`.
- Generated projects ship their own Claude skill under `.claude/skills/gostack/`,
  so an agent opening the project works from the same architecture contract the
  generator does.
- `install.sh` — a POSIX `sh` installer that checks the Go toolchain version,
  installs the CLI and puts the Go bin directory on `PATH`.

[Unreleased]: https://github.com/gos0001/gostack/compare/v0.2.0...HEAD
[0.2.0]: https://github.com/gos0001/gostack/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/gos0001/gostack/releases/tag/v0.1.0
