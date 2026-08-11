package commands

import (
	"bufio"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strings"

	"github.com/gos0001/gostack/internal/scaffold"
)

// readProjectContext loads the project manifest to build a base TemplateContext
// for feature generators. Projects created before manifests existed fall back to
// go.mod plus filesystem probing.
func readProjectContext(dir string) (scaffold.TemplateContext, error) {
	m, err := scaffold.ReadManifest(dir)
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			return scaffold.TemplateContext{}, err
		}

		modPath, modErr := readModulePath(dir)
		if modErr != nil {
			return scaffold.TemplateContext{}, fmt.Errorf(
				"not a gostack project (no %s, no go.mod): %w", scaffold.ManifestName, modErr)
		}
		parts := strings.Split(modPath, "/")
		m = scaffold.Manifest{
			Version:  1,
			Name:     parts[len(parts)-1],
			Module:   modPath,
			Features: scaffold.DetectFeatures(dir),
		}
	}

	return scaffold.NewProjectContext(m.Name, m.Module, m.Features), nil
}

// checkPkgNameFree rejects a new usecase whose package name is already taken by
// a different directory. Wire aliases packages by name, so two packages called
// get_profile in different groups would collide in cmd/wire.go.
func checkPkgNameFree(ctx scaffold.TemplateContext, dest string) error {
	data, err := os.ReadFile("cmd/wire.go")
	if err != nil {
		return nil // no wire.go yet — nothing to collide with
	}

	wantPath := ctx.ModulePath + "/" + scaffold.UsecasePkgPath(ctx)
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, ctx.PackageName+" \"") {
			continue
		}
		existing := strings.Trim(strings.TrimPrefix(line, ctx.PackageName+" "), `"`)
		if existing != wantPath {
			return fmt.Errorf(
				"a usecase package named %q already exists at %s\n"+
					"wire aliases packages by name, so %s would collide with it — pick a different name",
				ctx.PackageName, existing, dest)
		}
	}
	return nil
}

func readModulePath(dir string) (string, error) {
	f, err := os.Open(dir + "/go.mod")
	if err != nil {
		return "", err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "module ") {
			return strings.TrimPrefix(line, "module "), nil
		}
	}
	return "", fmt.Errorf("module directive not found in go.mod")
}
