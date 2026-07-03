package tools

func toolCallValid(toolName string, _ map[string]any) bool {
	allowedTools := []string{"ls_dir"}
	for _, tool := range allowedTools {
		if tool == toolName {
			return true
		}
	}
	return false
}

/*
func PromptForApproval(tool ToolCall) (bool, error) {
	reader := bufio.NewReader(os.Stdin)
	fmt.Printf("Approve tool call '%s'? [y/n]: ", tool.Function.Name)
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)

	if input == "y" || input == "Y" {
		return true, nil
	} else if input == "n" || input == "N" {
		return false, nil
	}

	return false, fmt.Errorf("invalid response: %s", input)
}
*/
