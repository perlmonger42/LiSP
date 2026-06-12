// SlurpedStateMachineScanner is a clean reimplementation of the Rob Pike / Ivy
// state-machine pattern applied to a pre-loaded string instead of a line-by-line
// io.ByteReader. The architecture is identical to 01_state_machine.go:
//
//   - State is a function: type slurpedFn func(*SlurpedStateMachineScanner) slurpedFn
//   - Tokens are sent on a small buffered channel via emit()
//   - Next() drives the machine by calling state functions until a token appears
//
// The sole structural difference is the input model: all bytes are available as
// a string from the start, so there is no loadLine(), no io.ByteReader, and no
// line-buffering overhead. This isolates the cost of the channel-based emit
// pattern from the io.ByteReader overhead of the original scanner.
//
// Token set: identical to InlineScanner (05_inline.go) — same delimiter set,
// same character-literal rules, no '#' mid-symbol, no block comments.
//
// Performance: ~65 MB/s. Faster than the original state machine (~41 MB/s)
// because the io.ByteReader overhead is removed, but slower than InlineScanner
// (~124 MB/s) because the channel send+receive pair per token remains.
// The 1.6× gain over the original isolates the io.ByteReader cost; the 1.9×
// gap remaining to InlineScanner isolates the channel-emit cost.
package lexers

import (
	"fmt"
	"unicode/utf8"
)

// slurpedFn is the state-function type for SlurpedStateMachineScanner.
// It is kept distinct from stateFn (which takes *StateMachineScanner) so both
// scanner types can coexist in the same package.
type slurpedFn func(*SlurpedStateMachineScanner) slurpedFn

// SlurpedStateMachineScanner implements the Rob Pike / Ivy state-machine lexer
// on a pre-loaded string.
type SlurpedStateMachineScanner struct {
	input  string
	pos    int
	start  int
	line   int
	tokens chan Token
	state  slurpedFn
}

// NewSlurpedStateMachineScanner returns a scanner for the given input string.
func NewSlurpedStateMachineScanner(input string) *SlurpedStateMachineScanner {
	return &SlurpedStateMachineScanner{
		input:  input,
		line:   1,
		tokens: make(chan Token, 2),
		state:  slAny,
	}
}

// emit sends a token of the given type to the caller. The token text is
// s.input[s.start:s.pos]; start is then advanced to pos.
func (s *SlurpedStateMachineScanner) emit(t Type) {
	s.tokens <- Token{t, s.line, s.input[s.start:s.pos]}
	s.start = s.pos
}

// emitError sends an Error token with the given message and resets start.
func (s *SlurpedStateMachineScanner) emitError(msg string) {
	s.tokens <- Token{Error, s.line, msg}
	s.start = s.pos
}

// errorf sends a formatted Error token and returns slAny.
func (s *SlurpedStateMachineScanner) errorf(format string, args ...interface{}) slurpedFn {
	s.emitError(fmt.Sprintf(format, args...))
	return slAny
}

// Next returns the next token, running the state machine until one is ready.
func (s *SlurpedStateMachineScanner) Next() Token {
	for s.state != nil {
		select {
		case tok := <-s.tokens:
			return tok
		default:
			s.state = s.state(s)
		}
	}
	if s.tokens != nil {
		close(s.tokens)
		s.tokens = nil
	}
	return Token{EOF, s.line, "<EOF>"}
}

// slAny is the top-level state. It skips whitespace and line comments, then
// dispatches on the first byte of the next token.
func slAny(s *SlurpedStateMachineScanner) slurpedFn {
	// Skip whitespace and line comments in a tight inner loop.
	for s.pos < len(s.input) {
		switch s.input[s.pos] {
		case '\n':
			s.line++
			s.pos++
			continue
		case ' ', '\t', '\r', '\v', '\f':
			s.pos++
			continue
		case ';':
			for s.pos < len(s.input) && s.input[s.pos] != '\n' {
				s.pos++
			}
			continue
		}
		break
	}
	s.start = s.pos // discard whitespace

	if s.pos >= len(s.input) {
		return nil
	}

	b := s.input[s.pos]
	s.pos++

	switch b {
	case '(':
		s.emit(LeftParen)
	case ')':
		s.emit(RightParen)
	case '[':
		s.emit(LeftBrack)
	case ']':
		s.emit(RightBrack)
	case '{':
		s.emit(LeftBrace)
	case '}':
		s.emit(RightBrace)
	case '\'':
		s.emit(Quote)
	case '`':
		s.emit(QuasiQuote)
	case ',':
		if s.pos < len(s.input) && s.input[s.pos] == '@' {
			s.pos++
			s.emit(UnquoteSplicing)
		} else {
			s.emit(Unquote)
		}
	case '#':
		return slHash
	case '"':
		return slString
	case '|':
		return slBarSymbol
	case '\\':
		s.emitError("unexpected: \\")
	default:
		s.pos-- // back up; slWord scans from s.start
		return slWord
	}
	return slAny
}

// slHash handles tokens beginning with '#': #f, #t, and character literals.
func slHash(s *SlurpedStateMachineScanner) slurpedFn {
	if s.pos >= len(s.input) {
		s.emitError("unexpected EOF after #")
		return slAny
	}
	switch s.input[s.pos] {
	case 'f':
		s.pos++
		s.emit(False)
	case 't':
		s.pos++
		s.emit(True)
	case '\\':
		s.pos++ // consume '\'
		return slChar
	default:
		s.emitError("unexpected: #")
	}
	return slAny
}

// slChar scans a #\ character literal. The '#' and '\' have already been
// consumed. Handles the same four forms as InlineScanner.scanChar:
//
//	#\uHHHH…  or  #\UHHHHHH…  — hex Unicode scalar
//	#\oOOO…                    — octal
//	#\name                      — ASCII letter run (named character)
//	#\x                         — any other single character
func slChar(s *SlurpedStateMachineScanner) slurpedFn {
	if s.pos >= len(s.input) {
		s.emitError("unexpected EOF in char literal")
		return slAny
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
		_, width := utf8.DecodeRuneInString(s.input[s.pos:])
		if width == 0 {
			width = 1
		}
		s.pos += width
	}
	s.emit(Char)
	return slAny
}

// slString scans a double-quoted string literal. The opening '"' has already
// been consumed.
func slString(s *SlurpedStateMachineScanner) slurpedFn {
	for s.pos < len(s.input) {
		b := s.input[s.pos]
		s.pos++
		switch b {
		case '"':
			s.emit(String)
			return slAny
		case '\n':
			s.pos--
			s.emitError("unterminated string")
			return slAny
		case '\\':
			if s.pos >= len(s.input) || s.input[s.pos] == '\n' {
				s.emitError("unterminated string")
				return slAny
			}
			s.pos++ // consume escaped character
		}
	}
	s.emitError("unterminated string")
	return slAny
}

// slBarSymbol scans a bar-quoted symbol |…|. The opening '|' has already been
// consumed.
func slBarSymbol(s *SlurpedStateMachineScanner) slurpedFn {
	for s.pos < len(s.input) && s.input[s.pos] != '|' {
		s.pos++
	}
	if s.pos >= len(s.input) {
		s.emitError("unterminated bar symbol")
		return slAny
	}
	s.pos++ // consume closing '|'
	s.emit(Symbol)
	return slAny
}

// slWord scans a run of non-delimiter bytes and classifies it as Dot, Fixnum,
// Flonum, or Symbol. s.pos must be at the first character of the word.
func slWord(s *SlurpedStateMachineScanner) slurpedFn {
	for s.pos < len(s.input) && !isDelimByte(s.input[s.pos]) {
		s.pos++
	}
	tok := classifyWord(s.input[s.start:s.pos], s.line)
	s.tokens <- tok
	s.start = s.pos
	return slAny
}
