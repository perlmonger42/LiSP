// RERC-style scanner tests: each case pairs an input string with the
// expected token stream rendered as Type("text") Type("text") ... .
// This format (input beside expected output) mirrors the Read-Eval-Read-Compare
// pattern used by cmd/LiSP/main.go for interpreter tests.
package scan

import (
	"fmt"
	"strings"
	"testing"
)

// scanAll scans input and returns all tokens (excluding EOF) as a
// space-separated "Type(\"text\")" list, matching the %v and %q formats of
// Type.String() and Go's string quoting respectively.
func scanAll(input string) string {
	var sb strings.Builder
	s := NewScanner("", input)
	for {
		tok := s.Next()
		if tok.Type == EOF {
			break
		}
		if sb.Len() > 0 {
			sb.WriteByte(' ')
		}
		fmt.Fprintf(&sb, "%v(%q)", tok.Type, tok.Text)
	}
	return sb.String()
}

func assertScan(t *testing.T, input, want string) {
	t.Helper()
	if got := scanAll(input); got != want {
		t.Errorf("scan %q\n  got:  %s\n  want: %s", input, got, want)
	}
}

func TestBrackets(t *testing.T) {
	assertScan(t, "( { [ ] } )",
		`LeftParen("(") LeftBrace("{") LeftBrack("[") RightBrack("]") RightBrace("}") RightParen(")")`)
}

func TestLineComments(t *testing.T) {
	assertScan(t, "; line comment\nx", `Symbol("x")`)
	assertScan(t, "x ; inline comment\ny", `Symbol("x") Symbol("y")`)
	assertScan(t, "; comment only", ``)
	assertScan(t, "a\n; skipped\nb", `Symbol("a") Symbol("b")`)
	// Comment at very end of input (no trailing newline)
	assertScan(t, "x ; eof comment", `Symbol("x")`)
}

func TestBlockComments(t *testing.T) {
	assertScan(t, "#| comment |# x", `Symbol("x")`)
	assertScan(t, "#||#x", `Symbol("x")`) // empty block comment
	assertScan(t, "#| outer #| inner |# still |# x", `Symbol("x")`)
	assertScan(t, "#| #| a |# b |# c", `Symbol("c")`) // balanced nesting
	assertScan(t, "#| line1\nline2 |# x", `Symbol("x")`)
	// |# spanning a line boundary must still close the comment
	assertScan(t, "#| a|\n# x", `Symbol("x")`)
	// #| spanning a line boundary must still open a nested comment
	assertScan(t, "#| a#\n|b |# |# x", `Symbol("x")`)
	// Unterminated
	assertScan(t, "#| oops", `Error("unterminated block comment")`)
}

func TestBarSymbols(t *testing.T) {
	assertScan(t, "|foo|", `Symbol("|foo|")`)
	assertScan(t, "|hello world|", `Symbol("|hello world|")`)
	assertScan(t, "||", `Symbol("||")`)
	assertScan(t, "|unterminated", `Error("unterminated bar symbol")`)
}

func TestScanHashErrors(t *testing.T) {
	assertScan(t, "#", `Error("unexpected EOF after #")`)
	assertScan(t, "#z x", `Error("unexpected: #") Symbol("x")`)
}

func TestScanBackslash(t *testing.T) {
	assertScan(t, `\`, `Error("unexpected: \\")`)
}

func TestQuoteFormsAll(t *testing.T) {
	assertScan(t, "' ` , ,@", `Quote("'") QuasiQuote("`+"`"+`") Unquote(",") UnquoteSplicing(",@")`)
}

func TestDot(t *testing.T) {
	assertScan(t, ". ..", `Dot(".") Symbol("..")`)
	assertScan(t, "(a . b)", `LeftParen("(") Symbol("a") Dot(".") Symbol("b") RightParen(")")`)
}

func TestBignums(t *testing.T) {
	// Integer too large for int64 → Error
	tok := NewScanner("", "99999999999999999999").Next()
	if tok.Type != Error {
		t.Errorf("bignum int: got %v, want Error", tok.Type)
	}
	// Float out of range → Error
	tok = NewScanner("", "1e99999").Next()
	if tok.Type != Error {
		t.Errorf("bignum float: got %v, want Error", tok.Type)
	}
}

func TestScanCharErrors(t *testing.T) {
	assertScan(t, `#\`, `Error("unexpected EOF in char literal")`)
}

func TestScanCharDefault(t *testing.T) {
	// Digit: not alpha, not u/U+hex, not o+octal — hits the default case.
	assertScan(t, `#\0`, `Char("#\\0")`)
	// Punctuation: same default path, single ASCII byte.
	assertScan(t, `#\!`, `Char("#\\!")`)
	// Multi-byte UTF-8: default path, utf8.DecodeRuneInString advances by > 1.
	assertScan(t, `#\⋮`, `Char("#\\⋮")`)
}

func TestScanStringUnterminated(t *testing.T) {
	// Backslash at end of line terminates the string
	assertScan(t, "\"a\\", `Error("unterminated quoted string")`)
}
