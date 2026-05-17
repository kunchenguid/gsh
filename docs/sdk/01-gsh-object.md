# Core Properties

This chapter documents the core properties of the `gsh` object.

## `gsh.version`

**Type:** `string` (read-only)  
**Availability:** REPL + Script

Returns the current gsh version.

```gsh
print("Running gsh " + gsh.version)
# Output: Running gsh 1.0.0
```

## `gsh.terminal`

**Type:** `object` (read-only)  
**Availability:** REPL + Script

Provides information about the current terminal.

### Properties

| Property              | Type      | Description                   |
| --------------------- | --------- | ----------------------------- |
| `gsh.terminal.width`  | `number`  | Terminal width in columns     |
| `gsh.terminal.height` | `number`  | Terminal height in rows       |
| `gsh.terminal.isTTY`  | `boolean` | Whether running in a terminal |

### Example

```gsh
if (gsh.terminal.isTTY) {
    print("Terminal: " + gsh.terminal.width + "x" + gsh.terminal.height)
} else {
    print("Running in non-interactive mode")
}
```

Use terminal dimensions to format output appropriately for the user's screen size.

## `gsh.logging`

**Type:** `object`
**Availability:** REPL + Script

Controls logging behavior.

### Properties

| Property            | Type                  | Description                                         |
| ------------------- | --------------------- | --------------------------------------------------- |
| `gsh.logging.level` | `string` (read/write) | Log level: `"debug"`, `"info"`, `"warn"`, `"error"` |
| `gsh.logging.file`  | `string` (read-only)  | Path to the log file                                |

### Log Levels

```gsh
gsh.logging.level = "debug"    # Most verbose - shows all debug info
gsh.logging.level = "info"     # Normal operation (default)
gsh.logging.level = "warn"     # Warnings and errors only
gsh.logging.level = "error"    # Errors only
```

### Example

```gsh
# Enable debug logging for troubleshooting
gsh.logging.level = "debug"

# Check where logs are written
print("Logs written to: " + gsh.logging.file)
```

View logs with:

```bash
tail -f ~/.gsh/gsh.log
```

## `gsh.completion`

**Type:** `object`
**Availability:** REPL + Script

Controls tab-completion menu behavior.

### Properties

| Property                         | Type                  | Description                                              |
| -------------------------------- | --------------------- | -------------------------------------------------------- |
| `gsh.completion.maxVisibleItems` | `number` (read/write) | Number of completion suggestions shown at once           |

The value must be an integer from `1` through the maximum supported Go `int` value.
The default is `10`.
Set this in `~/.gsh/repl.gsh` to show more or fewer completion options before the menu scrolls.

### Example

```gsh
# Show up to 20 tab-completion options at once
gsh.completion.maxVisibleItems = 20
```

## `gsh.prompt`

**Type:** `string` (write-only)  
**Availability:** REPL only

Sets the shell prompt string. Typically used in a `repl.prompt` event handler.

### Example

```gsh
tool myPrompt() {
    gsh.prompt = "my-shell> "
}
gsh.on("repl.prompt", myPrompt)
```

### Dynamic Prompts

Build prompts that reflect the current state:

```gsh
tool dynamicPrompt() {
    cwd = exec("pwd").stdout.trim()

    if (gsh.lastCommand.exitCode == 0) {
        gsh.prompt = "✓ " + cwd + " > "
    } else {
        gsh.prompt = "✗ " + cwd + " > "
    }
}
gsh.on("repl.prompt", dynamicPrompt)
```

For more prompt customization options including Starship integration, see the [Tutorial](../tutorial/02-configuration.md).

## `gsh.continuationPrompt`

**Type:** `string` (read/write)
**Availability:** REPL only

Sets the continuation prompt displayed on subsequent lines when entering multi-line input (e.g., unclosed quotes, heredocs, or trailing `|`). Defaults to `"> "`.

When Starship is available, gsh automatically uses `starship prompt --continuation` to set this (configurable via `starship.toml`).

### Example

```gsh
tool myPrompt() {
    gsh.prompt = "my-shell> "
    gsh.continuationPrompt = "... "
}
gsh.on("repl.prompt", myPrompt)
```

## `gsh.lastCommand`

**Type:** `object` (read-only)  
**Availability:** REPL only

Information about the most recently executed command.

### Properties

| Property                     | Type     | Description                              |
| ---------------------------- | -------- | ---------------------------------------- |
| `gsh.lastCommand.command`    | `string` | The command string that was executed     |
| `gsh.lastCommand.exitCode`   | `number` | Exit code of last command (0 = success)  |
| `gsh.lastCommand.durationMs` | `number` | Duration of last command in milliseconds |

### Example

```gsh
tool showStats() {
    cmd = gsh.lastCommand.command
    exitCode = gsh.lastCommand.exitCode
    durationSec = gsh.lastCommand.durationMs / 1000

    if (exitCode != 0) {
        print("Command failed: " + cmd)
        print("Exit code: " + exitCode)
    }
    print("Duration: " + durationSec + "s")
}
gsh.on("repl.prompt", showStats)
```

## `gsh.history`

**Type:** `object` (read-only)  
**Availability:** REPL only

Provides access to the command history database for script-based history features.

### Methods

#### `gsh.history.findPrefix(prefix, limit)`

Returns an array of history entries that start with the given prefix, ordered by most recent first.

| Parameter | Type     | Description                                                 |
| --------- | -------- | ----------------------------------------------------------- |
| `prefix`  | `string` | The prefix to search for                                    |
| `limit`   | `number` | Maximum number of entries to return (optional, default: 10) |

**Returns:** `array` - Array of history entry objects, each with:

- `command` (string): The command that was executed
- `exitCode` (number): Exit code (-1 if unknown/still running)
- `timestamp` (number): Unix timestamp when the command was executed

### Example

```gsh
# Find commands starting with "git"
entries = gsh.history.findPrefix("git", 10)
for entry of entries {
    print(entry.command + " (exit: " + entry.exitCode + ")")
}
```

### Use Case: Custom Prediction

The primary use case is implementing custom command prediction in the `repl.predict` event:

```gsh
tool historyPredictor(ctx, next) {
    if (ctx.trigger == "instant" && ctx.input != "") {
        entries = gsh.history.findPrefix(ctx.input, 10)
        # Find first successful command
        for entry of entries {
            if (entry.exitCode == 0) {
                return { prediction: entry.command }
            }
        }
    }
    return next(ctx)
}
gsh.use("repl.predict", historyPredictor)
```

---

**Next:** [Models](02-models.md) - Model tiers and configuration
