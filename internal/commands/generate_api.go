package commands

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/gos0001/gostack/internal/scaffold"
	"github.com/gos0001/gostack/templates"
	"github.com/spf13/cobra"
)

var apiMethod string

var generateAPICmd = &cobra.Command{
	Use:     "api <name>",
	Aliases: []string{"a"},
	Short:   "Generate a JSON API use case (usecase + http_v1 handler)",
	Example: "gostack g api create_post\n" +
		"  gostack g api users/ban_user\n" +
		"  gostack g api list_tags --method GET",
	Args: cobra.ExactArgs(1),
	RunE: runGenerateAPI,
}

func init() {
	generateAPICmd.Flags().StringVar(&apiMethod, "method", "POST",
		"HTTP method for the generated route (GET, POST, PUT, PATCH, DELETE)")
}

func runGenerateAPI(cmd *cobra.Command, args []string) error {
	raw := args[0]
	if err := scaffold.ValidateFeaturePath(raw); err != nil {
		return err
	}
	group, name := scaffold.SplitGroup(raw)

	projectCtx, err := readProjectContext(".")
	if err != nil {
		return err
	}

	ctx := scaffold.NewFeatureContext(projectCtx, group, name, false, true)
	eng := scaffold.NewEngine(templates.FS)

	dest := filepath.Join("internal", "usecases", filepath.FromSlash(group), name)
	if err := checkPkgNameFree(ctx, dest); err != nil {
		return err
	}
	fmt.Printf("Generating API: %s → %s\n", raw, dest)

	if err := eng.RenderTree("api", dest, ctx); err != nil {
		return fmt.Errorf("generate api: %w", err)
	}

	if err := scaffold.AppendToWireBuild(".", projectCtx.ModulePath, scaffold.UsecasePkgPath(ctx), ctx.PackageName); err != nil {
		fmt.Printf("  warning: could not update cmd/wire.go: %v\n", err)
	}

	// Register the route straight away: wire rejects a provider set nothing
	// consumes, so an unrouted handler would break the build.
	route := "/" + strings.ReplaceAll(name, "_", "-")
	if err := scaffold.AppendAPIRoute(".", projectCtx.ModulePath, ctx, apiMethod, route); err != nil {
		fmt.Printf("  warning: could not register route: %v\n", err)
	}

	scaffold.PostGenFeature(".")
	fmt.Printf("Done. Register route in internal/controller/http_v1/controller.go\n")
	return nil
}
