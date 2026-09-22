package agent

import (
	"agent-demo/internal/llm"
	"context"
	"fmt"
	"time"
)

type ChatModel interface {
	Stream(ctx context.Context, messages []llm.Message, tools []llm.ToolDef, onText func(string)) (llm.Response, error)
}

type Agent struct {
	Opt     Option
	history []llm.Message
}

type Option struct {
	LLM          ChatModel
	SystemPrompt string
	MaxSteps     int

	Tools              interface{}
	ToolsTimeout       time.Duration
	MaxToolOutPutBytes int
}

type Result struct {
	Text  string    // 第n轮的答复
	Step  int       // 第n轮对话
	Usage llm.Usage // 本轮对话消耗的token
}

func NewAgent() *Agent {
	return &Agent{}
}

// run 标识执行一轮input信息
func (a *Agent) Run(ctx context.Context, input string) {
	checkpoint := len(a.history)
	a.history = append(a.history, llm.Message{
		Role:    llm.RoleUser,
		Content: input,
	})

}

func (a *Agent) loop(ctx context.Context) (*Result, error) {
	for step := 1; step <= a.Opt.MaxSteps; step++ {
		resp, err := a.Opt.LLM.Stream(ctx, a.history, nil, nil)
		if err != nil {
			return nil, fmt.Errorf("step %d: %w", step, err)
		}

		a.history = append(a.history, resp.Message)
		if len(resp.Message.ToolCalls) == 0 {
			return &Result{
				Text: resp.Message.Content,
				Step: step,
			}, nil
		}

		// 将tool 执行的结果 放入到下轮的message中，重新发给大模型
	}

	return nil, fmt.Errorf("max steps exceed: %d", a.Opt.MaxSteps)
}
