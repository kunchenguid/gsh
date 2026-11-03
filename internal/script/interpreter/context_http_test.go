package interpreter

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestContextOpenAIWireLimits(t *testing.T) {
	for _, stream := range []bool{false, true} {
		t.Run(fmt.Sprint(stream), func(t *testing.T) {
			var requests []map[string]interface{}
			var requestsMu sync.Mutex
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var request map[string]interface{}
				if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
					http.Error(w, err.Error(), 400)
					return
				}
				requestsMu.Lock()
				requests = append(requests, request)
				requestsMu.Unlock()
				if request["stream"] == true {
					w.Header().Set("Content-Type", "text/event-stream")
					fmt.Fprint(w, "data: {\"choices\":[{\"index\":0,\"delta\":{\"content\":\"Final answer\"},\"finish_reason\":\"stop\"}]}\n\ndata: [DONE]\n\n")
					return
				}
				w.Header().Set("Content-Type", "application/json")
				fmt.Fprint(w, `{"choices":[{"message":{"role":"assistant","content":"Verified facts and remaining work."},"finish_reason":"stop"}]}`)
			}))
			defer server.Close()
			agent := contextTestAgent(NewOpenAIProvider())
			model := agent.Config["model"].(*ModelValue)
			model.Config = map[string]Value{"apiKey": &StringValue{Value: "ollama"}, "model": &StringValue{Value: "gpt-oss:20b"}, "baseURL": &StringValue{Value: server.URL}, "maxTokens": &NumberValue{Value: 8000}}
			var output strings.Builder
			_, err := New(nil).ExecuteAgentWithCallbacks(context.Background(), contextTestHistory(), agent, stream, &AgentCallbacks{OnChunk: func(chunk string) { output.WriteString(chunk) }})
			require.NoError(t, err)
			requestsMu.Lock()
			defer requestsMu.Unlock()
			require.Greater(t, len(requests), 1)
			for _, request := range requests[:len(requests)-1] {
				require.Equal(t, float64(256), request["max_tokens"])
				require.NotEqual(t, true, request["stream"])
				require.Empty(t, request["tools"])
			}
			require.Equal(t, float64(1024), requests[len(requests)-1]["max_tokens"])
			require.Equal(t, float64(8000), model.Config["maxTokens"].(*NumberValue).Value)
			if stream {
				require.Equal(t, "Final answer", output.String())
			}
		})
	}
}

func TestContextTruncatedStreamingToolIsNotExecuted(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, `data: {"choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"id":"partial","type":"function","function":{"name":"write","arguments":"{\"path\":"}}]},"finish_reason":"length"}]}`+"\n\ndata: [DONE]\n\n")
	}))
	defer server.Close()
	a := contextTestAgent(NewOpenAIProvider())
	a.Config["model"].(*ModelValue).Config = map[string]Value{"apiKey": &StringValue{Value: "ollama"}, "model": &StringValue{Value: "gpt-oss:20b"}, "baseURL": &StringValue{Value: server.URL}}
	a.Config["maxIterations"] = &NumberValue{Value: 1}
	calls := 0
	a.Config["tools"] = &ArrayValue{Elements: []Value{&NativeToolValue{Name: "write", Invoke: func(map[string]interface{}) (interface{}, error) { calls++; return "written", nil }}}}
	_, err := New(nil).ExecuteAgent(context.Background(), &ConversationValue{Messages: []ChatMessage{{Role: "user", Content: "Task"}}}, a, true)
	require.Zero(t, calls, "the provider received an unfinished JSON argument; do not execute it")
	require.ErrorContains(t, err, "reserveTokens")
}
