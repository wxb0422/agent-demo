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
	//maxStep := flag.Int("max-step", 10, "max step")
	flag.Parse()

	llmCfg := llm.GetDefaultConfig()
	if llmCfg.ApiKey == "" {
		return errors.New("Please set LLM_API_KEY environment variable")
	}

	dir, err := filepath.Abs(*workSpace)
	if err != nil {
		return err
	}

	rootDir, err := os.OpenRoot(dir)
	if err != nil {
		return errors.New(fmt.Sprintf("open root dir=%s, error=%s", dir, err.Error()))
	}
	defer rootDir.Close()

	term := terminal.NewTerminal(os.Stdin, os.Stdout)
	return repl(context.Background(), term, agent.NewAgent())
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
		case "/exit":
			return nil
		default:
			if err = runTurn(ctx, term, ag, line); err != nil {
				return err
			}
			term.Printf("finish\n")
		}
	}
}

func runTurn(ctx context.Context, term *terminal.Terminal, ag *agent.Agent, input string) error {
	ctx, stop := signal.NotifyContext(ctx, os.Interrupt)
	defer stop()

	term.Printf("start\n")

	return nil
}
