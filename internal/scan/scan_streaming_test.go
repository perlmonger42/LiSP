// Tests for NewStreamScanner: verifies that the streaming scanner produces the
// same tokens as NewScanner on multi-line input, and tracks line numbers correctly.
package scan

import (
	"fmt"
	"strings"
	"testing"
)

// streamAll scans r and returns all tokens (excluding EOF) formatted the same
// way as scanAll, so streaming and non-streaming output can be compared directly.
func streamAll(r *strings.Reader) string {
	var sb strings.Builder
	s := NewStreamScanner("", r)
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

// assertStream checks that streaming and non-streaming scanners produce
// identical token output for the same input.
func assertStream(t *testing.T, input, want string) {
	t.Helper()
	got := streamAll(strings.NewReader(input))
	if got != want {
		t.Errorf("stream %q\n  got:  %s\n  want: %s", input, got, want)
	}
	// Also verify it matches the non-streaming scanner.
	if nonStream := scanAll(input); nonStream != want {
		t.Errorf("non-stream %q\n  got:  %s\n  want: %s", input, nonStream, want)
	}
}

// TestStreamingMatchesInline checks that the streaming scanner produces the
// same tokens as the inline scanner on multi-line Scheme input.
func TestStreamingMatchesInline(t *testing.T) {
	input := "(define factorial\n" +
		"  (lambda (n)\n" +
		"    (if (= n 0)\n" +
		"        1\n" +
		"        (* n (factorial (- n 1))))))\n" +
		"(factorial 10)\n"

	want := `LeftParen("(") Symbol("define") Symbol("factorial")` +
		` LeftParen("(") Symbol("lambda") LeftParen("(") Symbol("n") RightParen(")")` +
		` LeftParen("(") Symbol("if") LeftParen("(") Symbol("=") Symbol("n") Fixnum("0") RightParen(")")` +
		` Fixnum("1")` +
		` LeftParen("(") Symbol("*") Symbol("n") LeftParen("(") Symbol("factorial")` +
		` LeftParen("(") Symbol("-") Symbol("n") Fixnum("1") RightParen(")") RightParen(")") RightParen(")") RightParen(")") RightParen(")") RightParen(")")` +
		` LeftParen("(") Symbol("factorial") Fixnum("10") RightParen(")")`

	assertStream(t, input, want)
}

// TestStreamingLineNumbers checks that line numbers are tracked correctly
// across multiple line terminators in streaming mode.
func TestStreamingLineNumbers(t *testing.T) {
	input := "a\nb\nc"
	s := NewStreamScanner("", strings.NewReader(input))

	cases := []struct {
		wantText string
		wantLine int
	}{
		{"a", 1},
		{"b", 2},
		{"c", 3},
	}
	for _, c := range cases {
		tok := s.Next()
		if tok.Text != c.wantText || tok.Line != c.wantLine {
			t.Errorf("got %q line %d, want %q line %d", tok.Text, tok.Line, c.wantText, c.wantLine)
		}
	}
}

// TestStreamingBlockComment checks that a block comment spanning multiple lines
// is correctly skipped in streaming mode.
func TestStreamingBlockComment(t *testing.T) {
	input := "#| this comment\n" +
		"   spans three\r" +
		"   lines |#\n" +
		"done\n"
	assertStream(t, input, `Symbol("done")`)
}

// TestStreamingLineComment checks that line comments on multiple lines are
// skipped and only the real tokens are returned.
func TestStreamingLineComment(t *testing.T) {
	input := "; first comment\n" +
		"alpha ; inline\n" +
		"; another comment\n" +
		"beta\n"
	assertStream(t, input, `Symbol("alpha") Symbol("beta")`)
}

// TestStreamingLineTerminators checks that all line terminator forms increment
// the line counter correctly in streaming mode, exercising the multi-byte
// terminator paths in refillLine().
func TestStreamingLineTerminators(t *testing.T) {
	nel := "\xC2\x85"
	ls := "\xE2\x80\xA8"
	ps := "\xE2\x80\xA9"

	cases := []struct {
		input    string
		desc     string
		wantLine int
	}{
		{"a\nb", "LF", 2},
		{"a\vb", "VT", 2},
		{"a\fb", "FF", 2},
		{"a\rb", "CR", 2},
		{"a\r\nb", "CR+LF (one line)", 2},
		{"a" + nel + "b", "NEL U+0085", 2},
		{"a" + ls + "b", "LS U+2028", 2},
		{"a" + ps + "b", "PS U+2029", 2},
		// Two terminators → line 3
		{"a\n\nb", "two LF", 3},
		{"a\r\r\nb", "CR then CR+LF (two lines)", 3},
	}
	for _, c := range cases {
		s := NewStreamScanner("<test>", strings.NewReader(c.input))
		s.Next() // consume "a"
		tok := s.Next()
		if tok.Line != c.wantLine {
			t.Errorf("%s: after terminator, want line %d, got %d (tok=%q)",
				c.desc, c.wantLine, tok.Line, tok.Text)
		}
	}
}

// TestStreamingNonTerminatorLookAlikes checks that characters whose UTF-8
// encoding shares a prefix with a line terminator are NOT treated as line
// terminators in streaming mode, exercising the byte-comparison guards in
// refillLine().
//
// Each look-alike is placed inside a Scheme string literal so the returned
// token can be verified, while still confirming the line counter is unaffected.
//
//   0xC2 0xA9        © U+00A9  — same first byte as NEL (0xC2 0x85)
//   0xE2 0x98 0x83   U+2603  — same first byte as LS/PS, second byte differs
//   0xE2 0x80 0xA6   U+2026  — same first two bytes as LS/PS, third differs
func TestStreamingNonTerminatorLookAlikes(t *testing.T) {
	cases := []struct {
		char string // raw bytes of the look-alike character
		desc string
	}{
		{"\xC2\xA9", "U+00A9 (0xC2 0xA9): not NEL"},
		{"\xE2\x98\x83", "U+2603 (0xE2 0x98 0x83): not LS/PS"},
		{"\xE2\x80\xA6", "U+2026 (0xE2 0x80 0xA6): not LS/PS"},
	}
	for _, c := range cases {
		// Line 1: symbol "a", a string literal containing the look-alike, symbol "b".
		// Line 2: symbol "c".
		input := "a \"" + c.char + "\" b\nc"
		wantText := "\"" + c.char + "\""

		s := NewStreamScanner("<test>", strings.NewReader(input))

		if tok := s.Next(); tok.Type != Symbol || tok.Text != "a" {
			t.Errorf("%s: token 1: got %v(%q), want Symbol(\"a\")", c.desc, tok.Type, tok.Text)
		}

		tok := s.Next() // the string literal containing the look-alike
		if tok.Type != String {
			t.Errorf("%s: token 2: got %v(%q), want String", c.desc, tok.Type, tok.Text)
		}
		if tok.Text != wantText {
			t.Errorf("%s: token 2 text: got %q, want %q", c.desc, tok.Text, wantText)
		}
		if tok.Line != 1 {
			t.Errorf("%s: look-alike advanced line counter to %d, want 1", c.desc, tok.Line)
		}

		s.Next() // "b" — line 1

		tok = s.Next() // "c" — after the real '\n', must be line 2
		if tok.Line != 2 {
			t.Errorf("%s: token after real newline should be line 2, got %d", c.desc, tok.Line)
		}
	}
}

// TestStreamingMixedTokenTypes exercises numbers, strings, chars, booleans,
// and punctuation spread across several lines in streaming mode.
func TestStreamingMixedTokenTypes(t *testing.T) {
	input := "(list 42 3.14\n" +
		`     "hello" #\space` + "\n" +
		"     #t #f)\n"
	assertStream(t, input,
		`LeftParen("(") Symbol("list") Fixnum("42") Flonum("3.14")`+
			` String("\"hello\"") Char("#\\space")`+
			` True("#t") False("#f") RightParen(")")`)
}
