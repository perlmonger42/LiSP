// Based on https://github.com/robpike/ivy/blob/master/ivy.go
package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"os"
	"reflect"
	"strings"

	"github.com/perlmonger42/LiSP/scan"
)

var (
	execute = flag.Bool("e", false, "execute arguments as a single expression")
	testing = flag.Bool("test", false, "execute Read Eval Read Compare Loop")
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

	if *execute {
		stringReader := strings.NewReader(strings.Join(flag.Args(), " "))
		scanner := scan.NewScanner("<args>", stringReader)
		Run(scanner, false)
		return
	}

	if flag.NArg() > 0 {
		for i := 0; i < flag.NArg(); i++ {
			name := flag.Arg(i)
			var reader io.ByteReader
			interactive := false
			if name == "-" {
				interactive = true
				name = "<stdin>"
				reader = scan.NewConsoleReader()
			} else {
				if f, err := os.Open(name); err != nil {
					fmt.Fprintf(os.Stderr, "LiSP: %s\n", err)
					os.Exit(1)
				} else {
					reader = bufio.NewReader(f)
				}
			}
			scanner := scan.NewScanner(name, reader)
			if ok := Run(scanner, interactive); !ok {
				break
			}
		}
		return
	}

	scanner := scan.NewScanner("<stdin>", scan.NewConsoleReader())
	Run(scanner, true)
}

func Run(scanner *scan.Scanner, interactive bool) bool {
	var err error
	if *testing {
		err = Rercl(scanner, interactive)
	} else {
		err = Repl(scanner, interactive)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
	}
	return err == nil
}

// runArgs executes the text of the command-line arguments as a LiSP program.
func runArgs() {
	stringReader := strings.NewReader(strings.Join(flag.Args(), " "))
	scanner := scan.NewScanner("<args>", stringReader)
	Repl(scanner, false)
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
		fmt.Println(String(value))
		return nil
	}
}

// ReadEval does (eval (read)).
// If err == io.EOF,
//   then the end of input was reached, and datum and value are nil.
// If datum == nil,
//   then err is the read error; else datum is what was read and value is nil.
// If value == nil,
//   then err is the evaluation error; else datum is what was read and value
//   is the result of evaluating datum.
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
	return err
}

// Rerc does a SINGLE Read, Eval, Read and Compare.
// Rerc reads a datum, evaluates it, reads another datum, and compares the
// evaluated first datum with the unevaluated second datum. It is an error if
// they are not equal.
//
// Special values for the second datum:
//   ***  this means that an error is expected
//   ---  this means that no error is expected but the value is unimportant
//
func Rerc(scanner *scan.Scanner) (failed bool, err error) {
	var datum, value, expect scmer
	var err2 error

	fail := func(why string) {
		failed = true
		fmt.Printf("%s\n   datum: %s\n   value: %s\n  expect: %s\n   error: %s\n",
			why, datum, value, expect, err)
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
