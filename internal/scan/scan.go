// Hand-written Scheme lexer using first-byte dispatch and direct byte scanning.
// Derived from InlineScanner, the winner of a six-way performance comparison
// (see research/scan-lexers/README.md for details and benchmarks).
//
// Line terminator handling:
//
//   All eight Unicode line terminators are handled in findLineEnd and
//   advanceLine — exactly once per line boundary. Every other function in this
//   file sees only content bytes; no byte in s.input is ever a line terminator.
//
//   The eight forms: LF U+000A, VT U+000B, FF U+000C, CR U+000D, CR+LF,
//   NEL U+0085 (0xC2 0x85), LS U+2028 (0xE2 0x80 0xA8),
//   PS U+2029 (0xE2 0x80 0xA9). CR+LF counts as one line.
//
// Non-streaming mode (NewScanner):
//   s.full holds the original input string, never modified. s.input is a
//   zero-copy substring of s.full covering the current line's content (no
//   terminator). Advancing to the next line updates s.input to the next
//   substring — no allocation.
//
// Streaming mode (NewStreamScanner):
//   refillLine reads one line from s.reader into a fresh []byte, excluding
//   the terminator. advanceLine calls refillLine and increments s.line.

//go:generate stringer -type Type

package scan

import (
	"fmt"
	"io"
	"strconv"
	"unicode"
	"unicode/utf8"
)

// Token represents a token returned from the scanner.
type Token struct {
	Type Type
	Line int
	Text string
}

// Type identifies the type of lex items.
type Type int

const (
	EOF   Type = iota // zero value so a closed channel delivers EOF
	Error             // error occurred; value is text of error

	// Scheme tokens
	LeftParen       // '('
	LeftBrack       // '['
	LeftBrace       // '{'
	Quote           // '\''
	QuasiQuote      // '`'
	Unquote         // ','
	UnquoteSplicing // ",@"
	False           // "#f"
	True            // "#t"
	Dot             // "."
	Ellipsis        // "..."
	Fixnum          // a number with no fractional component
	Flonum          // a number with a fractional component
	Char            // character literal, e.g. #\space or #\m or #\xFACE
	Symbol          // a Scheme symbol
	String          // quoted string (includes quotes)
	RightParen      // ')'
	RightBrack      // ']'
	RightBrace      // '}'
)

func (i Token) String() string {
	switch {
	case i.Type == EOF:
		return "<EOF>"
	case i.Type == Error:
		return "error: " + i.Text
	case len(i.Text) > 10:
		return fmt.Sprintf("%v: %.10q...", i.Type, i.Text)
	}
	return fmt.Sprintf("%v: %q", i.Type, i.Text)
}

// Scanner holds the state of the lexer.
type Scanner struct {
	full       string        // full input string (non-streaming only; never modified)
	lineStart  int           // absolute start of s.input within s.full (non-streaming)
	input      string        // current line content — no line terminator bytes
	pos        int           // current position within input
	line       int           // current line number
	reader     io.ByteReader // non-nil in streaming mode
	pending    byte          // one-byte lookahead for streaming CR+LF handling
	hasPending bool
	firstLoad  bool // streaming: true until the first line has been loaded
	lookahead  bool
	Lookahead  Token
}

// NewScanner creates a fast scanner for the given input string.
// s.input starts as a zero-copy substring of input covering the first line.
func NewScanner(name string, input string) *Scanner {
	end := findLineEnd(input, 0)
	return &Scanner{
		full:  input,
		input: input[:end],
		line:  1,
	}
}

// NewStreamScanner creates a scanner that reads from r lazily, one line at a
// time. Use this for file input or interactive readline sessions.
func NewStreamScanner(name string, r io.ByteReader) *Scanner {
	return &Scanner{reader: r, line: 1, firstLoad: true}
}

// findLineEnd returns the index in s of the first byte of any line terminator
// at or after from, or len(s) if there is none. The result is the exclusive
// end of the line content — the terminator bytes themselves are not included.
func findLineEnd(s string, from int) int {
	for i := from; i < len(s); i++ {
		switch s[i] {
		case '\n', '\r', '\v', '\f':
			return i
		case 0xC2: // NEL U+0085 = 0xC2 0x85
			if i+1 < len(s) && s[i+1] == 0x85 {
				return i
			}
		case 0xE2: // LS U+2028 = 0xE2 0x80 0xA8 or PS U+2029 = 0xE2 0x80 0xA9
			if i+2 < len(s) && s[i+1] == 0x80 && (s[i+2] == 0xA8 || s[i+2] == 0xA9) {
				return i
			}
		}
	}
	return len(s)
}

// advanceLine moves to the next line. For non-streaming mode it updates s.input
// to the next zero-copy substring of s.full. For streaming mode it calls
// refillLine. Returns false at EOF.
func (s *Scanner) advanceLine() bool {
	if s.reader != nil {
		if !s.refillLine() {
			return false
		}
		if s.firstLoad {
			s.firstLoad = false // line stays at 1
		} else {
			s.line++
		}
		return true
	}
	// Non-streaming: skip the terminator at s.full[absEnd].
	absEnd := s.lineStart + len(s.input)
	if absEnd >= len(s.full) {
		return false
	}
	var nextStart int
	switch s.full[absEnd] {
	case '\r':
		nextStart = absEnd + 1
		if nextStart < len(s.full) && s.full[nextStart] == '\n' {
			nextStart++ // CR+LF → one line
		}
	case 0xC2:
		nextStart = absEnd + 2 // NEL: 2-byte sequence
	case 0xE2:
		nextStart = absEnd + 3 // LS/PS: 3-byte sequence
	default: // '\n', '\v', '\f'
		nextStart = absEnd + 1
	}
	s.line++
	s.lineStart = nextStart
	nextEnd := findLineEnd(s.full, nextStart)
	s.input = s.full[nextStart:nextEnd]
	s.pos = 0
	return true
}

// refillLine reads one line from s.reader into s.input, excluding any line
// terminator. Returns false only when the reader is exhausted with no bytes
// read (true EOF). An empty line — a terminator with no preceding content —
// returns true with s.input == "".
func (s *Scanner) refillLine() bool {
	readByte := func() (byte, bool) {
		if s.hasPending {
			s.hasPending = false
			return s.pending, true
		}
		b, err := s.reader.ReadByte()
		return b, err == nil
	}

	var buf []byte
	hitTerminator := false
loop:
	for {
		b, ok := readByte()
		if !ok {
			break
		}
		switch b {
		case '\n', '\v', '\f':
			hitTerminator = true
			break loop
		case '\r':
			hitTerminator = true
			if next, ok2 := readByte(); ok2 && next != '\n' {
				s.pending = next
				s.hasPending = true
			}
			break loop
		case 0xC2:
			if next, ok2 := readByte(); ok2 {
				if next == 0x85 {
					hitTerminator = true
					break loop
				}
				buf = append(buf, b, next)
			} else {
				buf = append(buf, b)
			}
		case 0xE2:
			b2, ok2 := readByte()
			if !ok2 {
				buf = append(buf, b)
				break loop
			}
			b3, ok3 := readByte()
			if !ok3 {
				buf = append(buf, b, b2)
				break loop
			}
			if b2 == 0x80 && (b3 == 0xA8 || b3 == 0xA9) {
				hitTerminator = true
				break loop
			}
			buf = append(buf, b, b2, b3)
		default:
			buf = append(buf, b)
		}
	}
	if len(buf) == 0 && !hitTerminator {
		return false // true EOF
	}
	s.input = string(buf)
	s.pos = 0
	return true
}

// Peek returns the next token without consuming it.
func (s *Scanner) Peek() Token {
	if s.lookahead {
		return s.Lookahead
	}
	s.Lookahead = s.Next()
	s.lookahead = true
	return s.Lookahead
}

// Next returns the next token, skipping whitespace and line comments.
// When pos reaches the end of the current line, advanceLine is called;
// the caller never sees line-terminator bytes.
func (s *Scanner) Next() Token {
	if s.lookahead {
		s.lookahead = false
		return s.Lookahead
	}
outer:
	for {
		// Advance past empty or exhausted lines.
		for s.pos >= len(s.input) {
			if !s.advanceLine() {
				return Token{EOF, s.line, "<EOF>"}
			}
		}
		remaining := s.input[s.pos:]
		line := s.line

		switch remaining[0] {
		case ' ', '\t':
			s.skipWS()
			continue outer
		case ';':
			s.skipLineComment()
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
			if len(remaining) > 1 && remaining[1] == '|' {
				s.pos += 2
				if !s.skipBlockComment() {
					return Token{Error, line, "unterminated block comment"}
				}
				continue outer
			}
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

func (s *Scanner) skipWS() {
	for s.pos < len(s.input) {
		b := s.input[s.pos]
		if b != ' ' && b != '\t' {
			return
		}
		s.pos++
	}
}

// skipLineComment skips to the end of the current line.
// advanceLine (called from the Next outer loop) will handle the EOL itself.
func (s *Scanner) skipLineComment() {
	s.pos = len(s.input)
}

// skipBlockComment skips a nestable #|…|# block comment. The opening #| has
// already been consumed. Returns false if the comment is unterminated.
//
// last carries the previous byte across line boundaries so that |# or #|
// split across a line end is still recognised correctly.
func (s *Scanner) skipBlockComment() bool {
	depth := 1
	var last byte
	for {
		for s.pos < len(s.input) {
			b := s.input[s.pos]
			s.pos++
			switch {
			case last == '|' && b == '#':
				depth--
				last = 0
				if depth == 0 {
					return true
				}
			case last == '#' && b == '|':
				depth++
				last = 0
			case b == '|' || b == '#':
				last = b
			default:
				last = 0
			}
		}
		if !s.advanceLine() {
			return false
		}
	}
}

// scanHash handles tokens beginning with '#': #f, #t, and character literals.
func (s *Scanner) scanHash(line int) Token {
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
		s.pos += 2 // consume '#' and the unrecognised byte after it
		return Token{Error, line, "unexpected: " + bad}
	}
}

// scanChar scans a #\ character literal. Handles four forms:
//
//	#\xHHHH…  or  #\XHHHHHH…  (x/X followed by hex digits)
//	#\oOOO…                    (o followed by octal digits)
//	#\name                      (run of ASCII letters → named character)
//	#\c                         (any other single character, including Unicode)
func (s *Scanner) scanChar(line int) Token {
	start := s.pos
	s.pos += 2 // consume '#' and '\'
	if s.pos >= len(s.input) {
		return Token{Error, line, "unexpected EOF in char literal"}
	}
	b := s.input[s.pos]
	switch {
	case (b == 'x' || b == 'X') && s.pos+1 < len(s.input) && isHexByte(s.input[s.pos+1]):
		s.pos++ // consume 'x'/'X'
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
		_, width := utf8.DecodeRuneInString(s.input[s.pos:])
		if width == 0 {
			width = 1
		}
		s.pos += width
	}
	return Token{Char, line, s.input[start:s.pos]}
}

// scanString scans a double-quoted string literal. Reaching end-of-line before
// the closing '"' is an unterminated-string error.
func (s *Scanner) scanString(line int) Token {
	start := s.pos
	s.pos++ // consume opening '"'
	for s.pos < len(s.input) {
		b := s.input[s.pos]
		s.pos++
		switch b {
		case '"':
			return Token{String, line, s.input[start:s.pos]}
		case '\\':
			if s.pos >= len(s.input) {
				return Token{Error, line, "unterminated quoted string"}
			}
			s.pos++ // consume the escaped character
		}
	}
	return Token{Error, line, "unterminated quoted string"}
}

// scanBarSymbol scans a bar-quoted symbol |…|.
func (s *Scanner) scanBarSymbol(line int) Token {
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

// scanWord scans a run of non-delimiter bytes and classifies the result.
func (s *Scanner) scanWord(line int) Token {
	start := s.pos
	for s.pos < len(s.input) && !isDelimByte(s.input[s.pos]) {
		s.pos++
	}
	return classifyWord(s.input[start:s.pos], line)
}

// classifyWord interprets a non-delimiter run as Dot, Fixnum, Flonum, or Symbol.
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

// isDelimByte reports whether b terminates an unquoted symbol.
func isDelimByte(b byte) bool {
	switch b {
	case '(', ')', '[', ']', '{', '}', '"', ',', '\'', '`', ';', '#', '|', '\\',
		' ', '\t', '\n':
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

// CharLiteralToRune converts a #\… token to the rune it represents.
func CharLiteralToRune(s string) rune {
	runes := len([]rune(s))
	switch {
	case runes < 3 || s[0] != '#' || s[1] != '\\':
		// not a char literal
	case runes == 3:
		if r, size := utf8.DecodeRuneInString(s[2:]); size > 0 {
			return r
		}
	case s[2] == 'x' || s[2] == 'X':
		n, err := strconv.ParseInt(s[3:], 16, 64)
		if err == nil && n <= unicode.MaxRune {
			return rune(n)
		}
	default:
		if r := namedCharacter(s[2:]); r >= 0 {
			return r
		}
	}
	panic(fmt.Sprintf("invalid char literal %q", s))
}

func namedCharacter(s string) rune {
	switch s {
	case "nul", "null":
		return 0
	case "backspace":
		return '\010'
	case "tab":
		return '\011'
	case "newline", "linefeed":
		return '\012'
	case "vtab":
		return '\013'
	case "page":
		return '\014'
	case "return":
		return '\015'
	case "space":
		return '\040'
	case "rubout":
		return '\177'
	default:
		return -1
	}
}
