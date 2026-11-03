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

// applyContextConfig validates model-independent overrides. Model references
// remain dynamic, so a missing window is only an error at execution time.
func applyContextConfig(agent *AgentValue, policy *contextPolicy) (bool, error) {
	if value, ok := agent.Config["context"]; ok {
		if value.Type() == ValueTypeNull {
			return true, nil
		}
		obj, ok := value.(*ObjectValue)
		if !ok {
			return false, fmt.Errorf("agent context must be an object or null")
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
				return false, fmt.Errorf("unknown agent context setting %q", key)
			}
			n, err := contextInteger(key, obj.GetPropertyValue(key))
			if err != nil {
				return false, err
			}
			*target = n
		}
	}
	if policy.maxTokens > 0 && (policy.reserveTokens >= policy.maxTokens || policy.summaryTokens >= policy.maxTokens) {
		return false, fmt.Errorf("context reserveTokens and summaryTokens must each be smaller than maxTokens")
	}
	return false, nil
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
	disabled, err := applyContextConfig(agent, policy)
	if err != nil || disabled {
		return nil, err
	}
	if _, configured := agent.Config["context"]; configured && policy.maxTokens == 0 {
		return nil, fmt.Errorf("agent context requires maxTokens or model.contextWindow")
	}
	if policy.maxTokens == 0 {
		return nil, nil
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
		index := min(max(conv.SummaryIndex, 0), len(conv.Messages))
		messages = append(messages, conv.Messages[:index]...)
		messages = append(messages, contextMemoryMessage(conv.Summary))
		return append(messages, conv.Messages[index:]...)
	}
	return append(messages, conv.Messages...)
}

func contextMemoryMessage(summary string) ChatMessage {
	return ChatMessage{Role: "user", Content: "Historical conversation summary through this point (may be incomplete; later corrections supersede earlier values; verify facts):\n" + summary}
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
func compactionParts(messages []ChatMessage, keep int) ([]ChatMessage, []ChatMessage, int, error) {
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
				return nil, nil, 0, fmt.Errorf("context history contains an unmatched tool result")
			}
			delete(pending, message.ToolCallID)
		default:
			if len(pending) > 0 {
				return nil, nil, 0, fmt.Errorf("context history contains incomplete tool results")
			}
			if len(message.ToolCalls) > 0 {
				if message.Role != "assistant" {
					return nil, nil, 0, fmt.Errorf("context tool calls require an assistant message")
				}
				batchStart = index
				for _, call := range message.ToolCalls {
					if call.ID == "" || pending[call.ID] {
						return nil, nil, 0, fmt.Errorf("context history contains invalid tool call IDs")
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
		return nil, nil, 0, fmt.Errorf("context history contains incomplete tool results")
	}
	var older, retained []ChatMessage
	end := 0
	for index, message := range messages {
		if index >= cut || index == firstUser || index == lastUser {
			retained = append(retained, message)
		} else {
			older = append(older, message)
			end = index + 1
		}
	}
	return older, retained, end, nil
}

// Reconstruct the compacted span in chronology, including pinned users that
// fall inside it. On repeated compaction the previous memory stays at its
// recorded position, rather than being prepended to older retained requests.
func compactionTranscript(conv *ConversationValue, cut int) []ChatMessage {
	var transcript []ChatMessage
	firstUser := true
	for index := 0; index <= cut; index++ {
		if conv.Summary != "" && index == conv.SummaryIndex {
			transcript = append(transcript, contextMemoryMessage(conv.Summary))
		}
		if index == cut {
			break
		}
		message := conv.Messages[index]
		if message.Role == "user" && firstUser {
			// The pinned initial request already appears in task_context.
			firstUser = false
			continue
		}
		transcript = append(transcript, message)
	}
	return transcript
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
	older, retained, cut, err := compactionParts(conv.Messages, policy.keepRecentMessages)
	if err != nil {
		return nil, usage, err
	}
	if len(older) == 0 {
		if before > budget {
			return nil, usage, fmt.Errorf("context budget exceeded with no older messages to compact")
		}
		return conv, usage, nil
	}
	// Even the shortest nonempty memory needs a framed message. Reject an
	// impossible budget before charging for any summarization requests.
	// Place memory at the end of the compacted span, after any pinned users
	// inside that span and before the untouched recent suffix.
	summaryIndex := len(retained) - (len(conv.Messages) - cut)
	candidate := &ConversationValue{Messages: retained, Summary: "x", SummaryIndex: summaryIndex, CompactionCount: conv.CompactionCount}
	protected, err := estimateContextTokens(contextMessages(agent, candidate), tools)
	if err != nil {
		return nil, usage, err
	}
	if protected > budget {
		return nil, usage, fmt.Errorf("context budget cannot fit system prompt, tools, task, recent messages, and summary framing; increase maxTokens or reduce keepRecentMessages/input")
	}
	candidate.Summary, usage, err = summarizeHistory(ctx, model, policy, "", compactionTranscript(conv, cut), retained)
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
