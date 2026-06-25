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
	"math"
	"reflect"
	"strings"
	"unicode"

	"LiSP/internal/scan"
)

var depth int

func currentIndentation() int { return depth }
func setIndentLevel(n int)    { depth = n }
func indent()                 { depth += 1 }
func undent()                 { depth -= 1 }
func printIndent() {
	for i := 0; i < depth; i += 1 {
		fmt.Print("  ")
	}
}

// continuation is the interface implemented by all continuations.
// To get the name of the continuation k, use reflect.TypeOf(k).Name().
type continuation interface {
	resume(scmer) (c continuation, v scmer)
}

type bottomCont struct {
}

func (bottomCont) resume(e scmer) (continuation, scmer) {
	return nil, e
}

/*
Eval / Apply
*/
var Tracing bool

// Condition is the structured error object raised by Fail/Failc. `code` is
// machine-readable metadata — used by the test harness now, and the seed of the
// R7RS condition object later — while `message` is the human-readable text the
// REPL prints. `irritants` holds R7RS-style irritant objects (unused for now).
// Condition satisfies the error interface, so it flows through the existing
// error-returning paths unchanged; the code rides along for callers that want it.
type Condition struct {
	code      symbol
	message   string
	irritants []scmer
}

func (c Condition) Error() string { return c.message }

// Fail raises a Condition with the generic code `error`.
func Fail(format string, a ...any) {
	Failc(symbol("error"), format, a...)
}

// Failc raises a Condition tagged with a specific machine-readable code.
// The code is metadata only; it is not part of the printed message.
func Failc(code symbol, format string, a ...any) {
	panic(Condition{code: code, message: fmt.Sprintf(format, a...)})
}

func topLevelEvaluate(e scmer) (result scmer, err error) {
	defer func() {
		if r := recover(); r != nil {
			// panic() was called
			if c, ok := r.(Condition); ok {
				err = c // from Fail()/Failc(); Condition is an error, code preserved.
			} else {
				fmt.Printf("topLevelEvaluate: %v (%T)\n", r, r)
				panic(r) // re-panic if it wasn't Fail()/Failc() that called panic
			}
		}
	}()

	return topLevelEvaluator(e), nil
}

func topLevelEvaluator(e scmer) scmer {
	// hop on the trampoline
	cont, value := evaluate(e, &globalenv, bottomCont{})
	// continue jumping on the trampoline
	for cont != nil {
		//printIndent()
		//fmt.Printf("%s => %s\n", value, cont)
		cont, value = cont.resume(value)
	}
	return value
}

type traceCont struct {
	indentation int
	k           continuation
}

func (c traceCont) resume(value scmer) (continuation, scmer) {
	setIndentLevel(c.indentation)
	printIndent()
	fmt.Printf("<= %s\n", value)
	return c.k, value
}

func evaluate(expression scmer, r *env, k continuation) (cont continuation, value scmer) {
	if Tracing {
		printIndent()
		fmt.Printf("=> Evaluate %s\n", expression)
		k = &traceCont{currentIndentation(), k}
		indent()
	}
	if expression == nil {
		return k, nil
	}
	switch e := expression.(type) {
	case boolean, char, flonum, str: // 4.1.2 "Literal Expressions"
		// > Numerical constants, string constants, character constants,
		// > and boolean constants evaluate “to themselves”; they need
		// > not be quoted.
		return k, e
	case symbol: // R5RS 4.1.1 "Variable references"
		return evaluateVariable(e, r, k)
	case array: // R5RS 4.1.3 "Procedure calls"
		return evaluateProcedureCallSyntax(e, r, k)
	default:
		Fail("evaluate: unknown expression type: %T %e", expression, expression)
	}
	return
}

func evaluateVariable(e scmer, r *env, k continuation) (continuation, scmer) {
	return lookup(e, r, k)
}

func evaluateProcedureCallSyntax(e array, r *env, k continuation) (continuation, scmer) {
	if len(e) == 0 {
		Fail("missing functor in expression: ()")
		panic("unreachable")
	}
	switch car, _ := e[0].(symbol); car {
	case "#%undef": // invented for this interpreter
		return k, e // (#%undef obj...) evaluates to itself -- the arguments are not evaluated
	case "quote": // R5RS 4.1.2 "Literal Expressions"
		return evaluateQuoteSyntax(e, r, k)
	case "lambda": // R5RS 4.1.4 "Procedures"
		return evaluateLambdaSyntax(e, r, k)
	case "if": // R5RS 4.1.5 "Conditionals"
		return evaluateIfSyntax(e, r, k)
	case "set!": // R5RS 4.1.6 "Assignments"
		return evaluateSetVariableSyntax(e, r, k)
	case "cond": // R5RS 4.2.1 "Conditionals"
		return evaluateCondSyntax(e, r, k)
	// case "case": // R5RS 4.2.1 "Conditionals" -- not yet implemented
	case "and": // R5RS 4.2.1 "Conditionals"
		return evaluateAndSyntax(e, r, k)
	case "or": // R5RS 4.2.1 "Conditionals"
		return evaluateOrSyntax(e, r, k)
	case "let": // R5RS 4.2.2 "Binding constructs"
		return evaluateLetSyntax(e, r, k)
	//case "let*": // R5RS 4.2.2 "Binding constructs" --not yet implemented
	//case "letrec": // R5RS 4.2.2 "Binding constructs" --not yet implemented
	case "begin": // R5RS 4.2.3 "Sequencing"
		return evaluateBeginSyntax(e[1:], r, k)
	//case "do": // R5RS 4.2.4 "Iteration" -- not yet implemented
	//case "delay": // R5RS 4.2.5 "Delayed evaluation" -- not yet implemented
	//case "quasiquote": // R5RS 4.2.6 "Quasiquotation" not yet implemented
	//case "unquote": // R5RS 4.2.6 "Quasiquotation" not yet implemented
	//case "unquote-splicing": // R5RS 4.2.6 "Quasiquotation" not yet implemented
	// 4.3 Macros -- not yet implemented
	case "define": // R5RS 5.2 Definitions
		return evaluateDefineSyntax(e, r, k)
	default: // R5RS 4.1.3 "Procedure calls"
		return evaluateNormalProcedureCall(e, r, k)
	}
}

func evaluateQuoteSyntax(form array, r *env, k continuation) (continuation, scmer) {
	expectArgs("quote", form[1:], 1, 1)
	return k, form[1]
}

func evaluateLambdaSyntax(form array, r *env, k continuation) (continuation, scmer) {
	// Precondition: form[0] is |lambda|
	// Match: (lambda <formals> <body1> <body2> ...)
	if len(form) < 3 {
		Fail("invalid (lambda ...) syntax: %s", form)
	}
	// TODO: validate the shape of formals
	formals, body := form[1], form[2:]
	return k, proc{formals, body, r}
}

func evaluateIfSyntax(form array, r *env, k continuation) (continuation, scmer) {
	// Precondition: form[0] is |if|
	// Match: (if <test> <consequent> <alternative>)
	// Match: (if <test> <consequent>)
	expectArgs("if", form[1:], 2, 3)
	var ec, et, ef scmer = form[1], form[2], undefIf
	if len(form) == 4 {
		ef = form[3]
	}
	return evaluate(ec, r, ifCont{et, ef, r, k})
}

var undefIf = undef(symbol("if"))

type ifCont struct {
	consequent scmer
	alternate  scmer
	r          *env
	k          continuation
}

func (c ifCont) resume(testResult scmer) (continuation, scmer) {
	branch := c.consequent
	if isFalse(testResult) {
		branch = c.alternate
	}
	return evaluate(branch, c.r, c.k)
}

func evaluateSetVariableSyntax(form array, r *env, k continuation) (continuation, scmer) {
	expectArgs("set!", form[1:], 2, 2)
	sym, value := form[1], form[2]
	if s, ok := sym.(symbol); !ok {
		Fail("set!: first argument must be a symbol")
		panic("unreachable")
	} else {
		return evaluate(value, r, varSetCont{s, r, k})
	}
}

type varSetCont struct {
	sym symbol
	r   *env
	k   continuation
}

func (varSetCont) String() string { return "varSetCont" }

func (c varSetCont) resume(value scmer) (continuation, scmer) {
	c.r.Set(c.sym, value)
	return c.k, undef(symbol("set!"), c.sym)
}

func evaluateCondSyntax(form array, r *env, k continuation) (continuation, scmer) {
	// Precondition: form[0] is |cond|
	// Match: (cond <clause1> <clause2> ...)
	// Each <clause> must have one of these forms:
	//     (<test> <consequent1> ...)
	//     (<test> => <recipient>)
	//  and as a special case the last <clause> may be:
	//     (else <consequent1> <consequent2> ...)
	// Note that `else` is special only when it is the first element of the last clause.
	// Note that `=>` is special only when it is the second element of a three-element clause.
	checkCondSyntax(form)
	return evaluateCondClauses(form[1:], r, k)
}

// evaluateCondClauses evaluates a list of cond clauses, trying each in turn.
func evaluateCondClauses(clauses array, r *env, k continuation) (continuation, scmer) {
	if len(clauses) == 0 {
		return k, undef(symbol("cond"))
	}
	// checkCondSyntax has already verified every clause is a non-empty list.
	clause := clauses[0].(array)
	if len(clauses) == 1 && clause[0] == symbol("else") {
		// `else` is the catch-all test, but only as the last clause (R5RS). In
		// any other position `else` is an ordinary (here, unbound) test.
		return evaluateBeginSyntax(clause[1:], r, k)
	}
	if len(clause) == 3 && clause[1] == symbol("=>") {
		// (<test> => <functor>): if the test is true, apply <functor> to the
		// test value and return the result.
		return evaluate(clause[0], r, condArrowCont{clause[2], clauses, r, k})
	}
	return evaluate(clause[0], r, condCont{clause[1:], clauses, r, k})
}

func checkCondSyntax(form array) {
	clauses := form[1:]
	for i, clauseExpr := range clauses {
		isLastClause := i == len(clauses)-1
		if clause, ok := clauseExpr.(array); !ok {
			Failc("cond-clause-not-list", "cond: clause #%d is not a list, which is invalid syntax: %s", i+1, form)
			panic("unreachable")
		} else if len(clause) == 0 {
			Failc("cond-clause-empty", "cond: clause #%d is an empty list, which is invalid syntax: %s", i+1, form)
			panic("unreachable")
		} else if isLastClause && clause[0] == symbol("else") {
			// `(else ...)` is special, but only as the last clause.
			if len(clause) < 2 {
				Failc("cond-else-no-consequent", "cond: clause #%d is an else clause without at least one consequent, which is invalid syntax: %s", i+1, form)
				panic("unreachable")
			}
		}
	}
}

type condCont struct {
	consequent array
	clauses    array
	r          *env
	k          continuation
}

func (c condCont) resume(test scmer) (continuation, scmer) {
	if isFalse(test) {
		// test == #f: try the remaining clauses
		return evaluateCondClauses(c.clauses[1:], c.r, c.k)
	} else if len(c.consequent) == 0 {
		// (<test>) with no consequents: the test value is the result (R5RS)
		return c.k, test
	} else {
		return evaluateBeginSyntax(c.consequent, c.r, c.k)
	}
}

// condArrowCont handles a (<test> => <functor>) clause once <test> is evaluated.
type condArrowCont struct {
	functor scmer // the <functor> expression
	clauses array // clauses[0] is this clause
	r       *env
	k       continuation
}

func (c condArrowCont) resume(test scmer) (continuation, scmer) {
	if isFalse(test) {
		// test == #f: try the remaining clauses
		return evaluateCondClauses(c.clauses[1:], c.r, c.k)
	}
	// Evaluate <functor>, then apply it to the test value.
	return evaluate(c.functor, c.r, condRecipientCont{test, c.r, c.k})
}

// condRecipientCont applies an evaluated <functor> to the saved test value.
type condRecipientCont struct {
	arg scmer // the test value to pass to <functor>
	r   *env
	k   continuation
}

func (c condRecipientCont) resume(functor scmer) (continuation, scmer) {
	return invoke(functor, list(c.arg), c.r, c.k)
}

func evaluateAndSyntax(form array, r *env, k continuation) (continuation, scmer) {
	// Precondition: form[0] is |and|
	// Match: (and <term1>...)
	return evaluateAnd(form[1:], r, k)
}

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
	if isFalse(test) {
		return c.k, test
	} else {
		return evaluateAnd(c.terms[1:], c.r, c.k)
	}
}

func evaluateOrSyntax(form array, r *env, k continuation) (continuation, scmer) {
	// Precondition: form[0] is |or|
	// Match: (or <term1>...)
	return evaluateOr(form[1:], r, k)
}

func evaluateOr(terms array, r *env, k continuation) (continuation, scmer) {
	if len(terms) == 0 {
		return k, boolean(false)
	} else if len(terms) == 1 {
		return evaluate(terms[0], r, k)
	} else {
		return evaluate(terms[0], r, orCont{terms, r, k})
	}
}

type orCont struct {
	terms array
	r     *env
	k     continuation
}

func (c orCont) resume(test scmer) (continuation, scmer) {
	if isTrue(test) {
		return c.k, test
	} else {
		return evaluateOr(c.terms[1:], c.r, c.k)
	}
}

func evaluateLetSyntax(form array, r *env, k continuation) (continuation, scmer) {
	// Precondition: form[0] is |let|
	// Match:  (let ⟨bindings⟩ ⟨body1⟩ <body2> ...)
	// ⟨bindings⟩ should have the form ( ( ⟨variable1⟩ ⟨init1⟩ ) ...)
	// TODO: implement "named let": (let <variable> <bindings> <body>...)
	bindings, body := form[1], form[2:]
	b, ok := bindings.(array)
	if !ok {
		Fail("invalid (let ((var1 init1)...) body...) syntax: bindings must be a list")
	}
	vars := make(array, len(b)) // TODO: make vars []symbol instead of []scmer
	inits := make(array, len(b))
	for i, varVal := range b {
		if pair, ok := varVal.(array); !ok || len(pair) != 2 {
			Fail("invalid (let ((var1 init1)...) body...) syntax: bindings must be 2-element lists")
			panic("unreachable")
		} else if varName, ok := pair[0].(symbol); !ok {
			Fail("invalid (let ((var1 init1)...) body...) syntax: bindings must bind symbols")
			panic("unreachable")
		} else {
			vars[i] = varName
			inits[i] = pair[1]
		}
	}
	// transform (let ((var1 init1)...) body...)
	//   into
	// ((lambda (var1...) body...) init1...)
	return evalfunCont{inits, r, k}, proc{vars, body, r}
}

func evaluateBeginSyntax(body array, r *env, k continuation) (continuation, scmer) {
	switch len(body) {
	case 0:
		return k, undef(symbol("begin"))
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
	return evaluateBeginSyntax(c.body[1:], c.r, c.k)
}

func evaluateDefineSyntax(form array, r *env, k continuation) (continuation, scmer) {
	// Precondition: form[0] is |define|
	// Match: (define <symbol> <expression>) | (define (<symbol> ...) <body>)
	args := form[1:]
	if len(args) < 2 {
	} else if varName, val, ok := isVarDef(form); ok {
		return evaluate(val, r, varDefCont{varName, r, k})
	} else if fnName, args, body, ok := isFunDef(form); ok {
		r.Define(fnName, proc{args, body, r})
		return k, undef(symbol("define"), fnName)
	}
	Fail("invalid (define ...) syntax: %s", form)
	panic("unreachable")
}

func isVarDef(form array) (name symbol, val scmer, ok bool) {
	// Precondition: form[0] is |define|
	// Match: (define <symbol> <expression>)
	if len(form) == 3 {
		if varName, ok := form[1].(symbol); ok {
			return varName, form[2], true
		}
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
	return c.k, undef(symbol("define"), c.sym)
}

func isFunDef(form array) (name symbol, formals array, body array, ok bool) {
	// Precondition: form[0] is |define|
	// Match: (define (<symbol> <formals>) <body> ...)
	if template, firstArgIsList := form[1].(array); !firstArgIsList || len(template) < 1 {
		return
	} else if functionName, ok := template[0].(symbol); ok {
		return functionName, template[1:], form[2:], true
	}
	return
}

//
// Procedure Calls
//

func invoke(functor scmer, args array, r *env, k continuation) (continuation, scmer) {
	switch f := functor.(type) {
	case prim:
		return f.f(args, r, k)
	case cont:
		expectArgs("<#continuation>", args, 1, 1)
		return f.k, args[0]
	case proc:
		return invokeProc(f, args, r, k)
	}
	Fail("invalid functor: %T %s", functor, functor)
	panic("unreachable")
}

func invokeProc(f proc, args array, r *env, k continuation) (continuation, scmer) {
	switch a := f.params.(type) {
	case array:
		env := f.en.Extend(a, args)
		return evaluateBeginSyntax(f.body, env, k)
	case symbol:
		env := f.en.Extend1(a, args)
		return evaluateBeginSyntax(f.body, env, k)
	default:
		Fail("invalid proc parameters: %T %s", f.params, f.params)
		panic("unreachable")
	}
}

func evaluateNormalProcedureCall(e array, r *env, k continuation) (continuation, scmer) {
	functor, args := e[0], e[1:]
	return evaluate(functor, r, evalfunCont{args, r, k})
}

func asFunctor(e scmer) (functor scmer, ok bool) {
	switch e.(type) {
	case prim:
		return e, true
	case proc:
		return e, true
	case cont:
		return e, true
	}
	return nil, false
}

type evalfunCont struct {
	args array
	r    *env
	k    continuation
}

func (c evalfunCont) resume(functor scmer) (continuation, scmer) {
	return evaluateArguments(c.args, c.r, applyCont{functor, c.r, c.k})
}

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

type gatherCont struct {
	arg scmer
	k   continuation
}

func (c gatherCont) resume(args scmer) (continuation, scmer) {
	return c.k, cons(c.arg, args.(array))
}

type applyCont struct {
	f scmer
	r *env
	k continuation
}

func (c applyCont) resume(args scmer) (continuation, scmer) {
	return invoke(c.f, args.(array), c.r, c.k)
}

func lookup(e scmer, r *env, k continuation) (continuation, scmer) {
	s := e.(symbol)
	return k, r.Lookup(s)
	// table := r.Find(s)
	// if table == nil {
	// 	Fail("undefined symbol: %s", s)
	// 	panic("unreachable")
	// }
	// return k, table.vars[s]
}

type prim struct {
	name symbol
	f    func(actuals array, r *env, k continuation) (continuation, scmer)
}

func (x prim) String() string {
	return fmt.Sprintf("#<primitive:%s>", x.name)
}

type cont struct {
	k continuation
}

func (c cont) String() string {
	return "#<continuation>"
}

// proc represents a procedure created by a lambda expression.
// All procedures are closures.
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

type scope map[symbol]scmer
type env struct {
	scope
	outer *env
}

func (e *env) Find(s symbol) *env {
	if _, ok := e.scope[s]; ok {
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
	return r.scope[s]
}

func (r *env) Define(variable symbol, value scmer) {
	if _, ok := r.scope[variable]; ok {
		Fail("symbol %s already defined in this context", variable)
	}
	r.scope[variable] = value
}

func (orig *env) Extend1(variable symbol, value scmer) (e *env) {
	e = &env{make(scope), orig}
	e.scope[variable] = value
	return
}

func (orig *env) Extend(variables, values array) (e *env) {
	e = &env{make(scope), orig}
	if len(variables) != len(values) {
		Fail("arity mismatch")
	}
	for i, val := range values {
		e.scope[variables[i].(symbol)] = val
	}
	return
}

func (r *env) Set(variable symbol, value scmer) {
	if t := r.Find(variable); t == nil {
		Fail("set: symbol '%s' is not defined", variable)
	} else {
		t.scope[variable] = value
	}
}

/*
 Primitives
*/

// expectArgs panics if argument count is less than minArgs or greater than maxArgs.
// Use math.MaxUInt to specify "no upper bound".
func expectArgs(fnName string, arguments array, minArgs, maxArgs int) {
	argCount := len(arguments)
	if minArgs == maxArgs && argCount != minArgs {
		Fail("%s: expected exactly %d arguments but received %d", fnName, minArgs, argCount)
	} else if argCount < minArgs {
		Fail("%s: expected at least %d arguments but received %d", fnName, minArgs, argCount)
	} else if argCount > maxArgs {
		Fail("%s: expected at most %d arguments but received %d", fnName, maxArgs, argCount)
	}
}

var globalenv env

func listPrimitive() scmer {
	scanner := scan.NewScanner("<str>", "(lambda z z)")
	expr, _ := read(scanner)
	return topLevelEvaluator(expr)
}

func primApply(args array, r *env, k continuation) (continuation, scmer) {
	// (apply proc arg1... args)
	expectArgs("apply", args, 2, math.MaxInt)
	if functor, ok := asFunctor(args[0]); !ok {
		Fail("apply: first argument must be a procedure")
		panic("unreachable")
	} else if lastArg, ok := args[len(args)-1].(array); !ok {
		Fail("apply: last argument must be a list")
		panic("unreachable")
	} else {
		arguments := append(args[1:len(args)-1], lastArg...)
		return invoke(functor, arguments, r, k)
	}
}

func primCallCC(args array, r *env, k continuation) (continuation, scmer) {
	expectArgs("call-with-current-continuation", args, 1, 1)
	return invoke(args[0], list(cont{k}), r, k)
}

func init() {
	std := map[string]func(actuals array, r *env, k continuation) (continuation, scmer){
		"apply":                          primApply,
		"call/cc":                        primCallCC,
		"call-with-current-continuation": primCallCC,
		"write": func(a array, r *env, k continuation) (continuation, scmer) {
			expectArgs("write", a, 1, 1)
			fmt.Print(a[0].String())
			return k, undef(symbol("write"))
		},
		"newline": func(a array, r *env, k continuation) (continuation, scmer) {
			expectArgs("newline", a, 0, 0)
			fmt.Println()
			return k, undef(symbol("newline"))
		},
		"+": func(a array, r *env, k continuation) (continuation, scmer) {
			var v flonum = 0
			for _, i := range a {
				v += i.(flonum)
			}
			return k, v
		},
		"-": func(a array, r *env, k continuation) (continuation, scmer) {
			expectArgs("-", a, 1, math.MaxInt)
			if len(a) == 1 {
				return k, -a[0].(flonum)
			}
			v := a[0].(flonum)
			for _, i := range a[1:] {
				v -= i.(flonum)
			}
			return k, v
		},
		"*": func(a array, r *env, k continuation) (continuation, scmer) {
			var v flonum = 1
			for _, i := range a {
				v *= i.(flonum)
			}
			return k, v
		},
		"/": func(a array, r *env, k continuation) (continuation, scmer) {
			expectArgs("/", a, 1, math.MaxInt)
			if len(a) == 1 {
				return k, 1 / a[0].(flonum)
			}
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
			panic("unreachable")
		},
	}
	builtins := scope{}
	for k, v := range std {
		sym := symbol(k)
		builtins[sym] = prim{sym, v}
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

func isFalse(x scmer) bool {
	if b, ok := x.(boolean); ok {
		return b == false
	}
	return false
}

// In Scheme, only #f is falsy. Absolutely _everything_ else is truthy.
func isTrue(x scmer) bool {
	return !isFalse(x)
}

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

// undef() is like list(), but it adds |#%undef| as the first element.
func undef(sym symbol, obj ...scmer) scmer {
	return array(append([]scmer{symbol("#%undef"), sym}, obj...))
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
