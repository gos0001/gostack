package scaffold

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// PostGenProject runs post-generation commands after `gostack new`.
// Failures are non-fatal — printed as warnings.
func PostGenProject(projectDir string) {
	run(projectDir, "go", "mod", "tidy")
	run(projectDir, "go", "install", "github.com/google/wire/cmd/wire@latest")
	run(projectDir, "go", "install", "github.com/sqlc-dev/sqlc/cmd/sqlc@latest")
	run(projectDir, "go", "install", "github.com/air-verse/air@latest")
	// Generate wire_gen.go — without it the project has no InitializeApp and
	// will not build.
	runWire(projectDir)
	runFmt(projectDir)
}

// PostGenFeature runs wire + gofmt after generating a feature.
func PostGenFeature(projectDir string) {
	runWire(projectDir)
	runFmt(projectDir)
}

// PostGenCRUD runs sqlc → wire → gofmt after CRUD generation.
// sqlc must run first: wire cannot compile the adapter package until the
// generated/ package exists.
func PostGenCRUD(projectDir string, withPostgres bool) {
	if withPostgres {
		runTool(projectDir, "sqlc", "generate")
		// Generated code may pull in new modules (uuid, pgtype helpers).
		run(projectDir, "go", "mod", "tidy")
	}
	runWire(projectDir)
	runFmt(projectDir)
}

// runWire regenerates cmd/wire_gen.go.
func runWire(dir string) {
	runTool(dir, "wire", "./cmd/")
}

// runTool runs a Go-installed helper binary, falling back to GOPATH/bin when it
// is not on PATH — `go install` puts it there, and that directory is frequently
// absent from a non-login shell's PATH.
func runTool(dir, name string, args ...string) {
	bin, err := lookTool(name)
	if err != nil {
		fmt.Printf("  warning: %s not found — run `make tools` in the project, then `make generate`\n", name)
		return
	}
	cmd := exec.Command(bin, args...)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Printf("  warning: %s %v: %v\n", name, args, err)
	}
}

// lookTool resolves a helper binary by PATH first, then GOBIN, then GOPATH/bin.
func lookTool(name string) (string, error) {
	if p, err := exec.LookPath(name); err == nil {
		return p, nil
	}
	for _, dir := range goBinDirs() {
		candidate := filepath.Join(dir, name)
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("%s not found", name)
}

func goBinDirs() []string {
	var dirs []string
	if gobin := strings.TrimSpace(goEnv("GOBIN")); gobin != "" {
		dirs = append(dirs, gobin)
	}
	if gopath := strings.TrimSpace(goEnv("GOPATH")); gopath != "" {
		// GOPATH may be a list; the first entry is where `go install` writes.
		for _, p := range filepath.SplitList(gopath) {
			dirs = append(dirs, filepath.Join(p, "bin"))
		}
	}
	if home, err := os.UserHomeDir(); err == nil {
		dirs = append(dirs, filepath.Join(home, "go", "bin"))
	}
	return dirs
}

func goEnv(key string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	out, err := exec.Command("go", "env", key).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func run(dir string, name string, args ...string) {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Printf("  warning: %s %v: %v\n", name, args, err)
	}
}

func runFmt(dir string) {
	cmd := exec.Command("gofmt", "-w", ".")
	cmd.Dir = dir
	_ = cmd.Run()
}
