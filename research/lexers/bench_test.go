package lexers

import (
	"strings"
	"testing"
)

// schemeCorpus is a representative slice of Scheme source exercising all
// major token types: parens, brackets, fixnums, flonums, strings, char
// literals, #t/#f, quote, symbols, and line comments.
//
// It deliberately avoids '#' mid-symbol and nestable block comments so that
// all six scanners produce identical token streams.
const schemeCorpus = `; arithmetic and recursion
(define (fact n) (if (= n 0) 1 (* n (fact (- n 1)))))
(define (fib n) (cond ((= n 0) 0) ((= n 1) 1) (else (+ (fib (- n 1)) (fib (- n 2))))))
; numeric literals: fixnum, flonum, hex, negative
(define pi 3.14159265358979)
(define e 2.71828182845904)
(define avogadro 6.022e23)
(define small .00001)
(define big 0x7fffffff)
(define neg -42)
(define zero 0)
; string literals
(define greeting "Hello, world!")
(define escaped "quote:\"yep\" backslash:\\done")
; booleans and character literals
(define bools (list #t #f #t #f #t #f))
(define chars (list #\space #\newline #\tab #\a #\Z #\u22EE #\U1D417))
; quoting
'(quoted a b c d e f)
; brackets
[let ([x 1] [y 2] [z 3.0]) (+ x y z)]
; lambda and higher-order
(lambda (a b c d) (list a b c d))
(map (lambda (x) (* x x)) '(1 2 3 4 5 6 7 8 9 10))
(for-each (lambda (x) x) (list 0 1 2 3 4 5 6 7 8 9))
; various symbol shapes
(define snake_case 42)
(define kebab-case-name 42)
(define CamelCase 42)
(define set! void)
(define with? #t)
`

// makeLargeSchemeInput repeats schemeCorpus ~200 times for stable measurements
// (~40 KB).
func makeLargeSchemeInput() string {
	const reps = 200
	var sb strings.Builder
	sb.Grow(len(schemeCorpus) * reps)
	for i := 0; i < reps; i++ {
		sb.WriteString(schemeCorpus)
	}
	return sb.String()
}

var tokenSink Token

// BenchmarkStateMachine benchmarks the original Rob Pike-style state machine
// scanner (~40 MB/s). It reads from a strings.Reader to satisfy io.ByteReader.
func BenchmarkStateMachine(b *testing.B) {
	input := makeLargeSchemeInput()
	b.SetBytes(int64(len(input)))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		scanner := NewStateMachineScanner("<bench>", strings.NewReader(input))
		for {
			tok := scanner.Next()
			tokenSink = tok
			if tok.Type == EOF {
				break
			}
		}
	}
}

// BenchmarkRegexList benchmarks the list-of-matchers regex scanner (~7 MB/s).
func BenchmarkRegexList(b *testing.B) {
	input := makeLargeSchemeInput()
	b.SetBytes(int64(len(input)))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		scanner := NewRegexListScanner(input)
		for {
			tok := scanner.Next()
			tokenSink = tok
			if tok.Type == EOF {
				break
			}
		}
	}
}

// BenchmarkBigRegex benchmarks the single named-group regex scanner (~5 MB/s).
func BenchmarkBigRegex(b *testing.B) {
	input := makeLargeSchemeInput()
	b.SetBytes(int64(len(input)))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		scanner := NewBigRegexScanner(input)
		for {
			tok := scanner.Next()
			tokenSink = tok
			if tok.Type == EOF {
				break
			}
		}
	}
}

// BenchmarkDispatchRegex benchmarks the first-byte dispatch + 4-regex scanner
// (~35 MB/s).
func BenchmarkDispatchRegex(b *testing.B) {
	input := makeLargeSchemeInput()
	b.SetBytes(int64(len(input)))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		scanner := NewDispatchRegexScanner(input)
		for {
			tok := scanner.Next()
			tokenSink = tok
			if tok.Type == EOF {
				break
			}
		}
	}
}

// BenchmarkSlurpedStateMachine benchmarks the state-machine scanner rewritten
// on a pre-loaded string (no io.ByteReader, no line buffering). This isolates
// the channel-emit overhead from the io.ByteReader overhead of the original.
func BenchmarkSlurpedStateMachine(b *testing.B) {
	input := makeLargeSchemeInput()
	b.SetBytes(int64(len(input)))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		scanner := NewSlurpedStateMachineScanner(input)
		for {
			tok := scanner.Next()
			tokenSink = tok
			if tok.Type == EOF {
				break
			}
		}
	}
}

// BenchmarkInline benchmarks the winning inline hand-written scanner (~123 MB/s).
func BenchmarkInline(b *testing.B) {
	input := makeLargeSchemeInput()
	b.SetBytes(int64(len(input)))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		scanner := NewInlineScanner(input)
		for {
			tok := scanner.Next()
			tokenSink = tok
			if tok.Type == EOF {
				break
			}
		}
	}
}
