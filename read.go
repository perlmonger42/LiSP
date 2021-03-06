package main

import (
	"fmt"
	"io"
	"os"
	"strconv"

	"github.com/perlmonger42/LiSP/scan"
)

// Parser / Syntactic Analysis
func read(scanner *scan.Scanner) (scmer, error) {
	tok := scanner.Next()
	switch tok.Type {
	case scan.Quote:
		if item, err := read(scanner); err != nil {
			return nil, err
		} else {
			return array{symbol("quote"), item}, nil
		}
	case scan.LeftParen:
		var list array
		for {
			tok = scanner.Peek()
			if tok.Type == scan.RightParen {
				scanner.Next() // consume ")"
				return list, nil
				//// dotted pairs are not yet implemented
				// } else if tok.Type == scan.Dot {
				// 	scanner.Next() // consume "."
				// 	if tail, err := Read(scanner); err != nil {
				// 		return nil, err
				// 	} else {
				// 		*cdrRef = tail
				// 	}
				// 	if tok = scanner.Peek(); tok.Type == scan.RightParen {
				// 		scanner.Next() // consume ")"
				// 		return list, nil
				// 	}
				// 	return nil, fmt.Errorf("unterminated list: %s", list)
			} else if tok.Type == scan.EOF {
				list = append(list, symbol("#%EOF"))
				return list, fmt.Errorf("unterminated list: %s", list)
			} else if item, err := read(scanner); err != nil {
				return nil, err
			} else {
				list = append(list, item)
			}
		}
		return list, nil
	case scan.False:
		return boolean(false), nil
	case scan.True:
		return boolean(true), nil
	case scan.Char:
		r := scan.CharLiteralToRune(tok.Text)
		return char(r), nil
	case scan.String:
		return str(tok.Text), nil
	//// fixnums are not yet implemented
	// case scan.Fixnum:
	// 	if integer, err := strconv.ParseInt(tok.Text, 10, 64); err == nil {
	// 		return expr.NewFixnum(integer), nil
	// 	}
	// 	return nil, fmt.Errorf("int too big: %s", tok.Text)
	case scan.Flonum, scan.Fixnum:
		if float, err := strconv.ParseFloat(tok.Text, 64); err == nil {
			return flonum(float), nil
		}
		return nil, fmt.Errorf("invalid floating-point number: %s", tok.Text)
	case scan.Symbol:
		return symbol(tok.Text), nil
	case scan.EOF:
		return nil, io.EOF
	default:
		fmt.Fprintf(os.Stderr, "unexpected token: %s\n", tok)
		return symbol(tok.Text), nil
		////return nil, fmt.Errorf("unexpected token: %s", tok)
	}
}
