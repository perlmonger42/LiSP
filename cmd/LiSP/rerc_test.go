package main

import (
	"LiSP/internal/scan"
	"io"
	"os"
	"testing"
)

// rerc is a convenience wrapper: initialise the sentinel symbols that Rercl
// normally sets up, then run one Rerc cycle on the given input string.
func rerc(input string) (failed bool, err error) {
	expectError = symbol("***")
	dontCare = symbol("---")
	return Rerc(scan.NewScanner("", input))
}

// TestRerc_valueMatch: evaluated result equals expected literal.
func TestRerc_valueMatch(t *testing.T) {
	failed, err := rerc("(+ 1 2) 3")
	if failed || err != nil {
		t.Errorf("got failed=%v err=%v, want false nil", failed, err)
	}
}

// TestRerc_valueMismatch: evaluated result does not equal expected literal.
func TestRerc_valueMismatch(t *testing.T) {
	failed, err := rerc("(+ 1 2) 4")
	if !failed || err != nil {
		t.Errorf("got failed=%v err=%v, want true nil", failed, err)
	}
}

// TestRerc_dontCare: --- accepts any non-error result without checking value.
func TestRerc_dontCare(t *testing.T) {
	failed, err := rerc("(+ 1 2) ---")
	if failed || err != nil {
		t.Errorf("got failed=%v err=%v, want false nil", failed, err)
	}
}

// TestRerc_errorExpected_errorOccurred: *** passes when evaluation errors.
func TestRerc_errorExpected_errorOccurred(t *testing.T) {
	failed, err := rerc("undefined-sym ***")
	if failed || err != nil {
		t.Errorf("got failed=%v err=%v, want false nil", failed, err)
	}
}

// TestRerc_errorExpected_noError: *** fails when evaluation succeeds.
func TestRerc_errorExpected_noError(t *testing.T) {
	failed, err := rerc("(+ 1 2) ***")
	if !failed || err != nil {
		t.Errorf("got failed=%v err=%v, want true nil", failed, err)
	}
}

// TestRerc_errorCode_match: (*** code) passes when the error's Condition code
// matches. (cond ()) raises code cond-clause-empty.
func TestRerc_errorCode_match(t *testing.T) {
	failed, err := rerc("(cond ()) (*** cond-clause-empty)")
	if failed || err != nil {
		t.Errorf("got failed=%v err=%v, want false nil", failed, err)
	}
}

// TestRerc_errorCode_mismatch: (*** code) fails when an error occurs but its
// code differs from the expected one.
func TestRerc_errorCode_mismatch(t *testing.T) {
	failed, err := rerc("(cond ()) (*** cond-clause-not-list)")
	if !failed || err != nil {
		t.Errorf("got failed=%v err=%v, want true nil", failed, err)
	}
}

// TestRerc_errorCode_noError: (*** code) fails when evaluation succeeds (no
// error to match the code against).
func TestRerc_errorCode_noError(t *testing.T) {
	failed, err := rerc("(+ 1 2) (*** cond-clause-empty)")
	if !failed || err != nil {
		t.Errorf("got failed=%v err=%v, want true nil", failed, err)
	}
}

// TestRerc_errorList_anyCode: (***) — a list form with no code — accepts any
// error, like bare ***.
func TestRerc_errorList_anyCode(t *testing.T) {
	failed, err := rerc("(cond ()) (***)")
	if failed || err != nil {
		t.Errorf("got failed=%v err=%v, want false nil", failed, err)
	}
}

// TestRerc_errorCode_genericFallback: an uncoded Fail carries the generic code
// `error`, so (*** error) matches it.
func TestRerc_errorCode_genericFallback(t *testing.T) {
	failed, err := rerc("undefined-sym (*** error)")
	if failed || err != nil {
		t.Errorf("got failed=%v err=%v, want false nil", failed, err)
	}
}

// TestErrorExpectation parses the (*** [code]) datum form directly.
func TestErrorExpectation(t *testing.T) {
	expectError = symbol("***")
	cases := []struct {
		name      string
		expect    scmer
		wantCode  symbol
		wantHas   bool
		wantIsExp bool
	}{
		{"bare-symbol-not-list", symbol("***"), "", false, false},
		{"list-no-code", array{symbol("***")}, "", false, true},
		{"list-with-code", array{symbol("***"), symbol("my-code")}, "my-code", true, true},
		{"not-an-error-expectation", array{symbol("foo"), symbol("bar")}, "", false, false},
		{"plain-value", flonum(3), "", false, false},
	}
	for _, c := range cases {
		code, hasCode, isExp := errorExpectation(c.expect)
		if code != c.wantCode || hasCode != c.wantHas || isExp != c.wantIsExp {
			t.Errorf("%s: errorExpectation(%v) = (%q, %v, %v), want (%q, %v, %v)",
				c.name, c.expect, code, hasCode, isExp, c.wantCode, c.wantHas, c.wantIsExp)
		}
	}
}

// TestRerc_eof: empty input returns io.EOF without failure.
func TestRerc_eof(t *testing.T) {
	failed, err := rerc("")
	if failed || err != io.EOF {
		t.Errorf("got failed=%v err=%v, want false io.EOF", failed, err)
	}
}

// TestRercl_multipleFormsPass: Rercl processes several passing RERC pairs and
// returns nil (normal end-of-input).
func TestRercl_multipleFormsPass(t *testing.T) {
	input := "(+ 1 2) 3\n" +
		"(* 2 3) 6\n" +
		"(- 10 4) 6\n"
	err := Rercl(scan.NewScanner("", input), false)
	if err != nil {
		t.Errorf("Rercl: got %v, want nil", err)
	}
}

// TestRepl_happyPath: Repl evaluates all expressions and returns nil.
func TestRepl_happyPath(t *testing.T) {
	s := scan.NewScanner("", "(+ 1 2)\n(* 2 3)")
	if err := Repl(s, false); err != nil {
		t.Errorf("Repl: got %v, want nil", err)
	}
}

// TestRepl_errorNonInteractive: a bad expression in non-interactive mode
// causes Repl to return the error immediately.
func TestRepl_errorNonInteractive(t *testing.T) {
	s := scan.NewScanner("", "undefined-sym")
	if err := Repl(s, false); err == nil {
		t.Error("Repl non-interactive: expected error, got nil")
	}
}

// TestRepl_errorInteractive: a bad expression in interactive mode is printed
// and the loop continues; Repl still returns nil.
func TestRepl_errorInteractive(t *testing.T) {
	s := scan.NewScanner("", "undefined-sym\n(+ 1 2)")
	if err := Repl(s, true); err != nil {
		t.Errorf("Repl interactive: got %v, want nil", err)
	}
}

// TestRun_replMode: Run in REPL mode with valid input returns true.
func TestRun_replMode(t *testing.T) {
	t.Chdir(t.TempDir()) // isolate readline's LiSP.history file
	saved := *rercMode
	t.Cleanup(func() { *rercMode = saved })
	*rercMode = false

	if ok := Run(scan.NewScanner("", "(+ 1 2)"), false); !ok {
		t.Error("Run REPL mode: got false, want true")
	}
}

// TestRun_rercMode: Run in RERC mode with passing pairs returns true.
func TestRun_rercMode(t *testing.T) {
	t.Chdir(t.TempDir())
	saved := *rercMode
	t.Cleanup(func() { *rercMode = saved })
	*rercMode = true

	if ok := Run(scan.NewScanner("", "(+ 1 2) 3"), false); !ok {
		t.Error("Run RERC mode: got false, want true")
	}
}

// TestRun_replMode_error: Run in REPL mode with an eval error returns false.
func TestRun_replMode_error(t *testing.T) {
	t.Chdir(t.TempDir())
	saved := *rercMode
	t.Cleanup(func() { *rercMode = saved })
	*rercMode = false

	if ok := Run(scan.NewScanner("", "undefined-sym"), false); ok {
		t.Error("Run REPL mode error: got true, want false")
	}
}

// TestRun_rercMode_error: Run in RERC mode with a missing expected value returns false.
func TestRun_rercMode_error(t *testing.T) {
	t.Chdir(t.TempDir())
	saved := *rercMode
	t.Cleanup(func() { *rercMode = saved })
	*rercMode = true

	// "(+ 1 2)" has no expected value; Rercl returns an error, so Run returns false.
	if ok := Run(scan.NewScanner("", "(+ 1 2)"), false); ok {
		t.Error("Run RERC mode error: got true, want false")
	}
}

// TestRerc_missingExpected: no expected value after datum; read returns io.EOF for
// the expected slot, which Rerc wraps as a non-nil error.
func TestRerc_missingExpected(t *testing.T) {
	failed, err := rerc("(+ 1 2)")
	if failed || err == nil {
		t.Errorf("got failed=%v err=%v, want false non-nil", failed, err)
	}
}

// TestRerc_unexpectedError: eval errors but expected is a concrete value (not ***).
// fail() prints and clears err, so Rerc returns failed=true with err==nil.
func TestRerc_unexpectedError(t *testing.T) {
	failed, err := rerc("undefined-sym 42")
	if !failed || err != nil {
		t.Errorf("got failed=%v err=%v, want true nil", failed, err)
	}
}

// TestRercl_withFailure: a mismatched pair makes Rercl return a non-nil error
// (which propagates to a non-zero exit status) even though the per-pair details
// are also printed. The passing pair must not suppress that.
func TestRercl_withFailure(t *testing.T) {
	input := "(+ 1 2) 99\n(* 2 3) 6\n"
	if err := Rercl(scan.NewScanner("", input), false); err == nil {
		t.Error("Rercl with a failing pair: got nil, want non-nil error")
	}
}

// TestRercl_readError: missing expected value causes Rerc to return a non-EOF
// error, which Rercl propagates via its return err path.
func TestRercl_readError(t *testing.T) {
	if err := Rercl(scan.NewScanner("", "(+ 1 2)"), false); err == nil {
		t.Error("Rercl read error: expected non-nil error, got nil")
	}
}

// TestStdinReader: stdinReader must return a non-nil reader regardless of whether
// stdin is a terminal (covers the stdinReader wrapper itself).
func TestStdinReader(t *testing.T) {
	r, _ := stdinReader()
	if r == nil {
		t.Error("stdinReader: expected non-nil reader")
	}
}

// TestFileReader_nonTTY: a pipe is not a character device; fileReader must return
// interactive=false.
func TestFileReader_nonTTY(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal("os.Pipe:", err)
	}
	defer r.Close()
	defer w.Close()
	reader, interactive := fileReader(r)
	if interactive {
		t.Error("fileReader: expected non-interactive for pipe")
	}
	if reader == nil {
		t.Error("fileReader: expected non-nil reader")
	}
}

// TestFileReader_charDevice: /dev/null is a character device; fileReader must
// return interactive=true for it.
func TestFileReader_charDevice(t *testing.T) {
	f, err := os.Open("/dev/null")
	if err != nil {
		t.Skip("cannot open /dev/null:", err)
	}
	defer f.Close()
	_, interactive := fileReader(f)
	if !interactive {
		t.Error("fileReader: expected interactive=true for /dev/null (char device)")
	}
}
