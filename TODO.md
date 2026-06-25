To Do
========================================================================

- [ ] Implement equality comparisons correctly (`eqv?`, `eq?`, `equal?`)
- [ ] Change environment representation from hash table to assoc list.
- [ ] Make the command-line flag -trace available to lisp code via a global
  variable (e.g., #%trace).
- [ ] Implement S-expression comments.
  A `#;` starts an S-expression comment. When the reader encounters `#;`, it
  recursively reads one datum, and then discards it (continuing on to the next
  datum for the read result).
- [ ] Implement shebang comments.
  A `#! ` (which is `#!` followed by a space) or `#!/` starts a line comment that can
  be continued to the next line by ending a line with \. This form of comment
  normally appears at the beginning of a Unix script file.
- [ ] Strings are currently defined as `/"([^"\\]+|\\[^\n])*"/`. Implement a
  more complete string scan according to [parse-string](
  https://docs.racket-lang.org/reference/reader.html#%28part._parse-string%29).
- [ ] Numbers are currently defined as symbols that can be parsed by Go's
  strconv.ParseFloat. Instead, use Scheme (R5RS) or [Racket syntax](
  https://docs.racket-lang.org/reference/reader.html#%28part._parse-number%29).  
- [ ] (write obj) currently implements R7RS's write-shared. Implement
  (write-simple obj) which ignores sharing, and (write ...) which uses labels
  only for cycles (not for DAGs).
- [ ] Add port arguments to the read and write procedures.
- [ ] Check Racket's output for (display object) on oddball types (environment,
  eof, etc)
- [ ] Fix symbol input (convert "x\ y" and "|x y|" into a symbol named "x y").
- [ ] Fix symbol output (convert a symbol named "x y" into "|x y|" on output)
- [ ] Implement equal? that handles cyclic data structures.
- [ ] Implement `define-syntax` / `syntax-rules`. Once available, wire up the
  SAT-solver `amb` example in `test/SAT-solver.scm` (it uses `define-syntax`)
  as a test.


Done
========================================================================

- [X] Replace the simplistic lexer with a real one.


Type Assertion vs. Helper Method
------------------------------------------------------------------------
- [x] Find out which is faster: `sym, ok := expr.(*Symbol)` or `sym :=
  expr.AsSymbol()`.

Answer: The type assertion is about twice as fast as the `AsSymbol()` helper
method.  The benchmark is in `./bench/expr_bench_test.go`. Here's a sample run:
```
❯ ./BENCH.zsh
goos: darwin
goarch: arm64
pkg: LiSP/bench
cpu: Apple M4 Max
BenchmarkTypeAssertion-16    	1000000000	         0.6645 ns/op	       0 B/op	       0 allocs/op
BenchmarkMethodCall-16       	877630834	         1.454 ns/op	       0 B/op	       0 allocs/op
PASS
ok  	LiSP/bench	2.397s
```
