// Package lexers archives six Scheme lexer implementations built during
// performance research for the LiSP interpreter. See README.md for details.
package lexers

import (
	"fmt"
	"strconv"
)

// Token represents a token returned from any of the scanners.
type Token struct {
	Type Type   // The type of this item.
	Line int    // The line number on which this token appears
	Text string // The text of this item.
}

// Type identifies the type of lex items.
type Type int

const (
	EOF             Type = iota // zero value so closed channel delivers EOF
	Error                       // error occurred; value is text of error
	LeftParen                   // '('
	LeftBrack                   // '['
	LeftBrace                   // '{'
	Quote                       // '\''
	QuasiQuote                  // '`'
	Unquote                     // ','
	UnquoteSplicing             // ",@"
	False                       // "#f"
	True                        // "#t"
	Dot                         // "."
	Fixnum                      // a number with no fractional component
	Flonum                      // a number with a fractional component
	String                      // quoted string (includes quotes)
	Symbol                      // a Scheme symbol
	RightParen                  // ')'
	RightBrack                  // ']'
	RightBrace                  // '}'
	Char                        // character literal (#\space, etc.)
)

func (t Type) String() string {
	switch t {
	case EOF:
		return "EOF"
	case Error:
		return "Error"
	case LeftParen:
		return "LeftParen"
	case LeftBrack:
		return "LeftBrack"
	case LeftBrace:
		return "LeftBrace"
	case Quote:
		return "Quote"
	case QuasiQuote:
		return "QuasiQuote"
	case Unquote:
		return "Unquote"
	case UnquoteSplicing:
		return "UnquoteSplicing"
	case False:
		return "False"
	case True:
		return "True"
	case Dot:
		return "Dot"
	case Fixnum:
		return "Fixnum"
	case Flonum:
		return "Flonum"
	case String:
		return "String"
	case Symbol:
		return "Symbol"
	case RightParen:
		return "RightParen"
	case RightBrack:
		return "RightBrack"
	case RightBrace:
		return "RightBrace"
	case Char:
		return "Char"
	default:
		return fmt.Sprintf("Type(%d)", int(t))
	}
}

func (i Token) String() string {
	switch {
	case i.Type == EOF:
		return "<EOF>"
	case i.Type == Error:
		return "error: " + i.Text
	case len(i.Text) > 10:
		return fmt.Sprintf("%#v: %.10q...", i.Type, i.Text)
	}
	return fmt.Sprintf("%#v: %q", i.Type, i.Text)
}

// sentinel Type values used only within the matcher tables of the regex-based
// scanners.
const (
	reTokSkip Type = -1 // whitespace/comment: skip and re-scan
	reTokWord Type = -2 // non-delimiter run: Dot/Fixnum/Flonum/Symbol
)

// classifyWord examines a run of non-delimiter bytes and returns a Dot,
// Fixnum, Flonum, or Symbol token. Used by scanners 2–5.
func classifyWord(text string, line int) Token {
	if text == "." {
		return Token{Dot, line, text}
	}
	if _, err := strconv.ParseInt(text, 0, 64); err == nil {
		return Token{Fixnum, line, text}
	} else if ne, ok := err.(*strconv.NumError); ok && ne.Err == strconv.ErrRange {
		return Token{Error, line, "Bignums not yet implemented `" + text + "`"}
	}
	if _, err := strconv.ParseFloat(text, 64); err == nil {
		return Token{Flonum, line, text}
	} else if ne, ok := err.(*strconv.NumError); ok && ne.Err == strconv.ErrRange {
		return Token{Error, line, "Bignums not yet implemented `" + text + "`"}
	}
	return Token{Symbol, line, text}
}

// isDelimByte reports whether b is a Scheme delimiter byte (whitespace or one
// of the special characters that cannot appear unquoted in a symbol).
func isDelimByte(b byte) bool {
	switch b {
	case '(', ')', '[', ']', '{', '}', '"', ',', '\'', '`', ';', '#', '|', '\\',
		' ', '\t', '\r', '\n', '\v', '\f':
		return true
	}
	return false
}

func isHexByte(b byte) bool {
	return '0' <= b && b <= '9' || 'a' <= b && b <= 'f' || 'A' <= b && b <= 'F'
}

func isOctByte(b byte) bool {
	return '0' <= b && b <= '7'
}

func isAlphaByte(b byte) bool {
	return 'a' <= b && b <= 'z' || 'A' <= b && b <= 'Z'
}
