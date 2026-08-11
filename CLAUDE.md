# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this repository is

`gostack` is a scaffolding CLI (module `github.com/gos0001/gostack`) that generates
fullstack Go projects — NestJS-CLI in spirit. Nothing here runs in production; the
output does.

**Two levels, easily confused.** This repo is a plain flat Go CLI: cobra commands in
`internal/commands/`, a rendering engine in `internal/scaffold/`, templates in
`templates/`. The *generated* project is the one with vertical slices, wire DI, sqlc
and HTMX. Rules like "one package per use case" or "controllers only route" describe
what the templates *produce*, not how to organise code here.

## Commands

```bash
go build ./... && go vet ./...
go build -o /tmp/gostack ./cmd/gostack
```

There is no test suite. The working check is to generate projects of each shape and
build them — which is exactly what `.github/workflows/ci.yml` does on every push, across
both project shapes:

```bash
cd $(mktemp -d)
/tmp/gostack new full --module example.com/full --fullstack --postgres --redis --docker -y
/tmp/gostack new api  --module example.com/api  --api-only -y
(cd full && go build ./...) && (cd api && go build ./...)
```

Then exercise the generators inside a generated project — each must leave it building:

```bash
cd full
/tmp/gostack g crud users --pages
/tmp/gostack g page 'posts/[post_id]'
/tmp/gostack g api create_order
go build ./...
```

`-y` skips the interactive wizard; any feature flag does too. Without either, and on a
TTY, `gostack new` opens a `huh` form and will block.

## Architecture

### Rendering pipeline

`gostack new` runs `RenderLayers(dest, ctx, ProjectLayers(ctx)...)` → `WriteManifest` →
`PostGenProject`. `internal/scaffold/engine.go` is the whole rendering story.

`templates/project/` is split into layers picked by feature flags: `base/` always, then
`opt_frontend/`, `opt_postgres/`, `opt_redis/`, `opt_docker/`. A missing layer is
skipped. **Layers overwrite, they do not merge** — so files that must exist as a single
unit (`go.mod`, `Makefile`, `cmd/wire.go`, `SKILL.md`, `CLAUDE.md`, `README.md`) live in
`base` and branch inline on `.WithPostgres` and friends. Two layers may fill the same
*directory*, but never the same *file*.

### Three kinds of template file

| Suffix | Delimiters | For |
|---|---|---|
| `.tmpl` | `{{ }}` | Go source, Makefile, env files |
| `.raw.tmpl` | `<< >>` | files whose **output** contains `{{ }}` — Markdown docs, HTML views |
| none | — | copied byte for byte (CSS, JS, `views/*.html`) |

`RenderTree` tests `.raw.tmpl` strictly before `.tmpl`, since the former also ends in
the latter. The whole compound suffix is stripped: `SKILL.md.raw.tmpl` → `SKILL.md`.

`.raw.tmpl` exists because rendering a `{{define}}`-based file with default delimiters
does not fail — it silently yields *nothing*, because executing a template emits only
the text outside its definitions. Hence the asymmetry in `renderWith`:

- under `{{ }}`, an empty render means "skip this file" — a deliberate idiom for
  switching a whole file off with an `{{if}}` wrapper
- under `<< >>`, an empty render is a **hard error**

A `.raw.tmpl` file must never contain a literal `<<` — it would start an action. That
is why the delimiter convention itself is documented only in `README.md`, which is
never rendered.

### Code injection into generated projects

`g uc|api|page|crud` do not just write files; they splice into files the project
already has. `wire_append.go` and `routes_append.go` insert above marker comments —
`// gostack:imports`, `:params`, `:routes`, `:providers` — falling back to older
heuristics when a marker is absent. All of it is string surgery, not `go/ast`; `gofmt`
cleans up afterwards.

`TemplateContext` (in `context.go`) carries everything templates can branch on. Import
paths must be built with `path.Join`, never `filepath.Join` — helpers `UsecasePkgPath`
and `PagePkgPath` exist for this.

### Wire constraints that shape the CLI

Wire rejects a provider set nothing consumes. Three consequences that look arbitrary
until you hit the error:

- `g uc` deliberately does **not** add its `Set` to `cmd/wire.go` — a bare use case has
  no consumer yet
- `g api` registers a route in the same run, or the handler would be unreachable and
  the build would break
- adapter `Set`s enter the graph only when `g crud` creates their first consumer

Wire also rejects two providers of one type, which is why the generated pages
controller returns a marker type `*pages.Pages` rather than a second `*gin.Engine`.

### Orchestrators

`internal/orchestrator/{bootstrap,workers}/` in a generated project are callers of
use cases that are not the network — the lifecycle counterpart of
`internal/controller/http_v1`. Both ship in `base/`, so every project has them
regardless of feature flags.

They resolve the wire constraints above by being **aggregators**: `NewApp` takes a
`*bootstrap.Bootstrap` and a `*workers.Workers` from the very first build, so both
sets are consumed even with zero tasks registered, and each returns its own
distinct type. That is what makes `g uc --orchestrator` the one form of `g uc`
that may call `AppendToWireBuild` — the consumer already exists.

It also keeps `cmd/app.go` out of the splice path. `app.go` carries no marker
comments and never gains any: generators only ever edit
`internal/orchestrator/<name>/<name>.go`, which does.

`AppendOrchestratorTask` (in `routes_append.go`) is table-driven off
`orchestratorSpecs` — a third orchestrator is one map entry plus a
`templates/orchestrator/<name>/` directory. The CLI reuses the existing three
markers rather than inventing a fourth kind, so `insertImport` / `insertParam` /
`insertRoute` work unchanged.

The generated handler templates render **over** `templates/usecase/` into the same
directory, deliberately overwriting `wire.go` with a `Set` that includes the
handler constructor. Same last-writer-wins layering as `ProjectLayers`, applied to
a feature generator.

### Path forms for dynamic pages

The user types `posts/[post_id]`; four derived forms live in `TemplateContext`:

| Field | Value | Used for |
|---|---|---|
| `PagePath` | `posts/[post_id]` | display |
| `FsPagePath` | `posts/_post_id` | directory and import path (brackets are illegal) |
| `ViewPath` | `posts/post_id` | template name |
| `GinPath` | `posts/:post_id` | route |

### Project manifest

`gostack.json` at a generated project's root records its feature set.
`readProjectContext` (in `helpers.go`) reads it back so later `g` commands know whether
pages or Postgres exist; `DetectFeatures` probes the tree as a fallback. It is meant to
be committed, and the generated `.gitignore` must never list it.

## Releasing

`install.sh` is served raw from `main`, but it installs `@latest`, which resolves to
the newest **tag**. Editing the CLI and pushing to `main` therefore changes nothing
for anyone running the curl one-liner until a tag follows. Any change users are meant
to receive needs `git tag -a vX.Y.Z && git push origin vX.Y.Z`.

The same split bites in reverse: a fix to `install.sh` alone reaches users on the next
curl, with no tag needed.

Tag only a commit CI has already passed on `main` — `.github/workflows/release.yml`
re-runs the whole generation suite before it publishes anything, so a bad tag fails
loudly rather than shipping. The workflow then creates the GitHub release and does the
step that is easy to forget: **it asks `proxy.golang.org` for the new version.** The
proxy is pull-based, so until something requests a tag explicitly, `@latest` keeps
serving the previous answer and users report that "latest is broken". The workflow
finishes by asserting `go install …@latest` really does resolve to the tag.

Set `GOPROXY=direct` to bypass all of that when debugging a resolution problem by hand.

`install.sh` is POSIX `sh`, not bash — piping into `sh` lands in dash on Debian. No
`[[`, no arrays, no `local`. Check with `sh -n install.sh` and shellcheck.

`gostack --version` reads `debug.ReadBuildInfo` when ldflags did not run, which is the
normal case: `go install pkg@version` has no build step to inject flags into, so the
toolchain-stamped `Main.Version` is the only source. Local `go build` in a tagged repo
reports something like `v0.1.0+dirty`; a repo with no tags at all reports `dev`.

## Invariants that break silently

**`//go:embed all:project …` — the `all:` prefix is load-bearing.** Without it Go skips
every entry whose name begins with `.` or `_`, dropping `.env.development.tmpl`,
`.gitignore.tmpl`, `_post_id/` directories and the entire `.claude/` skill tree that
every generated project ships.

The CI job `e2e` is what catches this class of bug: it builds the CLI from
`git archive HEAD` — tracked files only, the same content the module proxy serves — and
runs every generator against *that* binary. A local `go build` reads the working tree and
cannot see the problem at all.

**Never add a bare `.claude/` rule to this repo's `.gitignore`.** It would exclude
`templates/project/base/.claude/` from git. Local builds keep working — embed reads the
working tree — while `go install` would produce a CLI that generates projects with no
skill and no error anywhere.

**`embed.FS` cannot represent an empty directory.** Where a generated project needs
one, ship a `.gitkeep`.

## Testing gotchas

Killing `go run ./cmd` leaves its child binary alive, so a stale server keeps answering
on :8080 and runtime checks pass against the wrong process. Use
`lsof -ti:8080 | xargs kill -9` before each run.

`grep` here resolves to ugrep, which rejects PCRE lookarounds like `(?!...)` and can
SIGPIPE the command feeding it — producing a half-written project and confusing
failures downstream.

## Further reading

`README.md` covers the CLI's user-facing commands and has a **Template authoring**
section with the same conventions written for a maintainer coming in cold.
