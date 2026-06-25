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

# Unit tests — write coverage to .coverage-data-files/
export GOCOVERDIR=.coverage-data-files
rm -rf $GOCOVERDIR && mkdir $GOCOVERDIR

# Build a coverage-instrumented binary for integration tests
go generate internal/scan/scan.go
go build -cover -o ./LiSP ./cmd/LiSP || (echo go build failed && false)
go test -cover ./cmd/LiSP/... ./internal/... || (echo go test failed && false)

# Integration tests — the binary writes profiles to GOCOVERDIR automatically
echo Running test/script.sh
./test/script.sh
echo Testing test/test-all.rercl
./LiSP -test test/test-all.rercl

echo Testing test/pythagorean-triples.scm
(cat test/pythagorean-triples.scm; echo '(3 4 5)') | ./LiSP -test -

# Merge all coverage data into a single text profile and open HTML report
go tool covdata textfmt -i=$GOCOVERDIR -o=.coverage-data.txt
go tool cover -html=.coverage-data.txt

# Old COVER.szh:
# ==============
### #!/usr/bin/env zsh
### set -e  # exit as soon as any command returns a nonzero status
###
### GOMOD=$(go env GOMOD)
### if [[ $GOMOD = /dev/null ]]; then
###   script_dir=${0:A:h}
###   echo "running in script_dir: ${script_dir}"
###   cd "${script_dir}"
###   GOMOD=$(go env GOMOD)
###   if [[ $GOMOD = /dev/null ]]; then
###     echo 1>&2 "$0 must be run with current working directory inside a Go project"
###     exit 1
###   fi
### fi
###
### PROJECT=${GOMOD:h}  # the root of the Go project (dir containing `go.mod`)
### cd "${PROJECT}"
###
### export GOCOVERDIR=.coverage-data-files
### rm -rf $GOCOVERDIR && mkdir $GOCOVERDIR
###
### export CGO_CFLAGS="-I$(brew --prefix readline)/include"
### export CGO_LDFLAGS="-L$(brew --prefix readline)/lib"
### go build -cover -o ./LiSP ./cmd/LiSP \
###     && go test ./... \
###     && test/script.sh \
###     && ./LiSP -e '(+ 2 3)' \
###     && (echo '(* 2 3)' | ./LiSP) \
###     && (echo '(* 2 3)' | ./LiSP - test/sample-input.scm) \
###     && (./LiSP --rapunzel && echo Should have failed || echo Okay) \
### || echo FAILED
###
### echo "Coverage Data:"
### ls $GOCOVERDIR
###
### go tool covdata textfmt -i=$GOCOVERDIR -o=.coverage-data.txt
### go tool cover -html=.coverage-data.txt
