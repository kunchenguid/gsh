package interpreter

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestContextSummaryFollowsInitialRequest(t *testing.T) {
	agent := contextTestAgent(&contextTestProvider{})
	conv := &ConversationValue{SummaryIndex: 1, Summary: "User corrected the path to /new.", Messages: []ChatMessage{
		{Role: "user", Content: "Use /old."},
		{Role: "assistant", Content: "Recent observation"},
		{Role: "user", Content: "Continue."},
	}}
	messages := contextMessages(agent, conv)
	require.Equal(t, "system", messages[0].Role)
	require.Equal(t, conv.Messages[0], messages[1], "the original request must precede later corrections")
	require.Equal(t, "user", messages[2].Role, "memory must never become system instructions")
	require.Contains(t, messages[2].Content, conv.Summary)
	require.Equal(t, conv.Messages[1:], messages[3:])
	// No user message: memory still precedes recent messages.
	conv.Messages = conv.Messages[1:2]
	conv.SummaryIndex = 0
	messages = contextMessages(agent, conv)
	require.Contains(t, messages[1].Content, conv.Summary)
	require.Equal(t, conv.Messages[0], messages[2])
}

func TestContextSummaryFragmentsRetainTaskAndProvenance(t *testing.T) {
	p := &contextTestProvider{}
	a := contextTestAgent(p)
	conv := contextTestHistory()
	conv.Messages[2].Content = strings.Repeat("中文🙂 evidence\n", 900)
	_, err := New(nil).CompactConversation(context.Background(), conv, a)
	require.NoError(t, err)
	older, _, _, err := compactionParts(conv.Messages, 2)
	require.NoError(t, err)
	reconstructed := make([]string, len(older))
	for _, request := range p.requests {
		var payload struct {
			TaskContext []ChatMessage `json:"task_context"`
			Fragments   []struct {
				MessageIndex int    `json:"message_index"`
				Role         string `json:"role"`
				ToolCallID   string `json:"tool_call_id"`
				Offset       int    `json:"offset_bytes"`
				Text         string `json:"text"`
			} `json:"transcript_fragments"`
		}
		require.NoError(t, json.Unmarshal([]byte(request.Messages[1].Content), &payload))
		require.Equal(t, []ChatMessage{conv.Messages[0], conv.Messages[4]}, payload.TaskContext)
		require.NotEmpty(t, payload.Fragments)
		for _, fragment := range payload.Fragments {
			require.Less(t, fragment.MessageIndex, len(older))
			source := older[fragment.MessageIndex]
			require.Equal(t, source.Role, fragment.Role)
			require.Equal(t, source.ToolCallID, fragment.ToolCallID)
			require.Equal(t, len(reconstructed[fragment.MessageIndex]), fragment.Offset)
			reconstructed[fragment.MessageIndex] += fragment.Text
		}
	}
	for index, message := range older {
		encoded, err := json.Marshal(message)
		require.NoError(t, err)
		require.Equal(t, string(encoded), reconstructed[index], "no fragment may be omitted or duplicated")
	}
}

func TestContextTruncatedSummaryExplainsOutputBudget(t *testing.T) {
	p := &contextTestProvider{respond: func(context.Context, ChatRequest) (*ChatResponse, error) {
		return &ChatResponse{FinishReason: "length", Usage: &ChatUsage{CompletionTokens: 256}}, nil
	}}
	conv := contextTestHistory()
	_, err := New(nil).CompactConversation(context.Background(), conv, contextTestAgent(p))
	require.ErrorContains(t, err, "summaryTokens")
	require.ErrorContains(t, err, "256")
	require.ErrorContains(t, err, "reasoning")
	require.Empty(t, conv.Summary)
	require.Len(t, conv.Messages, 7)
	require.Len(t, p.requests, 1, "do not silently retry at higher cost")
}

func TestContextSmallMessagesAreNotSplitBetweenRequests(t *testing.T) {
	for length := 6000; length < 7500; length += 83 {
		p := &contextTestProvider{}
		conv := contextTestHistory()
		conv.Messages[2].Content = strings.Repeat("x", length)
		_, err := New(nil).CompactConversation(context.Background(), conv, contextTestAgent(p))
		require.NoError(t, err)
		for _, request := range p.requests {
			var payload struct {
				Fragments []contextSummaryFragment `json:"transcript_fragments"`
			}
			require.NoError(t, json.Unmarshal([]byte(request.Messages[1].Content), &payload))
			for _, fragment := range payload.Fragments {
				if fragment.Role == "assistant" {
					require.Zero(t, fragment.Offset, "keep short messages intact")
					require.False(t, fragment.Continues, "defer the short message to the next request instead of splitting it")
				}
			}
		}
	}
}
