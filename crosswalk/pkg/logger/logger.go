package logger

import (
	"fmt"
	"io"
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

var (
	mu     sync.Mutex
	output io.Writer = os.Stdout
)

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
	fmt.Fprintf(output, "%s %s %s%s\n", now, levelStr, hostPart, msg)
}

// SetOutput은 배포 로그의 출력 대상을 교체하고, 이전 출력 대상으로 되돌리는 함수를 반환한다.
// 대시보드는 이 함수를 이용해 기존 배포 흐름의 로그를 화면에 표시한다.
func SetOutput(w io.Writer) func() {
	if w == nil {
		w = io.Discard
	}

	mu.Lock()
	previous := output
	output = w
	mu.Unlock()

	return func() {
		mu.Lock()
		output = previous
		mu.Unlock()
	}
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
