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

export CGO_CFLAGS="-I$( brew --prefix readline)/include"
export CGO_LDFLAGS="-L$(brew --prefix readline)/lib"
echo Build program
go generate internal/scan/scan.go || (echo go generate failed && false)
go build -o ./LiSP ./cmd/LiSP || (echo go build failed && false)

echo Run RERCL tests
./LiSP -test test/test-all.rercl

echo Test continuations
(cat test/pythagorean-triples.scm; echo '(3 4 5)') | ./LiSP -test -

echo Done
