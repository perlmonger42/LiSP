package main

import (
	"io"
	"testing"
)

// TestRead_booleans: #t and #f are self-evaluating.
func TestRead_booleans(t *testing.T) {
	for _, input := range []string{"#t #t", "#f #f"} {
		failed, err := rerc(input)
		if failed || err != nil {
			t.Errorf("rerc(%q): got failed=%v err=%v, want false nil", input, failed, err)
		}
	}
}

// TestRead_char: character literal is self-evaluating.
func TestRead_char(t *testing.T) {
	failed, err := rerc(`#\a #\a`)
	if failed || err != nil {
		t.Errorf("got failed=%v err=%v, want false nil", failed, err)
	}
}

// TestRead_string: string literal is self-evaluating.
func TestRead_string(t *testing.T) {
	failed, err := rerc(`"hello" "hello"`)
	if failed || err != nil {
		t.Errorf("got failed=%v err=%v, want false nil", failed, err)
	}
}

// TestRead_quote: 'x reads as (quote x) and evaluates to x.
func TestRead_quote(t *testing.T) {
	failed, err := rerc("'hello hello")
	if failed || err != nil {
		t.Errorf("got failed=%v err=%v, want false nil", failed, err)
	}
}

// TestRead_quoteError: ' followed by an unterminated list propagates the read
// error, returning nil datum.  Also covers the LeftParen-hits-EOF path.
func TestRead_quoteError(t *testing.T) {
	failed, err := rerc("'(a b")
	if failed || err == nil || err == io.EOF {
		t.Errorf("got failed=%v err=%v, want false non-EOF error", failed, err)
	}
}

// TestRead_listInnerError: a quote error nested inside a list causes the outer
// LeftParen's inner-read-error path (return nil, err) to fire.
func TestRead_listInnerError(t *testing.T) {
	failed, err := rerc("('(a b")
	if failed || err == nil || err == io.EOF {
		t.Errorf("got failed=%v err=%v, want false non-EOF error", failed, err)
	}
}

// TestRead_default: a token that matches no case (RightParen at top level) hits
// the default branch, which returns nil datum with an error.
func TestRead_default(t *testing.T) {
	failed, err := rerc(")")
	if failed || err == nil || err == io.EOF {
		t.Errorf("got failed=%v err=%v, want false non-EOF error", failed, err)
	}
}
