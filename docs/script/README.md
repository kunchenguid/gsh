# gsh Scripting Guide

Welcome to the gsh scripting documentation! This comprehensive guide teaches you how to write powerful automation scripts using the gsh scripting language.

## What You'll Learn

By the end of this guide, you'll know how to:

- ✅ Write and run gsh scripts (`.gsh` files)
- ✅ Use types, variables, and operators
- ✅ Control program flow with conditionals and loops
- ✅ Declare and use tools and functions
- ✅ Integrate with shell commands and environment variables
- ✅ Connect to MCP servers for external integrations
- ✅ Build AI-powered workflows with models and agents
- ✅ Debug and troubleshoot your scripts

For interactive shell usage and REPL configuration, see the **[Tutorial Guide](../tutorial/README.md)**.

## Prerequisites

Before starting, you should:

- Have gsh installed (see [main README](../../README.md) for installation)
- Understand basic programming concepts (variables, functions, control flow)
- Have a text editor for writing scripts
- Optionally: Familiarity with bash or another scripting language helps, but isn't required

## Structure of This Guide

This guide is organized into 7 logical parts, progressing from basics to advanced topics:

**Part 1: Getting Started** (2 chapters)

- [Chapter 01: Introduction](01-introduction.md) - Overview and why gsh scripting matters
- [Chapter 02: Hello World](02-hello-world.md) - Write and run your first script

**Part 2: Core Language Fundamentals** (5 chapters)

- [Chapter 03: Values and Types](03-values-and-types.md) - Strings, numbers, booleans, and more
- [Chapter 04: Variables and Assignment](04-variables-and-assignment.md) - Storing and working with data
- [Chapter 05: Operators and Expressions](05-operators-and-expressions.md) - Math, logic, and comparisons
- [Chapter 06: Arrays and Objects](06-arrays-and-objects.md) - Collections and structured data
- [Chapter 07: String Manipulation](07-string-manipulation.md) - Working with text

**Part 3: Control Flow** (3 chapters)

- [Chapter 08: Conditionals](08-conditionals.md) - Making decisions with if/else
- [Chapter 09: Loops](09-loops.md) - Repeating actions with for/while
- [Chapter 10: Error Handling](10-error-handling.md) - Structured exception handling with try/catch

**Part 4: Functions & Reusability** (2 chapters)

- [Chapter 11: Tool Declarations](11-tool-declarations.md) - Define custom tools and functions
- [Chapter 12: Tool Calls and Composition](12-tool-calls-and-composition.md) - Combine tools into workflows

**Part 5: External Integration** (4 chapters)

- [Chapter 13: Environment Variables](13-environment-variables.md) - Access system environment
- [Chapter 14: MCP Servers](14-mcp-servers.md) - Connect to external tools via Model Context Protocol
- [Chapter 15: MCP Tool Invocation](15-mcp-tool-invocation.md) - Call MCP server tools
- [Chapter 16: Shell Commands](16-shell-commands.md) - Execute and capture shell output

**Part 6: AI Agents** (5 chapters)

- [Chapter 17: Model Declarations](17-model-declarations.md) - Configure LLM providers
- [Chapter 18: Agent Declarations](18-agent-declarations.md) - Create AI agents
- [Chapter 19: Conversations and Pipes](19-conversations-and-pipes.md) - Pipe data to agents
- [Chapter 23: ACP Agents](23-acp-agents.md) - Integrate with external agents via ACP
- [Chapter 24: Context Management](24-context-management.md) - Budget and compact long agent conversations

**Part 7: Reference** (3 chapters)

- [Chapter 20: Debugging and Troubleshooting](20-debugging-and-troubleshooting.md) - Debug techniques and common issues
- [Chapter 21: Built-in Functions](21-builtin-functions.md) - Reference for all built-in functions
- [Chapter 22: Imports and Modules](22-imports-and-modules.md) - Reuse code across scripts

## Community and Support

- **GitHub Issues** - [Report bugs or ask questions](https://github.com/kunchenguid/gsh/issues)
- **Contributing** - Help improve gsh! See [CONTRIBUTING.md](../../CONTRIBUTING.md)

## Additional Resources

- **[gsh GitHub Repository](https://github.com/kunchenguid/gsh)** - Source code and issues
- **[Language Specification](../../spec/GSH_SCRIPT_SPEC.md)** - Formal language specification
- **[Tutorial Guide](../tutorial/README.md)** - Interactive shell usage
- **[SDK Guide](../sdk/README.md)** - Advanced REPL configuration

---

Ready to get started? Begin with **[Chapter 01: Introduction to gsh Scripting](01-introduction.md)** →
