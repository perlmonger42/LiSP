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
	"log"
	"reflect"
	"strings"

	"github.com/perlmonger42/LiSP/scan"
)

/*
 Eval / Apply
*/
var Tracing bool

func TopLevelEvaluate(e scmer) scmer {
	//// if isDefineForm(e) {
	//// 	return Define(e, &globalEnv)
	//// }
	return eval(e, &globalenv)
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
		fmt.Printf("=> Evaluate %s\n", String(expression))
		indent()
		defer func() {
			undent()
			print_indent()
			fmt.Printf("<= %s\n", String(value))
		}()
	}
	switch e := expression.(type) {
	case number:
		value = e
	case symbol:
		value = en.Find(e).vars[e]
	case []scmer:
		switch car, _ := e[0].(symbol); car {
		case "quote":
			value = e[1]
		case "if":
			if eval(e[1], en).(bool) {
				value = eval(e[2], en)
			} else {
				value = eval(e[3], en)
			}
		case "set!":
			v := e[1].(symbol)
			en.Find(v).vars[v] = eval(e[2], en)
			value = "ok"
		case "define":
			en.vars[e[1].(symbol)] = eval(e[2], en)
			value = "ok"
		case "lambda":
			value = proc{e[1], e[2], en}
		case "begin":
			for _, i := range e[1:] {
				value = eval(i, en)
			}
		default:
			operands := e[1:]
			values := make([]scmer, len(operands))
			for i, x := range operands {
				values[i] = eval(x, en)
			}
			value = apply(eval(e[0], en), values)
		}
	default:
		log.Println("Unknown expression type - EVAL", e)
	}
	return
}

func apply(procedure scmer, args []scmer) (value scmer) {
	switch p := procedure.(type) {
	case func(...scmer) scmer:
		value = p(args...)
	case proc:
		en := &env{make(vars), p.en}
		switch params := p.params.(type) {
		case []scmer:
			for i, param := range params {
				en.vars[param.(symbol)] = args[i]
			}
		default:
			en.vars[params.(symbol)] = args
		}
		value = eval(p.body, en)
	default:
		log.Println("Unknown procedure type - APPLY", p)
	}
	return
}

type proc struct {
	params, body scmer
	en           *env
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
	} else {
		return e.outer.Find(s)
	}
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
	globalenv = env{
		vars{ //aka an incomplete set of compiled-in functions
			"+": func(a ...scmer) scmer {
				v := a[0].(number)
				for _, i := range a[1:] {
					v += i.(number)
				}
				return v
			},
			"-": func(a ...scmer) scmer {
				v := a[0].(number)
				for _, i := range a[1:] {
					v -= i.(number)
				}
				return v
			},
			"*": func(a ...scmer) scmer {
				v := a[0].(number)
				for _, i := range a[1:] {
					v *= i.(number)
				}
				return v
			},
			"/": func(a ...scmer) scmer {
				v := a[0].(number)
				for _, i := range a[1:] {
					v /= i.(number)
				}
				return v
			},
			"<=": func(a ...scmer) scmer {
				return a[0].(number) <= a[1].(number)
			},
			"equal?": func(a ...scmer) scmer {
				return reflect.DeepEqual(a[0], a[1])
			},
			"cons": func(a ...scmer) scmer {
				switch car := a[0]; cdr := a[1].(type) {
				case []scmer:
					return append([]scmer{car}, cdr...)
				default:
					return []scmer{car, cdr}
				}
			},
			"car": func(a ...scmer) scmer {
				return a[0].([]scmer)[0]
			},
			"cdr": func(a ...scmer) scmer {
				return a[0].([]scmer)[1:]
			},
			"list": listPrimitive(),
		},
		nil}
}

/*
 Parsing
*/

//symbols, numbers, expressions, procedures, lists, ... all implement this interface, which enables passing them along in the interpreter
type scmer interface{}

type symbol string  //symbols are represented by strings
type number float64 //numbers by float64

/*
 Interactivity
*/

func String(v scmer) string {
	switch v := v.(type) {
	case []scmer:
		l := make([]string, len(v))
		for i, x := range v {
			l[i] = String(x)
		}
		return "(" + strings.Join(l, " ") + ")"
	default:
		return fmt.Sprint(v)
	}
}
