package main

import (
	"bufio"
	. "dan_code/logger"
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
	session := newSession()
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
		session.processMessage(input)
	}
}
