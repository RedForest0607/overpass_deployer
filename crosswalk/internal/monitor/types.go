package monitor

import (
	"io"
	"time"
)

type State string

const (
	StateRunning     State = "running"
	StateStopped     State = "stopped"
	StateUnknown     State = "unknown"
	StateUnavailable State = "unavailable"
)

// Target은 deploy.yml 안의 서버와 앱을 안전하게 식별하는 위치 정보다.
type Target struct {
	ServerIndex int
	AppIndex    int
}

// AppStatus는 한 원격 애플리케이션의 현재 프로세스 상태를 표현한다.
type AppStatus struct {
	Target     Target
	ServerName string
	Host       string
	AppName    string
	Port       int
	State      State
	PID        string
	Process    string
	Message    string
	ObservedAt time.Time
}

type Action string

const (
	ActionStart   Action = "start"
	ActionStop    Action = "stop"
	ActionRestart Action = "restart"
)

// LogStream은 원격 tail 세션과 SSH 연결의 생명주기를 함께 관리한다.
type LogStream struct {
	Reader  io.Reader
	closeFn func() error
}

func (s *LogStream) Close() error {
	if s == nil || s.closeFn == nil {
		return nil
	}
	return s.closeFn()
}
