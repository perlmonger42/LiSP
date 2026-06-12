// StateMachineScanner is a channel-based state machine scanner following the
// Rob Pike Ivy lexer pattern (https://github.com/robpike/ivy/blob/master/scan/scan.go).
//
// Design: each state is a function that consumes input and returns the next
// state function. Tokens are sent on a buffered channel; Next() drives the
// machine by calling state functions until a token appears on the channel.
// Input is read lazily line-by-line from an io.ByteReader, so the scanner
// works on streaming input, not just in-memory strings.
//
// Performance: ~40 MB/s. The goroutine-free "cooperative" channel design
// (driving the state machine on demand rather than in a background goroutine)
// keeps overhead reasonable, but io.ByteReader reads and line buffering add
// cost compared to scanning an in-memory string directly.
package lexers

import (
	"fmt"
	"io"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

const eof = -1

// stateFn represents the state of the scanner as a function that returns the
// next state.
type stateFn func(*StateMachineScanner) stateFn

// StateMachineScanner holds the state of the scanner.
type StateMachineScanner struct {
	tokens chan Token // channel of scanned items
	r      io.ByteReader
	done   bool
	name   string // the name of the input; used only for error reports
	buf    []byte
	input  string  // the line of text being scanned
	state  stateFn // the next lexing function to enter
	line   int     // line number in input
	pos    int     // current position in the input
	start  int     // start position of this item
	width  int     // width of last rune read from input

	lookahead bool  // Peek is usable
	Lookahead Token // The lookahead token
}

// loadLine reads the next line of input and stores it in (appends it to) the
// input. (l.input may have data left over when we are called.)
// It strips carriage returns to make subsequent processing simpler.
func (l *StateMachineScanner) loadLine() {
	l.buf = l.buf[:0]
	for {
		c, err := l.r.ReadByte()
		if err != nil {
			l.done = true
			break
		}
		if c != '\r' {
			l.buf = append(l.buf, c)
		}
		if c == '\n' {
			break
		}
	}
	l.input = l.smTokenText() + string(l.buf)
	l.pos -= l.start
	l.start = 0
}

// next returns the next rune in the input.
func (l *StateMachineScanner) next() (r rune) {
	if !l.done && int(l.pos) == len(l.input) {
		l.loadLine()
	}
	if int(l.pos) == len(l.input) {
		l.width = 0
		return eof
	}
	r, l.width = utf8.DecodeRuneInString(l.input[l.pos:])
	l.pos += l.width
	return r
}

// peek returns but does not consume the next rune in the input.
func (l *StateMachineScanner) peek() rune {
	r := l.next()
	l.backup()
	return r
}

// backup steps back one rune. Can only be called once per call of next.
func (l *StateMachineScanner) backup() {
	l.pos -= l.width
}

func (l *StateMachineScanner) newline() {
	l.line++
}

func (l *StateMachineScanner) smTokenText() string {
	return l.input[l.start:l.pos]
}

// emit passes an item back to the client.
func (l *StateMachineScanner) emit(t Type) {
	token := Token{t, l.line, l.smTokenText()}
	l.tokens <- token
	l.start = l.pos
	l.width = 0
}

// ignore skips over the pending input before this point.
func (l *StateMachineScanner) ignore() {
	l.start = l.pos
}

// acceptRun consumes a run of runes from the valid set.
func (l *StateMachineScanner) acceptRun(valid string) {
	for strings.IndexRune(valid, l.next()) >= 0 {
	}
	l.backup()
}

// acceptIsRun consumes a run of runes from the valid set.
func (l *StateMachineScanner) acceptIsRun(isValid func(rune) bool) {
	for isValid(l.next()) {
	}
	l.backup()
}

// acceptLimitedIsRun consumes up to maxCount runes from the valid set.
func (l *StateMachineScanner) acceptLimitedIsRun(isValid func(rune) bool, maxCount int64) {
	for isValid(l.next()) && maxCount > 0 {
		maxCount -= 1
	}
	l.backup()
}

// isLineSeparator reports whether the argument is a line separator.
// If r is '\r' and l.peek() is '\n', consumes the '\n' and returns true.
func (l *StateMachineScanner) isLineSeparator(r rune) bool {
	if r == '\r' && l.peek() == '\n' {
		l.next()
		return true
	}
	return r == '\n' || r == '\v' || r == '\f' || r == '\r' ||
		r == '\x85' || r == ' ' || r == ' '
}

// error returns an error token and continues to scan.
func (l *StateMachineScanner) error(msg string) stateFn {
	return l.errorf("%s `%s`", msg, l.smTokenText())
}

// errorf returns an error token and continues to scan.
func (l *StateMachineScanner) errorf(format string, args ...interface{}) stateFn {
	l.tokens <- Token{Error, l.start, fmt.Sprintf(format, args...)}
	return smLexAny
}

// NewStateMachineScanner creates a new scanner for the input stream.
func NewStateMachineScanner(name string, r io.ByteReader) *StateMachineScanner {
	l := &StateMachineScanner{
		r:      r,
		name:   name,
		line:   1,
		tokens: make(chan Token, 2),
		state:  smLexAny,
	}
	return l
}

// Next returns the next token.
func (l *StateMachineScanner) Next() (result Token) {
	// We have up to one token of lookahead.
	if l.lookahead {
		l.lookahead = false
		return l.Lookahead
	}
	// The lexer is concurrent but we don't want it to run in parallel
	// with the rest of the interpreter, so we only run the state machine
	// when we need a token.
	for l.state != nil {
		select {
		case tok := <-l.tokens:
			return tok
		default:
			// Run the machine
			l.state = l.state(l)
		}
	}
	if l.tokens != nil {
		close(l.tokens)
		l.tokens = nil
	}
	return Token{EOF, l.pos, "<EOF>"}
}

// Peek returns the next token without consuming it.
func (l *StateMachineScanner) Peek() (result Token) {
	if l.lookahead {
		return l.Lookahead
	}
	l.Lookahead = l.Next()
	l.lookahead = true
	return l.Lookahead
}

// state functions

// smLexLineComment scans a ;-to-eol comment. The `;` has been consumed.
func smLexLineComment(l *StateMachineScanner) stateFn {
	for r := l.next(); !l.isLineSeparator(r); r = l.next() {
		if r == eof {
			l.ignore()
			return smLexAny
		}
	}
	l.newline()
	l.ignore()
	return smLexAny
}

// smLexBlockComment scans a (nestable) #|-to-|# comment. The `#|` has been consumed.
func smLexBlockComment(l *StateMachineScanner) stateFn {
	depth := 1
	for {
		switch r := l.next(); {
		case r == '|' && l.peek() == '#':
			l.next()
			depth -= 1
			if depth <= 0 {
				l.ignore()
				return smLexAny
			}
		case r == '#' && l.peek() == '|':
			l.next()
			depth += 1
		case r == eof:
			return l.errorf("unterminated block comment")
		}
	}
}

// smLexAny scans non-space items.
func smLexAny(l *StateMachineScanner) stateFn {
	switch r := l.next(); {
	case r == eof:
		return nil
	case l.isLineSeparator(r):
		l.newline()
		l.ignore()
		return smLexAny
	case unicode.IsSpace(r):
		return smLexSpace

	case r == '(':
		l.emit(LeftParen)
		return smLexAny
	case r == ')':
		l.emit(RightParen)
		return smLexAny
	case r == '[':
		l.emit(LeftBrack)
		return smLexAny
	case r == ']':
		l.emit(RightBrack)
		return smLexAny
	case r == '{':
		l.emit(LeftBrace)
		return smLexAny
	case r == '}':
		l.emit(RightBrace)
		return smLexAny
	case r == '"':
		return smLexString
	case r == '.':
		if l.smIsDelimiter(l.peek()) {
			l.emit(Dot)
			return smLexAny
		}
		return smLexSymbol
	case r == ',':
		if l.peek() == '@' {
			l.next()
			l.emit(UnquoteSplicing)
		} else {
			l.emit(Unquote)
		}
		return smLexAny
	case r == '\'':
		l.emit(Quote)
		return smLexAny
	case r == '`':
		l.emit(QuasiQuote)
		return smLexAny
	case r == ';':
		return smLexLineComment
	case r == '#':
		return smLexPoundSign
	case r == '|':
		return smLexBarSymbol
	default:
		return smLexSymbol
	}
}

// smLexSpace scans a run of space characters. One space has already been seen.
func smLexSpace(l *StateMachineScanner) stateFn {
	for unicode.IsSpace(l.peek()) {
		r := l.next()
		if l.isLineSeparator(r) {
			l.line++
		}
	}
	l.ignore()
	return smLexAny
}

func smLexPoundSign(l *StateMachineScanner) stateFn {
	r := l.next()
	switch r {
	case '%':
		return smLexSymbol
	case '|':
		return smLexBlockComment
	case '\\':
		return smLexChar
	case 't', 'f':
		if l.smIsDelimiter(l.peek()) {
			if r == 'f' {
				l.emit(False)
			} else {
				l.emit(True)
			}
			return smLexAny
		}
		l.next()
		return l.error("bad # syntax")
	}
	return l.errorf("bad character following #: %#U", r)
}

// smLexSymbol scans a Scheme symbol.
func smLexSymbol(l *StateMachineScanner) stateFn {
Loop:
	for {
		switch r := l.next(); {
		case r == eof:
			break Loop
		case r == '\\':
			r = l.next()
			if r == eof {
				return l.error("eof after \\ in symbol")
			}
		case r == '#':
			// allowed in the middle of a symbol
		case r == '|':
			return smLexBarSymbol
		case l.smIsDelimiter(r):
			l.backup()
			break Loop
		}
	}

	// If the symbol looks like a number, it is a number.
	text := l.smTokenText()
	_, err := strconv.ParseInt(text, 0, 64)
	if err == nil {
		l.emit(Fixnum)
	} else if err.(*strconv.NumError).Err == strconv.ErrRange {
		return l.error("Bignums not yet implemented")
	} else if _, err = strconv.ParseFloat(text, 64); err == nil {
		l.emit(Flonum)
	} else if err.(*strconv.NumError).Err == strconv.ErrRange {
		return l.error("Bignums not yet implemented")
	} else if err.(*strconv.NumError).Err == strconv.ErrSyntax {
		l.emit(Symbol)
	} else {
		panic(fmt.Sprintf("unexpected strconv error on %q: %v", text, err))
	}
	return smLexAny
}

func smLexBarSymbol(l *StateMachineScanner) stateFn {
	for r := l.next(); r != eof && r != '|'; r = l.next() {
	}
	return smLexSymbol
}

// smIsDelimiter reports whether the argument is a delimiter character.
func (l *StateMachineScanner) smIsDelimiter(r rune) bool {
	return unicode.IsSpace(r) ||
		strings.IndexRune("()[]{}\",'`;#|\\", r) >= 0 ||
		r == eof
}

// smLexChar scans a character constant. The leading #\ is already scanned.
func smLexChar(l *StateMachineScanner) stateFn {
	switch r := l.next(); {
	case r == 'o' && smIsOctDigit(l.peek()):
		l.acceptIsRun(smIsOctDigit)
	case r == 'u' && smIsHexDigit(l.peek()):
		l.acceptLimitedIsRun(smIsHexDigit, 4)
	case r == 'U' && smIsHexDigit(l.peek()):
		l.acceptLimitedIsRun(smIsHexDigit, 6)
	case unicode.IsLetter(r) && unicode.IsLetter(l.peek()):
		l.acceptIsRun(unicode.IsLetter)
		if namedCharacter(l.input[l.start+2:l.pos]) < 0 {
			return l.error("unrecognized character name")
		}
	case smIsOctDigit(r):
		l.acceptRun("01234567")
		runes := len([]rune(l.smTokenText()))
		if runes != 3 && runes != 5 {
			return l.error("bad octal character syntax")
		}
	case unicode.IsLetter(r) != unicode.IsLetter(l.peek()):
		// <letter followed by nonletter> or <nonletter followed by letter>
	case l.smIsDelimiter(l.peek()):
		// character followed by delimiter
	default:
		// <nonletter followed by nonletter>
		if l.peek() != eof {
			return l.error("bad character syntax")
		}
	}
	l.emit(Char)
	return smLexAny
}

func smIsHexDigit(r rune) bool {
	return '0' <= r && r <= '9' ||
		'a' <= r && r <= 'f' ||
		'A' <= r && r <= 'F'
}

func smIsOctDigit(r rune) bool {
	return '0' <= r && r <= '7'
}

// namedCharacter maps a character name string to its rune value.
// Returns -1 if the name is not recognized.
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

// CharLiteralToRune converts a Scheme character literal string (e.g. "#\space",
// "#⋮", "#\a") to the corresponding rune. Panics on invalid input.
func CharLiteralToRune(s string) rune {
	runes := len([]rune(s))
	switch {
	case runes < 3 || s[0] != '#' || s[1] != '\\':
		// not even close to looking like a char literal
	case runes == 3:
		if r, size := utf8.DecodeRuneInString(s[2:]); size > 0 {
			return r
		}
	case s[2] == 'u' || s[2] == 'U': // runes > 3
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

// smLexString scans a quoted string.
func smLexString(l *StateMachineScanner) stateFn {
	for {
		switch r := l.next(); {
		case r == '\\':
			if r := l.next(); r != eof && !l.isLineSeparator(r) {
				break // switch
			}
			fallthrough
		case r == eof || l.isLineSeparator(r):
			return l.errorf("unterminated quoted string")
		case r == '"':
			l.emit(String)
			return smLexAny
		}
	}
}
