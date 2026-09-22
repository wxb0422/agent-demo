package terminal

import (
	"bufio"
	"fmt"
	"io"
	"strings"
	"sync"
)

const (
	ansiReset  = "\033[0m"
	ansiDim    = "\033[2m"
	ansiRed    = "\033[31m"
	ansiYellow = "\033[33m"
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
	t.Printf(color+format+ansiReset, args...)
}

func (t *Terminal) ReadLine(prompt string) (string, error) {
	t.Printf("%s", prompt)
	line, err := t.in.ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(line), nil
}
