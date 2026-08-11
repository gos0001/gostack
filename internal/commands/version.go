package commands

import "runtime/debug"

// version is injected at build time with
// -ldflags "-X github.com/gos0001/gostack/internal/commands.version=v1.2.3".
// Installs via `go install pkg@version` cannot set it — there is no build step to
// pass flags to — so those fall through to the module version below.
var version = ""

// Version reports the version this binary was built from.
//
// The three cases differ by how the binary came to exist:
//
//	release build   ldflags set version
//	go install @tag toolchain stamps Main.Version, ldflags never ran
//	go build .      Main.Version is "(devel)" — report it as "dev"
func Version() string {
	if version != "" {
		return version
	}
	if info, ok := debug.ReadBuildInfo(); ok {
		if v := info.Main.Version; v != "" && v != "(devel)" {
			return v
		}
	}
	return "dev"
}
