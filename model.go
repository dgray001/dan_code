package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

const OLLAMA_URL = "http://127.0.0.1:11434/api/chat"
const OLLAMA_CREATE_URL = "http://127.0.0.1:11434/api/create"
const BASE_MODEL = "qwen2.5-coder:14b"
const MODEL = "dan_code"
const CONTEXT_LIMIT = 80000
const PROMPTS_DIR = "prompts"

type Mode string

const (
	ModeAsk          Mode = "ask"
	ModeCode         Mode = "code"
	ModeArchitect    Mode = "architect"
	ModeDebug        Mode = "debug"
	ModeOrchestrator Mode = "orchestrator"
	ModeNone         Mode = ""
)

func (m Mode) modeValid() bool {
	switch m {
	case ModeAsk, ModeCode, ModeArchitect, ModeDebug, ModeOrchestrator, ModeNone:
		return true
	default:
		return false
	}
}

func (m Mode) str() string {
	if m == ModeNone {
		return "default"
	}
	return string(m)
}

func castMode(rawMode string) Mode {
	mode := Mode(rawMode)
	if !mode.modeValid() {
		return ModeNone
	}
	return mode
}

func InitLocalModel(rawMode string) error {
	mode := castMode(rawMode)
	Log.LogP("[System] Provisioning transient model space (Mode: %s)...", mode.str())

	systemPath := filepath.Join(PROMPTS_DIR, "system.md")
	systemContent, err := os.ReadFile(systemPath)
	if err != nil {
		return fmt.Errorf("failed to read foundational system.md: %v", err)
	}

	var compiledPrompt strings.Builder
	compiledPrompt.Write(systemContent)

	if mode != ModeNone {
		filename := string(mode) + ".md"
		modePath := filepath.Join(PROMPTS_DIR, filename)
		modeContent, err := os.ReadFile(modePath)
		if err != nil {
			return fmt.Errorf("mode directory mismatch for %s: %v", filename, err)
		}
		compiledPrompt.WriteString("\n\n=== Active Mode Directives ===\n")
		compiledPrompt.Write(modeContent)
	}

	modelfileString := fmt.Sprintf("FROM %s\nSYSTEM \"\"\"\n%s\n\"\"\"", BASE_MODEL, compiledPrompt.String())
	payload := map[string]any{
		"name":      MODEL,
		"modelfile": modelfileString,
		"stream":    false,
	}
	jsonBytes, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	resp, err := http.Post(OLLAMA_CREATE_URL, "application/json", bytes.NewBuffer(jsonBytes))
	if err != nil {
		return fmt.Errorf("failed to connect to Ollama daemon: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("ollama custom model compilation failed with status: %d", resp.StatusCode)
	}

	Log.Debug("Transient model %q initialized successfully with compiled prompts.", MODEL)
	return nil
}
