# Default Agent Middleware
# This middleware handles agent chat commands (prefixed with '#').

# Default agent for REPL chat interactions
agent __defaultAgent {
    model: gsh.models.workhorse,
    systemPrompt: "You are gsh, the generative shell. You help users with their tasks in the shell. " +
    "Unless explicitly stated otherwise, assume the user's intent is to work within the latest current directory. " +
    "Use concise plain text (no markdown) for response.",
    tools: [gsh.tools.exec, gsh.tools.grep, gsh.tools.view_file, gsh.tools.edit_file],
}

# Conversation state (null means no active conversation)
__conversation = null

# Track the last known directory to detect changes
__lastKnownDirectory = null

# Default input middleware - handles # prefix for agent chat
tool __defaultAgentMiddleware(ctx, next) {
    input = ctx.input.trim()
    
    # Skip empty input
    if (input == "") {
        return { handled: true }
    }
    
    # Handle # prefix for agent chat
    if (input.startsWith("#")) {
        message = input.substring(1).trim()
        
        # Handle /clear command
        if (message == "/clear") {
            __conversation = null
            __lastKnownDirectory = null
            print("Conversation cleared")
            return { handled: true }
        }

        if (message == "/compact") {
            if (__conversation == null) {
                print("No active conversation to compact")
            } else {
                try {
                    __conversation = gsh.compact(__conversation, __defaultAgent)
                    print("Conversation compacted (total: " + __conversation.compactionCount + ")")
                } catch (e) {
                    print("gsh: " + e.message)
                }
            }
            return { handled: true }
        }
        
        # Check if directory has changed since last agent interaction
        currentDir = gsh.currentDirectory
        if (__lastKnownDirectory != currentDir) {
            # Directory changed - prepend current directory info
            message = `<current_directory>${currentDir}</current_directory>` + "\n\n" + message
        }
        
        # Chat with the default agent using pipe expressions
        try {
            if (__conversation == null) {
                __conversation = message | __defaultAgent
            } else {
                __conversation = __conversation | message | __defaultAgent
            }
            __lastKnownDirectory = currentDir
        } catch (e) {
            if (e.conversation != null) {
                __conversation = e.conversation
                __lastKnownDirectory = currentDir
            }
            print("gsh: " + e.message)
        }
        return { handled: true }
    }
    
    # Not an agent command - pass to next middleware (fall through to shell)
    return next(ctx)
}

# Register the default middleware for command.input event
gsh.use("command.input", __defaultAgentMiddleware)
