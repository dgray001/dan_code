package main

import (
	"bytes"
	. "dan_code/logger"
	"dan_code/tools"
	"dan_code/utils"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

var ErrUserInterruptedStream = errors.New("User interrupted the stream")

type Session struct {
	History []Message
}

func newSession() *Session {
	return &Session{
		History: make([]Message, 0),
	}
}

func (s *Session) processMessage(userInput string) {
	s.History = append(s.History, Message{Role: "user", Content: userInput})
	Log.Log("User Input: %s\n", userInput)
	promptEvalCount := 0
	evalCount := 0

	for {
		response, err := queryOllama(s.History)
		promptEvalCount = response.PromptEvalCount
		evalCount = response.EvalCount
		if errors.Is(err, ErrUserInterruptedStream) {
			Log.LogP("[System] User interrupted stream")
			if response.Message.Content != "" {
				response.Message.Role = "assistant"
				s.History = append(s.History, response.Message)
			}
			s.History = append(s.History, Message{
				Role:    "user",
				Content: "--- Interrupted by user. Stop your previous train of thought. ---",
			})
			break
		} else if err != nil {
			Log.ErrP("Fatal Engine Error: %v\n", err)
			if len(s.History) > 0 {
				s.History = s.History[:len(s.History)-1]
			}
			break
		}
		Log.Debug("Raw model output: %q\n", response.Message.Content)

		inlineCalls := inlineToolCalls(response.Message.Content)
		if len(inlineCalls) > 0 {
			s.History = append(s.History, response.Message)
			breakLoop := false
			for _, inlineCall := range inlineCalls {
				Log.Log("[System] Intercepted inline JSON tool call for '%s'", inlineCall.Name)
				if s.executeTool(inlineCall.toTool(len(s.History))) {
					breakLoop = true
					break
				}
			}
			if breakLoop {
				break
			}
			continue
		}

		if len(response.Message.ToolCalls) > 0 {
			s.History = append(s.History, response.Message)
			breakLoop := false
			for _, call := range response.Message.ToolCalls {
				if s.executeTool(call) {
					breakLoop = true
					break
				}
			}
			if breakLoop {
				break
			}
			continue
		}

		if response.Message.Content != "" {
			Log.Log("%s", response.Message.Content)
			response.Message.Role = "assistant"
			s.History = append(s.History, response.Message)
		} else {
			Log.Debug("Model returned an empty completion chunk with no active tool execution requests. Breaking.")
			break
		}
	}
	Log.DebugP("[Context: %d/%d tokens | In: %d | Out: %d]", promptEvalCount+evalCount, CONTEXT_LIMIT, promptEvalCount, evalCount)
}

func (s *Session) executeTool(tool ToolCall) bool {
	if tool.Function.Name == "finish_task" {
		return true
	}
	/*approved, err := tools.PromptForApproval(tool)
	  if err != nil {
	      Log.ErrP("Failed to get approval for tool call: %v", err)
	      return false
	  }
	  if !approved {
	      Log.Log("[System] Tool call rejected by user.")
	      return false
	  }*/
	toolOutput := tools.ExecuteTool(tool.Function.Name, tool.Function.Arguments)
	s.History = append(s.History, Message{
		Role:       "tool",
		Name:       tool.Function.Name,
		ToolCallID: tool.ID,
		Content:    toolOutput,
	})
	return false
}

func inlineToolCalls(m string) []FunctionCall {
	var validCalls []FunctionCall
	mTrim := strings.TrimSpace(m)

	if EnableCompleteOnlyInlineParsing {
		content := strings.TrimPrefix(mTrim, "```json")
		content = strings.TrimSuffix(content, "```")
		content = strings.TrimSpace(content)
		processJSONBlock(mTrim, &validCalls)
		return validCalls
	}

	potentialBlocks := utils.ExtractBalancedJSONBlocks(mTrim)
	for _, block := range potentialBlocks {
		processJSONBlock(block, &validCalls)
	}
	return validCalls
}

func processJSONBlock(jsonStr string, validCalls *[]FunctionCall) {
	if strings.HasPrefix(jsonStr, "{") {
		var forcedCall InlineFunctionCall
		if err := json.Unmarshal([]byte(jsonStr), &forcedCall); err == nil && forcedCall.valid() {
			*validCalls = append(*validCalls, forcedCall.toCall())
		} else {
			Log.Debug("JSON parsing failed for forced tool call: %v", err)
		}
	} else if strings.HasPrefix(jsonStr, "[") {
		var rawCalls []InlineFunctionCall
		if err := json.Unmarshal([]byte(jsonStr), &rawCalls); err == nil {
			for _, call := range rawCalls {
				if call.valid() {
					*validCalls = append(*validCalls, call.toCall())
				}
			}
		} else {
			Log.Debug("JSON parsing failed for forced tool calls: %v", err)
		}
	}
}

func queryOllama(messages []Message) (ChatResponse, error) {
	stopSpinner := make(chan bool)
	spinnerDone := make(chan bool)
	go func() {
		frames := []string{"Loading   ", "Loading.  ", "Loading.. ", "Loading..."}
		i := 0
		for {
			select {
			case <-stopSpinner:
				fmt.Print("\r\033[K")
				close(spinnerDone)
				return
			default:
				fmt.Printf("\r\033[90m%s\033[0m", frames[i%len(frames)])
				os.Stdout.Sync()
				i++
				time.Sleep(200 * time.Millisecond)
			}
		}
	}()
	spinnerStopped := false
	stopSpinnerFunc := func() {
		if !spinnerStopped {
			stopSpinner <- true
			<-spinnerDone
			spinnerStopped = true
		}
	}

	var chatResp ChatResponse
	reqBody := ChatRequest{
		Model:    MODEL,
		Messages: messages,
		Tools:    tools.ActiveTools,
		Stream:   true,
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
	Log.Debug("====== INCOMING RESPONSE (HTTP %d) ======", resp.StatusCode)

	if resp.StatusCode != http.StatusOK {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return chatResp, fmt.Errorf("failed to read response body: %w", err)
		}
		var oErr OllamaError
		_ = json.Unmarshal(body, &oErr)
		if oErr.Error != "" {
			return chatResp, fmt.Errorf("ollama error (HTTP %d): %s", resp.StatusCode, oErr.Error)
		}
		return chatResp, fmt.Errorf("ollama returned bad status: %d (body: %s)", resp.StatusCode, string(body))
	}

	Log.Debug("====== STREAM STARTED ======")

	var fullContent strings.Builder
	var fullThinking strings.Builder
	var runningPromptEval int
	var runningEval int
	dec := json.NewDecoder(resp.Body)
	startedThinking := false
	finishedThinking := false
	displayedThinkingLines := 0
	startedContent := false

	for {
		select {
		case <-interruptChannel:
			fmt.Print("\r\033[K")
			chatResp.Message.Content = fullContent.String()
			chatResp.PromptEvalCount = runningPromptEval
			chatResp.EvalCount = runningEval
			return chatResp, ErrUserInterruptedStream
		default:
		}
		var chunk ChatResponseChunk
		if err := dec.Decode(&chunk); err == io.EOF {
			break
		} else if err != nil {
			stopSpinnerFunc()
			return chatResp, fmt.Errorf("stream decode failure: %w", err)
		}
		Log.Debug("%s", fmt.Sprint(chunk))
		if chunk.PromptEvalCount > 0 {
			runningPromptEval = chunk.PromptEvalCount
		}
		if chunk.EvalCount > 0 {
			runningEval = chunk.EvalCount
		}

		if chunk.Message.Thinking != "" {
			stopSpinnerFunc()
			if !startedThinking {
				Log.Debug("[Thinking] ")
				startedThinking = true
			}

			fullThinking.WriteString(chunk.Message.Thinking)

			for i := 0; i < displayedThinkingLines; i++ {
				fmt.Print("\033[F\033[K")
			}
			lines, count := getLastNLines(fullThinking.String(), 3)
			for _, line := range lines {
				fmt.Printf("\033[90m[Thinking]\033[0m %s\n", line)
			}
			displayedThinkingLines = count
			os.Stdout.Sync()
		}

		if chunk.Message.Content != "" {
			stopSpinnerFunc()
			if startedThinking && !finishedThinking {
				for i := 0; i < displayedThinkingLines; i++ {
					fmt.Print("\033[F\033[K")
				}
				Log.Debug("[Thinking Finished]")
				finishedThinking = true
			}
			if !startedContent {
				fmt.Print("\n\033[32m")
				startedContent = true
			}
			fmt.Print(chunk.Message.Content)
			os.Stdout.Sync()
			fullContent.WriteString(chunk.Message.Content)
		}

		if chunk.Done {
			chatResp.PromptEvalCount = chunk.PromptEvalCount
			chatResp.EvalCount = chunk.EvalCount
			chatResp.Message.Role = chunk.Message.Role
		}
	}

	stopSpinnerFunc()
	if startedContent {
		fmt.Print("\033[0m\n")
	}
	if fullThinking.Len() > 0 {
		chatResp.Message.Content = fmt.Sprintf("<think>\n%s\n</think>%s", fullThinking.String(), fullContent.String())
	} else {
		chatResp.Message.Content = fullContent.String()
	}
	return chatResp, nil
}

func getLastNLines(str string, n int) ([]string, int) {
	if str == "" {
		return nil, 0
	}
	termWidth := utils.GetTerminalWidth()
	maxAllowedLen := max(termWidth-15, min(10, termWidth))
	rawLines := strings.Split(str, "\n")
	var lines []string
	for _, l := range rawLines {
		trimmed := strings.TrimSpace(l)
		if trimmed != "" {
			lines = append(lines, truncateOrScrollLine(l, maxAllowedLen))
		}
	}
	if len(lines) == 0 {
		return nil, 0
	}
	if len(lines) <= n {
		return lines, len(lines)
	}
	return lines[len(lines)-n:], n
}

func truncateOrScrollLine(line string, maxLen int) string {
	if len(line) <= maxLen {
		return line
	}
	return "..." + line[len(line)-(maxLen-3):]
}
