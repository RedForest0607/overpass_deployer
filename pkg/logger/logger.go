package logger

import (
	"fmt"
	"os"
	"sync"
	"time"
)

type Level string

const (
	INFO  Level = "INFO"
	OK    Level = "OK"
	SKIP  Level = "SKIP"
	WARN  Level = "WARN"
	ERROR Level = "ERROR"
)

var mu sync.Mutex

func logMessage(level Level, host string, format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	now := time.Now().Format("15:04:05")

	levelStr := fmt.Sprintf("%5s", string(level))

	var hostPart string
	if host != "" {
		hostPart = fmt.Sprintf("[%s] ", host)
	}

	mu.Lock()
	defer mu.Unlock()
	fmt.Fprintf(os.Stdout, "%s %s %s%s\n", now, levelStr, hostPart, msg)
}

func Info(host, format string, args ...any) {
	logMessage(INFO, host, format, args...)
}

func Ok(host, format string, args ...any) {
	logMessage(OK, host, format, args...)
}

func Skip(host, format string, args ...any) {
	logMessage(SKIP, host, format, args...)
}

func Warn(host, format string, args ...any) {
	logMessage(WARN, host, format, args...)
}

func Error(host, format string, args ...any) {
	logMessage(ERROR, host, format, args...)
}

// Global versions when there's no host context
func GlobalInfo(format string, args ...any) {
	logMessage(INFO, "", format, args...)
}

func GlobalError(format string, args ...any) {
	logMessage(ERROR, "", format, args...)
}
