## Keeping code and docs in sync

When you made changes to gsh behavior, remember to update documentation under the `docs/` folder as needed.

## Writing gsh script examples

When writing a test case or code example that requires a custom model, always use a model from ollama like this:

```gsh
model exampleModel {
    provider: "openai",
    apiKey: "ollama",
    baseURL: "http://localhost:11434/v1",
    model: "gpt-oss:20b",
}
```

## Test-Driven Development

Always use TDD when developing new features. Write tests first based on the expected behavior (e.g., spec, protocol, or requirements), then write the implementation to make them pass. Writing tests after the implementation risks asserting what the code does rather than what it should do, which lets bugs slip through.

## Testing the binary

If you need to build the gsh binary for testing, you can just run `make build` or a custom build command outputting to `./bin/gsh`.

It's okay to overwrite the existing binary during testing.

The full test suite can take more than 3 minutes to run, so set timeout accordingly.

## UI Colors and Styling

**Yellow is the primary UI color for gsh.** All UI elements that need highlighting or emphasis should use yellow (ANSI color 11).

The centralized color definitions are in `internal/repl/render/styles.go`:

- `ColorYellow` (ANSI 11) - Primary UI color for headers, success indicators, tool status, exec start, system messages, spinners
- `ColorRed` (ANSI 9) - Error indicators only
- `ColorGray` (ANSI 8) - Dim/secondary information like timing

When adding new UI elements:

1. **Always import and use the color constants** from `internal/repl/render/styles.go`
2. **Never hardcode color values** like `lipgloss.Color("12")` - use the centralized constants
3. For new styles, consider adding them to `styles.go` if they'll be reused
4. **Only style the symbol, not the message text** - When rendering messages with symbols (like `→`, `▶`, `✓`), apply color only to the symbol itself. Example: `SystemMessageStyle.Render(SymbolSystemMessage) + " " + message`

## Rendering Hooks

When modifying agent rendering (tool status, exec output, headers/footers), there are **two places** that need to be updated:

1. **Go fallback code** in `internal/repl/render/renderer.go` - used when hooks fail or return empty
2. **Default hook implementations** in `cmd/gsh/defaults/` - the actual default behavior users see (modular structure with `init.gsh` as entry point)

Both should produce the same output format to maintain consistency.

## Code Organization

### Shared Utility Functions

The interpreter package (`internal/script/interpreter/`) contains canonical utility functions that should be reused rather than duplicated:

- `ValueToInterface(val Value) interface{}` - converts gsh Value to Go interface{}
- `InterfaceToValue(val interface{}) Value` - converts Go interface{} to gsh Value
- `CreateToolStartContext()` / `CreateToolEndContext()` - create tool event context objects

When implementing features that span multiple packages (e.g., interpreter and REPL), prefer:

1. Implementing canonical functions in the interpreter package
2. Exporting them (capitalize first letter) if needed by other packages
3. Having other packages import and call the canonical functions

### Avoiding Duplication

Before creating new helper functions for type conversion, context creation, or similar utilities:

1. Search for existing implementations. E.g. `grep -r "func.*ToInterface\|func.*ToValue" internal/`
2. Check `internal/script/interpreter/value_convert.go` and `internal/script/interpreter/value.go` for common existing utilities

### File Organization: Breaking Up Large Files

When a package file exceeds ~500 lines and contains multiple distinct concerns, consider splitting it:

**Example: conversation.go refactor**

- **Original:** 986 lines mixing pipe expressions, agentic loop, tool execution, event handling, and value conversion
- **Result:** 5 focused files:
  - `conversation.go` (~180 lines) - Pipe expressions & AgentCallbacks struct
  - `agent_loop.go` (~320 lines) - Core agentic loop (ExecuteAgentWithCallbacks)
  - `agent_events.go` (~160 lines) - Event constants & context creation helpers
  - `tool_execution.go` (~200 lines) - Tool execution & conversion functions
  - `value_convert.go` (~120 lines) - Value/interface conversion utilities

**Pattern to watch for:** If a file has multiple sections with disjoint imports or dependencies, it's a sign the file should be split.

## Adding Global Objects to the Interpreter

When adding new global objects (like `Math`, `DateTime`, `Regexp`) to the interpreter:

1. Register the object in `internal/script/interpreter/builtin_sdk.go` (in `registerGshSDK()`)
2. **Add the name to `builtinNames` in `internal/script/interpreter/builtin_core.go`**

The `builtinNames` map is used by `isBuiltin()` to filter out built-in objects when returning user-defined variables via `Variables()` and `GetVariables()`. Forgetting to add to this whitelist will cause tests that check variable counts to fail.

## Error Visibility

When logging errors that users need to see for debugging, use appropriate log levels:

- `Debug` - Only visible when `gsh.logging.level = "debug"` (hidden by default)
- `Info` - Visible with default settings
- `Warn` - Visible with default settings, indicates something may be wrong
- `Error` - Always visible, indicates failure

**Pattern to avoid:** Logging user-facing errors at `Debug` level, which makes them invisible by default. If an error would help users debug their scripts (e.g., event handler failures), use `Warn` level and consider a stderr fallback.

**Error message format:** User-facing errors printed to stderr should start with `gsh:` prefix:

```go
fmt.Fprintf(os.Stderr, "gsh: error message here\n")
```

## SDK Events and ACP Alignment

When adding new SDK events (in `internal/script/interpreter/agent_events.go`), check if they should align with the Agent Client Protocol (ACP):

- Use Context7 to look up ACP documentation for standard event names and lifecycle states
- ACP defines tool call statuses: `pending`, `in_progress`, `completed`, `failed`
- Prefer ACP-aligned naming (e.g., `agent.tool.pending` not `agent.tool.streaming`)

**Tool call event lifecycle:**

- `agent.tool.pending` - Tool call starts streaming from LLM (args not yet complete)
- `agent.tool.start` - Tool execution begins (args available)
- `agent.tool.end` - Tool execution completes

When debugging event handler issues, trace the full event flow in `agent_loop.go` to understand when each event fires relative to streaming vs execution phases.

## Interpreter vs REPL Separation

The interpreter (`internal/script/interpreter/`) should be **tool-agnostic**. It should not contain special-case logic for specific tools like "exec", "grep", or "view_file".

- **Interpreter responsibility:** Emit generic events (`agent.tool.start`, `agent.tool.end`) for ALL tools
- **REPL/rendering responsibility:** Handle tool-specific rendering in `cmd/gsh/defaults/events/agent.gsh` by checking `ctx.toolCall.name`

This keeps the interpreter clean and allows users to fully customize tool rendering in their own `repl.gsh`.

## Testing gsh Scripts

When writing gsh scripts (especially in `cmd/gsh/defaults/`), test the script logic with a standalone `.gsh` file before assuming language features work. gsh has a JavaScript-like syntax but not all JavaScript features are supported.

**Quick test pattern:**

```bash
cat > /tmp/tmp_rovodev_test.gsh << 'EOF'
# Test code here
result = "test"
print(result)
EOF
go build -o bin/gsh ./cmd/gsh && ./bin/gsh /tmp/tmp_rovodev_test.gsh
rm /tmp/tmp_rovodev_test.gsh
```

Use `JSON.parse()` for parsing JSON strings in gsh scripts - it's more reliable than manual string parsing.

## Maintaining this file

Keep this file for knowledge useful to almost every future agent session in this project.
Do not repeat what the codebase already shows; point to the authoritative file or command instead.
Prefer rewriting or pruning existing entries over appending new ones.
When updating this file, preserve this bar for all agents and keep entries concise.
