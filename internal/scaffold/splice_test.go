package scaffold

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The generators splice into files by string surgery rather than go/ast, so
// these fixtures are the real rendered shapes — the tests are only meaningful
// if the anchors they exercise are the ones the templates actually emit.

const controllerFixture = `package http_v1

import (
	"github.com/gin-gonic/gin"
	// gostack:imports
)

func New(
	// gostack:params
) *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery())

	v1 := r.Group("/api/v1")
	_ = v1

	// gostack:routes

	return r
}
`

const wireFixture = `//go:build wireinject

package main

import (
	"github.com/google/wire"

	"example.com/app/internal/controller/http_v1"
	"example.com/app/internal/orchestrator/bootstrap"
	"example.com/app/internal/orchestrator/cron"
	"example.com/app/pkg/logger"
	// gostack:imports
)

func InitializeApp() (*App, error) {
	wire.Build(
		LoadConfig,
		logger.Set,
		http_v1.Set,
		bootstrap.Set,
		cron.Set,
		// gostack:providers
		NewApp,
	)
	return nil, nil
}
`

const cronFixture = `package cron

import (
	"context"
	"sync"
	"time"

	"go.uber.org/zap"
	// gostack:imports
)

func New(
	logger *zap.SugaredLogger,
	cfg Config,
	// gostack:params
) *Cron {
	c := &Cron{logger: logger, cfg: cfg}
	// gostack:routes

	return c
}
`

// writeProject lays out just enough of a generated project for the Append*
// functions to find their targets.
func writeProject(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	files := map[string]string{
		"cmd/wire.go": wireFixture,
		"internal/controller/http_v1/controller.go": controllerFixture,
		"internal/orchestrator/cron/cron.go":        cronFixture,
	}
	for rel, content := range files {
		full := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func read(t *testing.T, dir, rel string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func countOccurrences(s, sub string) int {
	return strings.Count(s, sub)
}

func TestInsertBeforeMarker(t *testing.T) {
	const content = "a\n// marker\nb\n"
	got, ok := insertBeforeMarker(content, "// marker", "inserted")
	if !ok {
		t.Fatal("ok = false, want true when the marker is present")
	}
	if got != "a\ninserted\n// marker\nb\n" {
		t.Errorf("got %q", got)
	}

	// ok == false is the signal callers use to fall back to the older
	// brace-matching heuristics, so it must not be silently swallowed.
	if _, ok := insertBeforeMarker(content, "// absent", "inserted"); ok {
		t.Error("ok = true, want false when the marker is absent")
	}
}

// The marker keeps insertion order stable: each new line lands directly above
// the marker, so registrations appear in the order they were generated.
func TestInsertBeforeMarkerPreservesOrder(t *testing.T) {
	content := controllerFixture
	content = insertRoute(content, "\tfirst")
	content = insertRoute(content, "\tsecond")

	firstAt := strings.Index(content, "first")
	secondAt := strings.Index(content, "second")
	markerAt := strings.Index(content, MarkerRoutes)
	if !(firstAt < secondAt && secondAt < markerAt) {
		t.Errorf("expected first < second < marker, got %d < %d < %d", firstAt, secondAt, markerAt)
	}
}

func TestInsertImportUsesTheMarker(t *testing.T) {
	got := insertImport(controllerFixture, "\tfoo \"example.com/app/internal/usecases/foo\"")
	if !strings.Contains(got, "\tfoo \"example.com/app/internal/usecases/foo\"\n\t// gostack:imports") {
		t.Errorf("import not spliced above the marker:\n%s", got)
	}
}

// Without a marker, insertImport falls back to the "\n)" that closes the import
// block. That fallback exists for projects generated before markers, and it has
// to leave the anchor intact so a second insert still works.
func TestInsertImportFallbackKeepsItsAnchor(t *testing.T) {
	content := strings.Replace(controllerFixture, "\t// gostack:imports\n", "", 1)
	content = insertImport(content, "\tfirst \"a\"")
	content = insertImport(content, "\tsecond \"b\"")

	if !strings.Contains(content, "first \"a\"") || !strings.Contains(content, "second \"b\"") {
		t.Errorf("fallback dropped an import:\n%s", content)
	}
	if !strings.Contains(content, "\n)") {
		t.Errorf("fallback destroyed the \"\\n)\" anchor:\n%s", content)
	}
}

// insertRoute's fallback anchors on the literal "return router", which the
// generated controller does NOT contain — it returns `r`. So for the templates
// this repo actually ships, the marker is not a convenience, it is the only
// thing that works. Documented as a test so deleting a marker fails loudly here
// rather than silently producing an unrouted handler.
func TestInsertRouteFallbackDoesNotMatchTheGeneratedController(t *testing.T) {
	content := strings.Replace(controllerFixture, "\t// gostack:routes\n", "", 1)
	got := insertRoute(content, "\tv1.POST(\"/x\", xH.Handle)")
	if got != content {
		t.Errorf("insertRoute unexpectedly matched a fallback anchor:\n%s", got)
	}
	if strings.Contains(controllerFixture, "return router") {
		t.Error("the controller fixture now says `return router`; the fallback is live and this test is stale")
	}
}

func TestAppendToWireBuild(t *testing.T) {
	dir := writeProject(t)

	if err := AppendToWireBuild(dir, "example.com/app", "internal/usecases/create_order", "create_order"); err != nil {
		t.Fatal(err)
	}
	got := read(t, dir, "cmd/wire.go")
	if !strings.Contains(got, `create_order "example.com/app/internal/usecases/create_order"`) {
		t.Errorf("import missing:\n%s", got)
	}
	if !strings.Contains(got, "\t\tcreate_order.Set,") {
		t.Errorf("provider set missing:\n%s", got)
	}
	// Both must land above their markers, or the next generator inserts into
	// the wrong block.
	if strings.Index(got, "create_order.Set,") > strings.Index(got, MarkerProviders) {
		t.Error("Set was spliced below the providers marker")
	}
}

// Re-running a generator is routine — `gostack g crud` calls AppendToWireBuild
// five times, and users re-run commands. A second call must change nothing.
func TestAppendToWireBuildIsIdempotent(t *testing.T) {
	dir := writeProject(t)

	for i := 0; i < 3; i++ {
		if err := AppendToWireBuild(dir, "example.com/app", "internal/usecases/create_order", "create_order"); err != nil {
			t.Fatal(err)
		}
	}
	got := read(t, dir, "cmd/wire.go")
	if n := countOccurrences(got, `create_order "example.com/app`); n != 1 {
		t.Errorf("import appears %d times, want 1", n)
	}
	if n := countOccurrences(got, "create_order.Set,"); n != 1 {
		t.Errorf("provider set appears %d times, want 1", n)
	}
}

func TestAppendAPIRoute(t *testing.T) {
	dir := writeProject(t)
	ctx := NewFeatureContext(
		NewProjectContext("app", "example.com/app", Features{}),
		"", "create_order", false, true,
	)

	for i := 0; i < 2; i++ {
		if err := AppendAPIRoute(dir, "example.com/app", ctx, "POST", "/create-order"); err != nil {
			t.Fatal(err)
		}
	}

	got := read(t, dir, "internal/controller/http_v1/controller.go")
	for _, want := range []string{
		`create_order "example.com/app/internal/usecases/create_order"`,
		"\tcreate_orderH *create_order.HTTPv1Handler,",
		"\tv1.POST(\"/create-order\", create_orderH.Handle)",
	} {
		if n := countOccurrences(got, want); n != 1 {
			t.Errorf("%q appears %d times, want 1:\n%s", want, n, got)
		}
	}
}

// A grouped use case must import from its group directory, not the flat one.
func TestAppendAPIRouteUsesTheGroupPath(t *testing.T) {
	dir := writeProject(t)
	ctx := NewFeatureContext(
		NewProjectContext("app", "example.com/app", Features{}),
		"users", "ban_user", false, true,
	)
	if err := AppendAPIRoute(dir, "example.com/app", ctx, "POST", "/ban-user"); err != nil {
		t.Fatal(err)
	}
	got := read(t, dir, "internal/controller/http_v1/controller.go")
	if !strings.Contains(got, `"example.com/app/internal/usecases/users/ban_user"`) {
		t.Errorf("group missing from the import path:\n%s", got)
	}
}

func TestAppendOrchestratorTask(t *testing.T) {
	dir := writeProject(t)
	ctx := NewFeatureContext(
		NewProjectContext("app", "example.com/app", Features{}),
		"", "outbox_drain", false, false,
	)

	for i := 0; i < 2; i++ {
		if err := AppendOrchestratorTask(dir, "example.com/app", "cron", ctx); err != nil {
			t.Fatal(err)
		}
	}

	got := read(t, dir, "internal/orchestrator/cron/cron.go")
	for _, want := range []string{
		`outbox_drain "example.com/app/internal/usecases/outbox_drain"`,
		"\toutbox_drainC *outbox_drain.CronHandler,",
		"\tc.jobs = append(c.jobs, outbox_drainC)",
	} {
		if n := countOccurrences(got, want); n != 1 {
			t.Errorf("%q appears %d times, want 1:\n%s", want, n, got)
		}
	}
}

func TestAppendOrchestratorTaskRejectsUnknownOrchestrator(t *testing.T) {
	dir := writeProject(t)
	ctx := NewFeatureContext(
		NewProjectContext("app", "example.com/app", Features{}),
		"", "outbox_drain", false, false,
	)
	if err := AppendOrchestratorTask(dir, "example.com/app", "workers", ctx); err == nil {
		t.Error("expected an error for a removed orchestrator")
	}
}

// The splice functions read the file first, so a project missing the target
// must produce an error rather than silently doing nothing — the commands print
// it as a warning and the user gets to know their code was not wired.
func TestAppendReportsAMissingTarget(t *testing.T) {
	dir := t.TempDir()
	ctx := NewFeatureContext(
		NewProjectContext("app", "example.com/app", Features{}),
		"", "create_order", false, true,
	)
	if err := AppendToWireBuild(dir, "example.com/app", "internal/usecases/create_order", "create_order"); err == nil {
		t.Error("AppendToWireBuild on an empty dir = nil, want an error")
	}
	if err := AppendAPIRoute(dir, "example.com/app", ctx, "POST", "/x"); err == nil {
		t.Error("AppendAPIRoute on an empty dir = nil, want an error")
	}
	if err := AppendOrchestratorTask(dir, "example.com/app", "cron", ctx); err == nil {
		t.Error("AppendOrchestratorTask on an empty dir = nil, want an error")
	}
}

// `gostack g crud` registers five handlers and one route group in one pass.
func TestAppendCRUDRoutes(t *testing.T) {
	dir := writeProject(t)
	ctx := NewCRUDContext(NewProjectContext("app", "example.com/app", Features{}), "users")

	for i := 0; i < 2; i++ {
		if err := AppendCRUDRoutes(dir, "example.com/app", ctx); err != nil {
			t.Fatal(err)
		}
	}

	got := read(t, dir, "internal/controller/http_v1/controller.go")
	for _, op := range []string{"get", "list", "create", "update", "delete"} {
		imp := `user_` + op + ` "example.com/app/internal/usecases/users/user_` + op + `"`
		if n := countOccurrences(got, imp); n != 1 {
			t.Errorf("import for %s appears %d times, want 1", op, n)
		}
	}
	if n := countOccurrences(got, `usersGroup := v1.Group("/users")`); n != 1 {
		t.Errorf("route group appears %d times, want 1:\n%s", n, got)
	}
}
