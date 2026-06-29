package main

import (
	"bytes"
	. "dan_code/logger"
	"dan_code/tools"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"syscall"
	"time"
	"unsafe"
)

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

	for {
		response, err := queryOllama(s.History)
		if err != nil {
			Log.ErrP("Fatal Engine Error: %v\n", err)
			if len(s.History) > 0 {
				s.History = s.History[:len(s.History)-1]
			}
			return
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
			Log.LogP("\n%s\n", response.Message.Content)
			totalUsed := response.PromptEvalCount + response.EvalCount
			Log.DebugP("[Context: %d/%d tokens | In: %d | Out: %d]", totalUsed, CONTEXT_LIMIT, response.PromptEvalCount, response.EvalCount)
			response.Message.Role = "assistant"
			s.History = append(s.History, response.Message)
		} else {
			Log.Debug("Model returned an empty completion chunk with no active tool execution requests. Breaking.")
			break
		}
	}
}

func (s *Session) executeTool(tool ToolCall) bool {
	if tool.Function.Name == "finish_task" {
		return true
	}
	toolOutput := tools.ExecuteTool(tool.Function.Name, tool.Function.Arguments)
	s.History = append(s.History, Message{
		Role:       "tool",
		Name:       tool.Function.Name,
		ToolCallID: tool.ID,
		Content:    toolOutput,
	})
	return false
}

func inlineToolCalls(message string) []FunctionCall {
	var validCalls []FunctionCall
	contentClean := strings.TrimSpace(message)
	if strings.HasPrefix(contentClean, "```json") {
		contentClean = strings.ReplaceAll(contentClean, "```json", "")
		contentClean = strings.ReplaceAll(contentClean, "```", "")
		contentClean = strings.TrimSpace(contentClean)
	}
	if strings.HasPrefix(contentClean, "{") {
		var forcedCall InlineFunctionCall
		err := json.Unmarshal([]byte(contentClean), &forcedCall)
		if err == nil && forcedCall.valid() {
			validCalls = append(validCalls, forcedCall.toCall())
		} else {
			Log.Debug("JSON parsing failed for potential forced tool call: %v", err)
		}
	} else if strings.HasPrefix(contentClean, "[") {
		var rawCalls []InlineFunctionCall
		errArray := json.Unmarshal([]byte(contentClean), &rawCalls)
		if errArray == nil {
			for _, call := range rawCalls {
				if call.valid() {
					validCalls = append(validCalls, call.toCall())
				}
			}
		} else {
			Log.Debug("JSON parsing failed for potential forced tool calls: %v", errArray)
		}
	}
	return validCalls
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
	dec := json.NewDecoder(resp.Body)
	startedThinking := false
	finishedThinking := false
	displayedThinkingLines := 0

	for {
		var chunk ChatResponseChunk
		if err := dec.Decode(&chunk); err == io.EOF {
			break
		} else if err != nil {
			stopSpinnerFunc()
			return chatResp, fmt.Errorf("stream decode failure: %w", err)
		}
		Log.Debug("%s", fmt.Sprint(chunk))

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
	termWidth := getTerminalWidth()
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

func getTerminalWidth() int {
	type winsize struct {
		Row    uint16
		Col    uint16
		Xpixel uint16
		Ypixel uint16
	}
	ws := &winsize{}
	// syscall.TIOCGWINSZ gets window size of system stdout
	retCode, _, _ := syscall.Syscall(syscall.SYS_IOCTL,
		uintptr(syscall.Stdout),
		uintptr(syscall.TIOCGWINSZ),
		uintptr(unsafe.Pointer(ws)),
	)
	if int(retCode) == -1 || ws.Col == 0 {
		return 60
	}
	return int(ws.Col)
}

func truncateOrScrollLine(line string, maxLen int) string {
	if len(line) <= maxLen {
		return line
	}
	return "..." + line[len(line)-(maxLen-3):]
}
