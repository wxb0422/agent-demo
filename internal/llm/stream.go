package llm

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

type streamChunk struct {
	Choices []streamChunkChoice `json:"choices"`
	Usage   *Usage              `json:"usage"`
	Error   *streamChunkError   `json:"error"`
}

type streamChunkChoice struct {
	Delta        streamChunkChoiceDelta `json:"delta"`
	FinishReason string                 `json:"finish_reason"`
}

type streamChunkChoiceDelta struct {
	Content   string                                `json:"content"`
	ToolCalls []streamChunkChoiceDeltaToolCallDelta `json:"tool_calls"`
}

// toolCallDelta 是工具调用的一个分片。流式模式下，一次工具调用可能被拆成能多个分片；
// 第一个分片通常带有id和name, 后续分片只带argument的片段，通过index关联。
type streamChunkChoiceDeltaToolCallDelta struct {
	Index    int                                    `json:"index"`
	ID       string                                 `json:"id"`
	Function streamChunkChoiceDeltaToolCallFunction `json:"function"`
}

type streamChunkChoiceDeltaToolCallFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type streamChunkError struct {
	Message string `json:"message"`
}

// readStream 解析SSE响应并拼接出完整的消息体
// SSE协议：每个事件是若干行 "DATA:...", 事件之间已空行隔开
// 已":" 开头作为心跳，openAI 已"DATA:[DONE]" 表示结束。
func readStream(r io.Reader, onText func(string)) (*Response, error) {
	var (
		out     Response
		text    strings.Builder
		calls   []*ToolCall
		byIndex = make(map[int]*ToolCall)
		done    bool
	)

	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64<<10), 4<<20)

	for scanner.Scan() {
		line := scanner.Text()

		data, ok := strings.CutPrefix(line, "data:")
		if !ok {
			continue
		}

		data = strings.TrimSpace(data)

		if data == "[DONE]" {
			done = true
			break
		}

		var chunk streamChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			return nil, fmt.Errorf("failed to unmarshal chunk: %s, error=%s", data, err.Error())
		}
		if chunk.Error != nil {
			return nil, fmt.Errorf("llm stream error: %s", chunk.Error.Message)
		}
		if chunk.Usage != nil {
			out.Usage = *chunk.Usage
		}

		for _, choice := range chunk.Choices {
			if choice.Delta.Content != "" {
				text.WriteString(choice.Delta.Content)
			}
			if onText != nil {
				onText(choice.Delta.Content)
			}

			for _, tool := range choice.Delta.ToolCalls {
				call, ok := byIndex[tool.Index]
				if !ok {
					call = &ToolCall{
						Type: "function",
					}
					byIndex[tool.Index] = call
					calls = append(calls, call)
				}
				if tool.ID != "" {
					call.Id = tool.ID
				}
				if tool.Function.Name != "" {
					call.Function.Name = tool.Function.Name
				}
				call.Function.Arguments += tool.Function.Arguments
			}

			if choice.FinishReason != "" {
				out.FinishReason = choice.FinishReason
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read stream: %w", err)
	}
	if !done && out.FinishReason == "" {
		return nil, fmt.Errorf("llm stream error: %s", io.ErrUnexpectedEOF)
	}

	out.Message = Message{
		Role:    RoleAssistant,
		Content: text.String(),
	}

	for i, call := range calls {
		if call.Id == "" {
			call.Id = fmt.Sprintf("call_%d", i)
		}
		out.Message.ToolCalls = append(out.Message.ToolCalls, *call)
	}

	return &out, nil
}
