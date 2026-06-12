/*
 * A Scheme interpreter, based on Pieter Kelchtermans's `scm.go`,
 * which is itself based on `lis.py` and SICP.
 *
 * - [scm.go, by Pieter Kelchtermans](https://pkelchte.wordpress.com/2013/12/31/scm-go/)
 *   (2013, LICENSE WTFPL 2.0)
 *
 * - [lis.py, by Peter Norvig](http://norvig.com/lispy.html)
 *
 * - [SICP, the book](https://mitp-content-server.mit.edu/books/content/sectbyfn/books_pres_0/6515/sicp.zip/full-text/book/book-Z-H-26.html#%_sec_4.1.1)
 *
 * The particular section of SICP that Pieter Kelchtermans says inspired him is
 * "Structure and Interpretation of Computer Programs" > Metalinguistic
 * Abstraction > The Metacircular Evaluator > The Core of the Evaluator.
 */
package main

import (
	"fmt"
	"reflect"
	"strings"
	"unicode"

	"LiSP/internal/scan"
)

var depth int

func current_indentation() int { return depth }
func set_indent_level(n int)   { depth = n }
func indent()                  { depth += 1 }
func undent()                  { depth -= 1 }
func print_indent() {
	for i := 0; i < depth; i += 1 {
		fmt.Print("  ")
	}
}

type continuation interface {
	String() string
	resume(scmer) (c continuation, v scmer)
}

type bottomCont struct {
}

func (bottomCont) resume(e scmer) (continuation, scmer) {
	return nil, e
}
func (bottomCont) String() string { return "<bottomCont>" }

/*
Eval / Apply
*/
var Tracing bool

////func TopLevelEvaluate(e scmer) scmer {
////	//// if isDefineForm(e) {
////	//// 	return define(e.(array), &globalenv)
////	//// }
////	_, v := evaluate(e, &globalenv, bottomCont{})
////	return v
////}

func TopLevelEvaluate(e scmer) scmer {
	return top_level_evaluator(e)
}

func top_level_evaluator(e scmer) scmer {
	// hop on the trampoline
	cont, value := evaluate(e, &globalenv, bottomCont{})
	// continue jumping on the trampoline
	for cont != nil {
		//print_indent()
		//fmt.Printf("%s => %s\n", value, cont)
		cont, value = cont.resume(value)
	}
	return value
}

type traceCont struct {
	indentation int
	k           continuation
}

func (traceCont) String() string { return "<traceCont>" }

func (c traceCont) resume(value scmer) (continuation, scmer) {
	set_indent_level(c.indentation)
	set_indent_level(c.indentation)
	print_indent()
	fmt.Printf("<= %s\n", value)
	return c.k, value
}

func evaluate(expression scmer, r *env, k continuation) (cont continuation, value scmer) {
	if Tracing {
		print_indent()
		fmt.Printf("=> Evaluate %s\n", expression)
		k = &traceCont{current_indentation(), k}
		indent()
	}
	if expression == nil {
		return k, nil
	}
	switch e := expression.(type) {
	case boolean, char, flonum, str:
		return k, e
	case symbol:
		return evaluateVariable(e, r, k)
	case array:
		if len(e) == 0 {
			Fail("missing functor in '()")
		}
		switch car, _ := e[0].(symbol); car {
		case "quote":
			return evaluateQuote(e, r, k)
		case "if":
			return evaluateIf(e, r, k)
		case "and":
			return evaluateAnd(e[1:], r, k)
		case "set!":
			return evaluateSetVariable(e[1], e[2], r, k)
		case "define":
			if len(e) < 3 {
				Fail("invalid (define ...) form: %s", e)
			} else if sym, val, ok := isVarDef(e); ok {
				return evaluate(val, r, varDefCont{sym, r, k})
			} else if sym, args, body, ok := isFunDef(e); ok {
				r.Define(sym, proc{args, body, r})
				return k, list(symbol("#%undef"), symbol("define"), sym)
			}
			Fail("invalid define form")
		case "lambda":
			return evaluateLambda(e[1], e[2:], r, k)
		case "apply":
			return evaluateApplication(e[1], e[2:], r, k)
		case "begin":
			return evaluateBegin(e[1:], r, k)
		case "let":
			return evaluateLet(e[1], e[2:], r, k)
		case "cond":
			return evaluateCond(e[1:], r, k)
		default:
			return evaluateApplication(e[0], e[1:], r, k)
		}
	default:
		Fail("evaluate: unknown expression type: %T %e", expression, expression)
	}
	return
}

func evaluateVariable(e scmer, r *env, k continuation) (continuation, scmer) {
	return lookup(e, r, k)
}

func evaluateSetVariable(sym, value scmer, r *env, k continuation) (continuation, scmer) {
	s, ok := sym.(symbol)
	if !ok {
		Fail("set!: first argument must be a symbol")
	}
	return evaluate(value, r, varSetCont{s, r, k})
}

type varSetCont struct {
	sym symbol
	r   *env
	k   continuation
}

func (varSetCont) String() string { return "varSetCont" }

func (c varSetCont) resume(value scmer) (continuation, scmer) {
	c.r.Set(c.sym, value)
	return c.k, list(symbol("#%undef"), symbol("set!"), c.sym)
}

func evaluateQuote(e array, r *env, k continuation) (continuation, scmer) {
	expect("quote", e, 2, 2)
	return k, e[1]
}

func evaluateIf(e array, r *env, k continuation) (continuation, scmer) {
	expect("if", e, 4, 4)
	ec, et, ef := e[1], e[2], e[3]
	return evaluate(ec, r, ifCont{et, ef, r, k})
}

type ifCont struct {
	et, ef scmer
	r      *env
	k      continuation
}

func (c ifCont) resume(ec scmer) (continuation, scmer) {
	branch := c.et
	if flag, ok := ec.(boolean); ok && flag == false {
		branch = c.ef
	}
	return evaluate(branch, c.r, c.k)
}
func (ifCont) String() string { return "<ifCont>" }

func evaluateCond(clauses array, r *env, k continuation) (continuation, scmer) {
	if len(clauses) == 0 {
		return k, list(symbol("#%undef"), symbol("cond"))
	} else if clause, ok := clauses[0].(array); !ok || len(clause) < 2 {
		Fail("cond: invalid syntax in clause: %s", clauses[0])
		panic("Fail didn't panic")
	} else {
		return evaluate(clause[0], r, condCont{clause[1:], clauses, r, k})
	}
}

type condCont struct {
	consequent array
	clauses    array
	r          *env
	k          continuation
}

func (c condCont) resume(test scmer) (continuation, scmer) {
	if flag, ok := test.(boolean); ok && bool(!flag) {
		// test == #f
		return evaluateCond(c.clauses[1:], c.r, c.k)
	} else {
		return evaluateBegin(c.consequent, c.r, c.k)
	}
}
func (condCont) String() string { return "<condCont>" }

func evaluateAnd(terms array, r *env, k continuation) (continuation, scmer) {
	if len(terms) == 0 {
		return k, boolean(true)
	} else if len(terms) == 1 {
		return evaluate(terms[0], r, k)
	} else {
		return evaluate(terms[0], r, andCont{terms, r, k})
	}
}

type andCont struct {
	terms array
	r     *env
	k     continuation
}

func (c andCont) resume(test scmer) (continuation, scmer) {
	if flag, ok := test.(boolean); ok && bool(!flag) {
		// test == #f
		return c.k, test
	} else {
		return evaluateAnd(c.terms[1:], c.r, c.k)
	}
}
func (andCont) String() string { return "<andCont>" }

func evaluateBegin(body array, r *env, k continuation) (continuation, scmer) {
	switch len(body) {
	case 0:
		return k, list(symbol("#%undef"), symbol("begin"))
	case 1:
		return evaluate(body[0], r, k)
	default:
		return evaluate(body[0], r, beginCont{body, r, k})
	}
}

type beginCont struct {
	body array
	r    *env
	k    continuation
}

func (c beginCont) resume(e scmer) (continuation, scmer) {
	return evaluateBegin(c.body[1:], c.r, c.k)
}
func (beginCont) String() string { return "<beginCont>" }

func evaluateLambda(formals scmer, body array, r *env, k continuation) (continuation, scmer) {
	return k, proc{formals, body, r}
}

func evaluateLet(bindings scmer, body array, r *env, k continuation) (continuation, scmer) {
	b, ok := bindings.(array)
	if !ok {
		Fail("invalid let form: bindings must be ((var1 init1)...)")
	}
	vars, inits := array{}, array{}
	for _, varval := range b {
		pair, ok := varval.(array)
		if !ok || len(pair) != 2 {
			Fail("invalid let form: bindings must be ((var1 init1)...)")
		}
		vars, inits = append(vars, pair[0]), append(inits, pair[1])
	}
	// transform (let ((var1 init1)...) body...)
	//   into
	// ((lambda (var1...) body...) init1...)
	return evalfunCont{inits, r, k}, proc{vars, body, r}
}

func evaluateApplication(functor scmer, args array, r *env, k continuation) (continuation, scmer) {
	return evaluate(functor, r, evalfunCont{args, r, k})
}

type evalfunCont struct {
	args array
	r    *env
	k    continuation
}

func (c evalfunCont) resume(functor scmer) (continuation, scmer) {
	return evaluateArguments(c.args, c.r, applyCont{functor, c.r, c.k})
}
func (evalfunCont) String() string { return "<evalfunCont>" }

func evaluateArguments(args array, r *env, k continuation) (continuation, scmer) {
	if len(args) == 0 {
		return k, array([]scmer{})
	}
	return evaluate(args[0], r, argumentCont{args, r, k})
}

type argumentCont struct {
	args array
	r    *env
	k    continuation
}

func (c argumentCont) resume(arg scmer) (continuation, scmer) {
	return evaluateArguments(c.args[1:], c.r, gatherCont{arg, c.k})
}
func (argumentCont) String() string { return "<argumentCont>" }

type gatherCont struct {
	arg scmer
	k   continuation
}

func (c gatherCont) resume(args scmer) (continuation, scmer) {
	return c.k, cons(c.arg, args.(array))
}
func (gatherCont) String() string { return "<gatherCont>" }

type applyCont struct {
	f scmer
	r *env
	k continuation
}

func (c applyCont) resume(args scmer) (continuation, scmer) {
	return invoke(c.f, args.(array), c.r, c.k)
}
func (applyCont) String() string { return "<applyCont>" }

func invoke(functor scmer, args array, r *env, k continuation) (continuation, scmer) {
	switch f := functor.(type) {
	case primitive:
		return f.f(args, r, k)
	case proc:
		switch a := f.params.(type) {
		case array:
			env := f.en.Extend(a, args)
			return evaluateBegin(f.body, env, k)
		case symbol:
			env := f.en.Extend1(a, args)
			return evaluateBegin(f.body, env, k)
		}
	}
	if f, ok := functor.(continuation); ok {
		if len(args) != 1 {
			Fail("continuations expect one argument")
		}
		return f, args[0]
	}
	Fail("invalid functor: %T %s", functor, functor)
	panic("Fail didn't panic")
}

func lookup(e scmer, r *env, k continuation) (continuation, scmer) {
	s := e.(symbol)
	return k, r.Lookup(s)
	// table := r.Find(s)
	// if table == nil {
	// 	Fail("undefined symbol: %s", s)
	// 	panic("Fail didn't panic")
	// }
	// return k, table.vars[s]
}

func isVarDef(form array) (name symbol, val scmer, ok bool) {
	// assert: form === (define V E...)
	if varname, ok := form[1].(symbol); ok && len(form) == 3 {
		return varname, form[2], true
	}
	return
}

type varDefCont struct {
	sym symbol
	r   *env
	k   continuation
}

func (varDefCont) String() string { return "varDefCont" }

func (c varDefCont) resume(value scmer) (continuation, scmer) {
	c.r.Define(c.sym, value)
	return c.k, list(symbol("#%undef"), symbol("define"), c.sym)
}

func isFunDef(form array) (name symbol, args array, body array, ok bool) {
	// assert: form === (define T E...)
	if template, ok := form[1].(array); !ok || len(template) < 1 {
	} else if funcname, ok := template[0].(symbol); ok {
		return funcname, template[1:], form[2:], true
	}
	return
}

type primitive struct {
	name symbol
	f    func(actuals array, r *env, k continuation) (continuation, scmer)
}

func (x primitive) String() string {
	return fmt.Sprintf("#<primitive:%s>", x.name)
}

type proc struct {
	params scmer
	body   array
	en     *env
}

func (x proc) String() string {
	return cons(symbol("lambda"), cons(x.params, x.body)).String()
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

func (r *env) Define(variable symbol, value scmer) {
	if _, ok := r.vars[variable]; ok {
		Fail("symbol %s already defined in this context", variable)
	}
	r.vars[variable] = value
}

func (orig *env) Extend1(variable symbol, value scmer) (e *env) {
	e = &env{make(vars), orig}
	e.vars[variable] = value
	return
}

func (orig *env) Extend(variables, values array) (e *env) {
	e = &env{make(vars), orig}
	if len(variables) != len(values) {
		Fail("arity mismatch")
	}
	for i, val := range values {
		e.vars[variables[i].(symbol)] = val
	}
	return
}

func (r *env) Set(variable symbol, value scmer) {
	if t := r.Find(variable); t == nil {
		Fail("set: symbol '%s' is not defined", variable)
	} else {
		t.vars[variable] = value
	}
}

/*
 Primitives
*/

var globalenv env

func listPrimitive() scmer {
	scanner := scan.NewScanner("<str>", "(lambda z z)")
	expr, _ := read(scanner)
	return top_level_evaluator(expr)
}

func call_with_current_continuation(a array, r *env, k continuation) (continuation, scmer) {
	if len(a) != 1 {
		Fail("call-with-current-continuation: incorrect arity")
	}
	return invoke(a[0], list(k), r, k)
}

func expect(fnName string, a array, minArgs, maxArgs int) {
	if minArgs == maxArgs && len(a) != minArgs {
		Fail("%s: incorrect arity; exactly %d arguments expected", fnName, minArgs)
	} else if len(a) < minArgs {
		Fail("%s: incorrect arity; at least %d arguments expected", fnName, minArgs)
	} else if maxArgs > 0 && len(a) > maxArgs {
		Fail("%s: incorrect arity; at most %d arguments expected", fnName, maxArgs)
	}
}

func init() {
	std := map[string]func(actuals array, r *env, k continuation) (continuation, scmer){

		"call/cc":                        call_with_current_continuation,
		"call-with-current-continuation": call_with_current_continuation,
		"write": func(a array, r *env, k continuation) (continuation, scmer) {
			expect("write", a, 1, 1)
			fmt.Print(a[0].String())
			return k, list(symbol("#%undef"), symbol("write"))
		},
		"newline": func(a array, r *env, k continuation) (continuation, scmer) {
			expect("newline", a, 0, 0)
			fmt.Println()
			return k, list(symbol("#%undef"), symbol("newline"))
		},
		"+": func(a array, r *env, k continuation) (continuation, scmer) {
			v := a[0].(flonum)
			for _, i := range a[1:] {
				v += i.(flonum)
			}
			return k, v
		},
		"-": func(a array, r *env, k continuation) (continuation, scmer) {
			v := a[0].(flonum)
			for _, i := range a[1:] {
				v -= i.(flonum)
			}
			return k, v
		},
		"*": func(a array, r *env, k continuation) (continuation, scmer) {
			v := a[0].(flonum)
			for _, i := range a[1:] {
				v *= i.(flonum)
			}
			return k, v
		},
		"/": func(a array, r *env, k continuation) (continuation, scmer) {
			v := a[0].(flonum)
			for _, i := range a[1:] {
				v /= i.(flonum)
			}
			return k, v
		},
		"equal?": func(a array, r *env, k continuation) (continuation, scmer) {
			return k, boolean(reflect.DeepEqual(a[0], a[1]))
		},
		"=": func(a array, r *env, k continuation) (continuation, scmer) {
			return k, boolean(reflect.DeepEqual(a[0], a[1]))
		},
		"!=": func(a array, r *env, k continuation) (continuation, scmer) {
			return k, boolean(!reflect.DeepEqual(a[0], a[1]))
		},
		"<": func(a array, r *env, k continuation) (continuation, scmer) {
			return k, boolean(a[0].(flonum) < a[1].(flonum))
		},
		"<=": func(a array, r *env, k continuation) (continuation, scmer) {
			return k, boolean(a[0].(flonum) <= a[1].(flonum))
		},
		">": func(a array, r *env, k continuation) (continuation, scmer) {
			return k, boolean(a[0].(flonum) > a[1].(flonum))
		},
		">=": func(a array, r *env, k continuation) (continuation, scmer) {
			return k, boolean(a[0].(flonum) >= a[1].(flonum))
		},
		"cons": func(a array, r *env, k continuation) (continuation, scmer) {
			switch car := a[0]; cdr := a[1].(type) {
			case array:
				return k, cons(car, cdr)
			default:
				return k, array{car, cdr}
			}
		},
		"car": func(a array, r *env, k continuation) (continuation, scmer) {
			return k, a[0].(array)[0]
		},
		"cdr": func(a array, r *env, k continuation) (continuation, scmer) {
			return k, a[0].(array)[1:]
		},
		"length": func(a array, r *env, k continuation) (continuation, scmer) {
			return k, fixnum(len(a[0].(array)))
		},
		"not": func(a array, r *env, k continuation) (continuation, scmer) {
			if flag, ok := a[0].(boolean); ok && flag == false {
				return k, boolean(true)
			}
			return k, boolean(false)
		},
		"and": evaluateAnd,
		"null?": func(a array, r *env, k continuation) (continuation, scmer) {
			if list, ok := a[0].(array); ok && len(list) == 0 {
				return k, boolean(true)
			}
			return k, boolean(false)
		},
		"pair?": func(a array, r *env, k continuation) (continuation, scmer) {
			if list, ok := a[0].(array); ok && len(list) > 0 {
				return k, boolean(true)
			}
			return k, boolean(false)
		},
		"error": func(a array, r *env, k continuation) (continuation, scmer) {
			if len(a) == 0 {
				Fail("error")
			} else if len(a) == 1 {
				Fail("error: %s", a[0])
			} else {
				Fail("error: %s", a)
			}
			panic("Fail didn't panic")
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

// symbols, numbers, expressions, procedures, lists, ... all implement this interface, which enables passing them along in the interpreter
type scmer interface {
	String() string
}

type array []scmer  // lists are represented as arrays of scmers,
type symbol string  // ...symbols by strings,
type fixnum int64   // ...integers by int64,
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

func cons(e scmer, a array) array {
	if a == nil {
		return array{e}
	}
	return append(array{e}, a...)
}

func list(args ...scmer) array {
	return array(args)
}

func (x symbol) String() string {
	s := string(x)
	for i := 0; i < len(s); i++ {
		if symbolNeedsBarQuoting(s[i]) {
			return "|" + s + "|"
		}
	}
	return s
}

// symbolNeedsBarQuoting mirrors scan.isDelimByte: any byte that the scanner
// treats as a delimiter must be bar-quoted when printing a symbol.
func symbolNeedsBarQuoting(b byte) bool {
	switch b {
	case '(', ')', '[', ']', '{', '}', '"', ',', '\'', '`', ';', '#', '|', '\\',
		' ', '\t', '\n', '\r', '\f', '\v':
		return true
	}
	return false
}
func (x fixnum) String() string { return fmt.Sprintf("%d", x) }
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
		} else {
			return fmt.Sprintf("#\\x%x", c)
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
