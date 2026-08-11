package commands

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/gos0001/gostack/internal/scaffold"
	"github.com/gos0001/gostack/templates"
	"github.com/spf13/cobra"
)

var newOpts newOptions

var newCmd = &cobra.Command{
	Use:   "new <project-name>",
	Short: "Create a new gostack project",
	Long: "Create a new gostack project.\n\n" +
		"Run without flags on a terminal to get an interactive wizard. Pass any\n" +
		"feature flag, or --yes, to skip it — useful in CI.",
	Example: "gostack new blog-app\n" +
		"  gostack new api-svc --api-only --postgres\n" +
		"  gostack new quick --yes",
	Args: cobra.ExactArgs(1),
	RunE: runNew,
}

func init() {
	f := newCmd.Flags()
	f.StringVar(&newOpts.module, "module", "", "Go module path (default github.com/user/<name>)")
	f.BoolVar(&newOpts.apiOnly, "api-only", false, "JSON API only — no views, static, or SSR pages")
	f.BoolVar(&newOpts.fullstack, "fullstack", false, "API + SSR pages + HTMX (default)")
	f.BoolVar(&newOpts.postgres, "postgres", false, "include PostgreSQL + sqlc")
	f.BoolVar(&newOpts.redis, "redis", false, "include Redis")
	f.BoolVar(&newOpts.docker, "docker", false, "include docker-compose + Dockerfiles")
	f.BoolVarP(&newOpts.yes, "yes", "y", false, "skip the wizard and accept defaults")
	newCmd.MarkFlagsMutuallyExclusive("api-only", "fullstack")
}

func runNew(cmd *cobra.Command, args []string) error {
	name := args[0]
	if err := scaffold.ValidateName(name); err != nil {
		return err
	}

	dest := filepath.Join(".", name)
	if err := scaffold.DirMustNotExist(dest); err != nil {
		return err
	}

	modulePath, feats, err := resolveOptions(cmd, name, &newOpts)
	if err != nil {
		if errors.Is(err, errCancelled) {
			fmt.Println("cancelled")
			return nil
		}
		return err
	}

	ctx := scaffold.NewProjectContext(name, modulePath, feats)
	eng := scaffold.NewEngine(templates.FS)

	fmt.Printf("Creating %s...\n", name)
	if err := eng.RenderLayers(dest, ctx, scaffold.ProjectLayers(ctx)...); err != nil {
		// Do not leave a half-written project behind.
		os.RemoveAll(dest)
		return fmt.Errorf("scaffold project: %w", err)
	}

	if err := scaffold.WriteManifest(dest, scaffold.Manifest{
		Version: 1, Name: name, Module: modulePath, Features: feats,
	}); err != nil {
		return fmt.Errorf("write %s: %w", scaffold.ManifestName, err)
	}

	fmt.Println("Installing tools and generating wire...")
	scaffold.PostGenProject(dest)

	fmt.Printf("\nDone! Run:\n  cd %s\n  gostack dev\n", name)
	fmt.Println("\nConventions for this project: .claude/skills/gostack/SKILL.md")
	return nil
}
