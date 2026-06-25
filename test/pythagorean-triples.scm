;;; PROVENANCE: the time-traveling-search (amb) example below is adapted
;;; from Matt Might, "Continuations by example: Exceptions, time-traveling
;;; search, generators, threads, and coroutines":
;;; http://matt.might.net/articles/programming-with-continuations--exceptions-backtracking-search-threads-generators-coroutines/
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

