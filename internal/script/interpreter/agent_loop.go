package interpreter

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/kunchenguid/gsh/internal/acp"
)

// DefaultMaxIterations is the default maximum number of tool call iterations
// if not specified in the agent config.
const DefaultMaxIterations = 100

// ExecuteAgent executes an agent with a conversation and returns the updated conversation.
// streaming parameter enables streaming responses.
// SDK events (agent.start, agent.end, etc.) are emitted via the interpreter's event manager.
// The user message is inferred from the conversation.
func (i *Interpreter) ExecuteAgent(
	ctx context.Context,
	conv *ConversationValue,
	agent *AgentValue,
	streaming bool,
) (Value, error) {
	return i.executeAgentInternal(ctx, conv, agent, streaming, nil)
}

// ExecuteAgentWithCallbacks executes an agent with callbacks and returns the updated conversation.
// streaming parameter enables streaming responses.
// callbacks can be used to hook into agent execution events (tool execution, streaming chunks, etc.).
// SDK events (agent.start, agent.end, etc.) are emitted via the interpreter's event manager regardless of callbacks.
// The user message is inferred from the conversation.
func (i *Interpreter) ExecuteAgentWithCallbacks(
	ctx context.Context,
	conv *ConversationValue,
	agent *AgentValue,
	streaming bool,
	callbacks *AgentCallbacks,
) (Value, error) {
	return i.executeAgentInternal(ctx, conv, agent, streaming, callbacks)
}

// executeAgentInternal is the core agentic loop implementation.
// It supports both simple execution (no callbacks) and REPL execution (with callbacks).
// Events are always emitted via the interpreter's EmitEvent method.
func (i *Interpreter) executeAgentInternal(ctx context.Context, conv *ConversationValue, agent *AgentValue, streaming bool, callbacks *AgentCallbacks) (Value, error) {
	startTime := time.Now()

	// Track token usage across all iterations
	var totalInputTokens, totalOutputTokens, totalCachedTokens int

	// Get user message for events from conversation (find the last user message)
	userMessage := ""
	if len(conv.Messages) > 0 {
		// Find the last user message in conversation
		for j := len(conv.Messages) - 1; j >= 0; j-- {
			if conv.Messages[j].Role == "user" {
				userMessage = conv.Messages[j].Content
				break
			}
		}
	}

	// Emit agent.start event
	i.EmitEvent(EventAgentStart, createAgentStartContext(agent, userMessage))

	// Helper to call OnComplete callback with ACP-aligned result
	callOnComplete := func(stopReason acp.StopReason, err error) {
		// Emit agent.end event first
		durationMs := time.Since(startTime).Milliseconds()
		i.EmitEvent(EventAgentEnd, createAgentEndContext(agent, string(stopReason), durationMs, totalInputTokens, totalOutputTokens, totalCachedTokens, err))

		if callbacks != nil && callbacks.OnComplete != nil {
			result := acp.AgentResult{
				StopReason: stopReason,
				Duration:   time.Since(startTime),
				Usage: &acp.TokenUsage{
					PromptTokens:     totalInputTokens,
					CompletionTokens: totalOutputTokens,
					CachedTokens:     totalCachedTokens,
					TotalTokens:      totalInputTokens + totalOutputTokens,
				},
				Error: err,
			}
			callbacks.OnComplete(result)
		}
	}

	// Get model from agent config using ModelResolver for lazy resolution
	modelVal, ok := agent.Config["model"]
	if !ok {
		err := fmt.Errorf("agent '%s' has no model configured", agent.Name)
		callOnComplete(acp.StopReasonError, err)
		return nil, err
	}
	modelResolver, ok := modelVal.(ModelResolver)
	if !ok {
		err := fmt.Errorf("agent '%s' model config is not a model resolver", agent.Name)
		callOnComplete(acp.StopReasonError, err)
		return nil, err
	}
	// Resolve the model (handles both direct ModelValue and SDKModelRef)
	model := modelResolver.GetModel()
	if model == nil {
		err := fmt.Errorf("agent '%s' model could not be resolved (check gsh.models configuration)", agent.Name)
		callOnComplete(acp.StopReasonError, err)
		return nil, err
	}

	// Get max iterations from agent config, or use default
	maxIterations := DefaultMaxIterations
	if maxIterVal, ok := agent.Config["maxIterations"]; ok {
		if numVal, ok := maxIterVal.(*NumberValue); ok {
			maxIterations = int(numVal.Value)
			if maxIterations <= 0 {
				maxIterations = DefaultMaxIterations
			}
		}
	}

	policy, err := readContextPolicy(agent, model)
	if err != nil {
		callOnComplete(acp.StopReasonError, err)
		return nil, err
	}
	tools, err := i.agentChatTools(agent, callbacks)
	if err != nil {
		callOnComplete(acp.StopReasonError, err)
		return nil, err
	}

	// Create new conversation with response (excluding system messages)
	// System prompts should not be stored in conversations
	userMessages := []ChatMessage{}
	for _, msg := range conv.Messages {
		if msg.Role != "system" {
			userMessages = append(userMessages, msg)
		}
	}

	newConv := &ConversationValue{
		Messages:        make([]ChatMessage, len(userMessages)),
		Summary:         conv.Summary,
		CompactionCount: conv.CompactionCount,
	}
	copy(newConv.Messages, userMessages)

	// Determine if we should use streaming
	useStreaming := streaming

	// Agentic loop - continue until no tool calls or max iterations reached
	for iteration := 0; iteration < maxIterations; iteration++ {
		// Check for context cancellation
		if ctx.Err() != nil {
			err := ctx.Err()
			callOnComplete(acp.StopReasonCancelled, err)
			return newConv, err
		}

		// Emit agent.iteration.start event
		i.EmitEvent(EventAgentIterationStart, createIterationStartContext(agent, iteration))

		// Call iteration start callback
		if callbacks != nil && callbacks.OnIterationStart != nil {
			callbacks.OnIterationStart(iteration)
		}

		compacted, usage, compactErr := compactContext(ctx, newConv, agent, model, tools, policy, false)
		totalInputTokens += usage.PromptTokens
		totalOutputTokens += usage.CompletionTokens
		totalCachedTokens += usage.CachedTokens
		if compactErr != nil {
			reason := acp.StopReasonError
			if errors.Is(compactErr, context.Canceled) {
				reason = acp.StopReasonCancelled
			}
			callOnComplete(reason, compactErr)
			return newConv, compactErr
		}
		newConv = compacted
		requestModel := model
		if policy != nil {
			requestModel = modelWithOutputLimit(model, policy.reserveTokens, true)
		}

		// Create chat request
		request := ChatRequest{
			Model:    requestModel,
			Messages: contextMessages(agent, newConv),
			Tools:    tools,
		}

		// Call the model (streaming or non-streaming)
		var response *ChatResponse
		var err error

		if useStreaming {
			// Use streaming with tool call detection
			streamCallbacks := &StreamCallbacks{
				OnContent: func(content string) {
					// Emit agent.chunk event
					i.EmitEvent(EventAgentChunk, createChunkContext(agent, content))
					// Also call the original callback
					if callbacks != nil && callbacks.OnChunk != nil {
						callbacks.OnChunk(content)
					}
				},
				// Check for context cancellation (e.g., Ctrl+C)
				ShouldCancel: func() bool {
					return ctx.Err() != nil
				},
			}
			// Always emit SDK event when tool call enters pending state (streaming from LLM)
			streamCallbacks.OnToolPending = func(toolCallID string, toolName string) {
				i.EmitEvent(EventAgentToolPending, createToolPendingContext(agent, toolCallID, toolName))
				// Also call the original callback if provided
				if callbacks != nil && callbacks.OnToolPending != nil {
					callbacks.OnToolPending(toolCallID, toolName)
				}
			}
			response, err = requestModel.StreamingChatCompletion(ctx, request, streamCallbacks)
		} else {
			// Non-streaming call
			response, err = requestModel.ChatCompletion(ctx, request)
		}

		if err != nil {
			// Check if the error is due to context cancellation (e.g., Ctrl+C interrupt)
			if ctx.Err() == context.Canceled {
				err = fmt.Errorf("agent interrupted")
			} else {
				err = fmt.Errorf("agent execution failed: %w", err)
			}
			callOnComplete(acp.StopReasonError, err)
			if policy != nil {
				return newConv, err
			}
			return nil, err
		}

		// Accumulate token usage
		iterInputTokens, iterOutputTokens, iterCachedTokens := 0, 0, 0
		if response.Usage != nil {
			iterInputTokens = response.Usage.PromptTokens
			iterOutputTokens = response.Usage.CompletionTokens
			iterCachedTokens = response.Usage.CachedTokens
			totalInputTokens += iterInputTokens
			totalOutputTokens += iterOutputTokens
			totalCachedTokens += iterCachedTokens
		}

		// Call response callback
		if callbacks != nil && callbacks.OnResponse != nil {
			callbacks.OnResponse(response)
		}

		// If no tool calls, add final response and return
		if len(response.ToolCalls) == 0 {
			newConv.Messages = append(newConv.Messages, ChatMessage{
				Role:    "assistant",
				Content: response.Content,
			})
			// Emit agent.iteration.end event before completing
			i.EmitEvent(EventAgentIterationEnd, createIterationEndContext(agent, iteration, iterInputTokens, iterOutputTokens, iterCachedTokens))
			callOnComplete(acp.StopReasonEndTurn, nil)
			return newConv, nil
		}

		// Add assistant message with tool calls
		newConv.Messages = append(newConv.Messages, ChatMessage{
			Role:      "assistant",
			Content:   response.Content,
			ToolCalls: response.ToolCalls,
		})

		// Execute tool calls and add results
		for _, toolCall := range response.ToolCalls {
			// Create ACP-aligned tool call for callbacks
			acpToolCall := acp.ToolCall{
				ID:        toolCall.ID,
				Name:      toolCall.Name,
				Arguments: toolCall.Arguments,
				Status:    acp.ToolCallStatusPending,
				Kind:      classifyToolKind(toolCall.Name),
			}

			// Emit agent.tool.start event and check for override
			// If handler returns { result: "..." }, skip execution and use that result
			startCtx := createToolCallContext(agent, toolCall.ID, toolCall.Name, toolCall.Arguments, nil, nil, nil)
			startOverride := i.EmitEvent(EventAgentToolStart, startCtx)

			// Call tool start callback
			if callbacks != nil && callbacks.OnToolCallStart != nil {
				callbacks.OnToolCallStart(acpToolCall)
			}

			toolStart := time.Now()
			var toolResult string
			var toolErr error
			var skippedExecution bool

			// Check if agent.tool.start handler wants to override execution
			if override := extractToolOverride(startOverride); override != nil {
				// Handler returned an override - skip actual tool execution
				toolResult = override.Result
				if override.Error != "" {
					toolErr = fmt.Errorf("%s", override.Error)
				}
				skippedExecution = true
			} else {
				// Execute the tool normally
				toolResult, toolErr = i.executeToolCall(agent, toolCall)
			}

			toolDuration := time.Since(toolStart)
			toolDurationMs := toolDuration.Milliseconds()

			// Emit agent.tool.end event and check for override
			// If handler returns { result: "..." }, override the tool result
			endCtx := createToolCallContext(agent, toolCall.ID, toolCall.Name, toolCall.Arguments, &toolDurationMs, &toolResult, toolErr)
			endOverride := i.EmitEvent(EventAgentToolEnd, endCtx)

			// Check if agent.tool.end handler wants to override the result
			if override := extractToolOverride(endOverride); override != nil {
				toolResult = override.Result
				if override.Error != "" {
					toolErr = fmt.Errorf("%s", override.Error)
				} else if skippedExecution {
					// If we skipped execution due to start override, clear the error if end handler
					// provides a result without an error
					toolErr = nil
				}
			}

			// Call tool end callback with ACP-aligned update
			if callbacks != nil && callbacks.OnToolCallEnd != nil {
				status := acp.ToolCallStatusCompleted
				if toolErr != nil {
					status = acp.ToolCallStatusFailed
				}
				update := acp.ToolCallUpdate{
					ID:       toolCall.ID,
					Status:   status,
					Content:  toolResult,
					Duration: toolDuration,
					Error:    toolErr,
				}
				callbacks.OnToolCallEnd(acpToolCall, update)
			}

			if toolErr != nil {
				// On error, add error message as tool result so the model can recover
				toolResult = fmt.Sprintf("Error executing tool: %v", toolErr)
			}

			// Add tool result to conversation with proper tool_call_id
			newConv.Messages = append(newConv.Messages, ChatMessage{
				Role:       "tool",
				Content:    toolResult,
				Name:       toolCall.Name,
				ToolCallID: toolCall.ID,
			})
		}

		// Emit agent.iteration.end event
		i.EmitEvent(EventAgentIterationEnd, createIterationEndContext(agent, iteration, iterInputTokens, iterOutputTokens, iterCachedTokens))

		// Continue loop to make another call
	}

	// If we reach here, we hit max iterations - return what we have
	err = fmt.Errorf("agent reached maximum iterations (%d) without completing", maxIterations)
	callOnComplete(acp.StopReasonMaxIterations, err)
	return newConv, err
}
