// BigRegexScanner tokenizes Scheme source using a single compiled regular
// expression with one named capture group per token type.
//
// Design: the entire token grammar is encoded as a single alternation regex
// with named groups ((?P<Name>…)). FindStringSubmatchIndex runs the NFA once
// per token. After a match, we scan the subgroup result slice to find which
// group matched and look up the corresponding token type.
//
// Theoretically this approach makes one NFA pass per token rather than one per
// matcher, but in practice the 36-element subgroup index slice allocated on
// every call makes it the slowest of all six approaches. The allocation cost
// dominates the NFA savings.
//
// Limitations: same as RegexListScanner ('#' mid-symbol splits; no nestable
// block comments; ASCII-only whitespace).
//
// Performance: ~5 MB/s. Slowest of all six — per-call slice allocation hurts.
package lexers

import (
	"regexp"
	"strings"
	"unicode/utf8"
)

// BigRegexScanner tokenizes using a single compiled regex with named groups.
type BigRegexScanner struct {
	input string
	pos   int
	line  int
}

// NewBigRegexScanner returns a scanner for the given input string.
func NewBigRegexScanner(input string) *BigRegexScanner {
	return &BigRegexScanner{input: input, line: 1}
}

var (
	bigRe      *regexp.Regexp
	bigReTypes []Type // bigReTypes[i] → Type for capture group i (1-indexed)
)

func init() {
	bigRe = regexp.MustCompile(
		`^(?:` +
			`(?P<Skip>[ \t\r\n\v\f]+|;[^\n]*)|` +
			`(?P<String>"(?:[^"\\]|\\.)*")|` +
			`(?P<Char>#\\(?:[uU][0-9a-fA-F]+|o[0-7]+|[a-zA-Z]+|.))|` +
			`(?P<False>#f)|` +
			`(?P<True>#t)|` +
			`(?P<UnquoteSplicing>,@)|` +
			`(?P<LeftParen>\()|` +
			`(?P<RightParen>\))|` +
			`(?P<LeftBrack>\[)|` +
			`(?P<RightBrack>\])|` +
			`(?P<LeftBrace>\{)|` +
			`(?P<RightBrace>\})|` +
			`(?P<Quote>')|` +
			"(?P<QuasiQuote>`)|" +
			`(?P<Unquote>,)|` +
			`(?P<BarSymbol>\|[^|]*\|)|` +
			"(?P<Word>[^()\\[\\]{}\",';`#|\\\\ \\t\\r\\n\\v\\f]+)" +
			`)`,
	)

	nameToType := map[string]Type{
		"Skip":            reTokSkip,
		"String":          String,
		"Char":            Char,
		"False":           False,
		"True":            True,
		"UnquoteSplicing": UnquoteSplicing,
		"LeftParen":       LeftParen,
		"RightParen":      RightParen,
		"LeftBrack":       LeftBrack,
		"RightBrack":      RightBrack,
		"LeftBrace":       LeftBrace,
		"RightBrace":      RightBrace,
		"Quote":           Quote,
		"QuasiQuote":      QuasiQuote,
		"Unquote":         Unquote,
		"BarSymbol":       Symbol,
		"Word":            reTokWord,
	}

	names := bigRe.SubexpNames()
	bigReTypes = make([]Type, len(names))
	for i, name := range names {
		if t, ok := nameToType[name]; ok {
			bigReTypes[i] = t
		}
	}
}

// Next returns the next token, skipping whitespace and line comments.
func (s *BigRegexScanner) Next() Token {
outer:
	for {
		if s.pos >= len(s.input) {
			return Token{EOF, s.line, "<EOF>"}
		}
		remaining := s.input[s.pos:]
		loc := bigRe.FindStringSubmatchIndex(remaining)
		if loc == nil {
			_, width := utf8.DecodeRuneInString(remaining)
			if width == 0 {
				width = 1
			}
			s.pos += width
			return Token{Error, s.line, "unexpected: " + remaining[:width]}
		}
		text := remaining[:loc[1]]
		line := s.line
		s.pos += loc[1]
		s.line += strings.Count(text, "\n")

		for i := 2; i < len(loc); i += 2 {
			if loc[i] < 0 {
				continue
			}
			switch bigReTypes[i/2] {
			case reTokSkip:
				continue outer
			case reTokWord:
				return classifyWord(text, line)
			default:
				return Token{bigReTypes[i/2], line, text}
			}
		}
		return Token{Error, line, "internal: no group matched"}
	}
}
