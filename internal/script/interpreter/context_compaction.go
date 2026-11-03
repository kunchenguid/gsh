package interpreter

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
)

// Context management is opt-in. Estimates use serialized UTF-8 bytes rather
// than a tokenizer tied to one model family. This is deliberately conservative,
// but cannot guarantee the token accounting of an arbitrary remote provider.
type contextPolicy struct {
	maxTokens          int
	reserveTokens      int
	summaryTokens      int
	keepRecentMessages int
}

func contextInteger(name string, value Value) (int, error) {
	n, ok := value.(*NumberValue)
	if !ok || math.IsNaN(n.Value) || math.IsInf(n.Value, 0) || n.Value < 1 || n.Value > 1_000_000_000 || math.Trunc(n.Value) != n.Value {
		return 0, fmt.Errorf("context %s must be a positive integer no greater than 1000000000", name)
	}
	return int(n.Value), nil
}

func readContextPolicy(agent *AgentValue, model *ModelValue) (*contextPolicy, error) {
	policy := &contextPolicy{reserveTokens: 1024, summaryTokens: 1024, keepRecentMessages: 6}
	if window, ok := model.Config["contextWindow"]; ok {
		n, err := contextInteger("contextWindow", window)
		if err != nil {
			return nil, err
		}
		policy.maxTokens = n
	}
	if value, ok := agent.Config["context"]; ok {
		if value.Type() == ValueTypeNull {
			return nil, nil
		}
		obj, ok := value.(*ObjectValue)
		if !ok {
			return nil, fmt.Errorf("agent context must be an object or null")
		}
		for key := range obj.Properties {
			var target *int
			switch key {
			case "maxTokens":
				target = &policy.maxTokens
			case "reserveTokens":
				target = &policy.reserveTokens
			case "summaryTokens":
				target = &policy.summaryTokens
			case "keepRecentMessages":
				target = &policy.keepRecentMessages
			default:
				return nil, fmt.Errorf("unknown agent context setting %q", key)
			}
			n, err := contextInteger(key, obj.GetPropertyValue(key))
			if err != nil {
				return nil, err
			}
			*target = n
		}
		if policy.maxTokens == 0 {
			return nil, fmt.Errorf("agent context requires maxTokens or model.contextWindow")
		}
	}
	if policy.maxTokens == 0 {
		return nil, nil
	}
	if policy.reserveTokens >= policy.maxTokens || policy.summaryTokens >= policy.maxTokens {
		return nil, fmt.Errorf("context reserveTokens and summaryTokens must each be smaller than maxTokens")
	}
	if extra, ok := model.Config["extraBody"].(*ObjectValue); ok {
		for _, key := range []string{"messages", "tools", "model", "max_tokens", "max_completion_tokens", "functions", "function_call", "tool_choice"} {
			if _, exists := extra.Properties[key]; exists {
				return nil, fmt.Errorf("context management cannot be combined with extraBody.%s; configure messages, tools and output limits through gsh", key)
			}
		}
	}
	return policy, nil
}

func estimateContextTokens(messages []ChatMessage, tools []ChatTool) (int, error) {
	data, err := json.Marshal(struct {
		Messages []ChatMessage
		Tools    []ChatTool
	}{messages, tools})
	if err != nil {
		return 0, fmt.Errorf("estimate context: %w", err)
	}
	// Extra framing allowance in addition to the serialized content and schemas.
	return len(data) + 128 + 16*len(messages) + 32*len(tools), nil
}

func contextMessages(agent *AgentValue, conv *ConversationValue) []ChatMessage {
	messages := make([]ChatMessage, 0, len(conv.Messages)+2)
	if prompt, ok := agent.Config["systemPrompt"].(*StringValue); ok {
		messages = append(messages, ChatMessage{Role: "system", Content: prompt.Value})
	}
	if conv.Summary != "" {
		// Historical model-generated text is data, never elevated to system policy.
		// Keep chronology: the initial request precedes memory of later changes.
		// Otherwise its stale values can override corrections in the summary.
		firstUser := -1
		for index, message := range conv.Messages {
			if message.Role == "user" {
				firstUser = index
				break
			}
		}
		messages = append(messages, conv.Messages[:firstUser+1]...)
		messages = append(messages, ChatMessage{Role: "user", Content: "Historical conversation summary after the initial request (may be incomplete; later corrections supersede earlier values; verify facts):\n" + conv.Summary})
		return append(messages, conv.Messages[firstUser+1:]...)
	}
	return append(messages, conv.Messages...)
}

func modelWithOutputLimit(model *ModelValue, limit int, capExisting bool) *ModelValue {
	copyModel := *model
	copyModel.Config = make(map[string]Value, len(model.Config)+1)
	for key, value := range model.Config {
		copyModel.Config[key] = value
	}
	if old, ok := model.Config["maxTokens"].(*NumberValue); capExisting && ok && old.Value >= 1 && old.Value < float64(limit) {
		limit = int(old.Value)
	}
	copyModel.Config["maxTokens"] = &NumberValue{Value: float64(limit)}
	return &copyModel
}

// compactionParts keeps the first and latest user messages verbatim, plus at
// least the requested recent messages. A boundary may move backwards to retain
// a complete assistant tool-call batch and every corresponding tool result.
func compactionParts(messages []ChatMessage, keep int) ([]ChatMessage, []ChatMessage, error) {
	cut := len(messages) - keep
	if cut < 0 {
		cut = 0
	}
	firstUser, lastUser := -1, -1
	pending := map[string]bool{}
	batchStart := -1
	for index, message := range messages {
		if index == cut && len(pending) > 0 {
			cut = batchStart
		}
		switch message.Role {
		case "tool":
			if !pending[message.ToolCallID] {
				return nil, nil, fmt.Errorf("context history contains an unmatched tool result")
			}
			delete(pending, message.ToolCallID)
		default:
			if len(pending) > 0 {
				return nil, nil, fmt.Errorf("context history contains incomplete tool results")
			}
			if len(message.ToolCalls) > 0 {
				if message.Role != "assistant" {
					return nil, nil, fmt.Errorf("context tool calls require an assistant message")
				}
				batchStart = index
				for _, call := range message.ToolCalls {
					if call.ID == "" || pending[call.ID] {
						return nil, nil, fmt.Errorf("context history contains invalid tool call IDs")
					}
					pending[call.ID] = true
				}
			}
		}
		if message.Role == "user" {
			if firstUser < 0 {
				firstUser = index
			}
			lastUser = index
		}
	}
	if len(pending) > 0 {
		return nil, nil, fmt.Errorf("context history contains incomplete tool results")
	}
	var older, retained []ChatMessage
	for index, message := range messages {
		if index >= cut || index == firstUser || index == lastUser {
			retained = append(retained, message)
		} else {
			older = append(older, message)
		}
	}
	return older, retained, nil
}

func compactContext(ctx context.Context, conv *ConversationValue, agent *AgentValue, model *ModelValue, tools []ChatTool, policy *contextPolicy, force bool) (*ConversationValue, ChatUsage, error) {
	var usage ChatUsage
	if policy == nil {
		return conv, usage, nil
	}
	before, err := estimateContextTokens(contextMessages(agent, conv), tools)
	if err != nil {
		return nil, usage, err
	}
	budget := policy.maxTokens - policy.reserveTokens
	if !force && before <= budget {
		return conv, usage, nil
	}
	older, retained, err := compactionParts(conv.Messages, policy.keepRecentMessages)
	if err != nil {
		return nil, usage, err
	}
	candidate := &ConversationValue{Messages: retained, CompactionCount: conv.CompactionCount}
	protected, err := estimateContextTokens(contextMessages(agent, candidate), tools)
	if err != nil {
		return nil, usage, err
	}
	if protected >= budget {
		return nil, usage, fmt.Errorf("context budget cannot fit system prompt, tools, task, and recent messages; increase maxTokens or reduce keepRecentMessages/input")
	}
	if len(older) == 0 {
		if before > budget {
			return nil, usage, fmt.Errorf("context budget exceeded with no older messages to compact")
		}
		return conv, usage, nil
	}
	candidate.Summary, usage, err = summarizeHistory(ctx, model, policy, conv.Summary, older, retained)
	if err != nil {
		return nil, usage, err
	}
	after, err := estimateContextTokens(contextMessages(agent, candidate), tools)
	if err != nil {
		return nil, usage, err
	}
	if after > budget || after >= before {
		return nil, usage, fmt.Errorf("context compaction did not produce a smaller request within the context budget; original history retained")
	}
	candidate.CompactionCount++
	return candidate, usage, nil
}

// CompactConversation returns a compacted copy without executing the agent's
// tools. Errors leave the caller's conversation untouched. It uses the same
// policy and tool-schema accounting as automatic compaction.
func (i *Interpreter) CompactConversation(ctx context.Context, conv *ConversationValue, agent *AgentValue) (*ConversationValue, error) {
	resolver, ok := agent.Config["model"].(ModelResolver)
	if !ok || resolver.GetModel() == nil {
		return nil, fmt.Errorf("agent has no resolved model")
	}
	model := resolver.GetModel()
	policy, err := readContextPolicy(agent, model)
	if err != nil {
		return nil, err
	}
	if policy == nil {
		return nil, fmt.Errorf("manual compaction requires agent context or model.contextWindow")
	}
	tools, err := i.agentChatTools(agent, nil)
	if err != nil {
		return nil, err
	}
	result, _, err := compactContext(ctx, conv, agent, model, tools, policy, true)
	return result, err
}

func (i *Interpreter) builtinCompact(args []Value) (Value, error) {
	if len(args) != 2 {
		return nil, fmt.Errorf("gsh.compact requires a conversation and an agent")
	}
	conv, ok := args[0].(*ConversationValue)
	if !ok {
		return nil, fmt.Errorf("gsh.compact first argument must be a conversation")
	}
	agent, ok := args[1].(*AgentValue)
	if !ok {
		return nil, fmt.Errorf("gsh.compact second argument must be an agent")
	}
	return i.CompactConversation(i.Context(), conv, agent)
}
