# LiSP

Experiments in Lisp intepreters and compilers, inspired by
Christian Queinnec's __Lisp in Small Pieces__,
and based on
[Pieter Kelchtermans](https://github.com/pkelchte)'s
[`scm.go`](https://gist.github.com/pkelchte/c2bd76b9f8f9cd603b3c).

# Prerequisites

MacOS ships with `libedit`, not GNU Readline, so you'll need to install
readline and export a couple of environment variables for cgo to use
(when `go build` runs).

```zsh
brew install readline
export CGO_CFLAGS="-I$(brew --prefix readline)/include"
export CGO_LDFLAGS="-L$(brew --prefix readline)/lib"
```

You will get a warning from `ld`:
```
ld: warning: search path '/usr/local/opt/readline/lib' not found
```

This is harmless. It happens because `github.com/bobappleyard/readline`
hardcodes a path that isn't present on MacOS.

# Build and Run

`./RUN.zsh` will build the executable and run a few tests.

Then you can run `./LiSP` to get a REPL.

You can see a few a sample evaluations by running

    ./LiSP < test/sample-input.scm

# History

This project began as an exploration of the ideas in Christian Queinnec's
__Lisp in Small Pieces__, but it has grown into a Scheme interpreter in its own
right — in particular a vehicle for learning how to implement first-class
continuations.

A few stepping-stones along the way have been moved out of the main tree to
keep it focused:

- The six experimental lexers used to find the fastest scanner (the inline
  winner became `internal/scan`) now live in their own repository:
  [perlmonger42/go-lexer-research](https://github.com/perlmonger42/go-lexer-research).
- The reference implementations this work is based on — Peter Norvig's
  [`lis.py`](http://norvig.com/lispy.html) and Pieter Kelchtermans'
  [`scm.go`](https://gist.github.com/pkelchte/c2bd76b9f8f9cd603b3c) — are
  preserved on the [`reference-archive`](../../tree/reference-archive) branch.
