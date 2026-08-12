package commands

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/gos0001/gostack/internal/scaffold"
	"github.com/gos0001/gostack/templates"
	"github.com/spf13/cobra"
)

var ucOrchestrator string

var generateUCCmd = &cobra.Command{
	Use:   "uc <name>",
	Short: "Generate a use case package (business logic only)",
	Example: "gostack g uc get_user\n" +
		"  gostack g uc users/get_profile\n" +
		"  gostack g uc seed_super_admin --orchestrator bootstrap\n" +
		"  gostack g uc outbox_drain --orchestrator cron",
	Args: cobra.ExactArgs(1),
	RunE: runGenerateUC,
}

func init() {
	generateUCCmd.Flags().StringVar(&ucOrchestrator, "orchestrator", "",
		"Also generate a handler for this orchestrator and register it ("+
			strings.Join(scaffold.OrchestratorNames(), ", ")+")")
}

func runGenerateUC(cmd *cobra.Command, args []string) error {
	raw := args[0]
	if err := scaffold.ValidateFeaturePath(raw); err != nil {
		return err
	}
	if ucOrchestrator != "" {
		if err := scaffold.ValidateOrchestrator(ucOrchestrator); err != nil {
			return err
		}
	}
	group, name := scaffold.SplitGroup(raw)

	projectCtx, err := readProjectContext(".")
	if err != nil {
		return err
	}

	ctx := scaffold.NewFeatureContext(projectCtx, group, name, false, false)
	eng := scaffold.NewEngine(templates.FS)

	dest := filepath.Join("internal", "usecases", filepath.FromSlash(group), name)
	if err := checkPkgNameFree(ctx, dest); err != nil {
		return err
	}
	fmt.Printf("Generating use case: %s → %s\n", raw, dest)

	if err := eng.RenderTree("usecase", dest, ctx); err != nil {
		return fmt.Errorf("generate uc: %w", err)
	}

	if ucOrchestrator == "" {
		// Deliberately not added to cmd/wire.go: a bare use case has no
		// transport entry point yet, and wire rejects a provider set nothing
		// consumes. It gets wired when something asks for it — a page, an API
		// handler, an orchestrator, or another use case.
		scaffold.PostGenFeature(".")

		fmt.Printf("Done. Edit %s/usecase.go to add your logic.\n", dest)
		fmt.Printf("When something consumes it, add %s.Set to wire.Build in cmd/wire.go.\n", ctx.PackageName)
		return nil
	}

	// The orchestrator templates render over the use case ones into the same
	// directory: they add config.go and the handler file, and replace wire.go
	// with a Set that includes the handler's constructor. Same last-writer-wins
	// layering `gostack new` uses for its optional feature layers.
	if err := eng.RenderTree("orchestrator/"+ucOrchestrator, dest, ctx); err != nil {
		return fmt.Errorf("generate %s handler: %w", ucOrchestrator, err)
	}

	// Unlike the bare path above, registering with wire is correct here — the
	// orchestrator consumes the set the moment the task is appended below.
	if err := scaffold.AppendToWireBuild(".", projectCtx.ModulePath, scaffold.UsecasePkgPath(ctx), ctx.PackageName); err != nil {
		fmt.Printf("  warning: could not update cmd/wire.go: %v\n", err)
	}
	if err := scaffold.AppendOrchestratorTask(".", projectCtx.ModulePath, ucOrchestrator, ctx); err != nil {
		fmt.Printf("  warning: could not register with the %s orchestrator: %v\n", ucOrchestrator, err)
	}

	scaffold.PostGenFeature(".")

	fmt.Printf("Done. Edit %s/usecase.go to add your logic.\n", dest)
	fmt.Printf("Registered with the %s orchestrator — it is off until %s.\n",
		ucOrchestrator, scaffold.OrchestratorEnableHint(ucOrchestrator, name))
	return nil
}
