// Tests for the scanner's public API functions: Peek, Token.String,
// Type.String (via type_string.go), CharLiteralToRune, and namedCharacter.
package scan

import (
	"strings"
	"testing"
)

func TestPeek(t *testing.T) {
	s := NewScanner("", "(x)")

	// Peek twice: must return the same token both times.
	tok1 := s.Peek()
	tok2 := s.Peek()
	if tok1 != tok2 {
		t.Fatalf("Peek not idempotent: %v vs %v", tok1, tok2)
	}

	// Next must consume the peeked token.
	tok3 := s.Next()
	if tok3 != tok1 {
		t.Errorf("Next after Peek: got %v, want %v", tok3, tok1)
	}
	if tok3.Type != LeftParen {
		t.Errorf("first token: got %v, want LeftParen", tok3.Type)
	}

	// Following tokens are unaffected.
	if tok := s.Next(); tok.Type != Symbol || tok.Text != "x" {
		t.Errorf("second token: got %v %q, want Symbol \"x\"", tok.Type, tok.Text)
	}
	if tok := s.Next(); tok.Type != RightParen {
		t.Errorf("third token: got %v, want RightParen", tok.Type)
	}
	if tok := s.Next(); tok.Type != EOF {
		t.Errorf("fourth token: got %v, want EOF", tok.Type)
	}
}

func TestTokenString(t *testing.T) {
	// EOF has its own case.
	if s := (Token{EOF, 1, "<EOF>"}).String(); s != "<EOF>" {
		t.Errorf("EOF token String: got %q", s)
	}

	// Error has its own case.
	if s := (Token{Error, 1, "bad input"}).String(); s != "error: bad input" {
		t.Errorf("Error token String: got %q", s)
	}

	// Short text: printed in full.
	short := Token{Symbol, 1, "hi"}
	if s := short.String(); !strings.Contains(s, "hi") {
		t.Errorf("short token String %q missing text", s)
	}

	// Long text (> 10 bytes): truncated with "...".
	long := Token{String, 1, `"this is longer than ten characters"`}
	if s := long.String(); !strings.HasSuffix(s, "...") {
		t.Errorf("long token String %q should end with ...", s)
	}
}

func TestTypeString(t *testing.T) {
	cases := []struct {
		typ  Type
		want string
	}{
		{EOF, "EOF"},
		{Error, "Error"},
		{LeftParen, "LeftParen"},
		{Symbol, "Symbol"},
		{Fixnum, "Fixnum"},
		{Flonum, "Flonum"},
		{Char, "Char"},
		{True, "True"},
		{False, "False"},
	}
	for _, c := range cases {
		if got := c.typ.String(); got != c.want {
			t.Errorf("%d.String() = %q, want %q", int(c.typ), got, c.want)
		}
	}
	// Out-of-range value must not panic.
	_ = Type(999).String()
}

func TestCharLiteralToRune(t *testing.T) {
	cases := []struct {
		lit  string
		want rune
	}{
		// Named characters
		{`#\space`, ' '},
		{`#\newline`, '\n'},
		{`#\linefeed`, '\n'},
		{`#\tab`, '\t'},
		{`#\nul`, 0},
		{`#\null`, 0},
		{`#\backspace`, '\010'},
		{`#\vtab`, '\013'},
		{`#\page`, '\014'},
		{`#\return`, '\r'},
		{`#\rubout`, '\177'},
		// Hex Unicode (lowercase u)
		{`#\u0041`, 'A'},
		{`#\u22EE`, '⋮'}, // VERTICAL ELLIPSIS ⋮
		// Hex Unicode (uppercase U)
		{`#\U1F61E`, '\U0001F61E'}, // DISAPPOINTED FACE 😞
		// Single character (ASCII)
		{`#\a`, 'a'},
		{`#\Z`, 'Z'},
		{`#\0`, '0'},
		// Single character (multi-byte UTF-8): #\ followed by the actual rune
		{"#\\⋮", '⋮'},
	}
	for _, c := range cases {
		got := CharLiteralToRune(c.lit)
		if got != c.want {
			t.Errorf("CharLiteralToRune(%q) = %U, want %U", c.lit, got, c.want)
		}
	}
}

func TestCharLiteralToRunePanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("expected panic for unknown named character, got none")
		}
	}()
	CharLiteralToRune(`#\foobar`) // "foobar" is not a recognised named character
}
