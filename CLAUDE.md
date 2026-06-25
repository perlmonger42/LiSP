# LiSP — Claude notes

## Building and testing

The `github.com/bobappleyard/readline` dependency requires Homebrew readline
headers. Set these before any `go build` or `go test` invocation:

```zsh
export CGO_CFLAGS="-I$(brew --prefix readline)/include"
export CGO_LDFLAGS="-L$(brew --prefix readline)/lib"
```

The helper scripts (`RUN.zsh`, `COVER.zsh`, `BENCH.zsh`) already set these
exports internally, so running them directly works without manual setup.

### Expected linker warning

```
ld: warning: search path '/usr/local/opt/readline/lib' not found
```

This warning is harmless. The `bobappleyard/readline` package (last updated 2015)
hardcodes the old Homebrew path. The linker ignores the missing path and uses
`CGO_LDFLAGS` instead. See `RUN.zsh` for options to suppress it permanently.

## Scripts

| Script | Purpose |
|--------|---------|
| `RUN.zsh` | Build, test, and run integration tests |
| `COVER.zsh` | Like RUN.zsh but with `-cover` and opens HTML coverage report |
| `BENCH.zsh` | Runs type-assertion vs method-call benchmark in `cmd/type-assertion-benchmark` |

## Project layout

- `cmd/LiSP/` — the interpreter binary
- `internal/scan/` — the scanner (hand-written inline scanner, ~124 MB/s)
- `test/` — shell-script integration tests run by `RUN.zsh`

The six experimental lexers that led to `internal/scan` now live in their own
repo: https://github.com/perlmonger42/go-lexer-research. The reference Scheme
implementations (Norvig's `lis.py`, Kelchtermans' `scm.go`) are preserved on
the `reference-archive` branch of this repo.
