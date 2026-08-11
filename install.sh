#!/bin/sh
#
# gostack installer.
#
#   curl -fsSL https://raw.githubusercontent.com/gos0001/gostack/main/install.sh | sh
#
# Installs gostack with `go install` and puts the Go bin directory on your PATH.
# Go is a hard requirement rather than an inconvenience here: gostack shells out
# to `go mod tidy` and installs wire, sqlc and air with `go install`, so a machine
# without Go could not run a generated project anyway.
#
# Environment:
#   GOSTACK_VERSION          version to install (default: latest)
#   GOSTACK_NO_MODIFY_PATH   set to 1 to never touch your shell rc file
#
# Flags:
#   --no-modify-path         same as the variable above
#
# POSIX sh on purpose — piping into `sh` lands in dash on Debian, so no bashisms.

set -eu

MODULE="github.com/gos0001/gostack"
VERSION="${GOSTACK_VERSION:-latest}"
NO_MODIFY_PATH="${GOSTACK_NO_MODIFY_PATH:-0}"
MARKER="# gostack: Go bin dir on PATH"

# ---------------------------------------------------------------- output ----

if [ -t 1 ]; then
	BOLD=$(printf '\033[1m')
	RED=$(printf '\033[31m')
	GREEN=$(printf '\033[32m')
	YELLOW=$(printf '\033[33m')
	RESET=$(printf '\033[0m')
else
	BOLD=""; RED=""; GREEN=""; YELLOW=""; RESET=""
fi

info() { printf '%s\n' "$*"; }
warn() { printf '%swarning:%s %s\n' "$YELLOW" "$RESET" "$*" >&2; }
die() { printf '%serror:%s %s\n' "$RED" "$RESET" "$*" >&2; exit 1; }

# ------------------------------------------------------------------ args ----

while [ $# -gt 0 ]; do
	case "$1" in
	--no-modify-path) NO_MODIFY_PATH=1 ;;
	-h | --help)
		sed -n '3,20p' "$0" 2>/dev/null | sed 's/^# \{0,1\}//'
		exit 0
		;;
	*) die "unknown option: $1" ;;
	esac
	shift
done

# -------------------------------------------------------------- go check ----

command -v go >/dev/null 2>&1 || die "Go is not installed or not on PATH.
gostack needs it both to install itself and to run generated projects.
Get it from https://go.dev/dl/ and run this script again."

# The module declares go 1.26.5. An older toolchain refuses with a message about
# the go directive that reads like a bug in gostack, so catch it up front.
goversion=$(go env GOVERSION 2>/dev/null || echo "")
case "$goversion" in
go*)
	v=${goversion#go}
	major=${v%%.*}
	rest=${v#*.}
	minor=${rest%%.*}
	# Trim a pre-release suffix: "26rc1" -> "26".
	minor=${minor%%[!0-9]*}
	[ -n "$major" ] && [ -n "$minor" ] || minor=""
	if [ -n "$minor" ]; then
		if [ "$major" -lt 1 ] || { [ "$major" -eq 1 ] && [ "$minor" -lt 26 ]; }; then
			die "Go 1.26 or newer is required, found $goversion.
Upgrade from https://go.dev/dl/ and run this script again."
		fi
	else
		warn "could not parse Go version '$goversion' — skipping the version check"
	fi
	;;
*)
	warn "unrecognised Go version '$goversion' — skipping the version check"
	;;
esac

# ---------------------------------------------------------------- bindir ----

# Same resolution order as the CLI's own lookTool: GOBIN wins, else GOPATH/bin.
# This is also where `make tools` puts wire, sqlc and air, which the generated
# Makefile invokes by bare name — so one PATH entry covers everything.
bindir=$(go env GOBIN 2>/dev/null || echo "")
if [ -z "$bindir" ]; then
	gopath=$(go env GOPATH 2>/dev/null || echo "")
	[ -n "$gopath" ] || die "neither GOBIN nor GOPATH is set; cannot tell where to install"
	# GOPATH may be a list; `go install` writes to the first entry.
	bindir="${gopath%%:*}/bin"
fi

# --------------------------------------------------------------- install ----

info "Installing gostack ($VERSION) into $bindir ..."
if ! go install "$MODULE/cmd/gostack@$VERSION" 2>&1; then
	die "go install failed.
If the error mentions a module path conflict, the Go module proxy is serving a
stale answer for @latest; install an explicit tag instead:
  GOSTACK_VERSION=v0.2.0 sh install.sh"
fi

bin="$bindir/gostack"
[ -x "$bin" ] || die "go install reported success but $bin is missing"

# Older releases have no --version flag; that is not a reason to fail the install.
installed=$("$bin" --version 2>/dev/null) || installed=""

# ------------------------------------------------------------------ PATH ----

on_path=0
case ":$PATH:" in
*":$bindir:"*) on_path=1 ;;
esac

rc=""
rc_line=""
if [ "$on_path" -eq 0 ] && [ "$NO_MODIFY_PATH" != "1" ]; then
	case "$(basename "${SHELL:-}")" in
	zsh)
		rc="$HOME/.zshrc"
		# shellcheck disable=SC2016  # $PATH must reach the rc file unexpanded
		rc_line=$(printf 'export PATH="%s:$PATH"' "$bindir")
		;;
	bash)
		# macOS Terminal starts login shells, which read .bash_profile, not .bashrc.
		if [ "$(uname -s)" = "Darwin" ]; then
			rc="$HOME/.bash_profile"
		else
			rc="$HOME/.bashrc"
		fi
		# shellcheck disable=SC2016  # $PATH must reach the rc file unexpanded
		rc_line=$(printf 'export PATH="%s:$PATH"' "$bindir")
		;;
	fish)
		rc="${XDG_CONFIG_HOME:-$HOME/.config}/fish/config.fish"
		rc_line=$(printf 'fish_add_path %s' "$bindir")
		;;
	*)
		# Unknown shell: guessing the rc file would edit the wrong one silently.
		rc=""
		;;
	esac
fi

modified=0
if [ -n "$rc" ]; then
	mkdir -p "$(dirname "$rc")"
	[ -f "$rc" ] || : >"$rc"
	if grep -Fq "$MARKER" "$rc"; then
		: # already ours — a second run must not duplicate the entry
	else
		{
			printf '\n%s\n' "$MARKER"
			printf '%s\n' "$rc_line"
		} >>"$rc"
		modified=1
	fi
fi

# ---------------------------------------------------------------- report ----

printf '\n'
if [ -n "$installed" ]; then
	printf '%s%s%s installed to %s\n' "$GREEN" "$installed" "$RESET" "$bindir"
else
	printf '%sgostack%s installed to %s\n' "$GREEN" "$RESET" "$bindir"
fi

if [ "$on_path" -eq 1 ]; then
	printf '\n  %sgostack new my-app%s\n' "$BOLD" "$RESET"
	exit 0
fi

# Everything below is the PATH story. A script cannot change the PATH of the
# shell that invoked it — `curl | sh` runs in a subprocess — so the best we can
# do is edit the rc file and hand back one command for the current session.
if [ "$modified" -eq 1 ]; then
	printf 'Added %s to PATH in %s\n' "$bindir" "$rc"
	printf '\nActivate it in this shell:\n\n'
	printf '  %ssource %s%s\n' "$BOLD" "$rc" "$RESET"
	printf '\nThen:\n\n'
	printf '  %sgostack new my-app%s\n' "$BOLD" "$RESET"
elif [ -n "$rc" ]; then
	printf '%s already carries the PATH entry.\n' "$rc"
	printf '\nActivate it in this shell:\n\n'
	printf '  %ssource %s%s\n' "$BOLD" "$rc" "$RESET"
else
	if [ "$NO_MODIFY_PATH" = "1" ]; then
		printf 'Leaving your shell config alone as asked.\n'
	else
		printf 'Could not tell which shell config to edit (SHELL=%s).\n' "${SHELL:-unset}"
	fi
	printf '\nAdd this to your shell config, then reopen the terminal:\n\n'
	# shellcheck disable=SC2016  # printing the line for the user to copy, not running it
	printf '  %sexport PATH="%s:$PATH"%s\n' "$BOLD" "$bindir" "$RESET"
fi
