#!/usr/bin/env zsh
set -e  # exit as soon as any command returns a nonzero status

source ./BUILD.zsh
go test ./cmd/LiSP/... ./internal/... || (echo go test failed && false)
test/script.sh || (echo test script failed && false)
