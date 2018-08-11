package scan

import (
	"io"

	"github.com/bobappleyard/readline"
	// "gitlab.com/Scheming/interpreter/config" //// config not yet implemented
)

type gnuReadline struct {
	// config *config.T //// config not yet implemented
	line string
	next int
}

func NewConsoleReader( /* config *config.T */ ) io.ByteReader {
	return &gnuReadline{ /*////config,*/ "", 0}
}

func (r *gnuReadline) ReadByte() (byte, error) {
	var err error
	for r.next >= len(r.line) {
		//// prompt := r.config.Prompt
		//// if prompt == "" {
		//// 	prompt = "scm> "
		//// }
		prompt := "scm> "
		r.line, err = readline.String(prompt)
		if err != nil {
			return 0, err
		}
		// fmt.Printf("Got %q\n", r.line)
		readline.AddHistory(r.line)
		r.next = 0
		r.line += "\n"
	}
	var b byte = r.line[r.next]
	r.next++
	// fmt.Printf("Returning %c (%#c)\n", b, b)
	return b, nil
}

// func splitAtRuneBoundary(b []byte, s string) (byteCount int, remaining string) {
// 	if len(s) <= len(b) {
// 		copy(b, s)
// 		return len(s), ""
// 	}
// 	i := len(b)
// 	for i > 0 && i >= len(b)-utf8.UTFMax && !utf8.RuneStart(s[i]) {
// 		i--
// 	}
// 	copy(b, s[:i])
// 	return i, s[i:]
// }
