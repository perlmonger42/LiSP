// InlineScanner is a hand-written scanner built on the same first-byte
// dispatch as DispatchRegexScanner, but all four regex calls have been
// replaced by dedicated methods that scan bytes directly.
//
// Design: the first-byte switch is identical to DispatchRegexScanner, but
// instead of calling regexp.FindString for strings, char literals, bar-quoted
// symbols, and word tokens, each case calls a small method that walks the
// input byte-by-byte. This eliminates all regex overhead for the common token
// types, leaving only direct byte comparisons and slice operations.
//
// This became the production scanner in internal/scan after benchmarking showed
// it was 3× faster than the original state machine and ~17× faster than the
// naive regex-list approach.
//
// Limitations: same as DispatchRegexScanner ('#' mid-symbol splits; no
// nestable block comments; ASCII-only whitespace).
//
// Performance: ~123 MB/s. Winner — 3× faster than the state machine (~40),
// ~3.5× faster than the dispatch-regex scanner (~35), ~17× faster than the
// regex-list scanner (~7), and ~25× faster than the big-regex scanner (~5).
package lexers

import "unicode/utf8"

// InlineScanner is a hand-written, regex-free Scheme lexer.
type InlineScanner struct {
	input string
	pos   int
	line  int
}

// NewInlineScanner returns a scanner for the given input string.
func NewInlineScanner(input string) *InlineScanner {
	return &InlineScanner{input: input, line: 1}
}

// Next returns the next token, skipping whitespace and line comments.
func (s *InlineScanner) Next() Token {
outer:
	for {
		if s.pos >= len(s.input) {
			return Token{EOF, s.line, "<EOF>"}
		}
		remaining := s.input[s.pos:]
		line := s.line

		switch remaining[0] {
		case ' ', '\t', '\r', '\n', '\v', '\f':
			s.inlineSkipWS()
			continue outer
		case ';':
			s.inlineSkipLineComment()
			continue outer
		case '(':
			s.pos++
			return Token{LeftParen, line, remaining[:1]}
		case ')':
			s.pos++
			return Token{RightParen, line, remaining[:1]}
		case '[':
			s.pos++
			return Token{LeftBrack, line, remaining[:1]}
		case ']':
			s.pos++
			return Token{RightBrack, line, remaining[:1]}
		case '{':
			s.pos++
			return Token{LeftBrace, line, remaining[:1]}
		case '}':
			s.pos++
			return Token{RightBrace, line, remaining[:1]}
		case '\'':
			s.pos++
			return Token{Quote, line, remaining[:1]}
		case '`':
			s.pos++
			return Token{QuasiQuote, line, remaining[:1]}
		case ',':
			if len(remaining) > 1 && remaining[1] == '@' {
				s.pos += 2
				return Token{UnquoteSplicing, line, remaining[:2]}
			}
			s.pos++
			return Token{Unquote, line, remaining[:1]}
		case '#':
			return s.scanHash(line)
		case '"':
			return s.scanString(line)
		case '|':
			return s.scanBarSymbol(line)
		case '\\':
			s.pos++
			return Token{Error, line, "unexpected: " + remaining[:1]}
		default:
			return s.scanWord(line)
		}
	}
}

func (s *InlineScanner) inlineSkipWS() {
	for s.pos < len(s.input) {
		switch s.input[s.pos] {
		case '\n':
			s.line++
			s.pos++
		case ' ', '\t', '\r', '\v', '\f':
			s.pos++
		default:
			return
		}
	}
}

func (s *InlineScanner) inlineSkipLineComment() {
	for s.pos < len(s.input) && s.input[s.pos] != '\n' {
		s.pos++
	}
}

// scanHash handles tokens beginning with '#': #f, #t, and char literals.
func (s *InlineScanner) scanHash(line int) Token {
	if s.pos+1 >= len(s.input) {
		s.pos++
		return Token{Error, line, "unexpected EOF after #"}
	}
	switch s.input[s.pos+1] {
	case 'f':
		text := s.input[s.pos : s.pos+2]
		s.pos += 2
		return Token{False, line, text}
	case 't':
		text := s.input[s.pos : s.pos+2]
		s.pos += 2
		return Token{True, line, text}
	case '\\':
		return s.scanChar(line)
	default:
		bad := s.input[s.pos : s.pos+1]
		s.pos++
		return Token{Error, line, "unexpected: " + bad}
	}
}

// scanChar scans a #\ character literal starting at s.pos.
// Handles four forms:
//
//	#\uHHHH…  or  #\UHHHHHH…  (u/U followed by hex digits)
//	#\oOOO…                   (o followed by octal digits)
//	#\name                     (run of ASCII letters)
//	#\x                        (any other single character)
func (s *InlineScanner) scanChar(line int) Token {
	start := s.pos
	s.pos += 2 // consume '#' and '\'
	if s.pos >= len(s.input) {
		return Token{Error, line, "unexpected EOF in char literal"}
	}
	b := s.input[s.pos]
	switch {
	case (b == 'u' || b == 'U') && s.pos+1 < len(s.input) && isHexByte(s.input[s.pos+1]):
		s.pos++ // consume 'u'/'U'
		for s.pos < len(s.input) && isHexByte(s.input[s.pos]) {
			s.pos++
		}
	case b == 'o' && s.pos+1 < len(s.input) && isOctByte(s.input[s.pos+1]):
		s.pos++ // consume 'o'
		for s.pos < len(s.input) && isOctByte(s.input[s.pos]) {
			s.pos++
		}
	case isAlphaByte(b):
		for s.pos < len(s.input) && isAlphaByte(s.input[s.pos]) {
			s.pos++
		}
	default:
		// single character, possibly multi-byte UTF-8
		_, width := utf8.DecodeRuneInString(s.input[s.pos:])
		if width == 0 {
			width = 1
		}
		s.pos += width
	}
	return Token{Char, line, s.input[start:s.pos]}
}

// scanString scans a double-quoted string starting at s.pos.
func (s *InlineScanner) scanString(line int) Token {
	start := s.pos
	s.pos++ // consume opening '"'
	for s.pos < len(s.input) {
		b := s.input[s.pos]
		s.pos++
		switch b {
		case '"':
			return Token{String, line, s.input[start:s.pos]}
		case '\n':
			s.pos-- // leave the newline for the next token
			return Token{Error, line, "unterminated string"}
		case '\\':
			if s.pos >= len(s.input) || s.input[s.pos] == '\n' {
				return Token{Error, line, "unterminated string"}
			}
			s.pos++ // consume the escaped character
		}
	}
	return Token{Error, line, "unterminated string"}
}

// scanBarSymbol scans a bar-quoted symbol |…| starting at s.pos.
func (s *InlineScanner) scanBarSymbol(line int) Token {
	start := s.pos
	s.pos++ // consume opening '|'
	for s.pos < len(s.input) && s.input[s.pos] != '|' {
		s.pos++
	}
	if s.pos >= len(s.input) {
		return Token{Error, line, "unterminated bar symbol"}
	}
	s.pos++ // consume closing '|'
	return Token{Symbol, line, s.input[start:s.pos]}
}

// scanWord scans a run of non-delimiter bytes and classifies the result as
// Dot, Fixnum, Flonum, or Symbol.
func (s *InlineScanner) scanWord(line int) Token {
	start := s.pos
	for s.pos < len(s.input) && !isDelimByte(s.input[s.pos]) {
		s.pos++
	}
	return classifyWord(s.input[start:s.pos], line)
}
