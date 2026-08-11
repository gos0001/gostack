package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gos0001/gostack/internal/scaffold"
	"github.com/gos0001/gostack/templates"
	"github.com/spf13/cobra"
)

var crudWithPages bool

var generateCRUDCmd = &cobra.Command{
	Use:     "crud <entity-plural>",
	Short:   "Generate full CRUD (domain + 5 use cases + SQL + routes)",
	Example: "gostack g crud users\n  gostack g crud users --pages",
	Args:    cobra.ExactArgs(1),
	RunE:    runGenerateCRUD,
}

func init() {
	generateCRUDCmd.Flags().BoolVar(&crudWithPages, "pages", false, "Also generate SSR pages")
}

func runGenerateCRUD(cmd *cobra.Command, args []string) error {
	entityPlural := args[0]
	if err := scaffold.ValidateName(entityPlural); err != nil {
		return err
	}

	projectCtx, err := readProjectContext(".")
	if err != nil {
		return err
	}

	if crudWithPages && !projectCtx.WithFrontend {
		return fmt.Errorf("--pages needs a fullstack project; this one was created as api-only")
	}

	ctx := scaffold.NewCRUDContext(projectCtx, entityPlural)
	eng := scaffold.NewEngine(templates.FS)

	fmt.Printf("Generating CRUD for: %s (domain type: %s)\n", entityPlural, ctx.PascalEntity)
	if !projectCtx.WithPostgres {
		fmt.Println("  note: project has no postgres adapter — skipping SQL queries and sqlc")
	}

	// 1. Domain file
	domainDest := filepath.Join("internal", "domain", ctx.EntityName+".go")
	fmt.Printf("  → %s\n", domainDest)
	if err := eng.RenderFile("crud/domain.go.tmpl", domainDest, ctx); err != nil {
		return fmt.Errorf("generate domain: %w", err)
	}

	// The adapter set is only added to wire once something consumes it —
	// wire rejects a provider set nobody uses. These use cases are that
	// consumer. AppendToWireBuild is idempotent, so repeat runs are harmless.
	if projectCtx.WithPostgres {
		if err := scaffold.AppendToWireBuild(".", projectCtx.ModulePath,
			"internal/adapter/postgres", "postgresadapter"); err != nil {
			fmt.Printf("  warning: wire.go update for postgres adapter: %v\n", err)
		}
	}

	// 2. Five use case packages, grouped under internal/usecases/<plural>/
	groupDir := filepath.Join("internal", "usecases", filepath.FromSlash(ctx.GroupPath))
	ops := []string{"get", "list", "create", "update", "delete"}
	for _, op := range ops {
		pkgName := ctx.EntityName + "_" + op
		ucDest := filepath.Join(groupDir, pkgName)
		fmt.Printf("  → %s\n", ucDest)

		opCtx := ctx
		opCtx.FeatureName = pkgName
		opCtx.PackageName = pkgName
		opCtx.PascalName = scaffold.PascalFromContext(ctx, op)

		if err := eng.RenderTree("crud/op_"+op, ucDest, opCtx); err != nil {
			return fmt.Errorf("generate %s: %w", pkgName, err)
		}

		if err := scaffold.AppendToWireBuild(".", projectCtx.ModulePath, scaffold.UsecasePkgPath(opCtx), pkgName); err != nil {
			fmt.Printf("  warning: wire.go update for %s: %v\n", pkgName, err)
		}
	}

	// 3. Migration + SQL queries — only when the project has a postgres adapter.
	// The migration comes first: sqlc validates queries against the schema, so
	// without a CREATE TABLE it would reject the queries we just wrote.
	if projectCtx.WithPostgres {
		seq := nextMigrationSeq("migrations")
		for _, m := range []struct{ tmpl, suffix string }{
			{"crud/migration.up.sql.tmpl", "up"},
			{"crud/migration.down.sql.tmpl", "down"},
		} {
			dest := filepath.Join("migrations",
				fmt.Sprintf("%06d_create_%s.%s.sql", seq, ctx.EntityPlural, m.suffix))
			fmt.Printf("  → %s\n", dest)
			if err := eng.RenderFile(m.tmpl, dest, ctx); err != nil {
				fmt.Printf("  warning: generate migration: %v\n", err)
			}
		}

		sqlDest := filepath.Join("internal", "adapter", "postgres", "queries", ctx.EntityName+".sql")
		fmt.Printf("  → %s\n", sqlDest)
		if err := eng.RenderFile("crud/queries.sql.tmpl", sqlDest, ctx); err != nil {
			fmt.Printf("  warning: generate SQL: %v\n", err)
		}
	}

	// 4. Update http_v1 controller
	if err := scaffold.AppendCRUDRoutes(".", projectCtx.ModulePath, ctx); err != nil {
		fmt.Printf("  warning: update controller: %v\n", err)
	}

	// 5. Optional SSR pages
	if crudWithPages {
		fmt.Printf("  Generating SSR pages for %s...\n", entityPlural)
		generateCRUDPages(eng, projectCtx, ctx)
	}

	fmt.Println("\nRunning sqlc + wire...")
	scaffold.PostGenCRUD(".", projectCtx.WithPostgres)

	fmt.Printf(`
Done! Next steps:
  1. Add fields to internal/domain/%s.go
  2. Add columns to internal/adapter/postgres/queries/%s.sql
  3. Run: make generate  (sqlc → wire)
`, ctx.EntityName, ctx.EntityName)
	return nil
}

// nextMigrationSeq returns one past the highest NNNNNN_ prefix in dir, so
// generated migrations slot in after whatever is already there.
func nextMigrationSeq(dir string) int {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 1
	}
	max := 0
	for _, e := range entries {
		name := e.Name()
		idx := strings.IndexByte(name, '_')
		if idx <= 0 {
			continue
		}
		n, err := strconv.Atoi(name[:idx])
		if err == nil && n > max {
			max = n
		}
	}
	return max + 1
}

func generateCRUDPages(eng *scaffold.Engine, projectCtx, ctx scaffold.TemplateContext) {
	// List page: /<plural>   Detail page: /<plural>/[id]
	for _, pagePath := range []string{ctx.EntityPlural, ctx.EntityPlural + "/[id]"} {
		pageCtx := scaffold.NewPageContext(projectCtx, pagePath)
		pageCtx.EntityName = ctx.EntityName
		pageCtx.PascalEntity = ctx.PascalEntity
		emitPage(eng, projectCtx, pageCtx)
	}
}

// emitPage renders a page package plus its template and registers it with both
// the pages router and wire. Skipping either registration leaves the generated
// package as dead code.
func emitPage(eng *scaffold.Engine, projectCtx, pageCtx scaffold.TemplateContext) {
	dest := scaffold.PageDir(pageCtx.FsPagePath)
	fmt.Printf("  → %s\n", dest)
	if err := eng.RenderTree("page", dest, pageCtx); err != nil {
		fmt.Printf("  warning: page %s: %v\n", pageCtx.PagePath, err)
		return
	}

	tmplDest := filepath.Join("views", "pages", filepath.FromSlash(pageCtx.ViewPath)+".html")
	if err := eng.RenderAltDelims("page_tmpl/page.html", tmplDest, pageCtx); err != nil {
		fmt.Printf("  warning: template %s: %v\n", tmplDest, err)
	}

	if err := scaffold.AppendPageRoute(".", projectCtx.ModulePath, pageCtx); err != nil {
		fmt.Printf("  warning: routes.go for %s: %v\n", pageCtx.PagePath, err)
	}

	pkgPath := scaffold.PagePkgPath(pageCtx.FsPagePath)
	if err := scaffold.AppendToWireBuild(".", projectCtx.ModulePath, pkgPath, scaffold.PageAlias(pageCtx.FsPagePath)); err != nil {
		fmt.Printf("  warning: wire.go for %s: %v\n", pageCtx.PagePath, err)
	}
}
