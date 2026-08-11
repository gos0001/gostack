# gostack

A scaffolding CLI for fullstack Go, in the spirit of the NestJS CLI. It generates a
project laid out as vertical slices — one package per use case — with wire for DI,
sqlc for Postgres, and server-rendered pages driven by HTMX.

Generated projects ship with their own Claude skill, so any agent that opens the
repository works from the same rules the generator does.

```
curl -fsSL https://raw.githubusercontent.com/gos0001/gostack/main/install.sh | sh
gostack new blog-app
cd blog-app
gostack dev
```

---

## Install

```bash
curl -fsSL https://raw.githubusercontent.com/gos0001/gostack/main/install.sh | sh
```

The script installs gostack with `go install` and adds the Go bin directory to
your `PATH`. That directory matters beyond gostack itself: a generated project's
`Makefile` calls `air`, `wire`, `sqlc`, `migrate` and `golangci-lint` by bare
name, and `make tools` installs all of them to the same place. One `PATH` entry
covers the CLI and everything it generates.

**It cannot update the shell you ran it from** — piping into `sh` starts a
subprocess, and a subprocess cannot change its parent's environment. So the
script edits your shell rc file and prints the one command that applies it to the
current session:

```
gostack v0.2.0 installed to /Users/you/go/bin
Added /Users/you/go/bin to PATH in ~/.zshrc

Activate it in this shell:

  source ~/.zshrc
```

zsh, bash and fish are recognised (fish gets `fish_add_path`, not `export`). Any
other shell and the script prints the line for you to paste rather than guessing
at the wrong file. Re-running is safe: the entry carries a marker comment and is
written once.

| Knob | Effect |
|---|---|
| `GOSTACK_VERSION=v0.1.0` | install that tag instead of `latest` |
| `GOSTACK_NO_MODIFY_PATH=1` | never touch the rc file; print the line instead |
| `--no-modify-path` | same, as a flag: `… \| sh -s -- --no-modify-path` |

Prefer to do it by hand, or already have `$(go env GOPATH)/bin` on `PATH`:

```bash
go install github.com/gos0001/gostack/cmd/gostack@latest
```

**Requirements**

- Go 1.26 or newer — the module targets `go 1.26.5`, so older toolchains refuse.
  The installer checks this before doing anything. Generated projects themselves
  target `go 1.22`.
- Docker only if you pass `--postgres` or `--redis`, and only to run the
  services. Projects build and compile without it.

Go is not an incidental dependency you can drop by shipping a binary: gostack
shells out to `go mod tidy` and installs wire, sqlc and air with `go install`, so
a machine without Go could not run a generated project either. That is why the
installer builds from source rather than downloading a release artefact.

`gostack new` installs `wire`, `sqlc` and `air` on first run and then generates
`cmd/wire_gen.go`, so the first project takes a minute or two and pulls from the
network. If any of the three cannot be resolved afterwards the CLI warns and
tells you to run `make tools` in the project — nothing is silently skipped.

---

## Creating a project

Run `gostack new <name>` on a terminal and you get an interactive wizard: module
path, project type, adapters. Pass any feature flag — or `--yes` — and the wizard
is skipped entirely, which is what you want in CI.

```
gostack new blog-app                                  # wizard
gostack new api-svc --api-only --postgres             # exactly these features
gostack new quick --yes                               # defaults, no prompts
```

| Flag | Effect |
|---|---|
| `--module <path>` | Go module path (default `github.com/user/<name>`) |
| `--fullstack` | JSON API + SSR pages + HTMX (default) |
| `--api-only` | JSON only — no views, static, or page packages |
| `--postgres` | `pkg/postgres`, sqlc adapter, migrations |
| `--redis` | `pkg/redis` and its adapter |
| `--docker` | `docker-compose.yml`, `Dockerfile`, `Dockerfile.dev` |
| `-y, --yes` | Skip the wizard, take the defaults |

The chosen feature set is written to `gostack.json` at the project root. Later
`gostack g` commands read it back, so they know whether pages or Postgres exist.
That file is meant to be committed.

## Generators

| Command | Creates | Also mutates |
|---|---|---|
| `gostack g uc <name>` | `internal/usecases/[<group>/]<name>/` — usecase, dto, wire | nothing |
| `gostack g uc <name> --orchestrator bootstrap\|workers` | the same plus `config.go` and a `bootstrap.go` / `workers.go` handler | `cmd/wire.go`, `internal/orchestrator/<name>/<name>.go` |
| `gostack g api <name>` | the same plus `http_v1.go` | `cmd/wire.go`, `controller.go` (registers the route) |
| `gostack g page <path>` | `internal/web/pages/<path>/` — page, usecase, dto, wire + a view | `cmd/wire.go`, `internal/web/routes.go` |
| `gostack g crud <plural>` | domain file, five grouped usecases, migration pair, queries | `cmd/wire.go`, `controller.go` |
| `gostack dev` | — | runs `air`, falls back to `go run ./cmd` |

Both `g uc` and `g api` accept an optional group: `gostack g uc users/get_profile`
lands in `internal/usecases/users/get_profile/`.

`gostack g crud users --pages` additionally generates the list and detail SSR pages.

**Why `g uc` does not touch `cmd/wire.go`.** wire rejects a provider set that
nothing consumes, so adding a bare use case to the graph would break the build.
Its `Set` goes in when a consumer appears. `g api` registers a route in the same
run for exactly this reason — an unrouted handler would not compile either. So
does `g uc --orchestrator`, which is why that one form of `g uc` *does* edit
`cmd/wire.go`: the orchestrator is the consumer.

**Orchestrators.** `internal/orchestrator/` holds callers of use cases that are
not the network — `bootstrap/` runs tasks once at startup before the listener
opens, `workers/` runs background goroutines for the life of the process. Every
project gets both. A use case reached that way grows a handler file named after
its caller, exactly as an HTTP one grows `http_v1.go`:

```
internal/usecases/seed_super_admin/
├── usecase.go   dto.go
├── config.go        SEED_SUPER_ADMIN_ENABLED, off by default
├── bootstrap.go     the caller — same shape as http_v1.go
└── wire.go
```

## What gets generated

```
blog-app/
├── cmd/                     main, app, config, wire, wire_gen
├── internal/
│   ├── domain/              pure models + sentinel errors
│   ├── usecases/            one package per use case, grouped by entity
│   ├── controller/http_v1/  JSON API routes
│   ├── orchestrator/        bootstrap/ and workers/ — non-network callers
│   ├── web/                 the whole SSR layer
│   │   ├── routes.go        SSR routes — generated, do not edit
│   │   └── pages/           one package per page, all `package page`
│   └── adapter/             postgres (sqlc), redis
├── pkg/                     logger, http_server, views, postgres, redis
├── views/  static/          templates and assets
├── migrations/  sqlc.yaml
├── .claude/skills/gostack/  the project's own Claude skill
├── CLAUDE.md  README.md
└── gostack.json             feature manifest — committed
```

Conventions the templates encode: domain is pure (no tags, no adapter imports);
adapters map storage errors to domain errors; handlers answer through
`pkg/http_server`, never `c.JSON`; controllers only route; `pkg/` never imports
`internal/domain`; config lives per package via envconfig.

The frontend boundary is a directory, not a convention to remember:
`internal/web/` is the only tree that renders HTML or touches `pkg/views`, while
`internal/usecases/` stays backend and is reachable from the JSON API and from a
page alike. A page's own usecase sits beside its handler because it feeds exactly
one template; the moment it gains a second caller it belongs in
`internal/usecases/`. With `--api-only` the whole `internal/web/` tree, plus
`views/` and `static/`, is never generated.

---

## Template authoring

Everything below concerns hacking on gostack itself, not on generated projects.

### Layers

`templates/project/` is split into layers. `ProjectLayers` picks them from the
feature set and `RenderLayers` renders them in order into one destination:

```
base/           always
opt_frontend/   --fullstack
opt_postgres/   --postgres
opt_redis/      --redis
opt_docker/     --docker
```

A missing layer is skipped. Later layers win on identical filenames, but no two
layers should contribute the same *file* — a collision means something is filed
in the wrong place. Several layers may contribute to the same *directory*: the
skill's `sections/` is filled from `base` plus each optional layer.

Files that must always exist as a single unit — `SKILL.md`, `CLAUDE.md`,
`README.md`, `go.mod`, `Makefile`, `cmd/wire.go` — live in `base` and branch
inline on `.WithPostgres` and friends. Layers overwrite; they do not merge.

### Three kinds of template file

| Suffix | Delimiters | Use for |
|---|---|---|
| `.tmpl` | `{{ }}` | ordinary generation — Go source, Makefile, env files |
| `.raw.tmpl` | `<< >>` | files whose **output** contains `{{ }}` — Markdown docs, HTML views |
| none | — | copied byte for byte (CSS, JS, `views/*.html`) |

The suffix is stripped on write: `SKILL.md.raw.tmpl` → `SKILL.md`. `RenderTree`
tests `.raw.tmpl` strictly before `.tmpl`, since the former also ends in the latter.

`.raw.tmpl` exists because rendering a `{{define}}`-based file with the default
delimiters does not fail — it silently yields *nothing*, because executing a
template emits only the text outside its definitions. Combined with skip-empty
below, that produced empty or missing files with no error at all. Hence:

- under `{{ }}`, an empty render means "skip this file" — a deliberate idiom for
  switching a whole file off with an `{{if}}` wrapper
- under `<< >>`, an empty render is a **hard error**, because such files are
  almost entirely literal text

One consequence: a `.raw.tmpl` file must never contain a literal `<<`, which
would start an action. Documentation about this convention therefore lives here,
in a file that is never rendered.

### go:embed

```go
//go:embed all:project all:usecase all:api all:page all:page_tmpl all:crud
```

The `all:` prefix is load-bearing. Without it Go skips every entry whose name
begins with `.` or `_` during the tree walk, which would silently drop
`.env.development.tmpl`, `.gitignore.tmpl`, page directories like `_post_id/`,
and the entire `.claude/` skill tree. Never remove it.

For the same reason `.gitignore` must not contain a bare `.claude/` rule: it
would exclude `templates/project/base/.claude/` from the repository. Local builds
would still work — embed reads the working tree — while `go install` produced a
CLI that generates projects with no skill and no error.

`embed.FS` cannot represent an empty directory. Where a generated project needs
one, ship a `.gitkeep`.

### Splice markers

Generated files carry marker comments that the CLI injects at:

```
// gostack:imports     // gostack:params
// gostack:routes      // gostack:providers
```

`wire_append.go` and `routes_append.go` insert above the marker line, falling
back to older heuristics when it is absent. Deleting a marker from a generated
project silently disables code injection for that file.

### Path variables

Directory and file names in a template tree may contain `__ProjectName__`,
`__PackageName__`, `__EntityName__`, `__EntityPlural__`, `__PascalEntity__`,
`__FeatureName__`, `__PascalName__`, `__ModulePath__`. `expandPath` substitutes
them before rendering.

### Running the tests

There is no test suite yet. The working check is to build the CLI, generate a
project of each shape, and build those:

```bash
go build ./... && go vet ./...
go build -o /tmp/gostack ./cmd/gostack
cd $(mktemp -d)
/tmp/gostack new full --module example.com/full --fullstack --postgres --redis --docker -y
/tmp/gostack new api  --module example.com/api  --api-only -y
(cd full && go build ./...) && (cd api && go build ./...)
```

---

## License

MIT — see [LICENSE](LICENSE).
