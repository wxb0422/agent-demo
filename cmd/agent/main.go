package main

import (
	"agent-demo/internal/agent"
	"agent-demo/internal/llm"
	"agent-demo/internal/terminal"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "error:%s", err.Error())
		os.Exit(1)
	}
}

func run() error {
	workSpace := flag.String("workspace", ".", "workspace to use")
	maxStep := flag.Int("max-step", 10, "max step")

	flag.Parse()

	llmCfg := llm.GetDefaultConfig()
	if llmCfg.ApiKey == "" {
		return errors.New("Please set LLM_API_KEY environment variable")
	}

	dir, err := filepath.Abs(*workSpace)
	if err != nil {
		return err
	}

	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		return fmt.Errorf("workspace %s is not a directory", dir)
	}

	term := terminal.NewTerminal(os.Stdin, os.Stdout)
	return repl(context.Background(), term, agent.NewAgent(llmCfg, dir, *maxStep, term))
}

func repl(ctx context.Context, term *terminal.Terminal, ag *agent.Agent) error {
	for {
		line, err := term.ReadLine("\n> ")
		if err == io.EOF {
			term.Printf("\n")
			return nil
		}
		if err != nil {
			return err
		}

		switch line {
		case "":
			continue
		case "/exit":
			return nil
		default:
			if err = runTurn(ctx, term, ag, line); err != nil {
				term.Colorf(terminal.AnsiDim, "✗ %v\n", err)
			}
		}
	}
}

func runTurn(ctx context.Context, term *terminal.Terminal, ag *agent.Agent, input string) error {
	ctx, stop := signal.NotifyContext(ctx, os.Interrupt)
	defer stop()

	term.Printf("\n")
	res, err := ag.Run(ctx, input)
	term.Printf("\n")

	if err != nil {
		if ctx.Err() != nil {
			term.Colorf(terminal.AnsiDim, "（已中断）\n")
			return nil
		}
		return err
	}

	term.Colorf(terminal.AnsiDim, "[%d 步 输入%d token, 输出%d token]\n",
		res.Step, res.Usage.PromptTokens, res.Usage.CompletionTokens)

	return nil
}
