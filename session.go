package main

import (
	"bytes"
	. "dan_code/logger"
	"dan_code/tools"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type SessionManager struct {
	History []Message
}

func (s *SessionManager) processMessage(userInput string) {
	s.History = append(s.History, Message{Role: "user", Content: userInput})
	Log.LogP("User Input: %s\n", userInput)

	for {
		response, err := queryOllama(s.History)
		if err != nil {
			Log.ErrP("Fatal Engine Error: %v\n", err)
			s.History = s.History[:len(s.History)-1]
			return
		}

		if len(response.Message.ToolCalls) > 0 {
			s.History = append(s.History, response.Message)
			for _, call := range response.Message.ToolCalls {
				toolOutput := tools.ExecuteTool(call.Function.Name, call.Function.Arguments)
				s.History = append(s.History, Message{
					Role:    "tool",
					Content: toolOutput,
				})
			}
			continue
		}

		if response.Message.Content != "" {
			Log.LogP("\n%s\n", response.Message.Content)
			totalUsed := response.PromptEvalCount + response.EvalCount
			Log.DebugP("[Context: %d/%d tokens | In: %d | Out: %d]", totalUsed, CONTEXT_LIMIT, response.PromptEvalCount, response.EvalCount)
			response.Message.Role = "assistant"
			s.History = append(s.History, response.Message)
		}
		break
	}
}

func queryOllama(messages []Message) (ChatResponse, error) {
	var chatResp ChatResponse
	reqBody := ChatRequest{
		Model:    MODEL,
		Messages: messages,
		Tools:    tools.ActiveTools,
		Stream:   false,
		Options: map[string]any{
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

	if resp.StatusCode != http.StatusOK {
		var oErr OllamaError
		_ = json.Unmarshal(body, &oErr)
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
