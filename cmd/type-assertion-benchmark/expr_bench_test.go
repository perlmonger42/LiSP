package bench

import "testing"

// In this program, x.AsSymbol() returns x if x is a Symbol,
// and nil if x is not a Symbol.
//
// Which is faster, `x.AsSymbol()` or  `x.(*Symbol)`?
// I.e., is it faster to define a method, or to use Go's type assertion?
//
// Answer: Type assertions are faster by a factor of ~2.4.
//
//     ❯ cd ~/g/src/Go/LiSP
//     ❯ ./BENCH.zsh
//     goos: darwin
//     goarch: arm64
//     pkg: LiSP/cmd/type-assertion-benchmark
//     cpu: Apple M4
//     BenchmarkTypeAssertion-10    	1000000000	         0.6814 ns/op	       0 B/op	       0 allocs/op
//     BenchmarkMethodCall-10       	737686399	         1.631 ns/op	       0 B/op	       0 allocs/op
//     PASS
//     ok  	LiSP/cmd/type-assertion-benchmark	2.286s

// Symbol models the symbol type handled through interface values in the
// interpreter. The benchmarks compare retrieving it via a type assertion versus
// calling a helper method.
type Symbol struct {
	name string
}

// AsSymbol returns the receiver. Having the method lets us compare dynamic
// dispatch against a type assertion on the interface value.
func (s *Symbol) AsSymbol() *Symbol {
	return s
}

// NonSymbol implements Expression but does not represent a symbol; its
// AsSymbol helper returns nil.
type NonSymbol struct {
	id int
}

func (*NonSymbol) AsSymbol() *Symbol { return nil }

type Expression interface {
	AsSymbol() *Symbol
}

var (
	symbolExpr    Expression = &Symbol{name: "demo"}
	nonSymbolExpr Expression = &NonSymbol{id: 1}
	symbolSink    *Symbol
)

func BenchmarkTypeAssertion(b *testing.B) {
	b.ReportAllocs()

	symExpr := symbolExpr
	nonExpr := nonSymbolExpr
	for i := 0; i < b.N; i++ {
		sym, ok := symExpr.(*Symbol)
		if !ok {
			b.Fatal("type assertion failed")
		}
		symbolSink = sym

		if _, ok := nonExpr.(*Symbol); ok {
			b.Fatal("type assertion unexpectedly succeeded")
		}
	}
}

func BenchmarkMethodCall(b *testing.B) {
	b.ReportAllocs()

	symExpr := symbolExpr
	nonExpr := nonSymbolExpr
	for i := 0; i < b.N; i++ {
		sym := symExpr.AsSymbol()
		if sym == nil {
			b.Fatal("symbol expression returned nil")
		}
		symbolSink = sym

		if nonExpr.AsSymbol() != nil {
			b.Fatal("non-symbol expression returned a symbol")
		}
	}
}
