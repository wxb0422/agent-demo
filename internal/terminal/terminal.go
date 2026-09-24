package terminal

import (
	"bufio"
	"fmt"
	"io"
	"strings"
	"sync"
)

const (
	AnsiReset  = "\033[0m"
	AnsiDim    = "\033[2m"
	AnsiRed    = "\033[31m"
	AnsiYellow = "\033[33m"
)

type Terminal struct {
	in   *bufio.Reader
	out  io.Writer
	lock sync.Mutex
}

func NewTerminal(in io.Reader, out io.Writer) *Terminal {
	return &Terminal{in: bufio.NewReader(in), out: out}
}

func (t *Terminal) Printf(format string, args ...any) {
	t.lock.Lock()
	defer t.lock.Unlock()
	fmt.Fprintf(t.out, format, args...)
}

func (t *Terminal) Colorf(color, format string, args ...any) {
	t.Printf(color+format+AnsiReset, args...)
}

func (t *Terminal) ReadLine(prompt string) (string, error) {
	t.Printf("%s", prompt)
	line, err := t.in.ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(line), nil
}
