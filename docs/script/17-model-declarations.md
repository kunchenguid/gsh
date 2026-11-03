# Chapter 17: Model Declarations

You've learned how to integrate external tools via MCP, execute shell commands, and organize your code with custom tools. Now it's time to add intelligence to your scripts. In this chapter, you'll learn how to configure and use Large Language Models (LLMs) as part of your gsh scripts.

Before you can build agents that solve complex problems, you need to declare which LLM provider you want to use and configure it with the right settings. This chapter covers the `model` keyword and how to configure popular AI providers like OpenAI and Ollama.

---

## Why Models Matter

So far, your scripts have been procedural — you write exact steps for the computer to follow. But what if you want your script to understand natural language, make intelligent decisions, or solve novel problems? That's where LLMs come in.

A **model** in gsh is your connection to an AI provider. It specifies:

- Which LLM service to use (OpenAI? A local Ollama instance?)
- How to authenticate with that service
- What settings to use (temperature, model version, etc.)

Think of a model declaration as "I want to use gpt-5 from OpenAI, and here's how to reach it."

---

## Declaring Your First Model

The syntax is straightforward. Here's the basic structure:

```gsh
model modelName {
    provider: "providerName",
    apiKey: "your-api-key",
    model: "model-identifier",
}
```

Let's look at concrete examples for different providers.

### OpenAI Models

To use OpenAI's models (like GPT-4 or gpt-5), you need an API key from OpenAI:

```gsh
model gpt5 {
    provider: "openai",
    apiKey: env.OPENAI_API_KEY,
    model: "gpt-5",
}
```

**Output:** (No output — this just declares the model)

You can also customize the temperature (creativity level):

```gsh
model gpt5Precise {
    provider: "openai",
    apiKey: env.OPENAI_API_KEY,
    model: "gpt-5",
    temperature: 0.2,
}

model gpt5Creative {
    provider: "openai",
    apiKey: env.OPENAI_API_KEY,
    model: "gpt-5",
    temperature: 0.9,
}
```

**Output:** (No output — these declare different configurations)

The `temperature` parameter controls randomness:

- Low values (0.0-0.3) = More deterministic and focused
- Medium values (0.5-0.7) = Balanced
- High values (0.8-1.0) = More creative and varied

### Local Models with Ollama

Don't want to pay for API calls or send data to cloud services? Use Ollama to run models locally. First, make sure Ollama is running (`ollama serve`), then configure gsh to use it:

```gsh
model localLlama {
    provider: "openai",
    apiKey: "ollama",
    baseURL: "http://localhost:11434/v1",
    model: "devstral-small-2",
    timeout: 15000,
}
```

**Output:** (No output — this declares a local model)

The key points here:

- `provider` is `"openai"` (Ollama uses the OpenAI-compatible API)
- `apiKey` is literally the string `"ollama"` (not a real key)
- `baseURL` points to your local Ollama server
- `model` is the name of the model you've pulled into Ollama
- `timeout` is in milliseconds and protects callers from hanging forever if the backend stops responding

---

## Environment Variables for API Keys

Never hardcode sensitive API keys in your scripts! Always use environment variables:

```gsh
# ✓ Good: reads from environment
model gpt5 {
    provider: "openai",
    apiKey: env.OPENAI_API_KEY,
    model: "gpt-5",
}

# ✗ Bad: hardcoded secret (never do this!)
model badModel {
    provider: "openai",
    apiKey: "sk-very-secret-key-12345",
    model: "gpt-5",
}
```

Before running your script, set the environment variable:

```bash
export OPENAI_API_KEY="sk-..."
gsh run your_script.gsh
```

Or pass it inline:

```bash
OPENAI_API_KEY="sk-..." gsh run your_script.gsh
```

---

## Custom Headers

When working with API proxies, enterprise gateways, or services that require additional authentication, you can specify custom HTTP headers that will be sent with every LLM request:

```gsh
model proxyModel {
    provider: "openai",
    apiKey: env.OPENAI_API_KEY,
    model: "gpt-4",
    baseURL: "https://my-proxy.example.com/v1",
    headers: {
        "X-Proxy-Auth": env.PROXY_AUTH_TOKEN,
        "X-Team-ID": "my-team",
        "X-Request-Source": "gsh-script",
    },
}
```

The `headers` configuration accepts an object where:

- Keys are the header names (strings)
- Values must be strings (you can use environment variables)

**Common use cases for custom headers:**

- **API Proxies:** Authentication tokens for corporate proxies
- **Rate limiting:** Team or user identifiers for quota tracking
- **Observability:** Request tracing and correlation IDs
- **Custom authentication:** Additional auth tokens beyond the API key

---

## Extra Body Parameters

Some LLM providers support additional parameters in the request body that aren't part of the standard OpenAI API. The `extraBody` configuration allows you to pass provider-specific parameters that will be merged directly into the request body as top-level fields:

```gsh
model customModel {
    provider: "openai",
    apiKey: env.OPENAI_API_KEY,
    model: "gpt-4",
    baseURL: "https://custom-provider.example.com/v1",
    extraBody: {
        "custom_param": "custom-value",
        "provider_option": true,
        "nested_config": {
            "setting1": "value1",
            "setting2": 42,
        },
    },
}
```

The `extraBody` configuration accepts an object where:

- Keys are the parameter names (strings)
- Values can be any type (strings, numbers, booleans, objects, arrays)

**Common use cases for extra body parameters:**

- **Provider-specific features:** Enable features unique to certain LLM providers
- **Custom routing:** Parameters for load balancers or model routers
- **Advanced model options:** Provider-specific tuning parameters not in the standard API
- **Metadata:** Additional context that the provider can use for logging or routing

---

## Multiple Models in One Script

You can declare multiple models and choose which one to use for different tasks:

```gsh
#!/usr/bin/env gsh

# Fast and cheap for simple tasks
model fastModel {
    provider: "openai",
    apiKey: env.OPENAI_API_KEY,
    model: "gpt-5-mini",
    temperature: 0.3,
}

# Powerful for complex tasks (uses OpenAI's latest)
model powerfulModel {
    provider: "openai",
    apiKey: env.OPENAI_API_KEY,
    model: "gpt-5",
    temperature: 0.7,
}

# Local backup if APIs are down
model localFallback {
    provider: "openai",
    apiKey: "ollama",
    baseURL: "http://localhost:11434/v1",
    model: "llama3.2:3b",
}

print("Models configured:")
print("- fastModel: for quick, deterministic tasks")
print("- powerfulModel: for complex reasoning")
print("- localFallback: for offline use")
```

**Output:**

```
Models configured:
- fastModel: for quick, deterministic tasks
- powerfulModel: for complex reasoning
- localFallback: for offline use
```

---

## Understanding Model Parameters

### The Core Parameters

Every model declaration needs:

- **`provider`** - Where the model runs: `"openai"` (for both cloud OpenAI and local Ollama)
- **`apiKey`** - Authentication token (use `env.VARIABLE_NAME`)
- **`model`** - The model identifier (e.g., `"gpt-5"`, `"devstral-small-2"`)

### Optional Parameters

`contextWindow` is an optional positive integer enabling gsh's context budget
and automatic compaction for agents using this model. It is not sent as a server
configuration parameter. See [Context management](24-context-management.md) for
output reservations, per-agent overrides, and estimation limits.

- **`temperature`** (default: 0.7) - Controls randomness in responses (0.0-1.0)
- **`baseURL`** - For Ollama or self-hosted services, the URL to the API endpoint
- **`timeout`** - Request timeout in milliseconds for model API calls

### Practical Example: Choosing the Right Parameters

```gsh
#!/usr/bin/env gsh

# For code generation (needs consistency)
model codeGenerator {
    provider: "openai",
    apiKey: env.OPENAI_API_KEY,
    model: "gpt-5",
    temperature: 0.3,
}


# For brainstorming (needs diversity)
model ideaGenerator {
    provider: "openai",
    apiKey: env.OPENAI_API_KEY,
    model: "gpt-5-mini",
    temperature: 0.9,
}

print("Model configurations ready for different tasks:")
print("- codeGenerator: high temperature for diverse code solutions")
print("- factChecker: low temperature for accurate, consistent facts")
print("- ideaGenerator: very high temperature for creative ideas")
```

**Output:**

```
Model configurations ready for different tasks:
- codeGenerator: high temperature for diverse code solutions
- factChecker: low temperature for accurate, consistent facts
- ideaGenerator: very high temperature for creative ideas
```

---

## Currently Supported Providers

### OpenAI

OpenAI provider is fully supported and recommended for production use.

- **Setup:** Get API key from https://platform.openai.com/api-keys
- **Popular models:**
  - `gpt-5` - Latest multimodal model, fast and capable
  - `gpt-5-mini` - Fast and affordable

Example:

```gsh
model gpt5o {
    provider: "openai",
    apiKey: env.OPENAI_API_KEY,
    model: "gpt-5",
    temperature: 0.7,
}
```

### Ollama (Local)

Run models locally without API costs or data leaving your machine.

- **Setup:** `ollama serve` (in one terminal)
- **Download models:** `ollama pull llama3.2:3b`
- **Popular models:**
  - `llama3.2:3b` - Fast, compact
  - `mistral:latest` - Good reasoning
  - `neural-chat:latest` - Optimized for conversation

Example:

```gsh
model localLlama {
    provider: "openai",
    apiKey: "ollama",
    baseURL: "http://localhost:11434/v1",
    model: "llama3.2:3b",
}
```

---

## Checking Your Configuration

Before using a model in an agent, verify it's declared correctly. You can write a simple test:

```gsh
#!/usr/bin/env gsh

# Declare your model
model testModel {
    provider: "openai",
    apiKey: env.OPENAI_API_KEY,
    model: "gpt-5-mini",
}

# In the next chapter, we'll use this with agents
print("Model 'testModel' is declared and ready to use with agents")
print("")
print("Configuration summary:")
print("- Provider: openai")
print("- Model: gpt-5-mini")
print("- API Key: Set from OPENAI_API_KEY environment variable")
print("")
print("Next step: Declare an agent that uses this model!")
```

**Output:**

```
Model 'testModel' is declared and ready to use with agents

Configuration summary:
- Provider: openai
- Model: gpt-5-mini
- API Key: Set from OPENAI_API_KEY environment variable

Next step: Declare an agent that uses this model!
```

---

## Real-World Example: Multi-Model Pipeline

Here's a practical example showing how you might set up different models for different purposes:

```gsh
#!/usr/bin/env gsh

# Fast model for initial categorization
model classifier {
    provider: "openai",
    apiKey: env.OPENAI_API_KEY,
    model: "gpt-5-mini",
    temperature: 0.2,
}

# Detailed model for in-depth analysis
model analyzer {
    provider: "openai",
    apiKey: env.OPENAI_API_KEY,
    model: "gpt-5",
    temperature: 0.5,
}

# Creative model for generating suggestions
model suggester {
    provider: "openai",
    apiKey: env.OPENAI_API_KEY,
    model: "gpt-5",
    temperature: 0.8,
}

# Tool to process a task through the pipeline
tool processRequest(request: string): any {
    result = {
        request: request,
        models_available: [
            "classifier: Fast categorization with low temperature",
            "analyzer: Detailed analysis with balanced temperature",
            "suggester: Creative suggestions with high temperature"
        ]
    }
    return result
}

# Demonstrate the pipeline
data = processRequest("Analyze customer feedback")

print("Processing pipeline configured:")
print("")
print(`Request: ${data.request}`)
print("")
print("Available models:")
for (modelInfo of data.models_available) {
    print(`  - ${modelInfo}`)
}
print("")
print("In the next chapter, you'll learn how to use these models in agents!")
```

**Output:**

```
Processing pipeline configured:

Request: Analyze customer feedback

Available models:
  - classifier: Fast categorization with low temperature
  - analyzer: Detailed analysis with balanced temperature
  - suggester: Creative suggestions with high temperature

In the next chapter, you'll learn how to use these models in agents!
```

---

## Best Practices

1. **Always use environment variables for API keys** — Never hardcode secrets
2. **Choose appropriate temperature for your use case:**
   - Deterministic tasks: 0.0-0.3
   - Balanced: 0.5-0.7
   - Creative tasks: 0.8-1.0
3. **Declare models at the top of your script** — Makes them easy to find and modify
4. **Use descriptive names** — `codeGenerator` is better than `model1`
5. **Consider cost** — Cheaper models (`gpt-5-mini`) for high-volume tasks
6. **Have a fallback** — Consider declaring a local Ollama model as a backup

---

## Key Takeaways

- **The `model` keyword declares LLM configuration** with provider, API key, and model name
- **Common providers:** OpenAI (GPT) and Ollama (local)
- **Always use environment variables** for API keys, never hardcode them
- **`temperature` parameter controls creativity:** Low for precision, high for diversity
- **`headers` parameter allows custom HTTP headers** for proxies, auth, and observability
- **`extraBody` parameter allows provider-specific request body parameters**
- **Multiple models** can coexist in one script for different purposes
- **Models are passive** — they just sit there configured; agents actually use them (Chapter 18)
- **Ollama enables local execution** without API calls or cloud costs

---

## What's Next

You've now mastered model configuration. In Chapter 18, you'll learn about **Agent Declarations** — how to combine a model with tools and a system prompt to create intelligent agents that can solve complex problems, interact with external systems, and make decisions on your behalf.

Models are the brain. Agents bring them to life.

---

**Previous Chapter:** [Chapter 16: Shell Commands](16-shell-commands.md)

**Next Chapter:** [Chapter 18: Agent Declarations](18-agent-declarations.md)
