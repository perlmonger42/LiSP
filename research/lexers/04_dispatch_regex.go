// DispatchRegexScanner tokenizes Scheme source by inspecting the first byte
// and dispatching directly to the one applicable regex — or emitting
// single-character tokens inline without any regex call at all.
//
// Design: instead of searching matchers in order (O(matchers) per token), a
// switch on the first byte of the remaining input selects exactly the right
// action. Most tokens are emitted without any regex: parens, brackets, quotes,
// whitespace, comments, unquote, #f, and #t are all handled inline. Only four
// token types actually invoke a regex: strings, char literals, bar-quoted
// symbols, and general words/numbers. This eliminates the O(matchers) overhead
// of RegexListScanner while keeping the implementation concise.
//
// Limitations: same as RegexListScanner ('#' mid-symbol splits; no nestable
// block comments; ASCII-only whitespace).
//
// Performance: ~35 MB/s. Much faster than the regex-list approach; close to
// the original state machine despite using regexes for 4 token forms.
package lexers

import (
	"regexp"
	"unicode/utf8"
)

// DispatchRegexScanner tokenizes Scheme source via first-byte dispatch.
type DispatchRegexScanner struct {
	input string
	pos   int
	line  int
}

// NewDispatchRegexScanner returns a scanner for the given input string.
func NewDispatchRegexScanner(input string) *DispatchRegexScanner {
	return &DispatchRegexScanner{input: input, line: 1}
}

var (
	drString    = regexp.MustCompile(`^"(?:[^"\\]|\\.)*"`)
	drChar      = regexp.MustCompile(`^#\\(?:[uU][0-9a-fA-F]+|o[0-7]+|[a-zA-Z]+|.)`)
	drBarSymbol = regexp.MustCompile(`^\|[^|]*\|`)
	drWord      = regexp.MustCompile("^[^()\\[\\]{}\",';`#|\\\\ \\t\\r\\n\\v\\f]+")
)

// Next returns the next token, skipping whitespace and line comments.
func (s *DispatchRegexScanner) Next() Token {
outer:
	for {
		if s.pos >= len(s.input) {
			return Token{EOF, s.line, "<EOF>"}
		}
		remaining := s.input[s.pos:]
		line := s.line

		switch remaining[0] {

		// whitespace: scan inline, no regex
		case ' ', '\t', '\r', '\n', '\v', '\f':
			s.drSkipWS()
			continue outer

		// line comment: scan inline, no regex
		case ';':
			s.drSkipLineComment()
			continue outer

		// single-character punctuation: emit directly, no regex
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

		// comma: Unquote or UnquoteSplicing, no regex
		case ',':
			if len(remaining) > 1 && remaining[1] == '@' {
				s.pos += 2
				return Token{UnquoteSplicing, line, remaining[:2]}
			}
			s.pos++
			return Token{Unquote, line, remaining[:1]}

		// hash: disambiguate by second byte
		case '#':
			if len(remaining) < 2 {
				s.pos++
				return Token{Error, line, "unexpected EOF after #"}
			}
			switch remaining[1] {
			case 'f':
				s.pos += 2
				return Token{False, line, remaining[:2]}
			case 't':
				s.pos += 2
				return Token{True, line, remaining[:2]}
			case '\\':
				text := drChar.FindString(remaining)
				if text == "" {
					s.pos++
					return Token{Error, line, "bad char literal after #\\"}
				}
				s.pos += len(text)
				return Token{Char, line, text}
			default:
				s.pos++
				return Token{Error, line, "unexpected: " + remaining[:1]}
			}

		// string literal: one regex call
		case '"':
			text := drString.FindString(remaining)
			if text == "" {
				s.pos++
				return Token{Error, line, "unterminated string"}
			}
			s.pos += len(text)
			return Token{String, line, text}

		// bar-quoted symbol: one regex call
		case '|':
			text := drBarSymbol.FindString(remaining)
			if text == "" {
				s.pos++
				return Token{Error, line, "unterminated bar symbol"}
			}
			s.pos += len(text)
			return Token{Symbol, line, text}

		// everything else: number or symbol, one regex call
		default:
			text := drWord.FindString(remaining)
			if text == "" {
				_, width := utf8.DecodeRuneInString(remaining)
				if width == 0 {
					width = 1
				}
				s.pos += width
				return Token{Error, line, "unexpected: " + remaining[:width]}
			}
			s.pos += len(text)
			return classifyWord(text, line)
		}
	}
}

// drSkipWS advances past a run of ASCII whitespace, counting newlines.
func (s *DispatchRegexScanner) drSkipWS() {
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

// drSkipLineComment advances past a semicolon comment up to (but not
// including) the terminating newline.
func (s *DispatchRegexScanner) drSkipLineComment() {
	for s.pos < len(s.input) && s.input[s.pos] != '\n' {
		s.pos++
	}
}
