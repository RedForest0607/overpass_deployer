package monitor

import (
	"errors"
	"strings"
	"testing"

	"go-deployer/internal/config"
	"go-deployer/internal/ssh"
)

func TestParseStatus(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  parsedStatus
	}{
		{"running process", "state=running\npid=123\nprocess=00:10 1.2 3.4 java -jar sample.jar\n", parsedStatus{State: StateRunning, PID: "123", Process: "00:10 1.2 3.4 java -jar sample.jar"}},
		{"stopped process", "state=stopped\n", parsedStatus{State: StateStopped}},
		{"missing script", "state=unknown\nmessage=control script is not executable\n", parsedStatus{State: StateUnknown, Message: "control script is not executable"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if actual := parseStatus(test.input); actual != test.want {
				t.Fatalf("parseStatus() = %#v, want %#v", actual, test.want)
			}
		})
	}
}

func TestCollectTargetRetriesOnlySelectedServerAndPreservesIdentity(t *testing.T) {
	cfg := &config.Config{SSH: config.SSHConfig{Port: 22}, Servers: []config.ServerConfig{
		{Name: "healthy-server", Host: "healthy", Apps: []config.AppConfig{{Name: "first", Script: config.ScriptConfig{RemotePath: "/app/first.sh"}}}},
		{Name: "failed-server", Host: "failed", SSHPort: 2222, Apps: []config.AppConfig{{Name: "second", Script: config.ScriptConfig{RemotePath: "/app/second.sh"}}}},
	}}
	calls := 0
	service := Service{connect: func(settings config.SSHConfig, host string) (*ssh.Client, error) {
		calls++
		if host != "failed" || settings.Port != 2222 {
			t.Fatalf("wrong retry endpoint: %s:%d", host, settings.Port)
		}
		return nil, errors.New("connection timed out")
	}}
	target := Target{ServerIndex: 1, AppIndex: 0}
	status, err := service.CollectTarget(cfg, target)
	if err != nil || calls != 1 || status.Target != target || status.State != StateUnavailable || status.AppName != "second" {
		t.Fatalf("selected retry returned wrong identity/state: %#v, %v", status, err)
	}
	if _, err := service.CollectTarget(cfg, Target{ServerIndex: 5}); err == nil || calls != 1 {
		t.Fatal("invalid retry target must fail before connecting")
	}
}

func TestStatusCommandEscapesConfiguredValues(t *testing.T) {
	command := statusCommand(config.AppConfig{
		Name:    "app-default",
		Monitor: config.MonitorConfig{ProcessMatch: "app'unsafe"},
		Script:  config.ScriptConfig{RemotePath: "/app/example'unsafe/server.sh"},
	})
	if !strings.HasPrefix(command, "sh -lc '") || !strings.Contains(command, "'\"'\"'") {
		t.Fatalf("statusCommand() did not safely quote configured values: %q", command)
	}
	if strings.Contains(command, "pgrep") || !strings.Contains(command, "comm=") || !strings.Contains(command, "$2 == \"java\"") {
		t.Fatalf("statusCommand() must inspect actual java executables without matching its own shell: %q", command)
	}
}

func TestSettingsForUsesDefaultsAndOverrides(t *testing.T) {
	app := config.AppConfig{Name: "sample-api", BaseDir: "/app/sample-api"}
	if actual := settingsFor(app); actual.ProcessMatch != "sample-api" || actual.LogPath != "/app/sample-api/logs/sample-api.log" {
		t.Fatalf("settingsFor(defaults) = %#v", actual)
	}

	app.Monitor = config.MonitorConfig{ProcessMatch: "sample-api.jar", LogPath: "/var/log/apps/sample.log"}
	if actual := settingsFor(app); actual.ProcessMatch != "sample-api.jar" || actual.LogPath != "/var/log/apps/sample.log" {
		t.Fatalf("settingsFor(overrides) = %#v", actual)
	}
}

func TestLogCommandIsReadOnlyAndQuotesPath(t *testing.T) {
	command := logCommand("/var/log/app's/output.log")
	if strings.Contains(command, "mkdir") || strings.Contains(command, "touch") {
		t.Fatalf("logCommand() must not create or mutate remote files: %q", command)
	}
	if !strings.Contains(command, "test") && !strings.Contains(command, "[ ! -f") {
		t.Fatalf("logCommand() must reject a missing file: %q", command)
	}
	if !strings.Contains(command, "'\"'\"'") {
		t.Fatalf("logCommand() did not safely quote the configured path: %q", command)
	}
}

func TestResolveTargetRejectsInvalidSelectionAndMissingScript(t *testing.T) {
	cfg := &config.Config{Servers: []config.ServerConfig{{Name: "server-a", Apps: []config.AppConfig{{Name: "app-a"}}}}}
	if _, _, err := resolveTarget(cfg, Target{ServerIndex: 1, AppIndex: 0}); err == nil {
		t.Fatal("expected an out-of-range server error")
	}
	if _, _, err := resolveTarget(cfg, Target{ServerIndex: 0, AppIndex: 0}); err == nil {
		t.Fatal("expected a missing control script error")
	}
}
