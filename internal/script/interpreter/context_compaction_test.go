package interpreter

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/kunchenguid/gsh/internal/acp"
	"github.com/stretchr/testify/require"
)

type contextTestProvider struct {
	requests []ChatRequest
	respond  func(context.Context, ChatRequest) (*ChatResponse, error)
	streams  int
}

func (p *contextTestProvider) Name() string { return "context-test" }
func (p *contextTestProvider) ChatCompletion(ctx context.Context, r ChatRequest) (*ChatResponse, error) {
	p.requests = append(p.requests, r)
	if p.respond != nil {
		return p.respond(ctx, r)
	}
	return &ChatResponse{Content: "Preserved findings and unfinished work.", FinishReason: "stop", Usage: &ChatUsage{PromptTokens: 10, CompletionTokens: 5}}, nil
}
func (p *contextTestProvider) StreamingChatCompletion(ctx context.Context, r ChatRequest, c *StreamCallbacks) (*ChatResponse, error) {
	p.streams++
	response, err := p.ChatCompletion(ctx, r)
	if err == nil && c != nil && c.OnContent != nil {
		c.OnContent(response.Content)
	}
	return response, err
}
func contextTestAgent(p ModelProvider) *AgentValue {
	return &AgentValue{Name: "worker", Config: map[string]Value{
		"model":        &ModelValue{Name: "test", Provider: p, Config: map[string]Value{}},
		"systemPrompt": &StringValue{Value: "Follow the user's task."},
		"context":      InterfaceToValue(map[string]interface{}{"maxTokens": 6000, "reserveTokens": 1024, "summaryTokens": 256, "keepRecentMessages": 2}),
	}}
}
func contextTestHistory() *ConversationValue {
	return &ConversationValue{Messages: []ChatMessage{
		{Role: "user", Content: "Original task: diagnose the service without deploying."},
		{Role: "assistant", ToolCalls: []ChatToolCall{{ID: "old", Name: "read", Arguments: map[string]interface{}{"path": "log"}}}},
		{Role: "tool", ToolCallID: "old", Name: "read", Content: strings.Repeat("old evidence\n", 900)},
		{Role: "assistant", Content: "The service needs a configuration correction."},
		{Role: "user", Content: "Latest constraint: preserve the data."},
		{Role: "assistant", ToolCalls: []ChatToolCall{{ID: "new", Name: "read"}}},
		{Role: "tool", ToolCallID: "new", Name: "read", Content: "Recent evidence."},
	}}
}

func TestContextCompactionPreservesTaskAndToolGroups(t *testing.T) {
	p := &contextTestProvider{}
	a := contextTestAgent(p)
	original := contextTestHistory()
	result, err := New(nil).CompactConversation(context.Background(), original, a)
	require.NoError(t, err)
	require.NotEmpty(t, result.Summary)
	require.Equal(t, 1, result.CompactionCount)
	require.Empty(t, original.Summary)
	require.Len(t, original.Messages, 7)
	require.Equal(t, original.Messages[0], result.Messages[0])
	require.Equal(t, original.Messages[4:], result.Messages[1:])
	require.Greater(t, len(p.requests), 1, "oversized history must be summarized in bounded chunks")
	for _, r := range p.requests {
		require.Empty(t, r.Tools, "summarization must never invoke tools")
		size, err := estimateContextTokens(r.Messages, r.Tools)
		require.NoError(t, err)
		require.LessOrEqual(t, size+256, 6000)
	}
	model := a.Config["model"].(*ModelValue)
	require.NotContains(t, model.Config, "maxTokens", "request limits must not mutate shared models")
}

func TestContextAutomaticCompactionAndStreaming(t *testing.T) {
	for _, stream := range []bool{false, true} {
		t.Run(map[bool]string{false: "plain", true: "stream"}[stream], func(t *testing.T) {
			p := &contextTestProvider{}
			a := contextTestAgent(p)
			var inputTokens int
			result, err := New(nil).ExecuteAgentWithCallbacks(context.Background(), contextTestHistory(), a, stream, &AgentCallbacks{OnComplete: func(r acp.AgentResult) { inputTokens = r.Usage.PromptTokens }})
			require.NoError(t, err)
			conv := result.(*ConversationValue)
			require.Equal(t, 1, conv.CompactionCount)
			require.NotEmpty(t, conv.Summary)
			last := p.requests[len(p.requests)-1]
			size, err := estimateContextTokens(last.Messages, last.Tools)
			require.NoError(t, err)
			require.LessOrEqual(t, size+1024, 6000)
			require.Equal(t, 10*len(p.requests), inputTokens, "summary usage must count toward total usage")
			require.Equal(t, map[bool]int{false: 0, true: 1}[stream], p.streams, "summaries must not leak into response streaming")
		})
	}
}

func TestContextCompactionFailureIsAtomic(t *testing.T) {
	for _, mode := range []string{"error", "empty", "truncated", "tools", "oversized", "cancelled"} {
		t.Run(mode, func(t *testing.T) {
			p := &contextTestProvider{respond: func(ctx context.Context, r ChatRequest) (*ChatResponse, error) {
				switch mode {
				case "error":
					return nil, errors.New("provider unavailable")
				case "cancelled":
					return nil, context.Canceled
				case "empty":
					return &ChatResponse{FinishReason: "stop"}, nil
				case "truncated":
					return &ChatResponse{Content: "incomplete", FinishReason: "length"}, nil
				case "tools":
					return &ChatResponse{Content: "summary", ToolCalls: []ChatToolCall{{ID: "oops"}}}, nil
				default:
					return &ChatResponse{Content: strings.Repeat("x", 10000), FinishReason: "stop"}, nil
				}
			}}
			original := contextTestHistory()
			result, err := New(nil).CompactConversation(context.Background(), original, contextTestAgent(p))
			require.Error(t, err)
			require.Nil(t, result)
			require.Empty(t, original.Summary)
			require.Len(t, original.Messages, 7)
		})
	}
}

func TestContextProtectedContentCannotFit(t *testing.T) {
	p := &contextTestProvider{}
	original := &ConversationValue{Messages: []ChatMessage{{Role: "user", Content: strings.Repeat("x", 9000)}}}
	_, err := New(nil).ExecuteAgent(context.Background(), original, contextTestAgent(p), false)
	require.ErrorContains(t, err, "context budget")
	require.Empty(t, p.requests, "never submit an oversized protected request")
}

func TestContextDisabledDoesNotSummarize(t *testing.T) {
	p := &contextTestProvider{}
	a := contextTestAgent(p)
	delete(a.Config, "context")
	result, err := New(nil).ExecuteAgent(context.Background(), contextTestHistory(), a, false)
	require.NoError(t, err)
	require.Len(t, p.requests, 1)
	require.Empty(t, result.(*ConversationValue).Summary)
}

func TestContextConfigurationValidation(t *testing.T) {
	for _, config := range []string{`{maxTokens: 0}`, `{maxTokens: 3.5}`, `{maxTokens: "6000"}`, `{maxTokens: 6000, reserveTokens: 6000}`, `{maxTokens: 6000, keepRecentMessages: 0}`, `{maxTokens: 6000, unknown: 1}`} {
		_, err := New(nil).EvalString(`model exampleModel { provider: "openai", apiKey: "ollama", baseURL: "http://localhost:11434/v1", model: "gpt-oss:20b" }
agent worker { model: exampleModel, context: `+config+` }`, nil)
		require.Error(t, err, config)
	}
}

func TestContextManualCompactionScriptAndPersistence(t *testing.T) {
	i := New(nil)
	p := &contextTestProvider{}
	a := contextTestAgent(p)
	i.globalEnv.Set("worker", a)
	i.globalEnv.Set("conversation", contextTestHistory())
	_, err := i.EvalString(`compacted = gsh.compact(conversation, worker)
continued = compacted | "Continue with the same restrictions"
savedSummary = continued.summary
savedCount = continued.compactionCount`, nil)
	require.NoError(t, err)
	v, _ := i.globalEnv.Get("savedSummary")
	require.NotEmpty(t, v.(*StringValue).Value)
	v, _ = i.globalEnv.Get("savedCount")
	require.Equal(t, float64(1), v.(*NumberValue).Value)
}

func TestContextModelWindowEnablesDefaultPolicy(t *testing.T) {
	p := &contextTestProvider{}
	a := contextTestAgent(p)
	delete(a.Config, "context")
	a.Config["model"].(*ModelValue).Config["contextWindow"] = &NumberValue{Value: 8000}
	// Many completed messages leave room for the default recent-message policy.
	history := contextTestHistory()
	for n := 0; n < 10; n++ {
		history.Messages = append(history.Messages, ChatMessage{Role: "user", Content: "Continue"}, ChatMessage{Role: "assistant", Content: "Working"})
	}
	result, err := New(nil).ExecuteAgent(context.Background(), history, a, false)
	require.NoError(t, err)
	require.NotEmpty(t, result.(*ConversationValue).Summary)
}

func TestContextCompactionKeepsEntireParallelToolBatch(t *testing.T) {
	messages := []ChatMessage{
		{Role: "user", Content: "Task"}, {Role: "assistant", Content: "Old finding"},
		{Role: "assistant", ToolCalls: []ChatToolCall{{ID: "a"}, {ID: "b"}}},
		{Role: "tool", ToolCallID: "a", Content: "A"}, {Role: "tool", ToolCallID: "b", Content: "B"},
	}
	older, retained, err := compactionParts(messages, 1)
	require.NoError(t, err)
	require.Equal(t, messages[1:2], older)
	require.Equal(t, append([]ChatMessage{messages[0]}, messages[2:]...), retained)
	messages[4].ToolCallID = "missing"
	_, _, err = compactionParts(messages, 1)
	require.Error(t, err)
}

func TestContextToolSchemasCountTowardBudget(t *testing.T) {
	p := &contextTestProvider{}
	_, err := New(nil).ExecuteAgentWithCallbacks(context.Background(), contextTestHistory(), contextTestAgent(p), false,
		&AgentCallbacks{Tools: []ChatTool{{Name: "big", Description: strings.Repeat("schema", 1200)}}})
	require.ErrorContains(t, err, "context budget")
	require.Empty(t, p.requests)
}

func TestContextCannotBypassBudgetWithExtraBody(t *testing.T) {
	for _, field := range []string{"messages", "tools", "max_tokens", "max_completion_tokens", "model", "functions", "function_call", "tool_choice"} {
		p := &contextTestProvider{}
		a := contextTestAgent(p)
		a.Config["model"].(*ModelValue).Config["extraBody"] = InterfaceToValue(map[string]interface{}{field: 90000})
		_, err := New(nil).ExecuteAgent(context.Background(), contextTestHistory(), a, false)
		require.ErrorContains(t, err, "extraBody", field)
		require.Empty(t, p.requests)
	}
}

func TestContextOversizedToolBatchStopsWithoutReplaying(t *testing.T) {
	p := &contextTestProvider{}
	executions := 0
	a := contextTestAgent(p)
	a.Config["tools"] = &ArrayValue{Elements: []Value{&NativeToolValue{Name: "read", Invoke: func(args map[string]interface{}) (interface{}, error) {
		executions++
		return strings.Repeat("evidence", 2000), nil
	}}}}
	p.respond = func(ctx context.Context, r ChatRequest) (*ChatResponse, error) {
		if r.Model.Config["maxTokens"].(*NumberValue).Value == 256 {
			return &ChatResponse{Content: "Observed the log; do not read it again.", FinishReason: "stop"}, nil
		}
		if executions == 0 {
			return &ChatResponse{ToolCalls: []ChatToolCall{{ID: "read1", Name: "read"}, {ID: "read2", Name: "read"}}}, nil
		}
		return &ChatResponse{Content: "Finished", FinishReason: "stop"}, nil
	}
	// Retain one newest message: the entire current tool batch is protected and
	// cannot fit, so fail rather than split it or replay an executed operation.
	a.Config["context"].(*ObjectValue).Properties["keepRecentMessages"].Value = &NumberValue{Value: 1}
	result, err := New(nil).ExecuteAgent(context.Background(), &ConversationValue{Messages: []ChatMessage{{Role: "user", Content: "Read logs"}}}, a, false)
	require.ErrorContains(t, err, "context budget")
	require.Equal(t, 2, executions)
	require.Len(t, result.(*ConversationValue).Messages, 4)
}

func TestContextCompactsRepeatedlyWithinOneToolHeavyTurn(t *testing.T) {
	p := &contextTestProvider{}
	agent := contextTestAgent(p)
	executions := 0
	agent.Config["context"].(*ObjectValue).Properties["keepRecentMessages"].Value = &NumberValue{Value: 1}
	agent.Config["tools"] = &ArrayValue{Elements: []Value{&NativeToolValue{Name: "read", Invoke: func(args map[string]interface{}) (interface{}, error) {
		executions++
		return strings.Repeat("data ", 260), nil
	}}}}
	p.respond = func(ctx context.Context, r ChatRequest) (*ChatResponse, error) {
		if r.Model.Config["maxTokens"].(*NumberValue).Value == 256 {
			return &ChatResponse{Content: "Prior reads completed; continue with the next item.", FinishReason: "stop"}, nil
		}
		if executions < 8 {
			return &ChatResponse{ToolCalls: []ChatToolCall{{ID: fmt.Sprint(executions), Name: "read"}}}, nil
		}
		return &ChatResponse{Content: "Done", FinishReason: "stop"}, nil
	}
	value, err := New(nil).ExecuteAgent(context.Background(), &ConversationValue{Messages: []ChatMessage{{Role: "user", Content: "Read each item exactly once"}}}, agent, false)
	require.NoError(t, err)
	require.Equal(t, 8, executions)
	require.GreaterOrEqual(t, value.(*ConversationValue).CompactionCount, 2)
	for _, request := range p.requests {
		size, err := estimateContextTokens(request.Messages, request.Tools)
		require.NoError(t, err)
		require.LessOrEqual(t, size+int(request.Model.Config["maxTokens"].(*NumberValue).Value), 6000)
	}
}

func TestContextUTF8AndLaterChunkFailurePreserveOriginal(t *testing.T) {
	p := &contextTestProvider{}
	calls := 0
	p.respond = func(ctx context.Context, r ChatRequest) (*ChatResponse, error) {
		calls++
		for _, m := range r.Messages {
			require.True(t, utf8.ValidString(m.Content))
			require.NotContains(t, m.Content, "\\ufffd")
		}
		if calls == 2 {
			return nil, errors.New("second chunk failed")
		}
		return &ChatResponse{Content: "已读的内容必须保留", FinishReason: "stop"}, nil
	}
	conv := contextTestHistory()
	conv.Summary = "Earlier summary"
	conv.Messages[2].Content = strings.Repeat("中文🙂\n", 1800)
	_, err := New(nil).CompactConversation(context.Background(), conv, contextTestAgent(p))
	require.ErrorContains(t, err, "second chunk failed")
	require.Equal(t, "Earlier summary", conv.Summary)
	require.Len(t, conv.Messages, 7)
}

func TestContextExplicitDisableAndCancellation(t *testing.T) {
	p := &contextTestProvider{}
	a := contextTestAgent(p)
	a.Config["context"] = &NullValue{}
	a.Config["model"].(*ModelValue).Config["contextWindow"] = &NumberValue{Value: 6000}
	_, err := New(nil).ExecuteAgent(context.Background(), contextTestHistory(), a, false)
	require.NoError(t, err)
	require.Len(t, p.requests, 1)
	p.requests = nil
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = New(nil).CompactConversation(ctx, contextTestHistory(), contextTestAgent(p))
	require.ErrorIs(t, err, context.Canceled)
	require.Empty(t, p.requests)
}

func TestContextCancelledSummaryReportsCancellation(t *testing.T) {
	p := &contextTestProvider{respond: func(ctx context.Context, r ChatRequest) (*ChatResponse, error) { return nil, context.Canceled }}
	var reason acp.StopReason
	_, err := New(nil).ExecuteAgentWithCallbacks(context.Background(), contextTestHistory(), contextTestAgent(p), false, &AgentCallbacks{OnComplete: func(r acp.AgentResult) { reason = r.StopReason }})
	require.ErrorIs(t, err, context.Canceled)
	require.Equal(t, acp.StopReasonCancelled, reason)
}

func TestContextFailureExposesRecoverableConversationToScript(t *testing.T) {
	i := New(nil)
	p := &contextTestProvider{respond: func(ctx context.Context, r ChatRequest) (*ChatResponse, error) { return nil, errors.New("offline") }}
	i.globalEnv.Set("worker", contextTestAgent(p))
	i.globalEnv.Set("conversation", contextTestHistory())
	_, err := i.EvalString(`recovered = null
failure = ""
try { result = conversation | worker } catch (e) { recovered = e.conversation; failure = e.message }`, nil)
	require.NoError(t, err)
	v, _ := i.globalEnv.Get("recovered")
	require.IsType(t, &ConversationValue{}, v)
	require.Len(t, v.(*ConversationValue).Messages, 7)
	v, _ = i.globalEnv.Get("failure")
	require.Contains(t, v.(*StringValue).Value, "offline")
}
