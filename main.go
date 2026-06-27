package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
)

// Define JSON schema for Ollama tool exposure
func getToolsDefinition() []any {
	return []any{
		map[string]any{
			"type": "function",
			"function": map[string]any{
				"name":        "read_file",
				"description": "Read the contents of a local file in the workspace directory",
				"parameters": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"path": map[string]interface{}{
							"type":        "string",
							"description": "The relative path to the file",
						},
					},
					"required": []string{"path"},
				},
			},
		},
	}
}

// Securely read file contents from the local system workspace
func readFile(path string) string {
	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Sprintf("Error reading file: %v", err)
	}
	return string(content)
}

// Fire request payload over to Ollama's local chat endpoint
func queryOllama(messages []Message) (ChatResponse, error) {
	var chatResp ChatResponse

	reqBody := ChatRequest{
		Model:    MODEL,
		Messages: messages,
		Tools:    getToolsDefinition(),
		Stream:   false,
		Options: map[string]interface{}{
			"num_ctx": CONTEXT_LIMIT,
		},
	}

	jsonBytes, err := json.Marshal(reqBody)
	if err != nil {
		return chatResp, fmt.Errorf("failed to marshal request: %w", err)
	}

	Log.Debug("====== OUTGOING REQUEST (Messages: %d) ======\n%s\n", len(messages), string(jsonBytes))

	resp, err := http.Post(OLLAMA_URL, "application/json", bytes.NewBuffer(jsonBytes))
	if err != nil {
		return chatResp, fmt.Errorf("ollama connection failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return chatResp, fmt.Errorf("failed to read response body: %w", err)
	}

	Log.Debug("====== INCOMING RESPONSE (HTTP %d) ======\n%s\n", resp.StatusCode, string(body))

	// Catch non-200 OK statuses from Ollama
	if resp.StatusCode != http.StatusOK {
		var oErr OllamaError
		_ = json.Unmarshal(body, &oErr) // Try to extract Ollama's explicit error text
		if oErr.Error != "" {
			return chatResp, fmt.Errorf("ollama error (HTTP %d): %s", resp.StatusCode, oErr.Error)
		}
		return chatResp, fmt.Errorf("ollama returned bad status: %d (body: %s)", resp.StatusCode, string(body))
	}

	if err := json.Unmarshal(body, &chatResp); err != nil {
		return chatResp, fmt.Errorf("failed to unmarshal response payload: %w", err)
	}

	return chatResp, nil
}

func main() {
	initLogger()
	Log.Log("--- NEW SESSION STARTED ---")

	initialMode := ""
	if len(os.Args) > 1 {
		initialMode = os.Args[1]
	}
	if err := InitLocalModel(initialMode); err != nil {
		Log.ErrP("Model Initialization Failed: %v", err)
		os.Exit(1)
	}
	history := []Message{}

	Log.LogP("[System] Dancode assistant ready")
	scanner := bufio.NewScanner(os.Stdin)

	for {
		Log.LogCP("\n ❯ ", "0m")
		if !scanner.Scan() {
			break
		}
		input := strings.TrimSpace(scanner.Text())
		input = strings.TrimSpace(input)
		if input == "" {
			continue
		}
		if input == "exit" {
			break
		}
		if input == "reset" {
			history = []Message{history[0]}
			Log.DebugP("User triggered context reset.")
			continue
		}

		Log.Log("User Input: %s\n", input)
		history = append(history, Message{Role: "user", Content: input})

		for {
			response, err := queryOllama(history)
			if err != nil {
				Log.ErrP("Fatal Engine Error: %v", err)
				history = history[:len(history)-1]
				break
			}

			if len(response.Message.ToolCalls) == 0 {
				if response.Message.Content != "" {
					Log.LogCP("\n%s\n", "32m", response.Message.Content)
					totalUsed := response.PromptEvalCount + response.EvalCount
					Log.DebugP("[Context] %d/%d tokens | Input: %d | Output: %d", totalUsed, CONTEXT_LIMIT, response.PromptEvalCount, response.EvalCount)
					response.Message.Role = "assistant"
					history = append(history, response.Message)
				} else {
					fmt.Printf("[Empty Response]")
				}
				break
			}

			// Append assistant's request explicitly to the history state array
			history = append(history, response.Message)

			for _, call := range response.Message.ToolCalls {
				if call.Function.Name == "read_file" {
					pathVal, ok := call.Function.Arguments["path"]
					if !ok {
						Log.Err("Model failed to provide 'path' argument")
						history = append(history, Message{
							Role:    "tool",
							Content: "Error: Missing required argument 'path'",
						})
						continue
					}
					filePath, ok := pathVal.(string)
					if !ok {
						Log.Err("Model failed to provide 'path' argument")
						history = append(history, Message{
							Role:    "tool",
							Content: "Error: Argument 'path' must be a string",
						})
						continue
					}
					Log.LogP("[Tool] Reading file: %s...\n", filePath)

					fileContent := readFile(filePath)

					history = append(history, Message{
						Role:    "tool",
						Content: fileContent,
					})
				}
			}
		}
	}
}
