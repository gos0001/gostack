package commands

import (
	"fmt"
	"path/filepath"

	"github.com/gos0001/gostack/internal/scaffold"
	"github.com/gos0001/gostack/templates"
	"github.com/spf13/cobra"
)

var generatePageCmd = &cobra.Command{
	Use:     "page <path>",
	Aliases: []string{"p"},
	Short:   "Generate an SSR page handler + template",
	Example: "gostack g page home\n  gostack g page home/[user_id]",
	Args:    cobra.ExactArgs(1),
	RunE:    runGeneratePage,
}

func runGeneratePage(cmd *cobra.Command, args []string) error {
	pagePath := args[0]
	if err := scaffold.ValidatePagePath(pagePath); err != nil {
		return err
	}

	projectCtx, err := readProjectContext(".")
	if err != nil {
		return err
	}
	if !projectCtx.WithFrontend {
		return fmt.Errorf("this project was created as api-only — SSR pages are unavailable")
	}

	ctx := scaffold.NewPageContext(projectCtx, pagePath)
	eng := scaffold.NewEngine(templates.FS)

	// Generate page handler — use FsPagePath for directory (Go import path safe)
	handlerDest := scaffold.PageDir(ctx.FsPagePath)
	fmt.Printf("Generating page: %s → %s (gin: /%s)\n", pagePath, handlerDest, ctx.GinPath)

	if err := eng.RenderTree("page", handlerDest, ctx); err != nil {
		return fmt.Errorf("generate page handler: %w", err)
	}

	// Generate HTML template — use ViewPath (no brackets)
	tmplDest := filepath.Join("views", "pages", ctx.ViewPath+".html")
	if err := eng.RenderAltDelims("page_tmpl/page.html", tmplDest, ctx); err != nil {
		fmt.Printf("  warning: could not generate template: %v\n", err)
	}

	// Update routes.go
	if err := scaffold.AppendPageRoute(".", projectCtx.ModulePath, ctx); err != nil {
		fmt.Printf("  warning: could not update routes.go: %v\n", err)
	}

	// Register the page package with wire — without this the pages controller
	// asks for a *page.Handler that has no provider.
	pkgPath := scaffold.PagePkgPath(ctx.FsPagePath)
	if err := scaffold.AppendToWireBuild(".", projectCtx.ModulePath, pkgPath, scaffold.PageAlias(ctx.FsPagePath)); err != nil {
		fmt.Printf("  warning: could not update cmd/wire.go: %v\n", err)
	}

	scaffold.PostGenFeature(".")
	fmt.Printf("Done. Template: views/pages/%s.html\n", ctx.ViewPath)
	return nil
}
