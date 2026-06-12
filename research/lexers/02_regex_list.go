// RegexListScanner tokenizes Scheme source with a list-of-matchers approach.
// Each call to Next tries compiled regexes in order and takes the first match.
//
// Design: a slice of (Type, *regexp.Regexp) pairs is searched linearly. For
// each position in the input, we try each regex in turn until one matches at
// the current position. This is O(matchers) per token in the common case.
//
// Limitations vs the state machine scanner:
//   - '#' mid-symbol (e.g. "1#2-3") splits into multiple tokens.
//   - Nestable block comments (#|…|#) are not supported.
//   - Only ASCII whitespace is treated as a delimiter.
//
// Performance: ~7 MB/s. The sequential regex search dominates; even though
// each regex is anchored (^), the regexp engine still has to initialize per
// call. This is the slowest of the three regex-based approaches.
package lexers

import (
	"regexp"
	"strings"
	"unicode/utf8"
)

// RegexListScanner tokenizes Scheme source with a list-of-matchers approach.
type RegexListScanner struct {
	input string
	pos   int
	line  int
}

// NewRegexListScanner returns a scanner for the given input string.
func NewRegexListScanner(input string) *RegexListScanner {
	return &RegexListScanner{input: input, line: 1}
}

type reMatcher struct {
	typ Type
	re  *regexp.Regexp
}

// reMatchers is tried in order; first match wins.
var reMatchers = []reMatcher{
	// skip whitespace and line comments
	{reTokSkip, regexp.MustCompile(`^(?:[ \t\r\n\v\f]+|;[^\n]*)`)},
	// multi-character tokens that must precede their prefix chars
	{String, regexp.MustCompile(`^"(?:[^"\\]|\\.)*"`)},
	{Char, regexp.MustCompile(`^#\\(?:[uU][0-9a-fA-F]+|o[0-7]+|[a-zA-Z]+|.)`)},
	{False, regexp.MustCompile(`^#f`)},
	{True, regexp.MustCompile(`^#t`)},
	{UnquoteSplicing, regexp.MustCompile(`^,@`)},
	// single-character punctuation
	{LeftParen, regexp.MustCompile(`^\(`)},
	{RightParen, regexp.MustCompile(`^\)`)},
	{LeftBrack, regexp.MustCompile(`^\[`)},
	{RightBrack, regexp.MustCompile(`^\]`)},
	{LeftBrace, regexp.MustCompile(`^\{`)},
	{RightBrace, regexp.MustCompile(`^\}`)},
	{Quote, regexp.MustCompile(`^'`)},
	{QuasiQuote, regexp.MustCompile("^`")},
	{Unquote, regexp.MustCompile(`^,`)},
	// bar-quoted symbol |…|
	{Symbol, regexp.MustCompile(`^\|[^|]*\|`)},
	// any run of non-delimiter chars → Dot / Fixnum / Flonum / Symbol
	{reTokWord, regexp.MustCompile("^[^()\\[\\]{}\",';`#|\\\\ \\t\\r\\n\\v\\f]+")},
}

// Next returns the next token, skipping whitespace and line comments.
func (s *RegexListScanner) Next() Token {
outer:
	for {
		if s.pos >= len(s.input) {
			return Token{EOF, s.line, "<EOF>"}
		}
		remaining := s.input[s.pos:]
		for _, m := range reMatchers {
			text := m.re.FindString(remaining)
			if text == "" {
				continue
			}
			line := s.line
			s.pos += len(text)
			s.line += strings.Count(text, "\n")
			switch m.typ {
			case reTokSkip:
				continue outer
			case reTokWord:
				return classifyWord(text, line)
			default:
				return Token{m.typ, line, text}
			}
		}
		// No matcher succeeded: skip one rune and emit an error.
		_, width := utf8.DecodeRuneInString(remaining)
		if width == 0 {
			width = 1
		}
		s.pos += width
		return Token{Error, s.line, "unexpected: " + remaining[:width]}
	}
}
