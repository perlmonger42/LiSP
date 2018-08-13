/*
 * A minimal Scheme interpreter, as seen in lis.py and SICP
 * http://norvig.com/lispy.html
 * http://mitpress.mit.edu/sicp/full-text/sicp/book/node77.html
 *
 * Pieter Kelchtermans 2013
 * LICENSE: WTFPL 2.0
 */
package main

import (
	"fmt"
	"reflect"
	"strings"
	"unicode"

	"github.com/perlmonger42/LiSP/scan"
)

/*
 Eval / Apply
*/
var Tracing bool

func TopLevelEvaluate(e scmer) scmer {
	if isDefineForm(e) {
		return define(e.(array), &globalenv)
	}
	return eval(e, &globalenv)
}

func isDefineForm(form scmer) bool {
	if list, ok := form.(array); !ok {
		return false
	} else if len(list) < 3 {
		return false
	} else if sym, ok := list[0].(symbol); ok {
		return sym == "define"
	} else {
		return false
	}
}

func define(list array, r *env) (result scmer) {
	if Tracing {
		print_indent()
		fmt.Printf("=> Define %s\n", list)
		indent()
		defer func() {
			undent()
			print_indent()
			fmt.Printf("<= %s\n", result)
		}()
	}
	if len(list) != 3 {
		Fail("define requires at exactly 3 arguments: %s", list)
	}
	if sym, ok := list[1].(symbol); ok {
		if len(list) != 3 {
			Fail("define has trailing values: %s", list)
		}
		r.vars[sym] = eval(list[2], r)
		return array{symbol("#%undef"), symbol("define"), sym}
	}
	if args, ok := list[1].(array); ok {
		if sym, ok := args[0].(symbol); !ok {
			Fail("define has illegal structure")
		} else {
			val := proc{args[1:], list[2], r}
			r.vars[sym] = val
			return array{symbol("#%undef"), symbol("define"), sym}
		}
	}
	Fail("define: 1st arg must be symbol or func declaration: %s", list)
	panic("Fail didn't panic")
}

var depth int

func indent() { depth += 1 }
func undent() { depth -= 1 }
func print_indent() {
	for i := 0; i < depth; i += 1 {
		fmt.Print("  ")
	}
}

func eval(expression scmer, en *env) (value scmer) {
	if Tracing {
		print_indent()
		fmt.Printf("=> Evaluate %s\n", expression)
		indent()
		defer func() {
			undent()
			print_indent()
			fmt.Printf("<= %s\n", value)
		}()
	}
	switch e := expression.(type) {
	case boolean:
		value = e
	case char:
		value = e
	case flonum:
		value = e
	case str:
		value = e
	case symbol:
		value = en.Lookup(e)
	case array:
		switch car, _ := e[0].(symbol); car {
		case "quote":
			value = e[1]
		case "if":
			if eval(e[1], en).(boolean) {
				value = eval(e[2], en)
			} else {
				value = eval(e[3], en)
			}
		case "set!":
			v := e[1].(symbol)
			en.Find(v).vars[v] = eval(e[2], en)
			value = symbol("#%set!")
		case "define":
			value = define(e, en)
		case "lambda":
			value = proc{e[1], e[2], en}
		case "apply":
			functor := eval(e[1], en)
			value = apply(functor, eval(e[2], en).(array))
		case "begin":
			for _, i := range e[1:] {
				value = eval(i, en)
			}
		default:
			functor := eval(e[0], en)
			value = apply(functor, eval_all(e[1:], en))
		}
	default:
		Fail("eval: unknown expression type: %T %e", expression, expression)
	}
	return
}

func eval_all(list []scmer, r *env) []scmer {
	values := make(array, len(list))
	for i, x := range list {
		values[i] = eval(x, r)
	}
	return values
}

func apply(procedure scmer, args array) (value scmer) {
	//if Tracing {
	//	print_indent()
	//	fmt.Printf("apply %s to %s\n", procedure, args)
	//	indent()
	//	defer func() {
	//		undent()
	//		print_indent()
	//		fmt.Printf("return value from %s is %s\n", procedure, value)
	//	}()
	//}
	switch p := procedure.(type) {
	case primitive:
		value = p.f(args...)
	case proc:
		en := &env{make(vars), p.en}
		switch params := p.params.(type) {
		case array:
			for i, param := range params {
				//if Tracing {
				//	print_indent()
				//	fmt.Printf("set %s to %s\n", param, args[i])
				//}
				en.vars[param.(symbol)] = args[i]
			}
		default:
			//if Tracing {
			//	print_indent()
			//	fmt.Printf("set %s to %s\n", params, args)
			//}
			en.vars[params.(symbol)] = args
		}
		value = eval(p.body, en)
	default:
		Fail("apply: invalid functor: %T %s", procedure, procedure)
	}
	return
}

type primitive struct {
	name symbol
	f    func(...scmer) scmer
}

func (x primitive) String() string {
	return fmt.Sprintf("#<primitive:%s>", x.name)
}

type proc struct {
	params, body scmer
	en           *env
}

func (x proc) String() string {
	return fmt.Sprintf("(lambda %s %s)", x.params, x.body)
}

/*
 Environments
*/

type vars map[symbol]scmer
type env struct {
	vars
	outer *env
}

func (e *env) Find(s symbol) *env {
	if _, ok := e.vars[s]; ok {
		return e
	} else if e.outer == nil {
		return nil
	} else {
		return e.outer.Find(s)
	}
}

func (e *env) Lookup(s symbol) scmer {
	r := e.Find(s)
	if r == nil {
		Fail("undefined symbol: %s", s)
	}
	return r.vars[s]
}

/*
 Primitives
*/

var globalenv env

func listPrimitive() scmer {
	scanner := scan.NewScanner("<str>", strings.NewReader("(lambda z z)"))
	expr, _ := read(scanner)
	return eval(expr, &globalenv)
}

func init() {
	std := map[string]func(...scmer) scmer{
		"+": func(a ...scmer) scmer {
			v := a[0].(flonum)
			for _, i := range a[1:] {
				v += i.(flonum)
			}
			return v
		},
		"-": func(a ...scmer) scmer {
			v := a[0].(flonum)
			for _, i := range a[1:] {
				v -= i.(flonum)
			}
			return v
		},
		"*": func(a ...scmer) scmer {
			v := a[0].(flonum)
			for _, i := range a[1:] {
				v *= i.(flonum)
			}
			return v
		},
		"/": func(a ...scmer) scmer {
			v := a[0].(flonum)
			for _, i := range a[1:] {
				v /= i.(flonum)
			}
			return v
		},
		"equal?": func(a ...scmer) scmer {
			return boolean(reflect.DeepEqual(a[0], a[1]))
		},
		"!=": func(a ...scmer) scmer {
			return boolean(!reflect.DeepEqual(a[0], a[1]))
		},
		"<": func(a ...scmer) scmer {
			return boolean(a[0].(flonum) < a[1].(flonum))
		},
		"<=": func(a ...scmer) scmer {
			return boolean(a[0].(flonum) <= a[1].(flonum))
		},
		">": func(a ...scmer) scmer {
			return boolean(a[0].(flonum) > a[1].(flonum))
		},
		">=": func(a ...scmer) scmer {
			return boolean(a[0].(flonum) >= a[1].(flonum))
		},
		"cons": func(a ...scmer) scmer {
			switch car := a[0]; cdr := a[1].(type) {
			case array:
				return append(array{car}, cdr...)
			default:
				return array{car, cdr}
			}
		},
		"car": func(a ...scmer) scmer {
			return a[0].(array)[0]
		},
		"cdr": func(a ...scmer) scmer {
			return a[0].(array)[1:]
		},
	}
	builtins := vars{}
	for k, v := range std {
		sym := symbol(k)
		builtins[sym] = primitive{sym, v}
	}

	builtins[symbol("list")] = listPrimitive()
	builtins[symbol("null")] = array{}

	globalenv = env{builtins, nil}
}

/*
 Parsing
*/

//symbols, numbers, expressions, procedures, lists, ... all implement this interface, which enables passing them along in the interpreter
type scmer interface {
	String() string
}

type array []scmer  // lists are represented as arrays of scmers,
type symbol string  // ...symbols by strings,
type flonum float64 // ...numbers by float64,
type str string     // ...str by string,
type char rune      // ...char by rune
type boolean bool   // ...boolean by bool

func (a array) String() string {
	l := make([]string, len(a))
	for i, x := range a {
		l[i] = x.String()
	}
	return "(" + strings.Join(l, " ") + ")"
}
func (x symbol) String() string { return string(x) }
func (x flonum) String() string { return fmt.Sprintf("%g", x) }
func (x boolean) String() string {
	if x {
		return "#t"
	}
	return "#f"
}
func (x str) String() string { return string(x) }

func (x char) String() string {
	switch c := rune(x); c {
	case '\000':
		return "#\\nul"
	case '\010':
		return "#\\backspace"
	case '\011':
		return "#\\tab"
	case '\012':
		return "#\\newline"
	case '\013':
		return "#\\vtab"
	case '\014':
		return "#\\page"
	case '\015':
		return "#\\return"
	case '\040':
		return "#\\space"
	case '\177':
		return "#\\rubout"
	default:
		if c < 0 || c > unicode.MaxRune {
			return fmt.Sprintf("#\\nul#|0x%x|#", c)
		} else if !unicode.IsSpace(rune(c)) && unicode.IsGraphic(rune(c)) {
			return fmt.Sprintf("#\\%c", c)
		} else if c < 0x10000 {
			return fmt.Sprintf("#\\u%04x", c)
		} else {
			return fmt.Sprintf("#\\U%06x", c)
		}
	}
}

/*
 Interactivity
*/

// func String(v scmer) string {
// 	switch v := v.(type) {
// 	case []scmer:
// 		l := make([]string, len(v))
// 		for i, x := range v {
// 			l[i] = String(x)
// 		}
// 		return "(" + strings.Join(l, " ") + ")"
// 	default:
// 		return fmt.Sprint(v)
// 	}
// }
