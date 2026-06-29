package main

import (
	"fmt"
	"strings"
)

type Message struct {
	Role       string     `json:"role"`
	Content    string     `json:"content"`
	Name       string     `json:"name,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
}

type ToolCall struct {
	ID       string       `json:"id,omitempty"`
	Type     string       `json:"type"`
	Function FunctionCall `json:"function"`
}

type FunctionCall struct {
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments"`
}

func (fc FunctionCall) toTool(l int) ToolCall {
	return ToolCall{
		ID:       fmt.Sprintf("call_%s_%d", fc.Name, l),
		Type:     "function",
		Function: fc,
	}
}

type InlineFunctionCall struct {
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments"`
	Params    map[string]any `json:"parameters"` // Fallback case
}

func (ifc InlineFunctionCall) toCall() FunctionCall {
	args := ifc.Arguments
	if len(args) == 0 && ifc.Params != nil && len(ifc.Params) > 0 {
		args = ifc.Params
	}
	return FunctionCall{
		Name:      ifc.Name,
		Arguments: args,
	}
}

func (ifc InlineFunctionCall) valid() bool {
	return strings.TrimSpace(ifc.Name) != ""
}

type ChatRequest struct {
	Model    string           `json:"model"`
	Messages []Message        `json:"messages"`
	Tools    []map[string]any `json:"tools,omitempty"`
	Stream   bool             `json:"stream"`
	Options  map[string]any   `json:"options,omitempty"`
}

type ChunkMessage struct {
	Role     string `json:"role"`
	Content  string `json:"content"`
	Thinking string `json:"thinking"`
}

type ChatResponseChunk struct {
	Model           string       `json:"model"`
	CreatedAt       string       `json:"created_at"`
	Message         ChunkMessage `json:"message"`
	Done            bool         `json:"done"`
	PromptEvalCount int          `json:"prompt_eval_count"`
	EvalCount       int          `json:"eval_count"`
}

type ChatResponse struct {
	Message         Message `json:"message"`
	PromptEvalCount int     `json:"prompt_eval_count"`
	EvalCount       int     `json:"eval_count"`
}

type OllamaError struct {
	Error string `json:"error"`
}
