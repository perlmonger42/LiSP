package main

import (
	"fmt"

	"github.com/perlmonger42/LiSP/scan"
)

func main() {
	scanner := scan.NewScanner("<stdin>", scan.NewConsoleReader())
	for err := Repl(scanner); err != nil; err = Repl(scanner) {
		fmt.Println(err)
	}
}

//func main() {
//	scanner := scan.NewScanner("<str>", strings.NewReader("(list 1)"))
//	for err := Repl(scanner); err != nil; err = Repl(scanner) {
//		fmt.Println(err)
//	}
//}
