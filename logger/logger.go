package logger

import (
	"fmt"
	"log"
	"os"
)

type CustomLogger struct {
	fileLog *log.Logger
}

var Log *CustomLogger

func InitLogger() {
	file, err := os.OpenFile("debug.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		fmt.Println("\033[31m[Error] Could not open gate-debug.log, logging to stderr only.\033[0m")
		Log = &CustomLogger{fileLog: log.New(os.Stderr, "", log.LstdFlags)}
		return
	}
	Log = &CustomLogger{fileLog: log.New(file, "", log.LstdFlags|log.Lshortfile)}
}

func (cl *CustomLogger) Err(f string, a ...any) {
	m := fmt.Sprintf(f, a...)
	cl.fileLog.Printf("[ERROR] %s", m)
}

func (cl *CustomLogger) Log(f string, a ...any) {
	m := fmt.Sprintf(f, a...)
	cl.fileLog.Printf("[LOG] %s", m)
}

func (cl *CustomLogger) Debug(f string, a ...any) {
	m := fmt.Sprintf(f, a...)
	cl.fileLog.Printf("[DEBUG] %s", m)
}

func (cl *CustomLogger) ErrP(f string, a ...any) {
	m := fmt.Sprintf(f, a...)
	cl.Err("%s", m)
	fmt.Printf("\033[31m** %s **\033[0m\n", m)
}

func (cl *CustomLogger) LogP(f string, a ...any) {
	m := fmt.Sprintf(f, a...)
	cl.Log("%s", m)
	fmt.Printf("\033[36m%s\033[0m\n", m)
}

func (cl *CustomLogger) LogCP(f string, c string, a ...any) {
	m := fmt.Sprintf(f, a...)
	cl.Log("%s", m)
	fmt.Printf("\033[%s%s\033[0m", c, m)
}

func (cl *CustomLogger) DebugP(f string, a ...any) {
	m := fmt.Sprintf(f, a...)
	cl.Debug("%s", m)
	fmt.Printf("\033[90m%s\033[0m\n", m)
}
