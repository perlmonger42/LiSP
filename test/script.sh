#!/usr/bin/env zsh
set -e # exit as soon as any command returns a nonzero status

SCRIPT_DIR=${0:a:h}  # the directory containing this script

GOMOD=$(go env GOMOD)
LiSP_ROOT=${GOMOD:h}  # the root of the Go project (dir containing `go.mod`)


# `/tmp/LiSP.*(N)` expands to nothing if there are no matches
rm -rf -- /tmp/LiSP.*(N)

TMPDIR=$(mktemp -d /tmp/LiSP.XXXXXX)
if [ "$1" = --keep ]; then
  shift
  echo "Keeping directory $TMPDIR"
  KEEP=1
else
  trap '{ rm -rf $TMPDIR; }' EXIT
  KEEP=0
fi

cd $LiSP_ROOT
pwd

LiSP="${LiSP_ROOT}/LiSP"
echo "Using LiSP=$LiSP"


# ---------------------------------------------------------------------------
# Test 1: REPL mode — file input with diverse token types
# ---------------------------------------------------------------------------

cat > ${TMPDIR}/test-repl-input.txt <<'INPUT-S-EXPRESSIONS'
#t ; this is a line comment
#f #| this is a #|nested|# block comment |#
(list #\a #\z #\space #\ﬃ  #\uFB01 #\😀 #\U1F61E #\UE0020)
; LATIN SMALL LIGATURE FII, LATIN SMALL LIGATURE FI, GRINNING FACE, DISAPPOINTED FACE, TAG ASTERISK
42
3.14159265
"howdy"
(cons 1 (cons 2 (cons 3 (quote ()))))
(lambda (x) (cons x (cons (+ x 1) (cons (+ x 2) null))))
((lambda (x) (cons x (cons (+ x 1) (cons (+ x 2) null)))) 17)
(+ 1 2 4.5)
(list 1 2 3.4 (quote (10 9 8)) #t #f (lambda (x) x))
(define (double x) (* x 2))
(double 7)
(double 1.25)
(apply + 1000 200 30 4)
'(+ 8 9)
INPUT-S-EXPRESSIONS

cat > ${TMPDIR}/test-repl-expected.txt <<'EXPECTED_OUTPUT'
#t
#f
(#\a #\z #\space #\ﬃ #\ﬁ #\😀 #\😞 #\U0e0020)
42
3.14159265
"howdy"
(1 2 3)
(lambda (x) (cons x (cons (+ x 1) (cons (+ x 2) null))))
(17 18 19)
7.5
(1 2 3.4 (10 9 8) #t #f (lambda (x) x))
(|#%undef| define double)
14
2.5
1234
(+ 8 9)
EXPECTED_OUTPUT

$LiSP ${TMPDIR}/test-repl-input.txt > ${TMPDIR}/test-repl-output.txt
diff ${TMPDIR}/test-repl-expected.txt ${TMPDIR}/test-repl-output.txt


# ---------------------------------------------------------------------------
# Test 2: -trace -e mode
# ---------------------------------------------------------------------------

cat > ${TMPDIR}/test-trace-expected.txt <<'EXPECTED_OUTPUT_TWO'
=> Evaluate (define (d x) (* x 2))
<= (|#%undef| define d)
(|#%undef| define d)
=> Evaluate (d 7)
  => Evaluate d
  <= (lambda (x) (* x 2))
  => Evaluate 7
  <= 7
  => Evaluate (* x 2)
    => Evaluate *
    <= #<primitive:*>
    => Evaluate x
    <= 7
    => Evaluate 2
    <= 2
  <= 14
<= 14
14
EXPECTED_OUTPUT_TWO

$LiSP -trace -e '(define (d x) (* x 2)) (d 7)' \
  | grep -E '<=|=>|^\S' \
  > ${TMPDIR}/test-trace-output.txt
diff ${TMPDIR}/test-trace-expected.txt ${TMPDIR}/test-trace-output.txt


# ---------------------------------------------------------------------------
# Test 3: -e flag (runExprsFromCommandline)
# ---------------------------------------------------------------------------

got=$($LiSP -e '(+ 2 3)')
if [[ "$got" != "5" ]]; then
  echo "FAILED: -e test: got '$got', expected '5'"
  exit 1
fi


# ---------------------------------------------------------------------------
# Test 4: stdin via - argument (runFilesFromCommandline stdin path)
# ---------------------------------------------------------------------------

got=$(echo '(* 6 7)' | $LiSP -)
if [[ "$got" != "42" ]]; then
  echo "FAILED: stdin test: got '$got', expected '42'"
  exit 1
fi
got=$(echo '(+ 45 -2)'| $LiSP)
if [[ "$got" != "43" ]]; then
  echo "FAILED: -e test: got '$got', expected '43'"
  exit 1
fi


# ---------------------------------------------------------------------------
# Test 5: RERC mode (--test flag) — all pairs pass, no output
# ---------------------------------------------------------------------------

cat > ${TMPDIR}/test-rerc.scm <<'EOF'
; basic arithmetic
(+ 1 2)   3
(* 2 3)   6
(- 10 4)  6

; don't-care sentinel
(define x 99)  ---

; error-expected sentinel
undefined-sym  ***
EOF

rerc_output=$($LiSP --test ${TMPDIR}/test-rerc.scm 2>&1)
if [[ -n "$rerc_output" ]]; then
  echo "FAILED: RERC test produced unexpected output:"
  echo "$rerc_output"
  exit 1
fi


# ---------------------------------------------------------------------------
# Test 6: RERC mode — mismatch is reported on stdout
# ---------------------------------------------------------------------------

cat > ${TMPDIR}/test-rerc-fail.scm <<'EOF'
(+ 1 2)  99
EOF

rerc_fail_output=$($LiSP --test ${TMPDIR}/test-rerc-fail.scm 2>&1)
if ! echo "$rerc_fail_output" | grep -q "unexpected value"; then
  echo "FAILED: RERC mismatch test should have printed 'unexpected value'"
  echo "  got: $rerc_fail_output"
  exit 1
fi


# ---------------------------------------------------------------------------
# Test 7: file-not-found — non-zero exit and error message on stderr
# ---------------------------------------------------------------------------

if $LiSP ${TMPDIR}/no-such-file.scm 2>/dev/null; then
  echo "FAILED: file-not-found test: expected non-zero exit, got zero"
  exit 1
fi

err_output=$($LiSP ${TMPDIR}/no-such-file.scm 2>&1 || true)
if ! echo "$err_output" | grep -q "no-such-file"; then
  echo "FAILED: file-not-found test: expected filename in error output"
  echo "  got: $err_output"
  exit 1
fi


# ---------------------------------------------------------------------------
# Test 8: eval error — non-zero exit status
# ---------------------------------------------------------------------------

if $LiSP -e 'undefined-sym' 2>/dev/null; then
  echo "FAILED: eval-error test: expected non-zero exit, got zero"
  exit 1
fi


# ---------------------------------------------------------------------------
# Test 9: eval error in file — non-zero exit status
# ---------------------------------------------------------------------------

echo 'undefined-sym' > ${TMPDIR}/test-eval-error.scm
if $LiSP ${TMPDIR}/test-eval-error.scm 2>/dev/null; then
  echo "FAILED: file eval-error test: expected non-zero exit, got zero"
  exit 1
fi


# ---------------------------------------------------------------------------
# Test 10: usage — unknown flag exits with code 2 and prints "usage:" to stderr
# ---------------------------------------------------------------------------

usage_exit=0
$LiSP --no-such-flag 2>/dev/null || usage_exit=$?
if [[ $usage_exit -ne 2 ]]; then
  echo "FAILED: usage test: expected exit code 2, got $usage_exit"
  exit 1
fi

usage_output=$($LiSP --no-such-flag 2>&1 || true)
if ! echo "$usage_output" | grep -q "usage:"; then
  echo "FAILED: usage test: expected 'usage:' in stderr"
  echo "  got: $usage_output"
  exit 1
fi

# ---------------------------------------------------------------------------
# Test 11: special forms, control flow, and builtins (RERC)
# ---------------------------------------------------------------------------

cat > ${TMPDIR}/test-scm.scm <<'EOF'
; --- symbols ---
(quote ||)                 ||
(quote |!@#$|)             |!@#$|

; --- if ---
(if #t 1 2)                1
(if #f 1 2)                2
(if 0 "yes" "no")          "yes"

; --- cond ---
(cond (#t 42))                   42
(cond (#f 1) (#t 2))             2
(cond)                           (|#%undef| cond)

; --- and ---
(and)                            #t
(and 42)                         42
(and 1 2)                        2
(and #f 99)                      #f
(and 1 2 3)                      3

; --- begin ---
(begin)                          (|#%undef| begin)
(begin 1 2 3)                    3

; --- set! ---
(define set-x 0)                 ---
(set! set-x 77)                  ---
set-x                            77

; --- let ---
(let ((a 3) (b 4)) (* a b))      12

; --- variadic lambda (invoke proc.params symbol) ---
((lambda x x) 1 2 3)             (1 2 3)

; --- call/cc ---
(call/cc (lambda (k) 42))                42
(call/cc (lambda (k) (k 99) 0))          99
(call/cc (lambda (k) (k 1 2)))           ***
(call/cc (lambda (k) 1) (lambda (k) 2))  ***

; --- arithmetic ---
(/ 10 2)                         5

; --- comparisons ---
(equal? 1 1)                     #t
(equal? 1 2)                     #f
(= 1 2)                          #f
(= 2 2)                          #t
(!= 1 1)                         #f
(!= 1 2)                         #t
(< 1 2)                          #t
(< 2 2)                          #f
(< 3 2)                          #f
(<= 0 1)                         #t
(<= 1 1)                         #t
(<= 2 1)                         #f
(> 3 2)                          #t
(> 3 3)                          #f
(> 3 4)                          #f
(>= 1 0)                         #t
(>= 1 1)                         #t
(>= 1 2)                         #f

; --- car cdr cons ---
(car '(10 20 30))                10
(cdr '(10 20 30))                (20 30)
(cons 7 '(8 9))                  (7 8 9)
(cons 1 2)                       (1 2)

; --- length (returns fixnum, use --- to avoid type mismatch) ---
(length '(a b c))                ---

; --- not ---
(not #f)                         #t
(not #t)                         #f
(not 0)                          #f

; --- null? pair? ---
(null? null)                     #t
(null? '(1))                     #f
(pair? '(1))                     #t
(pair? null)                     #f

; --- error (all three arity branches) ---
(error)                          ***
(error "oops")                   ***
(error "a" "b")                  ***

; --- if arity errors (both branches of expect) ---
(if #t 1)                        ***
(if #t 1 2 3)                    ***

; --- invalid define forms ---
(define no-val)                  ***
(define (1 2) body)              ***

; --- duplicate define (env.Define error path) ---
(define dup-def 1)               ---
(define dup-def 2)               ***

; --- lambda arity mismatch (env.Extend error) ---
((lambda (x y) x) 1)             ***

; --- set! errors ---
(set! 42 0)                      ***
(set! no-such-set-var 0)         ***

; --- let errors ---
(let 1 body)                     ***
(let (1) body)                   ***
(let ((x)) body)                 ***

; --- cond invalid clause ---
(cond 1)                         ***

; --- missing functor ---
(())                             ***

EOF

test11_output=$($LiSP --test ${TMPDIR}/test-scm.scm 2>&1)
if [[ -n "$test11_output" ]]; then
  echo "FAILED: Test 11 (special forms) produced unexpected output:"
  echo "$test11_output"
  exit 1
fi


# ---------------------------------------------------------------------------
# Test 12: char.String() named characters
# ---------------------------------------------------------------------------

for char_case in \
  '#\nul:#\nul' \
  '#\backspace:#\backspace' \
  '#\tab:#\tab' \
  '#\newline:#\newline' \
  '#\vtab:#\vtab' \
  '#\page:#\page' \
  '#\return:#\return' \
  '#\rubout:#\rubout' \
  '#\u0085:#\u0085' \
; do
  char_lit=${char_case%:*}
  expected=${char_case#*:}
  got=$($LiSP -e "$char_lit")
  if [[ "$got" != "$expected" ]]; then
    echo "FAILED: char test $char_lit: got '$got', expected '$expected'"
    exit 1
  fi
done


# ---------------------------------------------------------------------------
# Test 13: fixnum.String() and write/newline primitives
# ---------------------------------------------------------------------------

got=$($LiSP -e '(length null)')
if [[ "$got" != "0" ]]; then
  echo "FAILED: fixnum test: got '$got', expected '0'"
  exit 1
fi

got=$($LiSP -e '(write "hello")')
if [[ "$got" != '"hello"(|#%undef| write)' ]]; then
  echo "FAILED: write test: got '$got', expected '\"hello\"(|#%undef| write)'"
  exit 1
fi

got=$($LiSP -e '(list 7' 2>&1)
if [[ "$got" != *unterminated list* ]]; then
  echo "FAILED: unterminated list test: got '$got', expected '*unterminated list*'"
  exit 1
fi


$LiSP -e '(newline)' > /dev/null


echo All tests passed
