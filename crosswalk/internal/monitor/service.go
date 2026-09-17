package monitor

import (
	"fmt"
	"strings"
	"time"

	"go-deployer/internal/config"
	"go-deployer/internal/ssh"
)

type Service struct {
	connect func(config.SSHConfig, string) (*ssh.Client, error)
}

func NewService() Service {
	return Service{connect: func(sshCfg config.SSHConfig, host string) (*ssh.Client, error) {
		return ssh.Connect(sshCfg.User, host, sshCfg.KeyPath, sshCfg.HostKeyChecking, sshCfg.KnownHosts, sshCfg.Port, sshCfg.TimeoutSec)
	}}
}

// Collect은 설정된 모든 앱의 프로세스 상태를 읽기 전용으로 조회한다.
func (s Service) Collect(cfg *config.Config) []AppStatus {
	statuses := make([]AppStatus, 0)
	for serverIndex, server := range cfg.Servers {
		apps := server.EffectiveApps()
		client, err := s.connect(server.SSHSettings(cfg.SSH), server.Host)
		if err != nil {
			for appIndex, app := range apps {
				statuses = append(statuses, unavailableStatus(serverIndex, appIndex, server, app, err))
			}
			continue
		}
		for appIndex, app := range apps {
			statuses = append(statuses, s.inspect(client, Target{ServerIndex: serverIndex, AppIndex: appIndex}, server, app))
		}
		_ = client.Close()
	}
	return statuses
}

func (s Service) inspect(client *ssh.Client, target Target, server config.ServerConfig, app config.AppConfig) AppStatus {
	status := AppStatus{Target: target, ServerName: server.Name, Host: server.Host, AppName: app.Name, Port: app.Port, ObservedAt: time.Now()}
	output, err := client.Run(statusCommand(app))
	if err != nil {
		status.State = StateUnavailable
		status.Message = fmt.Sprintf("status query failed: %v", err)
		return status
	}
	parsed := parseStatus(output)
	status.State, status.PID, status.Process, status.Message = parsed.State, parsed.PID, parsed.Process, parsed.Message
	return status
}

func unavailableStatus(serverIndex, appIndex int, server config.ServerConfig, app config.AppConfig, err error) AppStatus {
	return AppStatus{Target: Target{ServerIndex: serverIndex, AppIndex: appIndex}, ServerName: server.Name, Host: server.Host, AppName: app.Name, Port: app.Port, State: StateUnavailable, Message: fmt.Sprintf("SSH connection failed: %v", err), ObservedAt: time.Now()}
}

// Control은 start, stop, restart 허용 목록에 있는 원격 스크립트 동작만 실행한다.
func (s Service) Control(cfg *config.Config, target Target, action Action) (string, error) {
	if action != ActionStart && action != ActionStop && action != ActionRestart {
		return "", fmt.Errorf("unsupported control action %q", action)
	}
	server, app, err := resolveTarget(cfg, target)
	if err != nil {
		return "", err
	}
	client, err := s.connect(server.SSHSettings(cfg.SSH), server.Host)
	if err != nil {
		return "", fmt.Errorf("connect to %s: %w", server.Name, err)
	}
	defer client.Close()
	output, err := client.Run(controlCommand(app.Script.RemotePath, action))
	if err != nil {
		return output, fmt.Errorf("%s %s: %w", action, app.Name, err)
	}
	return output, nil
}

// OpenLog은 선택한 앱의 설정된 로그 파일을 최근 200줄부터 읽기 전용으로 이어서 읽는다.
func (s Service) OpenLog(cfg *config.Config, target Target) (*LogStream, error) {
	server, app, err := resolveTarget(cfg, target)
	if err != nil {
		return nil, err
	}
	client, err := s.connect(server.SSHSettings(cfg.SSH), server.Host)
	if err != nil {
		return nil, fmt.Errorf("connect to %s: %w", server.Name, err)
	}
	session, reader, err := client.Start(logCommand(logPath(app)))
	if err != nil {
		_ = client.Close()
		return nil, err
	}
	return &LogStream{Reader: reader, closeFn: func() error {
		_ = session.Close()
		return client.Close()
	}}, nil
}

func resolveTarget(cfg *config.Config, target Target) (config.ServerConfig, config.AppConfig, error) {
	if target.ServerIndex < 0 || target.ServerIndex >= len(cfg.Servers) {
		return config.ServerConfig{}, config.AppConfig{}, fmt.Errorf("server selection is out of range")
	}
	server := cfg.Servers[target.ServerIndex]
	apps := server.EffectiveApps()
	if target.AppIndex < 0 || target.AppIndex >= len(apps) {
		return config.ServerConfig{}, config.AppConfig{}, fmt.Errorf("application selection is out of range")
	}
	if strings.TrimSpace(apps[target.AppIndex].Script.RemotePath) == "" {
		return config.ServerConfig{}, config.AppConfig{}, fmt.Errorf("app %s has no remote control script", apps[target.AppIndex].Name)
	}
	return server, apps[target.AppIndex], nil
}

func statusCommand(app config.AppConfig) string {
	settings := settingsFor(app)
	inner := strings.Join([]string{
		"script=" + ssh.ShellQuote(app.Script.RemotePath),
		"process_match=" + ssh.ShellQuote(settings.ProcessMatch),
		"if [ ! -x \"$script\" ]; then printf 'state=unknown\\nmessage=control script is not executable\\n'; exit 0; fi",
		"pid=$(ps -eo pid=,comm=,args= | awk -v match=\"$process_match\" '$2 == \"java\" && index($0, match) { print $1; exit }')",
		"if [ -z \"$pid\" ]; then printf 'state=stopped\\n'; exit 0; fi",
		"printf 'state=running\\npid=%s\\n' \"$pid\"",
		"ps -p \"$pid\" -o etime= -o %cpu= -o %mem= -o command= | head -n 1 | sed 's/^/process=/'",
	}, "; ")
	return "sh -lc " + ssh.ShellQuote(inner)
}

func controlCommand(scriptPath string, action Action) string {
	inner := "script=" + ssh.ShellQuote(scriptPath) + "; test -x \"$script\" && \"$script\" " + ssh.ShellQuote(string(action))
	return "sh -lc " + ssh.ShellQuote(inner)
}

func logCommand(logFilePath string) string {
	inner := "log_file=" + ssh.ShellQuote(logFilePath) + "; if [ ! -f \"$log_file\" ]; then printf 'log file does not exist: %s\\n' \"$log_file\" >&2; exit 1; fi; exec tail -n 200 -F \"$log_file\" 2>&1"
	return "sh -lc " + ssh.ShellQuote(inner)
}

func logPath(app config.AppConfig) string {
	return settingsFor(app).LogPath
}

type parsedStatus struct {
	State   State
	PID     string
	Process string
	Message string
}

func parseStatus(output string) parsedStatus {
	status := parsedStatus{State: StateUnknown}
	for _, line := range strings.Split(output, "\n") {
		key, value, found := strings.Cut(line, "=")
		if !found {
			continue
		}
		switch key {
		case "state":
			switch State(value) {
			case StateRunning, StateStopped, StateUnknown:
				status.State = State(value)
			}
		case "pid":
			status.PID = value
		case "process":
			status.Process = strings.TrimSpace(value)
		case "message":
			status.Message = value
		}
	}
	return status
}
