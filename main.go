package main

import (
	"bufio"
	. "dan_code/logger"
	"flag"
	"os"
	"os/signal"
	"strings"
	"syscall"
)

var EnableCompleteOnlyInlineParsing bool

var interruptChannel = make(chan os.Signal, 1)

func main() {
	InitLogger()
	Log.Log("--- NEW SESSION STARTED ---")

	flag.BoolVar(&EnableCompleteOnlyInlineParsing, "flex-inline", false, "Enable flexible inline JSON parsing for tool calls")
	flag.Parse()

	args := flag.Args()
	initialMode := ""
	if len(args) > 1 {
		initialMode = args[1]
	}
	if err := InitLocalModel(initialMode); err != nil {
		Log.ErrP("Model Initialization Failed: %v", err)
		os.Exit(1)
	}
	session := newSession()
	Log.LogP("[System] Dancode assistant ready")
	scanner := bufio.NewScanner(os.Stdin)

	signal.Notify(interruptChannel, syscall.SIGINT)

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
