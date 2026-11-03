package interpreter

import "fmt"

// agentChatTools resolves the schemas used for both execution and context accounting.
func (i *Interpreter) agentChatTools(agent *AgentValue, callbacks *AgentCallbacks) ([]ChatTool, error) {
	// Prepare tools for the agent
	// First, add tools from callbacks (e.g., REPL built-in tools)
	tools := []ChatTool{}
	if callbacks != nil && len(callbacks.Tools) > 0 {
		tools = append(tools, callbacks.Tools...)
	}

	// Then add tools from agent config
	if toolsVal, ok := agent.Config["tools"]; ok {
		if toolsArr, ok := toolsVal.(*ArrayValue); ok {
			for _, toolValInterface := range toolsArr.Elements {
				// Handle different tool types
				switch toolVal := toolValInterface.(type) {
				case *ToolValue:
					// User-defined tool
					tool := i.convertUserToolToChatTool(toolVal)
					tools = append(tools, tool)
				case *MCPToolValue:
					// MCP tool
					tool, err := i.convertMCPToolToChatTool(toolVal)
					if err != nil {
						return nil, fmt.Errorf("failed to convert MCP tool: %w", err)
					}
					tools = append(tools, tool)
				case *NativeToolValue:
					// Native tool (gsh.tools.*)
					tool := i.convertNativeToolToChatTool(toolVal)
					tools = append(tools, tool)
				default:
					return nil, fmt.Errorf("invalid tool type in agent config: %s", toolVal.Type())
				}
			}
		}
	}

	return tools, nil
}
