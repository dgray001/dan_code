package tools

import (
	. "dan_code/logger"
	"encoding/json"
	"fmt"
)

type ToolHandler = func(args map[string]any) (string, error)

type Arg struct {
	Type        string
	Description string
	Enum        []string
	Items       map[string]string
}

type Tool struct {
	Name        string
	Description string
	Args        map[string]Arg
	Required    []string
	Handler     func(args map[string]any) (string, error)
	Definition  map[string]any
}

var ToolRegistry = make(map[string]Tool)
var ActiveTools []map[string]any

func init() {
	toolsArray := []Tool{
		readFileTool,
		grepFileTool,
		lsDirTool,
	}

	for _, t := range toolsArray {
		properties := make(map[string]any)
		for argName, argConfig := range t.Args {
			prop := map[string]any{
				"type":        argConfig.Type,
				"description": argConfig.Description,
			}
			if len(argConfig.Enum) > 0 {
				prop["enum"] = argConfig.Enum
			}
			if argConfig.Items != nil {
				prop["items"] = argConfig.Items
			}
			properties[argName] = prop
		}

		t.Definition = map[string]any{
			"type": "function",
			"function": map[string]any{
				"name":        t.Name,
				"description": t.Description,
				"parameters": map[string]any{
					"type":       "object",
					"properties": properties,
					"required":   t.Required,
				},
			},
		}
		ToolRegistry[t.Name] = t
		ActiveTools = append(ActiveTools, t.Definition)
	}
}

func ExecuteTool(name string, args map[string]any) string {
	tool, exists := ToolRegistry[name]
	if !exists {
		return fmt.Sprintf("Error: Tool '%s' is not supported", name)
	}
	argsJSON, _ := json.Marshal(args)
	Log.DebugP("[Tool] Invoked %s -> Args: %s", tool.Name, string(argsJSON))
	output, err := tool.Handler(args)
	if err != nil {
		Log.ErrP("[Tool] %s error: %v", tool.Name, err)
		return fmt.Sprintf("Error executing tool '%s': %v", tool.Name, err)
	}
	Log.DebugP("[Tool] %s success:\n%s", tool.Name, output)
	return output
}
