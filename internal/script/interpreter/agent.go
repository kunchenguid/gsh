package interpreter

import (
	"fmt"

	"github.com/kunchenguid/gsh/internal/script/parser"
)

// evalAgentDeclaration evaluates an agent declaration
func (i *Interpreter) evalAgentDeclaration(env *Environment, node *parser.AgentDeclaration) (Value, error) {
	agentName := node.Name.Value

	// Evaluate each config field and store as Value
	config := make(map[string]Value)

	for key, expr := range node.Config {
		value, err := i.evalExpression(env, expr)
		if err != nil {
			return nil, fmt.Errorf("failed to evaluate agent config field '%s': %w", key, err)
		}

		// Validate common config fields
		switch key {
		case "model":
			// model must be a ModelResolver (ModelValue or SDKModelRef)
			// gsh.models.* now returns SDKModelRef directly, so no special AST handling needed
			if _, ok := value.(ModelResolver); !ok {
				return nil, fmt.Errorf("agent config 'model' must be a model reference, got %s", value.Type())
			}
		case "systemPrompt":
			if _, ok := value.(*StringValue); !ok {
				return nil, fmt.Errorf("agent config 'systemPrompt' must be a string, got %s", value.Type())
			}
		case "tools":
			if _, ok := value.(*ArrayValue); !ok {
				return nil, fmt.Errorf("agent config 'tools' must be an array, got %s", value.Type())
			}
		case "metadata":
			if _, ok := value.(*ObjectValue); !ok {
				return nil, fmt.Errorf("agent config 'metadata' must be an object, got %s", value.Type())
			}
			// Allow other fields without validation for extensibility
		}

		config[key] = value
	}

	// Validate required fields
	if _, ok := config["model"]; !ok {
		return nil, fmt.Errorf("agent '%s' must have a 'model' field", agentName)
	}

	// Create the agent value
	agent := &AgentValue{
		Name:   agentName,
		Config: config,
	}

	// Validate only declaration-local overrides; the model may be replaced
	// through gsh.models before this agent is executed.
	policy := &contextPolicy{reserveTokens: 1024, summaryTokens: 1024, keepRecentMessages: 6}
	if _, err := applyContextConfig(agent, policy); err != nil {
		return nil, err
	}
	env.Set(agentName, agent)

	return agent, nil
}
