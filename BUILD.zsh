#!/usr/bin/env zsh
set -e  # exit as soon as any command returns a nonzero status

GOMOD=$(go env GOMOD)
if [[ $GOMOD = /dev/null ]]; then
  script_dir=${0:A:h}
  echo "running in script_dir: ${script_dir}"
  cd "${script_dir}"
  GOMOD=$(go env GOMOD)
  if [[ $GOMOD = /dev/null ]]; then
    echo 1>&2 "$0 must be run with current working directory inside a Go project"
    exit 1
  fi
fi

PROJECT=${GOMOD:h}  # the root of the Go project (dir containing `go.mod`)
cd "${PROJECT}"

# go clean -cache -testcache
export CGO_CFLAGS="-I$( brew --prefix readline)/include"
export CGO_LDFLAGS="-L$(brew --prefix readline)/lib"
go generate internal/scan/scan.go
go build -o ./LiSP ./cmd/LiSP

# You can ignore this warning:
#     ld: warning: search path '/usr/local/opt/readline/lib' not found
# The repo bobappleyard/readline was last updated 7 Jul 2015.  It has that path
# hardcoded, because it was valid on older versions of macOS.  The linker
# ignores the missing path, and finds readline via the CGO_LDFLAGS set by this
# script, and links successfully.
#
# CONSIDER these options to avoid the warning:
# 1. Fork bobappleyard/readline with and fix the path in these lines of readline.go:
#       37:#cgo darwin CFLAGS: -I/usr/local/opt/readline/include
#       38:#cgo darwin LDFLAGS: -L/usr/local/opt/readline/lib
# 2. Replace bobappleyard/readline with a maintained library, like one of these:
#   - github.com/chzyer/readline
#   - golang.org/x/term
# 3. Suppress it with a compatibility symlink:
#       sudo mkdir -p /usr/local/opt
#       sudo ln -s /opt/homebrew/opt/readline /usr/local/opt/readline
#   That makes the hardcoded path valid and the warning disappears.
#   It's a one-time fix and safe — many older packages have the same
#   assumption.
#
#   If the warning persists after creating the symlink, run `go clean -cache`.
#   (The issue is Go's build cache — the linker flags were evaluated
#   before the symlink existed, and the cached result is being reused.)

# `brew install readline` said this:
#     For compilers to find readline you may need to set:
#       export LDFLAGS="-L/opt/homebrew/opt/readline/lib"
#       export CPPFLAGS="-I/opt/homebrew/opt/readline/include"
#
#     For pkgconf to find readline you may need to set:
#       export PKG_CONFIG_PATH="/opt/homebrew/opt/readline/lib/pkgconfig"
