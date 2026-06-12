package scan

import (
	"fmt"
	"testing"
)

func expectToken(t *testing.T, scanner *Scanner, tokenType Type, tokenText string) {
	token := scanner.Next()
	if token.Type != tokenType {
		t.Errorf("line %d: wanted type %v, got %v", token.Line, tokenType, token.Type)
	}
	if token.Text != tokenText {
		t.Errorf("line %d: wanted text %q, got %q", token.Line, tokenText, token.Text)
	}
}

func TestPunctuation(t *testing.T) {
	scanner := NewScanner("<string>", "( [ ] )")
	want := func(tokenType Type, tokenText string) {
		expectToken(t, scanner, tokenType, tokenText)
	}
	want(LeftParen, "(")
	want(LeftBrack, "[")
	want(RightBrack, "]")
	want(RightParen, ")")
	want(EOF, "<EOF>")
}

type wanted struct {
	Type
	Text string
}

type testcase struct {
	input  string
	output []wanted
}

var testcases []testcase = []testcase{
	{
		input: "([])",
		output: []wanted{
			{LeftParen, "("},
			{LeftBrack, "["},
			{RightBrack, "]"},
			{RightParen, ")"},
			{EOF, "<EOF>"},
		},
	},
	{
		input: "'`,,@#f#t .",
		output: []wanted{
			{Quote, "'"},
			{QuasiQuote, "`"},
			{Unquote, ","},
			{UnquoteSplicing, ",@"},
			{False, "#f"},
			{True, "#t"},
			{Dot, "."},
			{EOF, "<EOF>"},
		},
	},
	{
		input: "000 1 42\r\n 3.1415926 1.2\r\n 3. .4",
		output: []wanted{
			{Fixnum, "000"},
			{Fixnum, "1"},
			{Fixnum, "42"},
			{Flonum, "3.1415926"},
			{Flonum, "1.2"},
			{Flonum, "3."},
			{Flonum, ".4"},
			{EOF, "<EOF>"},
		},
	},
	{
		input: `"" "?" "howdy" "\"\x" "unfinished business`,
		output: []wanted{
			{String, `""`},
			{String, `"?"`},
			{String, `"howdy"`},
			{String, `"\"\x"`},
			{Error, "unterminated quoted string"},
			{EOF, "<EOF>"},
		},
	},
	{
		// Note: '#' is a delimiter in this scanner and cannot appear
		// mid-symbol. The original state-machine scanner supported '#'
		// mid-symbol; the current inline scanner does not.
		input: `foo a^2+b^2=c^2 @@@ 12-3%4 kebab-case-names` +
			"\n~!@$%^&*_+-=:<>?./",
		output: []wanted{
			{Symbol, `foo`},
			{Symbol, `a^2+b^2=c^2`},
			{Symbol, `@@@`},
			{Symbol, `12-3%4`},
			{Symbol, `kebab-case-names`},
			{Symbol, `~!@$%^&*_+-=:<>?./`},
			{EOF, "<EOF>"},
		},
	},
	{
		input: `#\space  #\o77#\u#\U#\u22EE#\U1D417`,
		output: []wanted{
			{Char, `#\space`},  // "space" named character
			{Char, `#\o77`},    // "M"  (U+004D) LATIN CAPITAL LETTER M
			{Char, `#\u`},      // "u"  (U+0075) LATIN SMALL LETTER U
			{Char, `#\U`},      // "U"  (U+0055) LATIN CAPITAL LETTER U
			{Char, `#\u22EE`},  // "⋮"  (U+22EE) VERTICAL ELLIPSIS
			{Char, `#\U1D417`}, // "𝐗"  (U+1D417) MATHEMATICAL BOLD CAPITAL X
			{EOF, "<EOF>"},
		},
	},
}

func checkTestcase(t *testing.T, c *testcase) {
	scanner := NewScanner("<string>", c.input)
	for i, w := range c.output {
		reportInput := fmt.Sprintf("input: %q\n", c.input)
		token := scanner.Next()
		if token.Type != w.Type {
			t.Errorf("%s  token %d: wanted type %v, got %v",
				reportInput, i, w.Type, token.Type)
			reportInput = ""
		}
		if token.Text != w.Text {
			t.Errorf("%s  token %d: wanted text %q, got %q",
				reportInput, i, w.Text, token.Text)
		}
	}
}

func TestTokenizer(t *testing.T) {
	for i := range testcases {
		checkTestcase(t, &testcases[i])
	}
}

// TestUnicodeLineTerminators checks that all eight Unicode line terminators are
// counted correctly and that strings are rejected when they contain one.
func TestUnicodeLineTerminators(t *testing.T) {
	// NEL = U+0085 = 0xC2 0x85
	// LS  = U+2028 = 0xE2 0x80 0xA8
	// PS  = U+2029 = 0xE2 0x80 0xA9
	nel := "\xC2\x85"
	ls := "\xE2\x80\xA8"
	ps := "\xE2\x80\xA9"

	type lineCheck struct {
		input    string
		desc     string
		wantLine int // expected line number of the token after the terminators
	}
	checks := []lineCheck{
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
	for _, c := range checks {
		s := NewScanner("<test>", c.input)
		s.Next() // consume "a"
		tok := s.Next()
		if tok.Line != c.wantLine {
			t.Errorf("%s: after terminator, want line %d, got %d (tok=%q)",
				c.desc, c.wantLine, tok.Line, tok.Text)
		}
	}

	// Strings containing a line terminator must produce an error.
	errCases := []struct {
		input string
		desc  string
	}{
		{"\"a\nb\"", "LF in string"},
		{"\"a\rb\"", "CR in string"},
		{"\"a\vb\"", "VT in string"},
		{"\"a\fb\"", "FF in string"},
		{"\"a" + nel + "b\"", "NEL in string"},
		{"\"a" + ls + "b\"", "LS in string"},
		{"\"a" + ps + "b\"", "PS in string"},
	}
	for _, c := range errCases {
		s := NewScanner("<test>", c.input)
		tok := s.Next()
		if tok.Type != Error {
			t.Errorf("%s: want Error token, got %v %q", c.desc, tok.Type, tok.Text)
		}
	}
}
