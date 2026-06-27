package main

import (
	"bufio"
	. "dan_code/logger"
	"fmt"
	"os"
	"strings"
)

func main() {
	InitLogger()
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
		}
	}
}
