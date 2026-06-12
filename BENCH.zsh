#!/usr/bin/env zsh
# This benchmark compares the speed of Go's type assertions to
# the speed of an `x.AsSymbol()` method that returns nil if x isn't a Symbol.
# Answer: Type assertions are faster.


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

go test ./cmd/type-assertion-benchmark -bench=.
