package commands

import (
	"errors"
	"os"
	"slices"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/gos0001/gostack/internal/scaffold"
)

type newOptions struct {
	module    string
	apiOnly   bool
	fullstack bool
	postgres  bool
	redis     bool
	docker    bool
	yes       bool
}

// featureFlagNames are the flags whose presence means "the caller already knows
// what they want" — the wizard is skipped entirely when any is set.
var featureFlagNames = []string{"api-only", "fullstack", "postgres", "redis", "docker"}

// errCancelled is returned when the user aborts the wizard with Ctrl+C.
var errCancelled = errors.New("cancelled")

func isInteractive() bool {
	return term.IsTerminal(int(os.Stdin.Fd())) && term.IsTerminal(int(os.Stdout.Fd()))
}

// defaultFeatures is what --yes and non-interactive runs get.
func defaultFeatures() scaffold.Features {
	return scaffold.Features{Frontend: true, Postgres: true, Redis: false, Docker: true}
}

// resolveOptions decides between the wizard, explicit flags, and defaults.
// The wizard only runs on a real terminal, with no --yes and no feature flags —
// otherwise CI and scripted runs would hang waiting for input.
func resolveOptions(cmd *cobra.Command, name string, o *newOptions) (string, scaffold.Features, error) {
	defaultModule := "github.com/user/" + name

	explicit := slices.ContainsFunc(featureFlagNames, func(f string) bool {
		return cmd.Flags().Changed(f)
	})

	if explicit || o.yes || !isInteractive() {
		module := strings.TrimSpace(o.module)
		if module == "" {
			module = defaultModule
		}
		if explicit {
			return module, scaffold.Features{
				Frontend: !o.apiOnly,
				Postgres: o.postgres,
				Redis:    o.redis,
				Docker:   o.docker,
			}, nil
		}
		return module, defaultFeatures(), nil
	}

	return runWizard(name, defaultModule, o.module)
}

func runWizard(name, defaultModule, preset string) (string, scaffold.Features, error) {
	module := preset
	if module == "" {
		module = defaultModule
	}
	kind := "fullstack"
	adapters := []string{"postgres", "docker"}

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Go module path").
				Description("Import prefix for every generated package.").
				Value(&module).
				Validate(func(s string) error {
					s = strings.TrimSpace(s)
					if s == "" {
						return errors.New("module path is required")
					}
					if strings.ContainsAny(s, " \t") {
						return errors.New("module path cannot contain spaces")
					}
					return nil
				}),

			huh.NewSelect[string]().
				Title("Project type").
				Options(
					huh.NewOption("Fullstack — JSON API + SSR pages + HTMX", "fullstack"),
					huh.NewOption("API only — JSON, no views/static/pages", "api"),
				).
				Value(&kind),

			huh.NewMultiSelect[string]().
				Title("Adapters").
				Description("space to toggle, enter to confirm").
				Options(
					huh.NewOption("PostgreSQL + sqlc", "postgres"),
					huh.NewOption("Redis", "redis"),
					huh.NewOption("Docker compose", "docker"),
				).
				Value(&adapters),
		),
	).WithTheme(huh.ThemeCharm())

	if err := form.Run(); err != nil {
		if errors.Is(err, huh.ErrUserAborted) {
			return "", scaffold.Features{}, errCancelled
		}
		return "", scaffold.Features{}, err
	}

	has := func(v string) bool { return slices.Contains(adapters, v) }
	return strings.TrimSpace(module), scaffold.Features{
		Frontend: kind == "fullstack",
		Postgres: has("postgres"),
		Redis:    has("redis"),
		Docker:   has("docker"),
	}, nil
}
