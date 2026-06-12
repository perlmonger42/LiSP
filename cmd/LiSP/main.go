// Based on https://github.com/robpike/ivy/blob/master/ivy.go
package main

import (
	"LiSP/internal/scan"
	"bufio"
	"flag"
	"fmt"
	"io"
	"os"
	"reflect"
	"strings"

	"github.com/bobappleyard/readline"
)

var (
	execute  = flag.Bool("e", false, "execute arguments as a single expression")
	rercMode = flag.Bool("test", false, "execute Read Eval Read Compare Loop")
	// format  = flag.String("format", "", "use `fmt` as format for printing numbers; empty sets default format")
	// gformat = flag.Bool("g", false, `shorthand for -format="%.12g"`)
	// maxbits   = flag.Uint("maxbits", 1e9, "maximum size of an integer, in bits; 0 means no limit")
	// maxdigits = flag.Uint("maxdigits", 1e4, "above this many `digits`, integers print as floating point; 0 disables")
	prompt  = flag.String("prompt", "> ", "command `prompt`")
	prompt2 = flag.String("prompt2", "? ", "continued command `prompt`")
	// debugFlag = flag.String("debug", "", "comma-separated `names` of debug settings to enable")
)

func init() {
	flag.BoolVar(&Tracing, "trace", false, "print exprs before and after eval")
}

func usage() {
	fmt.Fprintf(os.Stderr, "usage: LiSP [options] [file ...]\n")
	fmt.Fprintf(os.Stderr, "Flags:\n")
	flag.PrintDefaults()
	os.Exit(2)
}

func main() {
	flag.Usage = usage
	flag.Parse()

	var ok bool
	if *execute {
		ok = runExprsFromCommandline()
	} else if flag.NArg() > 0 {
		ok = runFilesFromCommandline()
	} else {
		reader, interactive := stdinReader()
		scanner := scan.NewStreamScanner("<stdin>", reader)
		ok = Run(scanner, interactive)
	}
	if !ok {
		os.Exit(1)
	}
}

// runExprsFromCommandline executes the text of the command-line arguments as a LiSP program.
func runExprsFromCommandline() bool {
	scanner := scan.NewScanner("<args>", strings.Join(flag.Args(), " "))
	return Run(scanner, false)
}

// runFilesFromCommandline executes the contents of the files named by command-line arguments
func runFilesFromCommandline() bool {
	for i := 0; i < flag.NArg(); i++ {
		name := flag.Arg(i)
		var reader io.ByteReader
		interactive := false
		if name == "-" {
			name = "<stdin>"
			reader, interactive = stdinReader()
		} else if f, err := os.Open(name); err != nil {
			fmt.Fprintf(os.Stderr, "LiSP: %s\n", err)
			return false
		} else {
			reader = bufio.NewReader(f)
		}
		scanner := scan.NewStreamScanner(name, reader)
		if ok := Run(scanner, interactive); !ok {
			return false
		}
	}
	return true
}

// stdinReader returns a reader for os.Stdin and whether it is interactive.
func stdinReader() (io.ByteReader, bool) {
	return fileReader(os.Stdin)
}

// fileReader returns a reader for f and whether f is a character device
// (i.e. a terminal). When interactive, it uses readline for line editing and
// history; otherwise it reads f as a plain stream.
func fileReader(f *os.File) (io.ByteReader, bool) {
	fi, err := f.Stat()
	if err == nil && (fi.Mode()&os.ModeCharDevice) != 0 {
		return scan.NewConsoleReader(), true
	}
	return bufio.NewReader(f), false
}

func Run(scanner *scan.Scanner, interactive bool) bool {
	readline.LoadHistory("./LiSP.history")
	defer func() {
		readline.SaveHistory("./LiSP.history")
	}()
	var err error
	if *rercMode {
		err = Rercl(scanner, interactive)
	} else {
		err = Repl(scanner, interactive)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
	}
	return err == nil
}

var Failure error

func Fail(format string, a ...interface{}) {
	Failure = fmt.Errorf(format, a...)
	panic(Failure)
}

// Repl is a Read, Eval, Print Loop.
func Repl(scanner *scan.Scanner, interactive bool) (err error) {
	for {
		if err = Rep(scanner, interactive); err == io.EOF {
			break
		} else if err != nil && interactive {
			fmt.Printf("Error: %s\n", err)
		} else if err != nil {
			return err
		}
	}
	return nil
}

// Rep does a Read, Eval and Print.
func Rep(scanner *scan.Scanner, interactive bool) error {
	if _, value, err := ReadEval(scanner); err != nil {
		return err
	} else {
		fmt.Println(value)
		return nil
	}
}

// ReadEval does (eval (read)).
// If err == io.EOF,
//
//	then the end of input was reached, and datum and value are nil.
//
// If datum == nil,
//
//	then err is the read error; else datum is what was read and value is nil.
//
// If value == nil,
//
//	then err is the evaluation error; else datum is what was read and value
//	is the result of evaluating datum.
func ReadEval(scanner *scan.Scanner) (datum scmer, value scmer, err error) {
	defer func() {
		if Failure != nil {
			r := recover()
			if r == nil {
				err = Failure
			} else if e, ok := r.(error); ok && e == Failure {
				err = Failure
			} else {
				fmt.Printf("Error: %s\n", Failure)
				panic(r)
			}
		}
	}()

	Failure = nil
	if datum, err = read(scanner); err != nil {
		// Read error, so skip evaluation (includes err == io.EOF)
	} else if value = TopLevelEvaluate(datum); value == nil {
		err = fmt.Errorf("Evaluate failed (returned nil)")
	}
	return
}

var expectError, dontCare symbol

// Rercl is a Read, Eval, Read, Compare LOOP.
// Rercl reads a datum, evaluates, reads another datum, and compares the
// evaluated first datum with the unevaluated second datum. It is an error if
// they are not equal.
//
// Special values for the second datum:
//
//	***  this means that an error is expected
//	---  this means that no error is expected but the value is unimportant
func Rercl(scanner *scan.Scanner, interactive bool) error {
	expectError = symbol("***")
	dontCare = symbol("---")

	var err error = nil
	failed := false
	failures := 0

	for err == nil {
		failed, err = Rerc(scanner)
		if failed {
			failures += 1
		}
	}
	if err == io.EOF {
		return nil
	}
	return err
}

// Rerc does a SINGLE Read, Eval, Read and Compare.
// Rerc reads a datum, evaluates it, reads another datum, and compares the
// evaluated first datum with the unevaluated second datum. It is an error if
// they are not equal.
//
// Special values for the second datum:
//
//	***  this means that an error is expected
//	---  this means that no error is expected but the value is unimportant
func Rerc(scanner *scan.Scanner) (failed bool, err error) {
	var datum, value, expect scmer
	var err2 error

	fail := func(why string) {
		failed = true
		fmt.Printf("%s\n   input: %s\n  evaled: %s\n  expect: %s\n",
			why, datum, value, expect)
		if err != nil {
			fmt.Fprintf(os.Stderr, "   error: %v\n", err)
		}
	}

	if datum, value, err = ReadEval(scanner); err == io.EOF {
		// end of input; do nothing
	} else if datum == nil {
		// read error; do nothing more
	} else if expect, err2 = read(scanner); err2 != nil {
		err = fmt.Errorf("RERC: failed while reading expected value: %s", err2)
	} else if expect == dontCare {
		if err != nil {
			fail("unexpected error during evaluation")
		}
	} else if expect == expectError {
		if err == nil {
			fail("expected error, but none occurred")
		}
		err = nil
	} else if err != nil {
		fail("unexpected error")
	} else if !reflect.DeepEqual(value, expect) { // TODO: implement (equal? ...)
		fail("unexpected value")
	}
	return
}
