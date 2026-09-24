package tool

import (
	"agent-demo/internal/llm"
	"context"
	"encoding/json"
	"fmt"
)

// tool 定义为agent 可以调用的工具

// tool 是agent 能调用的能力。模型只能看到name,parameters,description
// 所以description 写得完不完整，直接决定模型会不会在正确的时机调用它。
type Tool interface {
	Name() string
	Description() string
	Parameters() json.RawMessage
	Run(ctx context.Context, args json.RawMessage) (string, error)
}

type Approver interface {
	RequiresApproval(ctx context.Context) bool
}

// 这个工具是否需要申请批准
func RequiresApprovalWithTool(tool Tool) bool {
	a, ok := tool.(Approver)
	return ok && a.RequiresApproval(context.Background())
}

type Func[T any] struct {
	name        string
	description string
	schema      json.RawMessage
	approval    bool
	fn          func(context.Context, T) (string, error)
}

func NewFuncTool[T any](name, description, schema string, executorFn func(context.Context, T) (string, error)) *Func[T] {
	if valid := json.Valid([]byte(schema)); !valid {
		panic(fmt.Sprintf("tool %s: invalid JSON schema", name))
	}

	return &Func[T]{
		name:        name,
		description: description,
		schema:      json.RawMessage(schema),
		fn:          executorFn,
	}
}

func (f *Func[T]) WithApproval() {
	f.approval = true
}

func (f *Func[T]) Name() string {
	return f.name
}

func (f *Func[T]) Description() string {
	return f.description
}

func (f *Func[T]) Parameters() json.RawMessage {
	return f.schema
}

func (f *Func[T]) RequiresApproval(ctx context.Context) bool {
	return f.approval
}

func (f *Func[T]) Run(ctx context.Context, raw json.RawMessage) (string, error) {
	var args T
	if err := json.Unmarshal(raw, &args); err != nil {
		return "", fmt.Errorf("tool %s: invalid JSON: %w", f.Name(), err)
	}

	return f.fn(ctx, args)
}

type ToolsFactory struct {
	tools map[string]Tool
	order []string
}

func (f *ToolsFactory) RegisterTool(tool Tool) {
	if f.tools == nil {
		f.tools = make(map[string]Tool)
	}
	if dup, ok := f.tools[tool.Name()]; ok {
		panic(fmt.Sprintf("tool %s: duplicate tool name: %s", tool.Name(), dup))
	}
	f.tools[tool.Name()] = tool
	f.order = append(f.order, tool.Name())
}

func NewToolsFactory(tools ...Tool) *ToolsFactory {
	tmp := &ToolsFactory{
		tools: make(map[string]Tool),
	}
	for _, tool := range tools {
		tmp.RegisterTool(tool)
	}

	return tmp
}

func (f *ToolsFactory) GetTool(name string) (Tool, bool) {
	tool, ok := f.tools[name]
	return tool, ok
}

func (f *ToolsFactory) AllToolsDefine() []llm.ToolDef {
	allDefs := make([]llm.ToolDef, 0, len(f.order))
	for _, name := range f.order {
		tool, exist := f.GetTool(name)
		if !exist {
			continue
		}
		allDefs = append(allDefs, llm.ToolDef{
			Type: "function",
			Function: llm.FunctionDef{
				Name:        tool.Name(),
				Description: tool.Description(),
				Parameters:  tool.Parameters(),
			},
		})
	}
}
