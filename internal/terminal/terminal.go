package terminal

import (
	"agent-demo/internal/llm"
	"bufio"
	"context"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"
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

func (t *Terminal) OnText(delta string) {
	t.Printf("%s", delta)
}

func (t *Terminal) OnToolStart(call llm.ToolCall) {
	t.Colorf(AnsiDim, "\n→ %s %s\n", call.Function.Name, abbreviate(call.Function.Arguments, 200))
}

func (t *Terminal) OnToolEnd(call llm.ToolCall, result string, err error, elapsed time.Duration) {
	if err != nil {
		t.Colorf(AnsiRed, "✗ %s：%v\n", call.Function.Name, err)
		return
	}
	t.Colorf(AnsiDim, "✓ %s（%s，%d 字节）\n", call.Function.Name, elapsed.Round(time.Millisecond), len(result))
}

func (t *Terminal) Approve(ctx context.Context, call llm.ToolCall) bool {
	t.Colorf(AnsiYellow, "\n⚠ 模型请求执行 %s：%s\n", call.Function.Name, call.Function.Arguments)
	answer, err := t.ReadLine("是否允许？[y/N] ")
	return err == nil && strings.EqualFold(answer, "y")
}

func abbreviate(s string, n int) string {
	if r := []rune(s); len(r) > n {
		return string(r[:n]) + "…"
	}
	return s
}
