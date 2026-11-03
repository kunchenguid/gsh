# Context budgets and conversation compaction

Long agent tasks accumulate file contents, tool results, and conversation turns.
Context management summarizes older history before the next model request would
exceed a configured input budget. It is **opt-in**: existing models and agents
retain their previous behavior until a window is configured.

## Enable it for the default REPL agent

Set the window to a value supported by the model **and its server configuration**:

```gsh
# ~/.gsh/repl.gsh
model exampleModel {
    provider: "openai",
    apiKey: "ollama",
    baseURL: "http://localhost:11434/v1",
    model: "gpt-oss:20b",
    contextWindow: 32768,
}
gsh.models.workhorse = exampleModel
```

`contextWindow` configures gsh's budget; it does not change the server's context
window. Models without this setting do not enable compaction by default.

In the REPL, `# /compact` compacts the active default-agent conversation manually.
`# /clear` still discards it. If compaction fails, the active conversation is
retained and an error is displayed. Neither command is forwarded to the model.

## Configure a scripted agent

```gsh
model exampleModel {
    provider: "openai",
    apiKey: "ollama",
    baseURL: "http://localhost:11434/v1",
    model: "gpt-oss:20b",
}
agent Investigator {
    model: exampleModel,
    systemPrompt: "Investigate carefully. Preserve the user's constraints.",
    tools: [gsh.tools.grep, gsh.tools.view_file],
    context: {
        maxTokens: 32768,
        reserveTokens: 4096,
        summaryTokens: 1024,
        keepRecentMessages: 6,
    },
}

conversation = "Investigate the test failures" | Investigator
conversation = conversation | "Continue investigating" | Investigator
conversation = gsh.compact(conversation, Investigator)
print(conversation.summary)
print(conversation.compactionCount)
```

| Setting | Default | Meaning |
| --- | --- | --- |
| `maxTokens` | Model's `contextWindow` | Total estimated input and reserved output budget. Required if the model has no window. |
| `reserveTokens` | 1024 | Space reserved for each normal agent response. Requests cap `maxTokens` to this value or the model's smaller existing limit. |
| `summaryTokens` | 1024 | Output limit of each internal summarization request, including reasoning tokens if the provider charges them against this limit. |
| `keepRecentMessages` | 6 | Minimum number of newest history messages retained verbatim, extended backwards when necessary to keep a complete tool-call batch. |

All settings are positive integers. Both output reservations must be smaller
than `maxTokens`. Unknown context settings are rejected. Agent settings override
the model window. `context: null` explicitly disables automatic compaction for
that agent, even when its model declares a window. Dynamic SDK model references
are resolved at execution time. Declarations validate override fields; the window
and model-dependent constraints are validated against the current model before
execution or manual compaction, so a later tier replacement is respected.

Reasoning models may need a larger `summaryTokens` (for example, 4096) to leave
room for the actual memory after internal reasoning. If the model stops with
`finish_reason=length`, gsh reports the exhausted summary budget and keeps the
original history. Increase this setting explicitly, keeping it below
`maxTokens` and leaving enough input space for task context and prior memory.
There is no automatic retry or silent increase in cost.

Normal responses also need room for reasoning and visible output. With context
management enabled, `finish_reason=length` fails with a recoverable conversation
and the `max_tokens` stop reason. Incomplete text is not committed to history,
and no tool calls from that response execute. Streamed text may already be
visible before the error arrives. Increase `context.reserveTokens` (and a smaller
`model.maxTokens`, if configured), within the total context window, before retrying.

## What is preserved

The first and latest user messages remain verbatim, as do the most recent
messages and the original system prompt. Assistant tool calls and their results
are never split at the retained-history boundary. Older messages are summarized
into separate conversation memory, which is sent as clearly labeled historical
user data rather than elevated to a system instruction.
Memory is inserted at the end of its compacted span, before the recent suffix.
Pinned user requests inside that span precede the memory of their subsequent
results. Repeated compaction includes those requests and prior memory in their
recorded order, preventing an old pending request from following its completion
record. The initial request cannot override later corrections in memory.

The summarizer is instructed to retain goals, constraints, observed results,
decisions, relevant identifiers, completed actions, and unfinished work. A model
summary is lossy and may be inaccurate: important facts still need verification.
`conversation.messages` contains the retained messages; `conversation.summary`
contains the compacted history; `conversation.compactionCount` counts successful
compactions. The memory follows the conversation when it is piped to another
agent, including agents with automatic compaction disabled.

Compaction is transactional. The original conversation is never overwritten by
a failed or cancelled summary. `gsh.compact(conversation, agent)` returns a new
conversation on success, or the original value for a no-op. Empty, truncated,
non-text, over-budget, and non-shrinking summaries are rejected. There are no
automatic retries or tool executions during summarization.

If there is no older history and the request already fits (including an exact
budget match), manual compaction is a no-op. When even a minimal summary plus
its message framing cannot fit alongside protected content, gsh fails before
making any summary calls.

## Recovering from an automatic-compaction error

An agent may have already completed tools before its next request reaches the
budget. With context management enabled, script errors from that execution can
include `error.conversation`, containing the history up to the failure:

```gsh
try {
    conversation = conversation | Investigator
} catch (error) {
    if (error.conversation != null) {
        conversation = error.conversation
    }
    print(error.message)
}
```

Resolve the reported budget or provider issue before continuing. Keeping this
history avoids losing the record of completed actions; it does not implement
exactly-once execution of external side effects. Manual-compaction failures do
not need recovery because the input conversation remains unchanged. The default
REPL middleware retains recoverable history automatically.
If an error returns no recoverable history, the REPL also keeps the previous
directory marker so a retry still includes any pending directory change.

## Accounting and limits

- The first implementation estimates input size from serialized UTF-8 bytes
  plus framing allowances, including system instructions, summaries, tool
  schemas, arguments, results, and multipart content. **This is a conservative
  heuristic, not the provider's tokenizer**, and can compact much earlier than
  the model's actual limit. Remote providers can use different accounting.
- The check runs before every agent model call, including calls within a single
  tool-heavy turn. If the protected task, recent messages, system prompt, and
  schemas already exceed the budget, gsh stops with an actionable error rather
  than discarding those messages. Reduce retained content or increase the window
  only if the model supports it.
- Summarization uses the same model and credentials, with tools disabled and
  streaming disabled. The retained first/latest user messages provide task
  context for choosing relevant information. Large transcripts are processed
  as bounded fragments with rolling memory; each fragment retains its source
  role, message index, tool-call ID, and byte offset, even inside a long tool
  result. No old history is silently truncated. Each
  compaction allows at most 32 summary requests, then fails without replacing
  history. These requests have latency and token costs.
- Automatic-compaction token usage is included in the agent's completion usage
  totals. Internal summary text is not emitted as the agent's streamed answer.
  Manual compaction also uses model calls but has no separate agent-completion
  event or usage report in this first version.
- Context-managed models cannot use `extraBody` to override `messages`, `tools`,
  `model`, `max_tokens`, `max_completion_tokens`, `functions`, `function_call`, or
  `tool_choice`, since those fields would
  bypass budgeting or enable tools during summarization. Configure those through
  gsh instead.
- This feature manages gsh's built-in agent conversations. ACP agents manage
  their own contexts. Persistence, cross-process resume, provider-specific
  tokenizers, and broad semantic quality benchmarks are separate work.

See [real-model fidelity evaluation](../context-compaction-evaluation.md) for an
opt-in, reproducible check of exact facts before and after repeated compaction.
