package commands

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
)

var devCmd = &cobra.Command{
	Use:   "dev",
	Short: "Start development server (uses air if installed)",
	RunE:  runDev,
}

func runDev(_ *cobra.Command, _ []string) error {
	env := devEnv()

	if _, err := exec.LookPath("air"); err == nil {
		cmd := exec.Command("air", airArgs()...)
		cmd.Env = env
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	}

	fmt.Println("air not found, falling back to go run ./cmd")
	fmt.Println("Install air for hot reload: go install github.com/air-verse/air@latest")

	cmd := exec.Command("go", "run", "./cmd")
	cmd.Env = env
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// airArgs points air at this project's config explicitly. Run bare, air only
// looks for ".air.toml" and silently falls back to its own defaults — building
// "." into ./tmp/main — which fails here, since main lives in ./cmd.
// "air.toml" is the name older gostack versions generated.
func airArgs() []string {
	for _, name := range []string{".air.toml", "air.toml"} {
		if _, err := os.Stat(name); err == nil {
			return []string{"-c", name}
		}
	}

	// No config at all: override air's defaults rather than let it guess.
	return []string{
		"-build.cmd", "go build -o ./bin/app ./cmd",
		"-build.bin", "./bin/app",
	}
}

// devEnv is the environment the dev server runs under: the current one plus
// .env.development. The generated app reads its config straight from the
// environment (envconfig, no dotenv loader), and air cannot source a file — so
// without this, POSTGRES_DSN and REDIS_URL are simply missing and the app dies
// on startup. Variables already exported in the shell win over the file.
func devEnv() []string {
	env := os.Environ()

	present := make(map[string]bool, len(env))
	for _, kv := range env {
		if k, _, ok := strings.Cut(kv, "="); ok {
			present[k] = true
		}
	}

	for _, name := range []string{".env.development", ".env"} {
		vars, err := readEnvFile(name)
		if err != nil {
			continue
		}
		for _, kv := range vars {
			if k, _, ok := strings.Cut(kv, "="); ok && !present[k] {
				present[k] = true
				env = append(env, kv)
			}
		}
		break
	}

	return env
}

// readEnvFile parses the subset of dotenv syntax the generated .env files use:
// KEY=value lines, blanks, # comments, an optional "export " prefix and
// optional surrounding quotes. Values are taken literally — no interpolation.
func readEnvFile(name string) ([]string, error) {
	f, err := os.Open(name)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var vars []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimPrefix(line, "export ")

		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}

		value = strings.TrimSpace(value)
		if len(value) >= 2 && (value[0] == '"' || value[0] == '\'') && value[len(value)-1] == value[0] {
			value = value[1 : len(value)-1]
		}

		vars = append(vars, key+"="+value)
	}

	return vars, scanner.Err()
}
