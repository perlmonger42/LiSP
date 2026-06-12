
;;; PROVENANCE:
;;; Matt Might, [Continuations by example: Exceptions, time-traveling search,
;;; generators, threads, and coroutines](http://matt.might.net/articles/programming-with-continuations--exceptions-backtracking-search-threads-generators-coroutines/)

; current-continuation : -> continuation
(define (current-continuation) 
  (call-with-current-continuation 
   (lambda (cc)
     ;(cc cc)))) WHY DOESN'T THIS WORK?
     ;(write (list "current continuation being returned is" cc))(newline);;;DEBUG
     cc )))

; fail-stack : list[continuation]
(define fail-stack '())

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

; (assert condition) will cause
; condition to be true, and if there
; is no way to make it true, then
; it signals an error in the program.
(define (assert condition)
  (if (not condition)
      (fail)
      #t))


; The following prints (3 4 5)
(let ((a (amb (list 1 2 3 4 5 6 7 8 9)))
      (b (amb (list 1 2 3 4 5 6 7 8 9)))
      (c (amb (list 1 2 3 4 5 6 7 8 9))))
    
  ; We're looking for dimensions of a legal right
  ; triangle using the Pythagorean theorem:
  (assert (= (* c c) (+ (* a a) (* b b))))
  
;  (write (list a b c))
;  (newline)
  
  ; And, we want the first side to be the shorter one:
  (assert (<= a b))

  ; Print out the answer:
  (write (list a b c))
  (newline)
)
;;;;; 
;;;;; 
;;;;; 
;;;;; 
;;;;; 
;;;;; ;; SAT-solving with amb.
;;;;; 
;;;;; (define (implies a b) (or (not a) b))
;;;;;   
;;;;; ;; The is not the most efficient implementation,
;;;;; ;; because a continuation is captured for each
;;;;; ;; occurrence of the same variable, instead of 
;;;;; ;; one for each variable.
;;;;; (define-syntax sat-solve
;;;;;   (syntax-rules (and or implies not)
;;;;;     ((_ formula body)
;;;;;      ; => 
;;;;;      (sat-solve formula body formula))
;;;;;     
;;;;;     ((_ (not phi) body assertion)
;;;;;      ; =>
;;;;;      (sat-solve phi body assertion))
;;;;;     
;;;;;     ((_ (and phi) body assertion)
;;;;;      ; =>
;;;;;      (sat-solve phi body assertion))
;;;;;     
;;;;;     ((_ (and phi1 phi2 ...) body assertion)
;;;;;      ; =>
;;;;;      (sat-solve phi1 (sat-solve (and phi2 ...) body assertion)))
;;;;;     
;;;;;     ((_ (or phi) body assertion)
;;;;;      ; =>
;;;;;      (sat-solve phi body assertion))
;;;;;     
;;;;;     ((_ (or phi1 phi2 ...) body assertion)
;;;;;      ; =>
;;;;;      (sat-solve phi1 (sat-solve (or phi2 ...) body assertion)))
;;;;;     
;;;;;     ((_ (implies phi1 phi2) body assertion)
;;;;;      ; =>
;;;;;      (sat-solve phi1 (sat-solve phi2 body assertion)))
;;;;;     
;;;;;     ((_ #t body assertion)
;;;;;      ; =>
;;;;;      body)
;;;;;     
;;;;;     ((_ #f body assertion)
;;;;;      ; =>
;;;;;      (fail))
;;;;;     
;;;;;     ((_ v body assertion)
;;;;;      (let ((v (amb (list #t #f))))
;;;;;        (if (not assertion)
;;;;;            (fail)
;;;;;            body)))))
;;;;;      
;;;;; 
;;;;; ; The following prints (#f #f #t)
;;;;; (display
;;;;;  (sat-solve (and (implies a (not b))
;;;;;                  (not a)
;;;;;                  c)
;;;;;             (list a b c)))
;;;;;   
;;;;;                  
