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
