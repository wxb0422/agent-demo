package agent

import (
	"agent-demo/internal/llm"
	"agent-demo/internal/tool"
	"agent-demo/internal/tool/builtin"
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

type ChatModel interface {
	Stream(ctx context.Context, messages []llm.Message, tools []llm.ToolDef, onText func(string)) (*llm.Response, error)
}

type Agent struct {
	Opt     Option
	history []llm.Message
	usage   llm.Usage
}

type Option struct {
	LLM          ChatModel
	SystemPrompt string
	MaxSteps     int

	Tools              *tool.ToolsFactory
	ToolsTimeout       time.Duration
	MaxToolOutPutBytes int
}

type Result struct {
	Text  string    // 第n轮的答复
	Step  int       // 第n轮对话
	Usage llm.Usage // 本轮对话消耗的token
}

func NewAgent(cfg *llm.Config, workSpaceDir string, oneLoopMaxStep int) *Agent {
	largeModel, err := llm.NewClient(*cfg)
	if err != nil {
		panic("new llm fail, err:" + err.Error())
	}

	toolFactory := tool.NewToolsFactory(
		builtin.CurrentTime(),
		builtin.Calculator(),
		builtin.HTTPGet(),
		builtin.RunCommand(workSpaceDir),
	)

	return &Agent{
		Opt: Option{
			LLM:                largeModel,
			SystemPrompt:       systemPrompt(workSpaceDir),
			MaxSteps:           oneLoopMaxStep,
			Tools:              toolFactory,
			ToolsTimeout:       60 * time.Second,
			MaxToolOutPutBytes: 32 << 10,
		},
		history: []llm.Message{
			{
				Role:    llm.RoleSystem,
				Content: systemPrompt(workSpaceDir),
			},
		},
	}
}

// run 标识执行一轮input信息
func (a *Agent) Run(ctx context.Context, input string) (*Result, error) {
	checkpoint := len(a.history)
	a.history = append(a.history, llm.Message{
		Role:    llm.RoleUser,
		Content: input,
	})

	res, err := a.loop(ctx)
	if err != nil {
		a.history = a.history[:checkpoint]
		return nil, err
	}

	return res, nil
}

func (a *Agent) loop(ctx context.Context) (*Result, error) {
	allToolDefs := a.Opt.Tools.AllToolsDefine()

	var usage llm.Usage
	for step := 1; step <= a.Opt.MaxSteps; step++ {
		resp, err := a.Opt.LLM.Stream(ctx, a.history, allToolDefs, nil)
		if err != nil {
			return nil, fmt.Errorf("step %d: %w", step, err)
		}

		usage.Add(resp.Usage)
		a.usage.Add(resp.Usage)
		a.history = append(a.history, resp.Message)

		// 没有工具调用，直接返回结果
		if len(resp.Message.ToolCalls) == 0 {
			return &Result{
				Text: resp.Message.Content,
				Step: step,
			}, nil
		}

		// 将tool 执行的结果 放入到下轮的message中，重新发给大模型
		a.history = append(a.history, a.runTools(ctx, resp.Message.ToolCalls)...)
	}

	return nil, fmt.Errorf("max steps exceed: %d", a.Opt.MaxSteps)
}

func (a *Agent) runTools(ctx context.Context, calls []llm.ToolCall) []llm.Message {
	result := make([]llm.Message, len(calls))
	var wg sync.WaitGroup

	for index, call := range calls {
		result[index] = llm.Message{
			Role:       llm.RoleTool,
			ToolCallID: call.Id,
		}

		t, err := a.prepareTool(ctx, call)
		if err != nil {
			result[index].Content = err.Error()
			continue
		}

		wg.Go(func() {
			result[index].Content = a.executeTool(ctx, t, call)
		})
	}

	wg.Wait()
	return result
}

func (a *Agent) prepareTool(ctx context.Context, call llm.ToolCall) (tool.Tool, error) {
	t, ok := a.Opt.Tools.GetTool(call.Function.Name)
	if !ok {
		return nil, fmt.Errorf("tool %s not found", call.Function.Name)
	}

	return t, nil
}

func (a *Agent) executeTool(ctx context.Context, tool tool.Tool, call llm.ToolCall) string {
	ctx, cancel := context.WithTimeout(ctx, a.Opt.ToolsTimeout)
	defer cancel()

	result, _ := safeRun(ctx, tool, json.RawMessage(call.Function.Arguments))
	return result
}

func safeRun(ctx context.Context, t tool.Tool, args json.RawMessage) (result string, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic: %v", r)
		}
	}()

	return t.Run(ctx, args)
}

func systemPrompt(workspace string) string {
	return fmt.Sprintf(`你是运行在用户终端里的 AI 助手，可以调用工具完成任务。

工作原则：
- 需要实时信息、文件内容或数值计算时，调用工具获取结果，不要凭记忆编造。
- 文件类工具的路径相对于工作目录：%s
- 工具返回 error 时，先分析原因，再调整参数重试或换一种方法。
- 能并行调用的工具尽量在一次回复中同时调用。
- 使用中文，回答简洁。`, workspace)
}
