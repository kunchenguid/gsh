# Chapter 04: Agents in the REPL

While command prediction helps you type faster, agents let you delegate entire tasks to AI.
In this chapter, you'll learn how to configure and use agents directly in your shell.

## What is an Agent?

An **agent** is an AI-powered assistant that:

- Understands your intent
- Learns from context
- Can reason about problems
- Has access to tools (files, commands, APIs)
- Can take multiple steps to solve problems

## Talking to the Default Agent

The REPL in gsh comes with a default agent. To chat with it, prefix your message with `#`:

```bash
gsh> # analyze this error log file and suggest fixes
# Agent reads the file, understands the error, and provides solutions

gsh> # look at my unstaged changes and write test cases for them
# Agent runs git diff, understands the changes, and generates test code
```

The `#` prefix tells gsh to send your message to the agent instead of executing it as a shell command.

> You can find the implementation of this agent [here](../../cmd/gsh/defaults/middleware/agent.gsh). It's written in gsh scripting language and can be customized as needed.

## Setting Up Your First Agent

### Step 1: Choose an Agent Model

For agents, you want a **capable model**. For example:

- **Ollama models**:

  - `devstral-small-2:24b`
  - `qwen3-coder:30b`

- **OpenAI models**:

  - `gpt-5.1-codex-mini`
  - `gpt-5.2`

- **Anthropic models**:
  - `claude-haiku-4.5`
  - `claude-opus-4.5`

### Step 2: Configure in `repl.gsh`

Create or update `~/.gsh/repl.gsh`:

```gsh
# ~/.gsh/repl.gsh

# Model for agent interactions (more capable than prediction)
model myAgentModel {
    provider: "openai",
    apiKey: "ollama",
    baseURL: "http://localhost:11434/v1",
    model: "devstral-small-2",
}

# Configure the workhorse model tier for agents
gsh.models.workhorse = myAgentModel
```

> **Note:** The default agent uses `gsh.models.workhorse` with dynamic resolution - if you change it later (e.g., in an event handler), the agent will automatically use the new model for subsequent conversations. See [Configuring Models](../repl/02-configuring-models.md#dynamic-model-resolution) for details.

### Step 3: Use It

Start a new gsh session:

```bash
gsh
```

Invoke the agent with `#` followed by your request:

```bash
gsh> # what's in my current directory and what should I do next?
# Agent provides helpful analysis
```

## Agent Basics

### Long conversations

Set `contextWindow` on your workhorse model to enable automatic context budgeting
and summarization. Use `# /compact` to compact the current conversation manually.
The first and latest user messages and recent tool interactions are retained;
failed compactions preserve the conversation. This is opt-in and uses additional
model calls. See [Context management](../script/24-context-management.md) for
configuration, estimation limits, and error recovery.

### Starting an Agent Conversation

Use `#` followed by your request:

```bash
gsh> # look at my unstaged changes and write test cases for them
```

The agent will:

- Run git command and analyze output
- Search through codebases to understand context
- Modify test files

### Multi-Turn Conversations

Agents remember the conversation history:

```bash
gsh> # what are the top 3 most common file types in this directory?
Agent: The top 3 most common file types are...

gsh> # count how many lines are in each of these file types
Agent: Here's the line counts for each file type...
```

Each response builds on the previous context.

### Clearing Conversations

To start fresh and clear the conversation history:

```bash
gsh> # /clear
Conversation cleared
```

This resets the current conversation with the agent. Use it when:

- You want to change topics completely
- You want to free up context for a new task

### Canceling Agent Output

If the agent is taking too long:

- **ESC** - Stop the agent
- The conversation ends and you return to the prompt

## Agent Tools

The default agent has access to these built-in tools:

- **exec** - Run shell commands
- **grep** - Search file contents
- **view_file** - Read file contents
- **edit_file** - Modify files

These tools let the agent explore your filesystem, run commands, and make changes when you ask it to.

## Understanding Agent Output

When you interact with an agent, gsh displays structured output to help you understand what's happening. Here's what you'll see:

### Agent Header and Footer

By default, each agent response is wrapped with a header and footer:

```
gsh> # what files are in this directory?

── gsh ────────────────────────────────────────────────────────────────────────
Let me check that for you.

The current directory contains:
- README.md - Project documentation
- src/ - Source code directory
- package.json - Node.js configuration

── 523 in · 324 out · 1.2s ────────────────────────────────────────────────────

gsh>
```

- **Header**: Shows the agent name (or "gsh" for the default agent)
- **Footer**: Shows token usage (input/output) and response time for your last command

### Tool Call Display

When agents use tools, you'll see their progress:

**Shell commands (exec tool):**

```
▶ ls -la
total 24
-rw-r--r--  1 user  staff  1234 file.txt
drwxr-xr-x  2 user  staff    64 src
✓ ls (0.1s)
```

The `▶` symbol indicates a shell command is running. Output streams directly, then shows a success (`✓`) or failure (`✗`) indicator with duration.

**Other tools:**

```
▶ grep
query: "error"
path: "/var/log/app.log"
```

Shows the tool name with the `▶` symbol at the start, followed by arguments on separate lines. When complete:

```
● grep ✓ (0.02s)
```

The `●` symbol indicates completion, with `✓` for success or `✗` for failure.

### Status Symbols Reference

| Symbol | Meaning               |
| ------ | --------------------- |
| `▶`   | Tool/command starting |
| `●`    | Tool/command complete |
| `✓`    | Success               |
| `✗`    | Error/failure         |

### Thinking Indicator

While waiting for the LLM to respond, you'll see a spinning indicator:

```
── gsh ────────────────────────────────────────────────────────────────────────
⠋ Thinking...
```

This animates to show the agent is processing your request.

> You can find the implementation of this display [here](../../cmd/gsh/defaults/events/agent.gsh). It's written in gsh scripting language and can be customized as needed.

## Troubleshooting

### Agent Not Responding

1. Check the model is configured:

   ```gsh
   print(gsh.models.workhorse)
   ```

2. Verify the model is reachable:

   ```bash
   # For Ollama
   ollama list
   ollama serve  # Make sure it's running

   # For OpenAI
   curl -H "Authorization: Bearer $OPENAI_API_KEY" \
     https://api.openai.com/v1/models
   ```

3. Check logs:
   ```bash
   tail -f ~/.gsh/gsh.log
   ```

### Agent Responses Are Poor Quality

1. Try a more capable model:

   ```gsh
   model: "claude-opus-4.5",  # Instead of a small local LLM
   ```

2. Improve your system prompt:

   ```gsh
   agent MyAgent {
       model: BetterModel,
       system: "<Improve your instructions here...>",
   }
   ```

3. Provide more context in your request:
   ```bash
   gsh> # Look at /home/user/projects/python/ml-project/spec.md for context. I need to...
   ```

### Agent Takes Too Long

Try switching to a faster model:

```gsh
model: "claude-haiku-4.5",  # Faster than opus-4.5
```

## Privacy and Security

### Local LLMs

Using local Ollama models:

- All conversations stay on your machine
- No external API calls
- Free to use

### Cloud LLMs

Using models from OpenAI, Anthropic, etc.:

- Conversations are sent to their servers
- Check their privacy policies
- Consider what you want to share (code, data, etc.)

## Customizing Agents

The default agent works well for most tasks, but gsh is fully customizable. You can:

- **Change the agent's model** - Use a different LLM provider or model
- **Customize the system prompt** - Give the agent different instructions
- **Add custom tools** - Connect MCP servers for additional capabilities
- **Change the prefix** - Use `@` instead of `#`, or add multiple agent prefixes

For customization options, see the [SDK Guide](../sdk/README.md), particularly the content about **Command Middleware**.

For scripting with agents, see [Chapter 19: Conversations and Pipes](../script/19-conversations-and-pipes.md).

## What's Next?

You now know how to use AI agents interactively! Chapter 05 covers **Executing gsh Scripts**—how to write and run `.gsh` files that combine everything you've learned into powerful automation workflows.

---

**Previous Chapter:** [Chapter 03: Command Prediction](03-command-prediction.md)

**Next Chapter:** [Chapter 05: Executing gsh Scripts](05-executing-gsh-scripts.md)
