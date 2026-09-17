package docker

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"

	"go-deployer/internal/config"
	"go-deployer/pkg/logger"
)

type fakeRemoteRunner struct {
	runOutput   string
	runErr      error
	stdinErr    error
	commands    []string
	stdinValues []string
	closed      bool
}

func (f *fakeRemoteRunner) Run(command string) (string, error) {
	f.commands = append(f.commands, command)
	return f.runOutput, f.runErr
}

func (f *fakeRemoteRunner) RunWithStdin(command string, input io.Reader) (string, error) {
	f.commands = append(f.commands, command)
	value, err := io.ReadAll(input)
	if err != nil {
		return "", err
	}
	f.stdinValues = append(f.stdinValues, string(value))
	return "", f.stdinErr
}

func (f *fakeRemoteRunner) Close() error {
	f.closed = true
	return nil
}

func useFakeDockerSSH(t *testing.T, runner remoteRunner) *int {
	t.Helper()
	original := connectDockerSSH
	connections := 0
	connectDockerSSH = func(config.SSHConfig, string) (remoteRunner, error) {
		connections++
		return runner, nil
	}
	t.Cleanup(func() { connectDockerSSH = original })
	return &connections
}

func TestRunPreflightUsesStdinAndRedactsToken(t *testing.T) {
	const token = "  super-secret  "
	t.Setenv("HARBOR_TOKEN", token)
	runner := &fakeRemoteRunner{runOutput: "28.4.0\n"}
	connections := useFakeDockerSSH(t, runner)

	var output bytes.Buffer
	restore := logger.SetOutput(&output)
	t.Cleanup(restore)

	if err := Run(dockerConfig(), Options{Preflight: true}); err != nil {
		t.Fatal(err)
	}
	if *connections != 1 || !runner.closed {
		t.Fatalf("connections=%d closed=%t", *connections, runner.closed)
	}
	if len(runner.commands) != 2 || runner.commands[0] != dockerVersionCommand {
		t.Fatalf("commands=%q", runner.commands)
	}
	if strings.Contains(strings.Join(runner.commands, " "), "super-secret") {
		t.Fatalf("token leaked into command: %q", runner.commands)
	}
	if len(runner.stdinValues) != 1 || runner.stdinValues[0] != token+"\n" {
		t.Fatalf("stdin=%q", runner.stdinValues)
	}
	if strings.Contains(output.String(), "super-secret") || strings.Contains(output.String(), "28.4.0") {
		t.Fatalf("sensitive remote data leaked into log: %q", output.String())
	}
}

func TestRunPreflightRejectsMissingTokenBeforeSSH(t *testing.T) {
	t.Setenv("HARBOR_TOKEN", "   ")
	connections := useFakeDockerSSH(t, &fakeRemoteRunner{runOutput: "28.4.0"})

	err := Run(dockerConfig(), Options{Preflight: true})
	if err == nil || !strings.Contains(err.Error(), "HARBOR_TOKEN") {
		t.Fatalf("error=%v", err)
	}
	if *connections != 0 {
		t.Fatalf("SSH must not run without a token, connections=%d", *connections)
	}
}

func TestRunPreflightReportsDockerAccessFailureWithoutLogin(t *testing.T) {
	t.Setenv("HARBOR_TOKEN", "secret")
	runner := &fakeRemoteRunner{runErr: errors.New("permission denied")}
	useFakeDockerSSH(t, runner)

	err := Run(dockerConfig(), Options{Preflight: true})
	if err == nil || !strings.Contains(err.Error(), "Docker daemon is unavailable") {
		t.Fatalf("error=%v", err)
	}
	if len(runner.commands) != 1 || len(runner.stdinValues) != 0 {
		t.Fatalf("registry login must not run after Docker failure: commands=%q stdin=%q", runner.commands, runner.stdinValues)
	}
}

func TestRunPreflightRedactsRegistryFailure(t *testing.T) {
	const token = "registry-secret"
	t.Setenv("HARBOR_TOKEN", token)
	runner := &fakeRemoteRunner{runOutput: "28.4.0", stdinErr: errors.New("remote echoed registry-secret")}
	useFakeDockerSSH(t, runner)

	err := Run(dockerConfig(), Options{Preflight: true})
	if err == nil || !strings.Contains(err.Error(), "registry login to harbor.example.com failed") {
		t.Fatalf("error=%v", err)
	}
	if strings.Contains(err.Error(), token) {
		t.Fatalf("token leaked into error: %v", err)
	}
}

func TestRunRejectsMutuallyExclusiveModes(t *testing.T) {
	err := Run(dockerConfig(), Options{DryRun: true, Preflight: true})
	if err == nil || !strings.Contains(err.Error(), "only one") {
		t.Fatalf("error=%v", err)
	}
}
