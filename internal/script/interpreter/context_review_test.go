package interpreter

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/kunchenguid/gsh/internal/acp"
	"github.com/stretchr/testify/require"
)

func TestContextManualNoopAtExactBudget(t *testing.T) {
	p := &contextTestProvider{}
	a := contextTestAgent(p)
	conv := &ConversationValue{Messages: []ChatMessage{{Role: "user", Content: "Nothing to compact"}}}
	size, err := estimateContextTokens(contextMessages(a, conv), nil)
	require.NoError(t, err)
	policy := &contextPolicy{maxTokens: size + 1024, reserveTokens: 1024, summaryTokens: 256, keepRecentMessages: 1}
	result, _, err := compactContext(context.Background(), conv, a, a.Config["model"].(*ModelValue), nil, policy, true)
	require.NoError(t, err)
	require.Same(t, conv, result)
	require.Empty(t, p.requests)
}

func TestContextRejectsTruncatedAgentResponses(t *testing.T) {
	for _, streaming := range []bool{false, true} {
		for _, withTool := range []bool{false, true} {
			t.Run(fmt.Sprintf("stream=%t/tool=%t", streaming, withTool), func(t *testing.T) {
				p := &contextTestProvider{respond: func(context.Context, ChatRequest) (*ChatResponse, error) {
					response := &ChatResponse{FinishReason: "length", Usage: &ChatUsage{PromptTokens: 10, CompletionTokens: 1024}}
					if withTool {
						response.Content = "Partial answer"
						response.ToolCalls = []ChatToolCall{{ID: "partial", Name: "write"}}
					}
					return response, nil
				}}
				a := contextTestAgent(p)
				a.Config["maxIterations"] = &NumberValue{Value: 1}
				calls := 0
				a.Config["tools"] = &ArrayValue{Elements: []Value{&NativeToolValue{Name: "write", Invoke: func(map[string]interface{}) (interface{}, error) {
					calls++
					return "written", nil
				}}}}
				conv := &ConversationValue{Messages: []ChatMessage{{Role: "user", Content: "Task"}}}
				var completed acp.AgentResult
				result, err := New(nil).ExecuteAgentWithCallbacks(context.Background(), conv, a, streaming, &AgentCallbacks{OnComplete: func(r acp.AgentResult) { completed = r }})
				require.ErrorContains(t, err, "reserveTokens")
				require.Equal(t, acp.StopReasonMaxTokens, completed.StopReason)
				require.Equal(t, 1024, completed.Usage.CompletionTokens)
				require.Zero(t, calls, "a truncated tool batch must never execute")
				require.Equal(t, conv.Messages, result.(*ConversationValue).Messages)
			})
		}
	}
}

func TestContextRejectsImpossibleSummaryBeforeCallingModel(t *testing.T) {
	p := &contextTestProvider{}
	a := contextTestAgent(p)
	conv := contextTestHistory()
	_, retained, _, err := compactionParts(conv.Messages, 2)
	require.NoError(t, err)
	tools := []ChatTool{{Name: "read", Description: strings.Repeat("schema", 1000)}}
	protected, err := estimateContextTokens(contextMessages(a, &ConversationValue{Messages: retained}), tools)
	require.NoError(t, err)
	policy := &contextPolicy{maxTokens: protected + 1024 + 8, reserveTokens: 1024, summaryTokens: 256, keepRecentMessages: 2}
	_, _, err = compactContext(context.Background(), conv, a, a.Config["model"].(*ModelValue), tools, policy, true)
	require.Error(t, err)
	require.Empty(t, p.requests, "even a one-byte memory plus its message framing cannot fit; do not charge for a doomed summary")
	require.Empty(t, conv.Summary)
}

func BenchmarkContextLargeHistory(b *testing.B) {
	conv := contextTestHistory()
	conv.Messages[2].Content = strings.Repeat("x", 1024*1024)
	b.ReportAllocs()
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		p := &contextTestProvider{}
		_, err := New(nil).CompactConversation(context.Background(), conv, contextTestAgent(p))
		if err == nil || !strings.Contains(err.Error(), "32 summary requests") || len(p.requests) != 32 {
			b.Fatalf("expected bounded failure after 32 summaries, got %v (%d calls)", err, len(p.requests))
		}
	}
}

func TestContextReviewLazyContextModel(t *testing.T) {
	i := New(nil)
	// REPL defaults already provide a model; user configuration replaces it later.
	i.sdkConfig.GetModels().Workhorse = contextTestAgent(&contextTestProvider{}).Config["model"].(*ModelValue)
	_, err := i.EvalString(`
agent worker { model: gsh.models.workhorse, context: { reserveTokens: 2048 } }
model exampleModel {
    provider: "openai", apiKey: "ollama",
    baseURL: "http://localhost:11434/v1", model: "gpt-oss:20b",
    contextWindow: 32768,
}
gsh.models.workhorse = exampleModel
`, nil)
	require.NoError(t, err, "the window belongs to a lazily resolved model, configured before execution")
}

func TestContextReviewSummaryChronology(t *testing.T) {
	p := &contextTestProvider{respond: func(context.Context, ChatRequest) (*ChatResponse, error) {
		return &ChatResponse{Content: "After the user requested the migration, it completed successfully. Do not run it again.", FinishReason: "stop"}, nil
	}}
	a := contextTestAgent(p)
	a.Config["context"].(*ObjectValue).Properties["keepRecentMessages"].Value = &NumberValue{Value: 1}
	conv := &ConversationValue{Messages: []ChatMessage{
		{Role: "user", Content: "Maintain the service."},
		{Role: "assistant", Content: "Ready."},
		{Role: "user", Content: "The migration is pending. Run it once, then inspect the logs."},
		{Role: "assistant", ToolCalls: []ChatToolCall{{ID: "migrate", Name: "migrate"}}},
		{Role: "tool", ToolCallID: "migrate", Content: "Migration completed successfully. " + strings.Repeat("log ", 1500)},
		{Role: "assistant", ToolCalls: []ChatToolCall{{ID: "logs", Name: "read"}}},
		{Role: "tool", ToolCallID: "logs", Content: "Logs clean."},
	}}
	compacted, err := New(nil).CompactConversation(context.Background(), conv, a)
	require.NoError(t, err)
	messages := contextMessages(a, compacted)
	requestIndex, completionIndex := -1, -1
	for index, message := range messages {
		if message.Content == conv.Messages[2].Content {
			requestIndex = index
		}
		if strings.Contains(message.Content, "After the user requested the migration") {
			completionIndex = index
		}
	}
	t.Logf("normal model input: migration request index=%d; later completion summary index=%d", requestIndex, completionIndex)
	require.Less(t, requestIndex, completionIndex, "the later completion must not precede the earlier pending-work request")

	// A new user turns the old pinned migration request into removable history.
	// Its later completion memory must still follow it in summary input.
	i := New(nil)
	i.globalEnv.Set("saved", compacted)
	_, err = i.EvalString(`continued = saved | "Report the migration outcome; do not run it again."`, nil)
	require.NoError(t, err)
	value, _ := i.globalEnv.Get("continued")
	continued := value.(*ConversationValue)
	require.Equal(t, compacted.SummaryIndex, continued.SummaryIndex)
	i.globalEnv.Set("worker", a)
	_, err = i.EvalString(`answered = saved | worker`, nil)
	require.NoError(t, err)
	answered, _ := i.globalEnv.Get("answered")
	require.Equal(t, compacted.SummaryIndex, answered.(*ConversationValue).SummaryIndex)
	lastRequest := p.requests[len(p.requests)-1].Messages
	require.Equal(t, conv.Messages[2].Content, lastRequest[2].Content)
	require.Contains(t, lastRequest[3].Content, compacted.Summary)
	continued.Messages = append(continued.Messages,
		ChatMessage{Role: "assistant", Content: strings.Repeat("log ", 1500)},
		ChatMessage{Role: "assistant", Content: "Recent observation"})
	start := len(p.requests)
	again, err := i.CompactConversation(context.Background(), continued, a)
	require.NoError(t, err)
	var transcript strings.Builder
	for _, request := range p.requests[start:] {
		var payload struct {
			Fragments []contextSummaryFragment `json:"transcript_fragments"`
		}
		require.NoError(t, json.Unmarshal([]byte(request.Messages[1].Content), &payload))
		for _, fragment := range payload.Fragments {
			transcript.WriteString(fragment.Text)
		}
	}
	source := transcript.String()
	pending := strings.Index(source, "The migration is pending")
	completed := strings.Index(source, "After the user requested the migration")
	require.NotEqual(t, -1, pending)
	require.Greater(t, completed, pending)
	require.Equal(t, 2, again.CompactionCount)
}

func TestContextDynamicModelValidatedAtExecution(t *testing.T) {
	i := New(nil)
	p := &contextTestProvider{}
	i.sdkConfig.GetModels().Workhorse = contextTestAgent(p).Config["model"].(*ModelValue)
	_, err := i.EvalString(`agent worker { model: gsh.models.workhorse, context: {reserveTokens: 2048} }`, nil)
	require.NoError(t, err)
	value, _ := i.globalEnv.Get("worker")
	a := value.(*AgentValue)
	conv := &ConversationValue{Messages: []ChatMessage{{Role: "user", Content: "Task"}}}
	_, err = i.ExecuteAgent(context.Background(), conv, a, false)
	require.ErrorContains(t, err, "context requires")
	require.Empty(t, p.requests)
	replacement := contextTestAgent(p).Config["model"].(*ModelValue)
	replacement.Config["contextWindow"] = &NumberValue{Value: 8000}
	i.sdkConfig.GetModels().Workhorse = replacement
	_, err = i.ExecuteAgent(context.Background(), conv, a, false)
	require.NoError(t, err)
	require.Equal(t, float64(2048), p.requests[0].Model.Config["maxTokens"].(*NumberValue).Value)
	replacement.Config["contextWindow"] = &NumberValue{Value: 2048}
	_, err = i.ExecuteAgent(context.Background(), conv, a, false)
	require.ErrorContains(t, err, "smaller than maxTokens")
	require.Len(t, p.requests, 1)
}
