package oaichat

import (
	"testing"

	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResponseOpenAI2ClaudeToolUseInputIsObject(t *testing.T) {
	tests := []struct {
		name string
		args string
		want map[string]interface{}
	}{
		{name: "object", args: `{"q":"x"}`, want: map[string]interface{}{"q": "x"}},
		{name: "empty", args: "", want: map[string]interface{}{}},
		{name: "invalid", args: "{", want: map[string]interface{}{}},
		{name: "null", args: "null", want: map[string]interface{}{}},
		{name: "array", args: `["x"]`, want: map[string]interface{}{}},
		{name: "string", args: `"x"`, want: map[string]interface{}{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := dto.Message{Role: "assistant"}
			msg.SetToolCalls([]dto.ToolCallRequest{
				{
					ID:   "call_1",
					Type: "function",
					Function: dto.FunctionRequest{
						Name:      "lookup",
						Arguments: tt.args,
					},
				},
			})

			resp := ResponseOpenAI2Claude(&dto.OpenAITextResponse{
				Id:    "chatcmpl_1",
				Model: "gpt-test",
				Choices: []dto.OpenAITextResponseChoice{
					{Message: msg, FinishReason: "tool_calls"},
				},
			}, nil)

			require.Len(t, resp.Content, 1)
			assert.Equal(t, "tool_use", resp.Content[0].Type)
			assert.Equal(t, tt.want, resp.Content[0].Input)
		})
	}
}

func TestResponseOpenAI2ClaudeUsageCarriesOpenAIBillingUsage(t *testing.T) {
	resp := ResponseOpenAI2Claude(&dto.OpenAITextResponse{
		Id:    "chatcmpl_1",
		Model: "gpt-test",
		Choices: []dto.OpenAITextResponseChoice{
			{Message: dto.Message{Role: "assistant", Content: "hello"}, FinishReason: "stop"},
		},
		Usage: dto.Usage{
			PromptTokens:     11,
			CompletionTokens: 5,
			TotalTokens:      16,
		},
	}, nil)

	require.NotNil(t, resp.Usage)
	assert.Equal(t, 11, resp.Usage.InputTokens)
	assert.Equal(t, 5, resp.Usage.OutputTokens)
	require.NotNil(t, resp.Usage.BillingUsage)
	require.NotNil(t, resp.Usage.BillingUsage.OpenAIUsage)
	assert.Equal(t, dto.BillingUsageSourceOAIChat, resp.Usage.BillingUsage.Source)
	assert.Equal(t, dto.BillingUsageSemanticOpenAI, resp.Usage.BillingUsage.Semantic)
	assert.Equal(t, 11, resp.Usage.BillingUsage.OpenAIUsage.PromptTokens)
	assert.Equal(t, 5, resp.Usage.BillingUsage.OpenAIUsage.CompletionTokens)
	assert.Equal(t, 16, resp.Usage.BillingUsage.OpenAIUsage.TotalTokens)
	assert.Nil(t, resp.Usage.BillingUsage.OpenAIUsage.BillingUsage)
}

func TestBuildClaudeUsageFromOpenAICacheWriteUsage(t *testing.T) {
	usage := buildClaudeUsageFromOpenAIUsage(&dto.Usage{
		PromptTokens:     3619,
		CompletionTokens: 36,
		TotalTokens:      3655,
		PromptTokensDetails: dto.InputTokenDetails{
			CachedTokens:     2921,
			CacheWriteTokens: 3616,
		},
	})

	require.NotNil(t, usage)
	// Claude semantics reports input_tokens excluding cache read/write; the
	// overlapping unadjusted prefixes drive the remainder negative, clamp to 0.
	assert.Equal(t, 0, usage.InputTokens)
	assert.Equal(t, 2921, usage.CacheReadInputTokens)
	assert.Equal(t, 3616, usage.CacheCreationInputTokens)
	assert.Equal(t, 36, usage.OutputTokens)
	require.NotNil(t, usage.BillingUsage)
	require.NotNil(t, usage.BillingUsage.OpenAIUsage)
	assert.Equal(t, dto.BillingUsageSemanticOpenAI, usage.BillingUsage.Semantic)
	assert.Equal(t, 3616, usage.BillingUsage.OpenAIUsage.PromptTokensDetails.CacheWriteTokens)
}

func TestStreamResponseOpenAI2ClaudeClosesTextThinkingAndToolBlocks(t *testing.T) {
	info := &relaycommon.RelayInfo{
		ClaudeConvertInfo: &relaycommon.ClaudeConvertInfo{
			LastMessagesType: relaycommon.LastMessageTypeNone,
		},
	}

	info.SendResponseCount = 1
	textResponses := StreamResponseOpenAI2Claude(&dto.ChatCompletionsStreamResponse{
		Id:    "chatcmpl_1",
		Model: "gpt-test",
		Choices: []dto.ChatCompletionsStreamResponseChoice{
			{
				Delta: dto.ChatCompletionsStreamResponseChoiceDelta{
					Content: ptr("hello"),
				},
			},
		},
	}, info)
	require.Len(t, textResponses, 3)
	assert.Equal(t, "message_start", textResponses[0].Type)
	assert.Equal(t, "content_block_start", textResponses[1].Type)
	assert.Equal(t, 0, textResponses[1].GetIndex())
	assert.Equal(t, "content_block_delta", textResponses[2].Type)

	info.SendResponseCount = 2
	thinkingResponses := StreamResponseOpenAI2Claude(&dto.ChatCompletionsStreamResponse{
		Id:    "chatcmpl_1",
		Model: "gpt-test",
		Choices: []dto.ChatCompletionsStreamResponseChoice{
			{
				Delta: dto.ChatCompletionsStreamResponseChoiceDelta{
					ReasoningContent: ptr("thinking"),
				},
			},
		},
	}, info)
	require.Len(t, thinkingResponses, 3)
	assert.Equal(t, "content_block_stop", thinkingResponses[0].Type)
	assert.Equal(t, 0, thinkingResponses[0].GetIndex())
	assert.Equal(t, "content_block_start", thinkingResponses[1].Type)
	assert.Equal(t, 1, thinkingResponses[1].GetIndex())
	assert.Equal(t, "thinking", thinkingResponses[1].ContentBlock.Type)
	assert.Equal(t, "content_block_delta", thinkingResponses[2].Type)

	info.SendResponseCount = 3
	toolResponses := StreamResponseOpenAI2Claude(&dto.ChatCompletionsStreamResponse{
		Id:    "chatcmpl_1",
		Model: "gpt-test",
		Choices: []dto.ChatCompletionsStreamResponseChoice{
			{
				Delta: dto.ChatCompletionsStreamResponseChoiceDelta{
					ToolCalls: []dto.ToolCallResponse{
						{
							Index: ptr(0),
							ID:    "call_1",
							Type:  "function",
							Function: dto.FunctionResponse{
								Name:      "lookup",
								Arguments: `{"q":"x"}`,
							},
						},
					},
				},
			},
		},
	}, info)
	require.Len(t, toolResponses, 3)
	assert.Equal(t, "content_block_stop", toolResponses[0].Type)
	assert.Equal(t, 1, toolResponses[0].GetIndex())
	assert.Equal(t, "content_block_start", toolResponses[1].Type)
	assert.Equal(t, 2, toolResponses[1].GetIndex())
	assert.Equal(t, "tool_use", toolResponses[1].ContentBlock.Type)
	assert.Equal(t, "content_block_delta", toolResponses[2].Type)

	info.SendResponseCount = 4
	finishResponses := StreamResponseOpenAI2Claude(&dto.ChatCompletionsStreamResponse{
		Id:    "chatcmpl_1",
		Model: "gpt-test",
		Choices: []dto.ChatCompletionsStreamResponseChoice{
			{FinishReason: ptr("tool_calls")},
		},
		Usage: &dto.Usage{
			PromptTokens:     7,
			CompletionTokens: 3,
			TotalTokens:      10,
		},
	}, info)
	require.Len(t, finishResponses, 3)
	assert.Equal(t, "content_block_stop", finishResponses[0].Type)
	assert.Equal(t, 2, finishResponses[0].GetIndex())
	assert.Equal(t, "message_delta", finishResponses[1].Type)
	assert.Equal(t, "tool_use", *finishResponses[1].Delta.StopReason)
	require.NotNil(t, finishResponses[1].Usage)
	require.NotNil(t, finishResponses[1].Usage.BillingUsage)
	require.NotNil(t, finishResponses[1].Usage.BillingUsage.OpenAIUsage)
	assert.Equal(t, 7, finishResponses[1].Usage.BillingUsage.OpenAIUsage.PromptTokens)
	assert.Equal(t, 3, finishResponses[1].Usage.BillingUsage.OpenAIUsage.CompletionTokens)
	assert.Equal(t, "message_stop", finishResponses[2].Type)
}

func TestStreamResponseOpenAI2ClaudeToolCallsFinishReasonDeferredUsage(t *testing.T) {
	// Reproduces the minimax-m3 streaming shape: the tool_calls arrive in the SAME
	// chunk as finish_reason="tool_calls" with NO usage, and usage is sent in a
	// later usage-only chunk. Previously the converter hit an early return that
	// dropped delta.tool_calls, so the client saw stop_reason="tool_use" with no
	// tool_use content block ("malformed tool call"). The tool_use block must now
	// be emitted and only the closing deferred until usage arrives.
	info := &relaycommon.RelayInfo{
		ClaudeConvertInfo: &relaycommon.ClaudeConvertInfo{
			LastMessagesType: relaycommon.LastMessageTypeNone,
		},
	}

	// chunk 1: message_start only
	info.SendResponseCount = 1
	start := StreamResponseOpenAI2Claude(&dto.ChatCompletionsStreamResponse{
		Id:    "chatcmpl_1",
		Model: "minimax-m3",
		Choices: []dto.ChatCompletionsStreamResponseChoice{
			{Delta: dto.ChatCompletionsStreamResponseChoiceDelta{}},
		},
	}, info)
	require.Len(t, start, 1)
	assert.Equal(t, "message_start", start[0].Type)

	// chunk 2: tool_calls + finish_reason="tool_calls", NO usage (the bug trigger)
	info.SendResponseCount = 2
	toolFinish := StreamResponseOpenAI2Claude(&dto.ChatCompletionsStreamResponse{
		Id:    "chatcmpl_1",
		Model: "minimax-m3",
		Choices: []dto.ChatCompletionsStreamResponseChoice{
			{
				Delta: dto.ChatCompletionsStreamResponseChoiceDelta{
					ToolCalls: []dto.ToolCallResponse{
						{
							Index: ptr(0),
							ID:    "call_1",
							Type:  "function",
							Function: dto.FunctionResponse{
								Name:      "get_weather",
								Arguments: `{"city":"Beijing"}`,
							},
						},
					},
				},
				FinishReason: ptr("tool_calls"),
			},
		},
	}, info)
	require.NotEmpty(t, toolFinish, "tool_calls must be converted even when finish_reason has no usage yet")
	assert.Equal(t, "content_block_start", toolFinish[0].Type)
	assert.Equal(t, "tool_use", toolFinish[0].ContentBlock.Type)
	assert.Equal(t, "get_weather", toolFinish[0].ContentBlock.Name)
	var sawInputDelta bool
	for _, r := range toolFinish {
		assert.NotEqual(t, "message_stop", r.Type, "must defer message_stop until usage arrives")
		if r.Type == "content_block_delta" && r.Delta != nil && r.Delta.Type == "input_json_delta" {
			sawInputDelta = true
			require.NotNil(t, r.Delta.PartialJson)
			assert.Equal(t, `{"city":"Beijing"}`, *r.Delta.PartialJson)
		}
	}
	assert.True(t, sawInputDelta, "expected an input_json_delta for the tool_use block")

	// chunk 3: usage-only chunk closes the tool block and the message
	info.SendResponseCount = 3
	done := StreamResponseOpenAI2Claude(&dto.ChatCompletionsStreamResponse{
		Id:    "chatcmpl_1",
		Model: "minimax-m3",
		Choices: []dto.ChatCompletionsStreamResponseChoice{},
		Usage: &dto.Usage{
			PromptTokens:     7,
			CompletionTokens: 3,
			TotalTokens:      10,
		},
	}, info)
	require.Len(t, done, 3)
	assert.Equal(t, "content_block_stop", done[0].Type)
	assert.Equal(t, "message_delta", done[1].Type)
	require.NotNil(t, done[1].Delta)
	require.NotNil(t, done[1].Delta.StopReason)
	assert.Equal(t, "tool_use", *done[1].Delta.StopReason)
	assert.Equal(t, "message_stop", done[2].Type)
}

func TestStreamResponseOpenAI2ClaudeParallelToolCallsDeferredUsage(t *testing.T) {
	// Two parallel tool_calls arrive in the SAME finish_reason chunk with no usage,
	// then a usage-only chunk closes the stream. Both tool_use blocks must be emitted
	// and BOTH closed (the stopOpenBlocks Tools loop must cover every open block).
	info := &relaycommon.RelayInfo{
		ClaudeConvertInfo: &relaycommon.ClaudeConvertInfo{LastMessagesType: relaycommon.LastMessageTypeNone},
	}

	info.SendResponseCount = 1
	start := StreamResponseOpenAI2Claude(&dto.ChatCompletionsStreamResponse{
		Id: "c1", Model: "minimax-m3",
		Choices: []dto.ChatCompletionsStreamResponseChoice{{Delta: dto.ChatCompletionsStreamResponseChoiceDelta{}}},
	}, info)
	require.Len(t, start, 1)

	info.SendResponseCount = 2
	tc := StreamResponseOpenAI2Claude(&dto.ChatCompletionsStreamResponse{
		Id: "c1", Model: "minimax-m3",
		Choices: []dto.ChatCompletionsStreamResponseChoice{{
			Delta: dto.ChatCompletionsStreamResponseChoiceDelta{ToolCalls: []dto.ToolCallResponse{
				{Index: ptr(0), ID: "call_a", Type: "function", Function: dto.FunctionResponse{Name: "alpha", Arguments: `{"x":1}`}},
				{Index: ptr(1), ID: "call_b", Type: "function", Function: dto.FunctionResponse{Name: "beta", Arguments: `{"y":2}`}},
			}},
			FinishReason: ptr("tool_calls"),
		}},
	}, info)
	var starts []int
	var inputDeltas int
	for _, r := range tc {
		assert.NotEqual(t, "message_stop", r.Type, "must defer close until usage arrives")
		if r.Type == "content_block_start" && r.ContentBlock != nil && r.ContentBlock.Type == "tool_use" {
			starts = append(starts, r.GetIndex())
		}
		if r.Type == "content_block_delta" && r.Delta != nil && r.Delta.Type == "input_json_delta" {
			inputDeltas++
		}
	}
	assert.Equal(t, []int{0, 1}, starts)
	assert.Equal(t, 2, inputDeltas)

	info.SendResponseCount = 3
	done := StreamResponseOpenAI2Claude(&dto.ChatCompletionsStreamResponse{
		Id: "c1", Model: "minimax-m3",
		Choices: []dto.ChatCompletionsStreamResponseChoice{},
		Usage:  &dto.Usage{PromptTokens: 1, CompletionTokens: 2, TotalTokens: 3},
	}, info)
	var stops []int
	for _, r := range done {
		if r.Type == "content_block_stop" {
			stops = append(stops, r.GetIndex())
		}
	}
	assert.Equal(t, []int{0, 1}, stops)
	require.GreaterOrEqual(t, len(done), 3)
	assert.Equal(t, "message_delta", done[len(done)-2].Type)
	require.NotNil(t, done[len(done)-2].Delta)
	require.NotNil(t, done[len(done)-2].Delta.StopReason)
	assert.Equal(t, "tool_use", *done[len(done)-2].Delta.StopReason)
	assert.Equal(t, "message_stop", done[len(done)-1].Type)
}

func TestStreamResponseOpenAI2ClaudeDeferredFinishPreservesTrailingText(t *testing.T) {
	// A trailing text delta arriving in the finish_reason chunk (no usage) must not
	// be dropped. Previously this content was lost to the early deferral return.
	info := &relaycommon.RelayInfo{
		ClaudeConvertInfo: &relaycommon.ClaudeConvertInfo{LastMessagesType: relaycommon.LastMessageTypeNone},
	}

	info.SendResponseCount = 1
	StreamResponseOpenAI2Claude(&dto.ChatCompletionsStreamResponse{
		Id: "c1", Model: "m",
		Choices: []dto.ChatCompletionsStreamResponseChoice{{Delta: dto.ChatCompletionsStreamResponseChoiceDelta{Content: ptr("hello")}}},
	}, info)

	info.SendResponseCount = 2
	tc := StreamResponseOpenAI2Claude(&dto.ChatCompletionsStreamResponse{
		Id: "c1", Model: "m",
		Choices: []dto.ChatCompletionsStreamResponseChoice{{
			Delta:        dto.ChatCompletionsStreamResponseChoiceDelta{Content: ptr(" world")},
			FinishReason: ptr("stop"),
		}},
	}, info)
	var sawWorld bool
	for _, r := range tc {
		assert.NotEqual(t, "message_stop", r.Type)
		if r.Type == "content_block_delta" && r.Delta != nil && r.Delta.Type == "text_delta" &&
			r.Delta.Text != nil && *r.Delta.Text == " world" {
			sawWorld = true
		}
	}
	assert.True(t, sawWorld, "trailing text delta in the finish chunk must be preserved")

	info.SendResponseCount = 3
	done := StreamResponseOpenAI2Claude(&dto.ChatCompletionsStreamResponse{
		Id: "c1", Model: "m",
		Choices: []dto.ChatCompletionsStreamResponseChoice{},
		Usage:  &dto.Usage{PromptTokens: 1, CompletionTokens: 1, TotalTokens: 2},
	}, info)
	require.GreaterOrEqual(t, len(done), 3)
	assert.Equal(t, "content_block_stop", done[0].Type)
	assert.Equal(t, "message_delta", done[len(done)-2].Type)
	require.NotNil(t, done[len(done)-2].Delta.StopReason)
	assert.Equal(t, "end_turn", *done[len(done)-2].Delta.StopReason)
	assert.Equal(t, "message_stop", done[len(done)-1].Type)
}

func TestStreamResponseOpenAI2ClaudeDeferredFinishPreservesTrailingReasoning(t *testing.T) {
	// A trailing reasoning delta arriving in the finish_reason chunk (no usage) must
	// not be dropped either.
	info := &relaycommon.RelayInfo{
		ClaudeConvertInfo: &relaycommon.ClaudeConvertInfo{LastMessagesType: relaycommon.LastMessageTypeNone},
	}

	info.SendResponseCount = 1
	StreamResponseOpenAI2Claude(&dto.ChatCompletionsStreamResponse{
		Id: "c1", Model: "m",
		Choices: []dto.ChatCompletionsStreamResponseChoice{{Delta: dto.ChatCompletionsStreamResponseChoiceDelta{ReasoningContent: ptr("step1")}}},
	}, info)

	info.SendResponseCount = 2
	tc := StreamResponseOpenAI2Claude(&dto.ChatCompletionsStreamResponse{
		Id: "c1", Model: "m",
		Choices: []dto.ChatCompletionsStreamResponseChoice{{
			Delta:        dto.ChatCompletionsStreamResponseChoiceDelta{ReasoningContent: ptr(" step2")},
			FinishReason: ptr("stop"),
		}},
	}, info)
	var sawStep2 bool
	for _, r := range tc {
		assert.NotEqual(t, "message_stop", r.Type)
		if r.Type == "content_block_delta" && r.Delta != nil && r.Delta.Type == "thinking_delta" &&
			r.Delta.Thinking != nil && *r.Delta.Thinking == " step2" {
			sawStep2 = true
		}
	}
	assert.True(t, sawStep2, "trailing reasoning delta in the finish chunk must be preserved")
}

func TestStreamResponseOpenAI2ClaudeSingleChunkToolCall(t *testing.T) {
	// Some upstreams return the entire tool call in the first (message_start) chunk
	// together with finish_reason and usage. Exercises the SendResponseCount==1
	// IsToolCall branch plus the in-chunk finish close.
	info := &relaycommon.RelayInfo{
		ClaudeConvertInfo: &relaycommon.ClaudeConvertInfo{LastMessagesType: relaycommon.LastMessageTypeNone},
	}
	info.SendResponseCount = 1
	resp := StreamResponseOpenAI2Claude(&dto.ChatCompletionsStreamResponse{
		Id: "c1", Model: "m",
		Choices: []dto.ChatCompletionsStreamResponseChoice{{
			Delta: dto.ChatCompletionsStreamResponseChoiceDelta{ToolCalls: []dto.ToolCallResponse{
				{Index: ptr(0), ID: "call_1", Type: "function", Function: dto.FunctionResponse{Name: "get_weather", Arguments: `{"city":"Beijing"}`}},
			}},
			FinishReason: ptr("tool_calls"),
		}},
		Usage: &dto.Usage{PromptTokens: 5, CompletionTokens: 2, TotalTokens: 7},
	}, info)
	require.GreaterOrEqual(t, len(resp), 5)
	assert.Equal(t, "message_start", resp[0].Type)
	assert.Equal(t, "content_block_start", resp[1].Type)
	require.NotNil(t, resp[1].ContentBlock)
	assert.Equal(t, "tool_use", resp[1].ContentBlock.Type)
	assert.Equal(t, "get_weather", resp[1].ContentBlock.Name)
	var sawInput bool
	for _, r := range resp {
		if r.Type == "content_block_delta" && r.Delta != nil && r.Delta.Type == "input_json_delta" {
			sawInput = true
		}
	}
	assert.True(t, sawInput)
	assert.Equal(t, "message_delta", resp[len(resp)-2].Type)
	require.NotNil(t, resp[len(resp)-2].Delta.StopReason)
	assert.Equal(t, "tool_use", *resp[len(resp)-2].Delta.StopReason)
	assert.Equal(t, "message_stop", resp[len(resp)-1].Type)
}

func TestStreamResponseOpenAI2ClaudeFinishWithCachedUsageClosesImmediately(t *testing.T) {
	// The finish_reason chunk carries no chunk-level usage, but
	// info.ClaudeConvertInfo.Usage was already populated from a prior chunk. The
	// closer must use the cached usage and close immediately instead of deferring.
	info := &relaycommon.RelayInfo{
		ClaudeConvertInfo: &relaycommon.ClaudeConvertInfo{
			LastMessagesType: relaycommon.LastMessageTypeNone,
			Usage:            &dto.Usage{PromptTokens: 9, CompletionTokens: 4, TotalTokens: 13},
		},
	}
	info.SendResponseCount = 1
	StreamResponseOpenAI2Claude(&dto.ChatCompletionsStreamResponse{
		Id: "c1", Model: "m",
		Choices: []dto.ChatCompletionsStreamResponseChoice{{Delta: dto.ChatCompletionsStreamResponseChoiceDelta{Content: ptr("hi")}}},
	}, info)
	info.SendResponseCount = 2
	resp := StreamResponseOpenAI2Claude(&dto.ChatCompletionsStreamResponse{
		Id: "c1", Model: "m",
		Choices: []dto.ChatCompletionsStreamResponseChoice{{
			Delta:        dto.ChatCompletionsStreamResponseChoiceDelta{Content: ptr("!")},
			FinishReason: ptr("stop"),
		}},
	}, info)
	var sawStop, sawMsgDelta, sawMsgStop bool
	for _, r := range resp {
		switch r.Type {
		case "content_block_stop":
			sawStop = true
		case "message_delta":
			sawMsgDelta = true
			require.NotNil(t, r.Delta.StopReason)
			assert.Equal(t, "end_turn", *r.Delta.StopReason)
			require.NotNil(t, r.Usage)
		case "message_stop":
			sawMsgStop = true
		}
	}
	assert.True(t, sawStop && sawMsgDelta && sawMsgStop, "must close immediately using cached usage rather than deferring")
	assert.True(t, info.ClaudeConvertInfo.Done)
}

func TestNormalizeCacheCreationSplit(t *testing.T) {
	cache5m, cache1h := NormalizeCacheCreationSplit(10, 3, 2)
	assert.Equal(t, 8, cache5m)
	assert.Equal(t, 2, cache1h)

	cache5m, cache1h = NormalizeCacheCreationSplit(3, 5, 1)
	assert.Equal(t, 5, cache5m)
	assert.Equal(t, 1, cache1h)
}

func ptr[T any](value T) *T {
	return &value
}
