# gsh SDK Reference

This reference documents the built-in `gsh` SDK object and all its capabilities.
The `gsh` object is the primary API for configuring and extending gsh, available in both REPL mode and script execution.

## Quick Reference

| Property/Method              | Description                                  | Availability  |
| ---------------------------- | -------------------------------------------- | ------------- |
| `gsh.version`                | Current gsh version                          | REPL + Script |
| `gsh.terminal`               | Terminal dimensions and TTY info             | REPL + Script |
| `gsh.logging`                | Log level and file configuration             | REPL + Script |
| `gsh.completion`             | Tab-completion menu configuration            | REPL + Script |
| `gsh.models`                 | Model tier system (lite, workhorse, premium) | REPL + Script |
| `gsh.tools`                  | Built-in tools for agents                    | REPL + Script |
| `gsh.prompt`                 | Set the shell prompt                         | REPL only     |
| `gsh.lastCommand`            | Exit code and duration of last command       | REPL only     |
| `gsh.use()` / `gsh.remove()` / `gsh.removeAll()` | Event/middleware handler registration        | REPL + Script |
| `gsh.ui.styles`              | Text styling helpers                         | REPL + Script |
| `gsh.ui.spinner`             | Loading spinner API                          | REPL + Script |

## Configuration File

The `~/.gsh/repl.gsh` file uses the gsh scripting language to configure your environment. This file is optional—gsh uses sensible defaults if it doesn't exist.

```gsh
# ~/.gsh/repl.gsh

# Configure models
model myModel {
    provider: "openai",
    apiKey: env.OPENAI_API_KEY,
    model: "gpt-5.2",
}
gsh.models.workhorse = myModel

# Set logging level
gsh.logging.level = "info"

# Show more tab-completion options at once
gsh.completion.maxVisibleItems = 20
```

You can study the default configuration in `cmd/gsh/defaults/` as a reference.

## Chapters

1. **[Core Properties](01-gsh-object.md)** - Version, terminal, logging, completion, prompt, lastCommand
2. **[Models](02-models.md)** - Model tiers and model declaration syntax
3. **[Tools](03-tools.md)** - Built-in tools for agents (exec, grep, view_file, edit_file)
4. **[Agents](04-agents.md)** - Defining and using custom agents
5. **[Events](05-events.md)** - Unified event/middleware system with gsh.use() and gsh.remove()
6. **[UI](06-ui.md)** - Styling helpers and spinner API

## Related Resources

- **[Tutorial](../tutorial/README.md)** - Guided introduction to gsh
- **[Script Guide](../script/README.md)** - Full gsh scripting language reference
- **[Main README](../../README.md)** - Installation and overview

## Debugging

Enable debug logging to troubleshoot configuration issues:

```gsh
gsh.logging.level = "debug"
```

Then view the logs:

```bash
tail -f ~/.gsh/gsh.log
```
