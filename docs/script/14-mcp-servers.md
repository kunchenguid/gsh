# Chapter 14: MCP Servers

You've learned how to write gsh scripts with values, variables, control flow, and custom tools. You've even read environment variables to configure your scripts. But what if you need access to external services—like a file system, GitHub API, or a database?

That's where **MCP servers** come in. MCP stands for **Model Context Protocol**, and it's a standardized way to connect your gsh scripts to external tools and services. In this chapter, you'll learn how to declare MCP servers and use them to access powerful external capabilities.

---

## What is an MCP Server?

An MCP server is a program that exposes a set of **tools** (functions) that your gsh script can call. Think of it as a bridge between your script and some external service or system.

There are two types of MCP servers:

1. **Local Process Servers** - Programs that run on your machine (like a Node.js CLI tool)
2. **Remote HTTP/SSE Servers** - Services running on a remote host that you connect to over HTTP

### Why Use MCP Servers?

Without MCP servers, your gsh scripts are limited to:

- Variables and data structures
- Custom tools you write yourself
- Shell commands via `exec()` (which you'll learn in the next chapter)

With MCP servers, you can leverage external tools and services to do much more:

- Access the filesystem safely and portably
- Call GitHub APIs
- Query databases
- Connect to cloud services
- Extend gsh with specialized tools

---

## Declaring Local Process MCP Servers

The most common case is a **local process server** — a command that runs on your machine and communicates with your script via JSON-RPC over stdin/stdout.

### Basic Syntax

```gsh
mcp serverName {
    command: "command-name",
    args: ["arg1", "arg2"],
    env: {
        ENV_VAR: "value",
    },
}
```

### Example: The Filesystem Server

The most useful MCP server for gsh scripts is the filesystem server. It lets you safely read, write, and explore files.

Here's how to declare it:

```gsh
mcp filesystem {
    command: "npx",
    args: ["-y", "@modelcontextprotocol/server-filesystem", "/home/user/myproject"],
}
```

Let me break this down:

- `mcp filesystem` - Declares an MCP server named `filesystem`
- `command: "npx"` - The command to run (Node.js package runner)
- `args` - Arguments passed to the command:
  - `-y` tells npx to run without prompting
  - `@modelcontextprotocol/server-filesystem` is the MCP server package
  - `/home/user/myproject` is the root directory the server can access (sandbox)
- `env` (optional) - Environment variables for the server process

Once declared, the filesystem server is available in your script with all its tools.

### Real Example: Reading a File

```gsh
#!/usr/bin/env gsh

mcp filesystem {
    command: "npx",
    args: ["-y", "@modelcontextprotocol/server-filesystem", "."],
}

# Read a file
content = filesystem.read_file({path: "config.json"})
print(content)
```

**Output:**

```
{"version": "1.0", "name": "my-app"}
```

The filesystem server exposes tools like:

- `read_file({path: "..."})` - Read file contents
- `write_file({path: "...", content: "..."})` - Write to a file
- `list_directory({path: "..."})` - List directory contents
- And more!

### Passing Environment Variables

Many MCP servers need environment variables for authentication. Use the `env` field:

```gsh
mcp github {
    command: "npx",
    args: ["-y", "@modelcontextprotocol/server-github"],
    env: {
        GITHUB_TOKEN: env.GITHUB_TOKEN,
    },
}
```

Notice how we use `env.GITHUB_TOKEN` (the environment variable from the host system) to configure the MCP server's environment. This keeps your credentials secure and separate from your code.

---

## Declaring Remote HTTP/SSE Servers

Sometimes you don't want to run a server locally. Maybe it's hosted remotely, or you prefer not to manage it as a subprocess.

**Remote servers** communicate over HTTP using Server-Sent Events (SSE) for real-time communication.

### Syntax

```gsh
mcp serverName {
    url: "http://localhost:3000/mcp",
    headers: {
        Authorization: "Bearer YOUR_API_KEY",
    },
}
```

### Example: A Remote Database Server

```gsh
mcp database {
    url: "http://db.example.com/mcp",
    headers: {
        Authorization: `Bearer ${env.DB_API_KEY}`,
    },
}

# Now you can call database tools
result = database.query("SELECT * FROM users")
print(result)
```

**Output:**

```
[
    {id: 1, name: "Alice", email: "alice@example.com"},
    {id: 2, name: "Bob", email: "bob@example.com"}
]
```

Notice how we use string interpolation in headers: `Bearer ${env.DB_API_KEY}`. This is powerful for building authentication headers dynamically.

### Example: Keyless Web Search and Fetch

[Parallel Search MCP](https://docs.parallel.ai/integrations/mcp/search-mcp) provides
free web search and page extraction without an API key. It connects through gsh's
Streamable HTTP transport, so no local MCP server or extra package is needed.
Anonymous access has rate limits.

Run the [complete example](examples/parallel-search.gsh) from the repository root
with an installed gsh:

```bash
gsh run docs/script/examples/parallel-search.gsh
```

To build gsh from this checkout instead, install Go and the development tools
(`make install-tools`, plus [golangci-lint](https://golangci-lint.run/docs/welcome/install/)
if it was not installed by that command), then run:

```bash
make build
./bin/gsh run docs/script/examples/parallel-search.gsh
```

The script declares `https://search.parallel.ai/mcp` with a `User-Agent` header,
calls `parallel.web_search` with an objective and keyword queries, then calls
`parallel.web_fetch` for a specific documentation URL. Each call prints source
URLs and excerpts. Edit the queries, URL and objectives for your task. The script
generates one session ID and reuses it across both calls.

These are direct MCP tool calls: no model configuration or inference is needed.
The example is opt-in and does not change your REPL configuration.

---

## Calling MCP Tools

Once you've declared an MCP server, calling its tools is simple: use dot notation.

### Basic Tool Call

```gsh
result = serverName.toolName(arguments)
```

### With Arguments

MCP tools accept arguments as an object:

```gsh
# Example: GitHub API
github.create_issue("myorg/myrepo", {
    title: "Bug Report",
    body: "There's an issue with the login page",
})
```

### Capturing Results

MCP tools return structured data. Capture it in a variable:

```gsh
files = filesystem.list_directory(".")
for (file of files) {
    print(file.name)
}
```

### Error Handling

MCP tool calls can fail. Use try-catch to handle errors:

```gsh
try {
    content = filesystem.read_file("missing.txt")
} catch (error) {
    print(`Error: ${error.message}`)
}
```

---

## Complete Example: Reading and Processing a File

Let's build a real script that uses the filesystem MCP server to read and process a file.

```gsh
#!/usr/bin/env gsh

mcp filesystem {
    command: "npx",
    args: ["-y", "@modelcontextprotocol/server-filesystem", "."],
}

# Count lines in a file
tool countLines(content: string): number {
    lines = content.split("\n")
    return lines.length
}

# Main script - read a markdown file and count lines
print("=== File Analysis ===")
print("")

try {
    # Read a file
    result = filesystem.read_file({path: "README.md"})

    # The result is a content object - extract the text
    content = result.content ?? result

    lineCount = countLines(content)
    wordCount = content.split(" ").length

    print(`README.md has ${lineCount} lines and ${wordCount} words`)

} catch (error) {
    print(`Error: ${error.message}`)
}
```

**Output:**

```
=== File Analysis ===

README.md has 259 lines and 1133 words
```

---

## Key Takeaways

1. **MCP servers** let your gsh scripts access external tools and services
2. **Local process servers** run as subprocesses (most common case)
3. **Remote servers** communicate over HTTP (for hosted services)
4. **Declare servers** with the `mcp` keyword at the top of your script
5. **Call tools** with dot notation: `serverName.toolName(args)`
6. **Handle errors** with try-catch blocks for robustness
7. **Pass environment variables** via the `env` field for authentication

---

## What's Next?

Now that you know how to declare and use MCP servers, you have access to powerful external tools. But sometimes you need even more control — you want to run arbitrary shell commands directly.

In the next chapter, **Shell Commands**, you'll learn how to use the `exec()` function to run bash commands and capture their output. This gives you complete flexibility to shell out when you need to.

---

**Previous Chapter:** [Chapter 13: Environment Variables](13-environment-variables.md)

**Next Chapter:** [Chapter 15: MCP Tool Invocation](15-mcp-tool-invocation.md)
