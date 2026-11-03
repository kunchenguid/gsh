# Chapter 18: Agent Declarations

Agents can opt into [context budgets and automatic compaction](24-context-management.md)
using a `context` configuration object or their model's `contextWindow` setting.

## Opening: What's an Agent?

You've learned to declare models (LLM connections) in Chapter 17. But models alone are passive — they sit there configured, waiting. An **agent** is where things get intelligent. An agent brings together three things:

1. A **model** to think
2. A **system prompt** to define personality and instructions
3. **Tools** to act on the world

Think of it this way: a model is like a smart human with no context. A system prompt gives them instructions ("Be a data analyst"). Tools are their hands and eyes to interact with systems. Together, an agent is an autonomous entity that can solve problems.

In this chapter, you'll learn to orchestrate intelligent agents that solve real problems.

---

## Core Concepts: Agent Architecture

### What Goes Into an Agent Declaration?

An agent needs at least a model and a system prompt. Here's the minimal form:

```gsh
agent AnalystBot {
    model: ollama,
    systemPrompt: "You analyze data and provide insights",
}
```

**Output:** (No output — agents are declared, not executed)

But agents become powerful when you give them tools. Here's the full picture:

```gsh
agent DataExpert {
    model: ollama,

    systemPrompt: """
        You are a data expert. Your job is to analyze datasets,
        find patterns, and generate reports using available tools.
        Always explain your reasoning.
    """,

    tools: [filesystem.read_file, filesystem.write_file, analyzeData],

    temperature: 0.5,  # Optional: override model's temperature
}
```

**Output:** (No output — agents are declared, not executed)

### Breaking Down the Fields

**`model` (required):**

- Must reference a declared model by name or use `gsh.models.*` tiers
- This is where the agent gets its "brain"
- The agent uses this model for all reasoning
- When using `gsh.models.lite`, `gsh.models.workhorse`, or `gsh.models.premium`, the model is resolved dynamically at runtime (see below)
- The `gsh.models` object is available in both REPL mode and standalone script execution

**`systemPrompt` (recommended):**

- A string that tells the agent what to do
- Sets the agent's personality and expertise
- Can be single-line or multi-line
- Think of it as a detailed job description

**`tools` (optional):**

- An array of functions the agent can call
- Can include MCP tools: `filesystem.read_file`, `github.get_issue`, etc.
- Can include your own custom tools (defined with `tool` keyword)
- Without tools, the agent can only reason; with tools, it can act

**`temperature` (optional):**

- Overrides the model's default temperature
- Range: 0.0 (deterministic) to 1.0 (creative)
- Default: inherits from the model

**`metadata` (optional):**

- An object containing arbitrary key-value pairs
- Use for custom configuration, categorization, or behavior flags
- Accessible via `agentName.metadata.propertyName` in scripts
- Useful for differentiating agent behavior in event handlers or conditional logic

**Reserved metadata properties:**

- `hidden: true` - Hides the agent from UI rendering (no headers, footers, or tool status)
- `streaming: false` - Disables streaming responses for this agent (useful for background agents)

Example:

```gsh
agent QuickHelper {
    model: ollama,
    systemPrompt: "You help with quick tasks",
    metadata: {
        category: "utilities",
        timeout: 30000,
        features: ["fast", "concise"],
    },
}

# Access metadata in scripts
print(QuickHelper.name)                    # "QuickHelper"
print(QuickHelper.metadata.category)       # "utilities"
print(QuickHelper.metadata.timeout)        # 30000

# Use in conditional logic
if (QuickHelper.metadata.timeout < 60000) {
    print("This is a fast agent")
}
```

**Background agent example:**

```gsh
# Agent that runs without streaming (e.g., for predictions or background tasks)
agent BackgroundProcessor {
    model: gsh.models.lite,
    systemPrompt: "Process data and return JSON results",
    tools: [],
    metadata: {
        hidden: true,      # No UI output
        streaming: false,  # Non-streaming for efficiency
    },
}
```

### Dynamic vs. Static Model Assignment

When you assign a model to an agent, gsh supports two modes:

**Direct model assignment (static):**

```gsh
model myCustomModel {
    provider: "openai",
    apiKey: env.OPENAI_API_KEY,
    model: "gpt-4o",
}

agent MyAgent {
    model: myCustomModel,  # Always uses this specific model
    systemPrompt: "You are helpful.",
}
```

The agent will always use `myCustomModel`, even if you later change `gsh.models.workhorse`.

**SDK model tier reference (dynamic):**

```gsh
agent MyAgent {
    model: gsh.models.workhorse,  # Resolved at runtime
    systemPrompt: "You are helpful.",
}
```

The agent looks up `gsh.models.workhorse` each time it runs. This means:

- You can change `gsh.models.workhorse` at any time and the agent will use the new model
- Event handlers can switch models based on context
- Multiple agents can share the same tier but use different models if the tier is updated

This dynamic resolution is useful for:

- Switching between local and cloud models based on network availability
- A/B testing different models
- Adjusting model choice based on task complexity

---

## Examples: From Simple to Sophisticated

### Example 1: A Helpful Assistant

The simplest agent needs only a model and prompt:

```gsh
#!/usr/bin/env gsh

model ollama {
    provider: "openai",
    apiKey: "ollama",
    baseURL: "http://localhost:11434/v1",
    model: "devstral-small-2",
}

agent Greeter {
    model: ollama,
    systemPrompt: "You greet people warmly and ask about their day",
}

print("Agent 'Greeter' is ready to chat!")
print("Model: devstral-small-2 (via Ollama)")
print("Role: Friendly conversation partner")
```

**Output:**

```
Agent 'Greeter' is ready to chat!
Model: devstral-small-2 (via Ollama)
Role: Friendly conversation partner
```

### Example 2: A Data Analyst with Tools

Now let's give an agent tools to work with real data:

```gsh
#!/usr/bin/env gsh

mcp filesystem {
    command: "npx",
    args: ["-y", "@modelcontextprotocol/server-filesystem", "/tmp"],
}

model ollama {
    provider: "openai",
    apiKey: "ollama",
    baseURL: "http://localhost:11434/v1",
    model: "devstral-small-2",
}

tool analyzeJSON(jsonText: string): string {
    parsed = JSON.parse(jsonText)
    itemCount = 0
    if (parsed.Type() == "array") {
        itemCount = parsed.Elements.Length
    }
    return `Found ${itemCount} items in the JSON`
}

agent DataAnalyst {
    model: ollama,

    systemPrompt: """
        You are a data analyst. Your job is to:
        1. Read data files when asked
        2. Analyze the data structure and content
        3. Provide insights and summaries

        Use the available tools to examine files and understand data.
        Always be precise with numbers and facts.
    """,

    tools: [filesystem.read_file, analyzeJSON],
}

print("Agent 'DataAnalyst' is ready!")
print("Available tools: filesystem.read_file, analyzeJSON")
print("This agent can read files and analyze their contents")
```

**Output:**

```
Agent 'DataAnalyst' is ready!
Available tools: filesystem.read_file, analyzeJSON
This agent can read files and analyze their contents
```

### Example 3: Multiple Agents for Different Roles

Different jobs need different personalities. Create specialized agents:

```gsh
#!/usr/bin/env gsh

model thinking {
    provider: "openai",
    apiKey: "ollama",
    baseURL: "http://localhost:11434/v1",
    model: "devstral-small-2",
    temperature: 0.3,  # Deterministic for analysis
}

model creative {
    provider: "openai",
    apiKey: "ollama",
    baseURL: "http://localhost:11434/v1",
    model: "devstral-small-2",
    temperature: 0.8,  # Creative for writing
}

agent CodeReviewer {
    model: thinking,
    systemPrompt: """
        You are an experienced code reviewer.
        Examine code carefully. Look for bugs, inefficiencies, and security issues.
        Be constructive and specific in your feedback.
    """,
}

agent ContentWriter {
    model: creative,
    systemPrompt: """
        You are a creative writer and editor.
        Write engaging, clear, and compelling content.
        Adapt your tone to the audience.
    """,
}

agent QAEngineer {
    model: thinking,
    systemPrompt: """
        You are a QA engineer who thinks through test scenarios.
        Identify edge cases, error conditions, and integration points.
        Create comprehensive test plans.
    """,
}

print("Specialized agents created:")
print("- CodeReviewer (deterministic, analytical)")
print("- ContentWriter (creative, engaging)")
print("- QAEngineer (systematic, thorough)")
```

**Output:**

```
Specialized agents created:
- CodeReviewer (deterministic, analytical)
- ContentWriter (creative, engaging)
- QAEngineer (systematic, thorough)
```

### Example 4: Comprehensive Agent with Many Tools

A fully-featured agent for complex tasks:

```gsh
#!/usr/bin/env gsh

mcp filesystem {
    command: "npx",
    args: ["-y", "@modelcontextprotocol/server-filesystem", "/tmp"],
}

model ollama {
    provider: "openai",
    apiKey: "ollama",
    baseURL: "http://localhost:11434/v1",
    model: "devstral-small-2",
}

# Custom tools for business logic
tool calculateMetrics(data: string): string {
    parsed = JSON.parse(data)
    return `Metrics calculated for ${parsed.length} records`
}

tool formatReport(analysis: string): string {
    return `# Report\n\n${analysis}`
}

tool validateData(data: string): string {
    try {
        JSON.parse(data)
        return "Data is valid JSON"
    } catch (err) {
        return `Invalid data: ${err.message}`
    }
}

agent ReportGenerator {
    model: ollama,

    systemPrompt: """
        You are a professional report generator.

        Your responsibilities:
        1. Validate incoming data
        2. Calculate key metrics and insights
        3. Format findings into a professional report
        4. Save reports to disk for archival

        Be thorough, accurate, and professional.
        Always double-check calculations before reporting.
    """,

    tools: [
        filesystem.read_file,
        filesystem.write_file,
        calculateMetrics,
        formatReport,
        validateData,
    ],

    temperature: 0.4,  # Slightly lower for professional consistency
}

print("ReportGenerator agent initialized")
print("Capabilities:")
print("- Read and write files")
print("- Validate and analyze data")
print("- Generate professional reports")
```

**Output:**

```
ReportGenerator agent initialized
Capabilities:
- Read and write files
- Validate and analyze data
- Generate professional reports
```

### Example 5: Agent Configuration Patterns

Here are practical patterns you'll use over and over:

```gsh
#!/usr/bin/env gsh

model base {
    provider: "openai",
    apiKey: "ollama",
    baseURL: "http://localhost:11434/v1",
    model: "devstral-small-2",
}

# Pattern 1: Temperature Specialization
agent FastDecisionMaker {
    model: base,
    systemPrompt: "Make quick, decisive recommendations",
    temperature: 0.1,  # Very deterministic
}

agent CreativeIdeaGenerator {
    model: base,
    systemPrompt: "Generate novel, creative ideas",
    temperature: 0.9,  # Very creative
}

# Pattern 2: Tool Scoping (different agents, different capabilities)
agent DataAgent {
    model: base,
    systemPrompt: "Work with data and files",
    tools: [filesystem.read_file, filesystem.write_file],
}

agent LogicAgent {
    model: base,
    systemPrompt: "Reason about logic problems",
    # No tools - pure reasoning
}

# Pattern 3: Multiline Prompts for Complex Instructions
agent DocumentAnalyzer {
    model: base,
    systemPrompt: """
        You are an expert document analyst.

        When you receive documents:
        1. Identify the document type
        2. Extract key information
        3. Summarize main points
        4. Flag any important warnings or notices

        Be systematic and thorough.
        Format your findings clearly.
    """,
}

print("Agent patterns demonstrated")
print("✓ Temperature specialization")
print("✓ Tool scoping for different roles")
print("✓ Complex multiline prompts")
```

**Output:**

```
Agent patterns demonstrated
✓ Temperature specialization
✓ Tool scoping for different roles
✓ Complex multiline prompts
```

---

## Understanding the Details

### System Prompts: Crafting Good Instructions

Your system prompt is critical. It tells the agent how to behave. Here are dos and don'ts:

**Good system prompts are:**

- **Specific**: "Analyze customer feedback for sentiment" (not just "analyze")
- **Instructive**: Include step-by-step guidance
- **Role-focused**: Define an identity the agent can adopt
- **Bounded**: Set limits on what the agent should do

**Compare these:**

```gsh
# ✗ Weak prompt
agent Helper {
    model: ollama,
    systemPrompt: "Help users",
}

# ✓ Strong prompt
agent CustomerSupport {
    model: ollama,
    systemPrompt: """
        You are a helpful customer support specialist.

        When responding to customers:
        - Be empathetic and professional
        - Provide clear, step-by-step solutions
        - If you don't know the answer, escalate to a human
        - Always end with a question to confirm they're satisfied
    """,
}
```

### Tools: What the Agent Can Do

Tools determine what an agent can act on. Without tools, an agent can only think and talk. With tools, it can:

- **Read/write files** with `filesystem.read_file`, `filesystem.write_file`
- **Query APIs** with custom tools that call `exec()`
- **Process data** with custom tools for validation, transformation, parsing
- **Access external services** via MCP tools (GitHub, Slack, etc.)

When you add tools to an agent, the agent can call them intelligently to accomplish its job.

### Temperature: Controlling Behavior

Remember from Chapter 17: temperature controls randomness.

- **Low temperature (0.0-0.3)**: For analytical, deterministic tasks
- **Medium temperature (0.4-0.6)**: For balanced, practical work
- **High temperature (0.7-1.0)**: For creative, exploratory tasks

You can override the model's temperature for specific agents:

```gsh
model shared {
    provider: "openai",
    apiKey: "ollama",
    baseURL: "http://localhost:11434/v1",
    model: "devstral-small-2",
    temperature: 0.5,
}

agent Analyst {
    model: shared,
    systemPrompt: "Analyze data precisely",
    temperature: 0.2,  # Override to be more deterministic
}

agent Brainstormer {
    model: shared,
    systemPrompt: "Generate creative ideas",
    temperature: 0.9,  # Override to be more creative
}
```

**Output:** (No output — agents are declared)

---

## Key Takeaways

1. **Agents = Model + Prompt + Tools** - An agent is intelligent through all three components
2. **System Prompts Matter** - A clear, specific prompt guides agent behavior
3. **Tools Enable Action** - Without tools, agents can only advise; with tools, they execute
4. **Temperature Shapes Personality** - Use it to specialize agent behavior for different tasks
5. **Declare Once, Use Many Times** - Once declared, an agent is reusable throughout your script (via the pipe operator, which we'll cover in Chapter 19)
6. **Multiple Agents Can Coexist** - Different agents for different roles in the same script

---

## What's Next

Now you have agents declared and ready. But how do you actually use them? How do you talk to them, give them instructions, and get results back?

That's where the **pipe operator** comes in. In **Chapter 19: Conversations and the Pipe Operator**, you'll learn how to:

- Start conversations with agents using `|`
- Send messages and get responses
- Build multi-turn conversations
- Chain multiple agents together for complex workflows

For now, you've learned to declare powerful, specialized agents. That's the foundation. Next chapter, we'll put them to work.

---

**Previous Chapter:** [Chapter 17: Model Declarations](17-model-declarations.md)

**Next Chapter:** [Chapter 19: Conversations and the Pipe Operator](19-conversations-and-pipes.md)
