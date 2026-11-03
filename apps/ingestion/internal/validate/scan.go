package validate

import (
	"bufio"
	"io"
)

func newBoundedScanner(reader io.Reader) *bufio.Scanner {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 64*1024), 16*1024*1024)
	return scanner
}
