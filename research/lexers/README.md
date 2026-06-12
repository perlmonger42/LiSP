# Scheme Lexer Performance Research Archive

This directory archives six Scheme lexer implementations built to identify the
fastest scanner for the LiSP interpreter. All six live in `package lexers`
and share the same `Token` and `Type` definitions (`types.go`).

The winning implementation — the inline hand-written scanner — became the
production scanner in `internal/scan`.

The output of an actual benchmark run is shown in the last section.

## Why We Built These

The original scanner (`internal/scan/scan.go`) was a port of Rob Pike's Ivy
lexer: a channel-based state machine driven by line-buffered `io.ByteReader`
reads. While correct, it felt unnecessarily complex, it wasn't obvious how fast
it was.  We built six alternatives to quantify the gap and identify the fastest
approach.

## The Six Scanners

### 1. State Machine — `01_state_machine.go` (~41 MB/s)

The original. Each scanner state is a function (`stateFn`) that processes some
input and returns the next state. Tokens are sent on a small buffered channel;
`Next()` drives the state machine by calling state functions until a token
appears. Input arrives via `io.ByteReader`, read one line at a time into an
internal buffer.

Constructor: `NewStateMachineScanner(name string, r io.ByteReader) *StateMachineScanner`

### 2. Regex List — `02_regex_list.go` (~7 MB/s)

Operates on an in-memory string. For each position, tries a slice of compiled
`*regexp.Regexp` values in order; the first that matches wins. This is O(N
matchers) per token in the common case and allocates per `FindString` call.
Slowest of the three regex approaches.

Constructor: `NewRegexListScanner(input string) *RegexListScanner`

### 3. Big Regex — `03_big_regex.go` (~5.7 MB/s)

Encodes the entire token grammar as a single compiled regex with named capture
groups. `FindStringSubmatchIndex` runs the NFA once per token, then scans the
subgroup index slice to identify which group matched. Theoretically one NFA
pass per token, but the 36-element index slice allocated on every call makes
this the slowest approach of all — allocation cost dominates.

Constructor: `NewBigRegexScanner(input string) *BigRegexScanner`

### 4. Dispatch Regex — `04_dispatch_regex.go` (~37 MB/s)

Inspects the first byte of the remaining input and dispatches to exactly the
right action, avoiding the O(matchers) search. Most tokens (parens, brackets,
quotes, whitespace, comments, `#f`, `#t`, unquote) are emitted without any
regex at all. Only four token forms still call a regex: strings, char
literals, bar-quoted symbols, and general word/number tokens.

Constructor: `NewDispatchRegexScanner(input string) *DispatchRegexScanner`

### 5. Inline (winner) — `05_inline.go` (~124 MB/s)

Same first-byte dispatch as the dispatch-regex scanner, but all four remaining
regex calls are replaced by small methods that scan bytes directly. No regex
engine is invoked at all. This eliminates all regex overhead, leaving only
direct byte comparisons and slice indexing. 3× faster than the original state
machine; this became the production `internal/scan` scanner.

Constructor: `NewInlineScanner(input string) *InlineScanner`

### 6. Slurped State Machine — `06_slurped_state_machine.go` (~65 MB/s)

A clean reimplementation of the Rob Pike / Ivy state-machine pattern (#1 above)
that operates on a pre-loaded string instead of an `io.ByteReader`. The
architecture is otherwise identical: `slurpedFn` state functions, channel-based
`emit()`, and `Next()` driving the machine with a `select`. The sole change is
the removal of `loadLine()` and all line-buffering machinery.

This scanner exists to answer the question: *how much of the original's overhead
came from `io.ByteReader` reads vs. the channel-based emission pattern?*

Result: removing the `io.ByteReader` gives **1.6×** improvement (41→65 MB/s).
The remaining gap to InlineScanner (**1.9×**, 65→124 MB/s) is the cost of the
channel emit pattern itself — the select, channel send, and channel receive that
occur for every token.

Constructor: `NewSlurpedStateMachineScanner(input string) *SlurpedStateMachineScanner`

## Performance Summary

Results from `go test -bench=. -benchmem -count=3` on an Apple M4 Max (~40 KB
synthetic Scheme corpus, 200× repetition). Speedup is relative to the original
state machine scanner.

| Scanner | Approach | Performance | Speedup |
|---------|----------|-------------|---------|
| State Machine | Rob Pike Ivy pattern, io.ByteReader + channel | ~41 MB/s | 1.0× (baseline) |
| Regex List | Try N regexes in order | ~7 MB/s | 0.2× |
| Big Regex | Single regex with named groups | ~5.7 MB/s | 0.1× |
| Dispatch Regex | First-byte dispatch + 4 regexes | ~37 MB/s | 0.9× |
| Slurped State Machine | State machine on pre-loaded string | ~65 MB/s | 1.6× |
| **Inline (winner)** | **First-byte dispatch + direct byte scanning** | **~124 MB/s** | **3.0×** |

### What the numbers reveal

The 3× gap between the original and the inline winner has two separable causes:

1. **io.ByteReader overhead** (scanners 1 vs 6): 1→1.6× improvement. Removing
   line-buffered `io.ByteReader` reads accounts for roughly half of the gap.

2. **Channel-emit overhead** (scanners 6 vs 5): 1.6→3.0× improvement. Replacing
   the channel send+receive per token with a direct function return accounts for
   the other half. Each `emit()` causes a channel send; each `Next()` call
   causes a channel receive. For ~28 000 tokens in the 40 KB corpus that's
   ~56 000 channel operations per benchmark iteration.

## Shared Types

All six scanners use the same `Token`, `Type`, and constants from `types.go`.
Helper functions shared by scanners 2–6 (`classifyWord`, `isDelimByte`,
`isHexByte`, `isOctByte`, `isAlphaByte`) also live there. The `namedCharacter`
and `CharLiteralToRune` functions are in `01_state_machine.go` since they are
specific to the state machine's character literal handling.

## Limitations Common to Scanners 2–6

- `#` appearing mid-symbol (e.g. `1#2`) is split into separate tokens.
- Nestable block comments (`#|…|#`) are not supported.
- Only ASCII whitespace (`[ \t\r\n\v\f]`) is treated as a delimiter.

The original state machine scanner (01) handles all of these correctly.

## Conclusion

The inline hand-written scanner achieves ~124 MB/s by eliminating all
intermediate abstractions: no regex engine, no channel, no `io.ByteReader`, no
line buffering — just a switch on the first byte and tight byte loops. The
slurped state machine (06) pins down exactly which abstractions are expensive:
`io.ByteReader` and channel emission each cost roughly the same amount, and
together they account for the full 3× gap between the original and the winner.


# Actual Benchmark Results

```
$ system_profiler SPHardwareDataType | egrep -i 'model|chip|core|memory'
2026-06-12 08:25:28.165 system_profiler[84986:24497708] hw.cpufamily: 0x17d5b93a
      Model Name: MacBook Pro
      Model Identifier: Mac16,5
      Model Number: MX2W3LL/A
      Chip: Apple M4 Max
      Total Number of Cores: 16 (12 Performance and 4 Efficiency)
      Memory: 48 GB
```

```
$ go test -bench . -benchmem -count=3
goos: darwin
goarch: arm64
pkg: LiSP/research/lexers
cpu: Apple M4 Max
BenchmarkStateMachine-16           	     225	   5026208 ns/op	  40.07 MB/s	 2416659 B/op	  106209 allocs/op
BenchmarkStateMachine-16           	     238	   5019664 ns/op	  40.12 MB/s	 2416659 B/op	  106209 allocs/op
BenchmarkStateMachine-16           	     236	   5047831 ns/op	  39.90 MB/s	 2416656 B/op	  106209 allocs/op
BenchmarkRegexList-16              	      40	  29189946 ns/op	   6.90 MB/s	 2189707 B/op	  100015 allocs/op
BenchmarkRegexList-16              	      40	  29260070 ns/op	   6.88 MB/s	 2194024 B/op	  100015 allocs/op
BenchmarkRegexList-16              	      36	  29713531 ns/op	   6.78 MB/s	 2194942 B/op	  100016 allocs/op
BenchmarkBigRegex-16               	      31	  36239987 ns/op	   5.56 MB/s	27817300 B/op	  188866 allocs/op
BenchmarkBigRegex-16               	      32	  36022091 ns/op	   5.59 MB/s	27818751 B/op	  188867 allocs/op
BenchmarkBigRegex-16               	      32	  36284635 ns/op	   5.55 MB/s	27819545 B/op	  188874 allocs/op
BenchmarkDispatchRegex-16          	     214	   5648932 ns/op	  35.65 MB/s	 2190195 B/op	  100010 allocs/op
BenchmarkDispatchRegex-16          	     211	   5629863 ns/op	  35.77 MB/s	 2189401 B/op	  100010 allocs/op
BenchmarkDispatchRegex-16          	     212	   5637819 ns/op	  35.72 MB/s	 2190495 B/op	  100011 allocs/op
BenchmarkSlurpedStateMachine-16    	     386	   3122672 ns/op	  64.50 MB/s	 2173062 B/op	  100003 allocs/op
BenchmarkSlurpedStateMachine-16    	     384	   3157545 ns/op	  63.78 MB/s	 2173060 B/op	  100003 allocs/op
BenchmarkSlurpedStateMachine-16    	     380	   3157199 ns/op	  63.79 MB/s	 2173057 B/op	  100003 allocs/op
BenchmarkInline-16                 	     741	   1621825 ns/op	 124.18 MB/s	 2172820 B/op	  100000 allocs/op
BenchmarkInline-16                 	     739	   1627370 ns/op	 123.76 MB/s	 2172819 B/op	  100000 allocs/op
BenchmarkInline-16                 	     740	   1629227 ns/op	 123.62 MB/s	 2172820 B/op	  100000 allocs/op
PASS
ok  	LiSP/research/lexers	26.464s
```
