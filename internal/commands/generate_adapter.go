package commands

import (
	"fmt"
	"path"
	"path/filepath"

	"github.com/gos0001/gostack/internal/scaffold"
	"github.com/gos0001/gostack/templates"
	"github.com/spf13/cobra"
)

// reservedAdapters ship with `gostack new` flags and have their own
// connection layer under pkg/. This generator scaffolds the stateful
// in-process shape only, so those names are taken.
var reservedAdapters = map[string]string{
	"postgres": "--postgres",
	"redis":    "--redis",
}

var generateAdapterCmd = &cobra.Command{
	Use:   "adapter <name>",
	Short: "Generate a stateful in-memory adapter (constructor + example storage)",
	Example: "gostack g adapter inmemory\n" +
		"  gostack g adapter rate_limit",
	Args: cobra.ExactArgs(1),
	RunE: runGenerateAdapter,
}

func runGenerateAdapter(cmd *cobra.Command, args []string) error {
	raw := args[0]
	if err := scaffold.ValidateName(raw); err != nil {
		return err
	}

	projectCtx, err := readProjectContext(".")
	if err != nil {
		return err
	}

	ctx := scaffold.NewFeatureContext(projectCtx, "", raw, false, false)
	if flag, reserved := reservedAdapters[ctx.PackageName]; reserved {
		return fmt.Errorf(
			"adapter %q ships with `gostack new %s` and has its own connection layer under pkg/ — "+
				"this generator is for stateful in-process adapters, pick another name",
			raw, flag)
	}

	dest := filepath.Join("internal", "adapter", ctx.PackageName)
	if err := checkWireAliasFree(ctx.PackageName,
		path.Join(ctx.ModulePath, "internal", "adapter", ctx.PackageName), dest); err != nil {
		return err
	}

	fmt.Printf("Generating adapter: %s → %s\n", raw, dest)

	eng := scaffold.NewEngine(templates.FS)
	if err := eng.RenderTree("adapter", dest, ctx); err != nil {
		return fmt.Errorf("generate adapter: %w", err)
	}

	scaffold.PostGenFeature(".")

	fmt.Printf("Done. Extend it under %s — one storage per file; example_storage.go shows the shape.\n", dest)
	fmt.Printf("Not in cmd/wire.go yet: add %s.Set when the first use case takes *%s.Adapter.\n",
		ctx.PackageName, ctx.PackageName)
	return nil
}
