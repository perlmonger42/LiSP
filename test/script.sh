#!/usr/bin/env bash
set -e # exit as soon as any command returns a nonzero status
trap "{ rm -f ~/tmp/test-repl*; }" EXIT



go install github.com/perlmonger42/LiSP



cat > ~/tmp/test-repl-input.txt <<'INPUT-S-EXPRESSIONS' 
#t ; this is a line comment
#f #| this is a #|nested|# block comment |#
(list #\a #\z #\space #\ﬃ  #\uFB01 #\😀 #\U1F61E)
; LATIN SMALL LIGATURE FII, LATIN SMALL LIGATURE FI, GRINNING FACE, DISAPPOINTED FACE
42
3.14159265
"howdy"
(cons 1 (cons 2 (cons 3 (quote ()))))
(lambda (x) (cons x (cons (+ x 1) (cons (+ x 2) null))))
((lambda (x) (cons x (cons (+ x 1) (cons (+ x 2) null)))) 17)
(+ 1 2 4.5)
(list 1 2 3.4 (quote (10 9 8)) #t #f (lambda (x) x))
;(define (double x) (* x 2))
(define double (lambda (x) (* x 2)))
(double 7)
(double 1.25)
INPUT-S-EXPRESSIONS

cat > ~/tmp/test-repl-expected.txt <<'EXPECTED_OUTPUT'
#t
#f
(#\a #\z #\space #\ﬃ #\ﬁ #\😀 #\😞)
42
3.14159265
"howdy"
(1 2 3)
(lambda (x) (cons x (cons (+ x 1) (cons (+ x 2) null))))
(17 18 19)
7.5
(1 2 3.4 (10 9 8) #t #f (lambda (x) x))
(#%undef define double)
14
2.5
EXPECTED_OUTPUT

"$GOPATH"/bin/LiSP ~/tmp/test-repl-input.txt > ~/tmp/test-repl-output.txt
diff ~/tmp/test-repl-expected.txt ~/tmp/test-repl-output.txt



cat > ~/tmp/test-repl-expected-2.txt <<'EXPECTED_OUTPUT_TWO'
=> Define (define d (lambda (x) (* x 2)))
  => Evaluate (lambda (x) (* x 2))
  <= (lambda (x) (* x 2))
<= (#%undef define d)
(#%undef define d)
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

"$GOPATH"/bin/LiSP -trace -e '(define d (lambda (x) (* x 2))) (d 7)' > ~/tmp/test-repl-output-2.txt



echo All tests passed
