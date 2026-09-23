package llm

type streamChunk struct {
	Choices []streamChunkChoice `json:"choices"`
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
