package main

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/kunchenguid/gsh/internal/script/interpreter"
	"github.com/stretchr/testify/require"
)

type middlewareContextProvider struct {
	fail     bool
	err      error
	requests []interpreter.ChatRequest
}

func (p *middlewareContextProvider) Name() string { return "middleware-test" }
func (p *middlewareContextProvider) ChatCompletion(ctx context.Context, r interpreter.ChatRequest) (*interpreter.ChatResponse, error) {
	p.requests = append(p.requests, r)
	if p.err != nil {
		return nil, p.err
	}
	if p.fail {
		return &interpreter.ChatResponse{Content: "partial", FinishReason: "length"}, nil
	}
	return &interpreter.ChatResponse{Content: "Retained observations", FinishReason: "stop"}, nil
}

func TestDefaultAgentFailureRetainsDirectoryContextForRetry(t *testing.T) {
	for _, recoverable := range []bool{false, true} {
		t.Run(map[bool]string{false: "no_recovered_history", true: "recovered_history"}[recoverable], func(t *testing.T) {
			i := interpreter.New(nil)
			source, err := defaultConfigFS.ReadFile("defaults/middleware/agent.gsh")
			require.NoError(t, err)
			p := &middlewareContextProvider{err: errors.New("offline")}
			model := &interpreter.ModelValue{Name: "test", Provider: p, Config: map[string]interpreter.Value{}}
			if recoverable {
				model.Config["contextWindow"] = &interpreter.NumberValue{Value: 16000}
			}
			i.SetVariable("exampleModel", model)
			_, err = i.EvalString("gsh.models.workhorse = exampleModel", nil)
			require.NoError(t, err)
			_, err = i.EvalString(string(source), nil)
			require.NoError(t, err)
			i.SetVariable("__lastKnownDirectory", &interpreter.StringValue{Value: "old-directory"})
			i.SetVariable("__conversation", &interpreter.ConversationValue{Messages: []interpreter.ChatMessage{{Role: "user", Content: "Previous task in old-directory"}}})
			i.EmitEvent("command.input", interpreter.InterfaceToValue(map[string]interface{}{"input": "# first attempt"}))
			p.err = nil
			i.EmitEvent("command.input", interpreter.InterfaceToValue(map[string]interface{}{"input": "# retry"}))
			require.Len(t, p.requests, 2)
			var directoryMessages int
			for _, message := range p.requests[1].Messages {
				if strings.Contains(message.Content, "<current_directory>") {
					directoryMessages++
				}
			}
			require.Equal(t, 1, directoryMessages, "a failed request must not lose the directory update or duplicate recovered context")
		})
	}
}
func (p *middlewareContextProvider) StreamingChatCompletion(ctx context.Context, r interpreter.ChatRequest, c *interpreter.StreamCallbacks) (*interpreter.ChatResponse, error) {
	return p.ChatCompletion(ctx, r)
}

func TestDefaultAgentManualCompaction(t *testing.T) {
	for _, fail := range []bool{false, true} {
		t.Run(map[bool]string{false: "success", true: "failure"}[fail], func(t *testing.T) {
			i := interpreter.New(nil)
			source, err := defaultConfigFS.ReadFile("defaults/middleware/agent.gsh")
			require.NoError(t, err)
			model := &interpreter.ModelValue{Name: "test", Provider: &middlewareContextProvider{fail: fail}, Config: map[string]interpreter.Value{"contextWindow": &interpreter.NumberValue{Value: 16000}}}
			i.SetVariable("exampleModel", model)
			_, err = i.EvalString("gsh.models.workhorse = exampleModel", nil)
			require.NoError(t, err)
			_, err = i.EvalString(string(source), nil)
			require.NoError(t, err)
			original := &interpreter.ConversationValue{Messages: []interpreter.ChatMessage{{Role: "user", Content: "Original task"}, {Role: "assistant", Content: strings.Repeat("old findings", 1000)}}}
			for n := 0; n < 7; n++ {
				original.Messages = append(original.Messages, interpreter.ChatMessage{Role: "user", Content: "Continue"})
			}
			i.SetVariable("__conversation", original)
			handled := i.EmitEvent("command.input", interpreter.InterfaceToValue(map[string]interface{}{"input": "# /compact"}))
			require.IsType(t, &interpreter.ObjectValue{}, handled)
			require.True(t, handled.(*interpreter.ObjectValue).GetPropertyValue("handled").(*interpreter.BoolValue).Value)
			conversation := i.GetVariables()["__conversation"].(*interpreter.ConversationValue)
			if fail {
				require.Same(t, original, conversation)
			} else {
				require.Equal(t, 1, conversation.CompactionCount)
				require.NotEmpty(t, conversation.Summary)
			}
			require.Empty(t, original.Summary)
		})
	}
}
