#!/usr/bin/env zsh

set -e  # exit as soon as any command returns a nonzero status

GOMOD=$(go env GOMOD)
if [[ $GOMOD = /dev/null ]]; then
  script_dir=${0:A:h}
  echo "running in script_dir: ${script_dir}"
  cd "${script_dir}"
  GOMOD=$(go env GOMOD)
  if [[ $GOMOD = /dev/null ]]; then
    echo 1>&2 "$0 must be run with current working directory inside a Go project"
    exit 1
  fi
fi
PROJECT=${GOMOD:h}  # the root of the Go project (dir containing `go.mod`)
cd "${PROJECT}"

export CGO_CFLAGS="-I$( brew --prefix readline)/include"
export CGO_LDFLAGS="-L$(brew --prefix readline)/lib"
echo Build program
go generate internal/scan/scan.go || (echo go generate failed && false)
go build -o ./LiSP ./cmd/LiSP || (echo go build failed && false)

echo Run tests
./LiSP -test <<'EOF'
    42 ; this is a line comment
    42

    43 #| this is a #|nested|# block comment |#
    43

    (list #\a #\z #\space #\ﬃ  #\xFB01 #\😀 #\X1F61E #\xE0020)
    ; LATIN SMALL LIGATURE FII, LATIN SMALL LIGATURE FI, GRINNING FACE, DISAPPOINTED FACE, TAG ASTERISK
    (#\a #\z #\space #\ﬃ #\ﬁ #\😀 #\😞 #\xe0020)

    3.14159265  3.14159265
    (/ 355 113) 3.1415929203539825

    "howdy" "howdy"
    (cons 1 (cons 2 (cons 3 (quote ()))))  (1 2 3)

    ; TODO: fails to compare with DeepEquals
    ; (lambda (x) (cons x (cons (+ x 1) (cons (+ x 2) null))))
    ; (lambda (x) (cons x (cons (+ x 1) (cons (+ x 2) null))))

    ((lambda (x) (cons x (cons (+ x 1) (cons (+ x 2) null)))) 17)
    (17 18 19)

    (+ 1 2 4.5)  7.5

    ; TODO: fails to compare with DeepEquals
    ;(list 1 2 3.4 (quote (10 9 8)) #t #f (lambda (x) x))
    ;(1 2 3.4 (10 9 8) #t #f (lambda (x) x))

    (define (double x) (* x 2)) ---
    (double 7)                    14
    (double 1.25)                 2.5

    (apply + '(1000 200 30 4))    1234
    (apply * 2 3 '(5 7))          210
    '(+ 8 9)                      (+ 8 9)

    ; basic arithmetic
    (+)       0
    (+ 11)    11
    (+ 9 1)   10
    (+ 2 5 8) 15
    (*)       1
    (* 13)    13
    (* 2 3)   6

    (-)         ***
    (- 14)      -14
    (- 10 4)    6
    (- 10 1 5)  4
    (/)         ***
    (/ 4)       0.25
    (/ 10 4)    2.5
    (/ 30 2 3)  5

    ; character syntax
    #\c          #\x63
    #\nul        #\x0
    #\backspace  #\x8
    #\tab        #\x9
    #\newline    #\xA
    #\vtab       #\xB
    #\page       #\xC
    #\return     #\xD
    #\rubout     #\x7F
    #\x00000085  #\x85

    ; --- symbols ---
    '()                        ()
    (quote ())                 ()
    (quote ||)                 ||
    (quote |!@#$|)             |!@#$|
    '|x|                       |x|
    undefined-sym              ***

    ; --- missing functor ---
    (())                             ***
    ; --- invalid functor ---
    (1)                              ***

    ; --- if ---
    (if #t      )              ***  ; invalid arity
    (if #t 1 2)                1
    (if #f 1 2)                2
    (if #t 1 2 3)              ***  ; invalid arity
    (if 0 "yes" "no")          "yes"
    (if #t "ok")               "ok"
    (if #f "ok")               (|#%undef| if)

    ; --- cond ---
    (cond 1)                         *** ; invalid syntax
    (cond (#t 42))                   42
    (cond (#f 1) (#t 2))             2
    (cond)                           (|#%undef| cond)

    ; --- and ---
    (and)                           #t
    (and #f)                        #f
    (and  1)                        1
    (and #f #f)                     #f
    (and #f  8)                     #f
    (and  2 #f)                     #f
    (and  3  9)                     9
    (and #f #f #f)                  #f
    (and #f #f  5)                  #f
    (and #f  1 #f)                  #f
    (and #f  2  6)                  #f
    (and  4 #f #f)                  #f
    (and  5 #f  7)                  #f
    (and  6  3 #f)                  #f
    (and  7  4  8)                  8

    ; --- or ---
    (or)                            #f
    (or #f)                         #f
    (or  1)                         1
    (or #f #f)                      #f
    (or #f  8)                      8
    (or  2 #f)                      2
    (or  3  9)                      3
    (or #f #f #f)                   #f
    (or #f #f  5)                   5
    (or #f  1 #f)                   1
    (or #f  2  6)                   2
    (or  4 #f #f)                   4
    (or  5 #f  7)                   5
    (or  6  3 #f)                   6
    (or  7  4  8)                   7

    ; --- begin ---
    (begin)                          (|#%undef| begin)
    (begin 1 2 3)                    3

    ; --- set! ---
    (define var-x 0)                 (|#%undef| define var-x)
    (set! var-x 77)                  (|#%undef| set! var-x)
    var-x                            77

    ; --- let ---
    (let ((a 3) (b 4)) (* a b))      12
    (let 1 body)                     *** ; invalid syntax
    (let (1) body)                   *** ; invalid syntax
    (let ((x)) body)                 *** ; invalid syntax

    ; --- variadic lambda (invoke proc.params symbol) ---
    ;(lambda l l)                     (lambda l l)
    ((lambda x x) 1 2 3)             (1 2 3)

    ; --- call/cc ---
    (call/cc (lambda (k) 42))                42
    (call/cc (lambda (k) (k 99) 0))          99
    (call/cc (lambda (k) (k 1 2)))           ***
    (call/cc (lambda (k) 1) (lambda (k) 2))  ***

    ; --- arithmetic ---
    (/ 10 2)                         5
    (+ (* (/ 1000 100) 9) 8)         98

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

    ; TODO: Add fixnum datatypes
    ; TODO: Properly implement = eq? eqv? equal?
    ; These currently fail because length returns fixnum, 3 scans as flonum,
    ; and -test uses reflect.DeepEqual(value, expect) rather than a
    ; proper comparison.
    ; (length '(a b c))                3
    ; (length null)                    0

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

    ; --- invalid define forms ---
    (define no-val)                  ***
    (define (1 2) body)              ***

    ; --- duplicate define
    (define dup-def 1)               ---
    (define dup-def 2)               ***

    ; --- lambda arity mismatch
    ((lambda (x y) x) 1)             ***

    ; --- set! errors ---
    (set! 42 0)                      ***
    (set! no-such-set-var 0)         ***

EOF

echo Test continuations
#./LiSP -verbose -test <<'EOF'
./LiSP -test <<'EOF'
    (define (current-continuation)
      (call-with-current-continuation
       (lambda (cc) cc) ) )
    (|#%undef| define current-continuation)

    ; fail-stack : list[continuation]
    (define fail-stack '())
    (|#%undef| define fail-stack)

    ; fail : -> ...
    (define (fail)
      ;(write (list "failing with fail-stack:" fail-stack))(newline);;;DEBUG
      (if (not (pair? fail-stack))
          (error "back-tracking stack exhausted!")
          (begin
            (let ((back-track-point (car fail-stack)))
              ;(write (list "back-track-point: " back-track-point))(newline);;;DEBUG
              (set! fail-stack (cdr fail-stack))
              (cond ((pair? back-track-point) (set! back-track-point (car back-track-point))))
              ;(write (list "fail-stack is now:" fail-stack))(newline);;;DEBUG
              (back-track-point back-track-point)))))
    (|#%undef| define fail)

    ; amb : list[a] -> a
    (define (amb choices)
      ;(write (list "amb choices:" choices))(newline);;;DEBUG
      (let ((cc (current-continuation)))
        (cond
         ;((not (write (list "cond: choices is:" choices " and cc is " cc))(newline)) (write "huh?")(newline));;;DEBUG
          ((null? choices)      ;(write "amb: choices is null")(newline);;;DEBUG
                                (fail))
          ((pair? choices)      (let ((choice (car choices)))
                                  (set! choices (cdr choices))
                                  (set! fail-stack (cons cc fail-stack))
                                  ;(write (list "fail-stack is now " fail-stack))(newline);;;DEBUG
                                  ;(write "amb returning ")(write choice)(newline);;;DEBUG
                                  choice)))))
    (|#%undef| define amb)

    ; (assert condition) will cause
    ; condition to be true, and if there
    ; is no way to make it true, then
    ; it signals an error in the program.
    (define (assert condition)
      (if (not condition)
          (fail)
          #t))
    (|#%undef| define assert)

    ; The following returns (3 4 5)
    (let ((a (amb (list 1 2 3 4 5 6 7 8 9)))
          (b (amb (list 1 2 3 4 5 6 7 8 9)))
          (c (amb (list 1 2 3 4 5 6 7 8 9))))

      ; We're looking for dimensions of a legal right
      ; triangle using the Pythagorean theorem:
      (assert (= (* c c) (+ (* a a) (* b b))))

      ; And, we want the first side to be the shorter one:
      (assert (<= a b))

      ; Print out the answer:
      (list a b c)
    )
    (3 4 5)
EOF

echo Done
