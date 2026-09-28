package compose

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"go-deployer/internal/config"
	"go-deployer/internal/docker"
	"go-deployer/pkg/logger"
)

type fakeRemoteRunner struct {
	commands    []string
	stdinValues []string
	closed      bool
	composeErr  error
	loginErr    error
	runHook     func(string) (string, error)
}

func (f *fakeRemoteRunner) Run(command string) (string, error) {
	f.commands = append(f.commands, command)
	if f.runHook != nil {
		return f.runHook(command)
	}
	switch command {
	case "docker version --format '{{.Server.Version}}'":
		return "28.4.0\n", nil
	case composeVersionCommand:
		if f.composeErr != nil {
			return "", f.composeErr
		}
		return "2.29.7\n", nil
	default:
		return "", nil
	}
}

func (f *fakeRemoteRunner) RunWithStdin(command string, input io.Reader) (string, error) {
	f.commands = append(f.commands, command)
	value, err := io.ReadAll(input)
	if err != nil {
		return "", err
	}
	f.stdinValues = append(f.stdinValues, string(value))
	return "", f.loginErr
}

func (f *fakeRemoteRunner) Close() error {
	f.closed = true
	return nil
}

func useFakeComposeSSH(t *testing.T, runner docker.RemoteRunner) *int {
	t.Helper()
	original := connectComposeSSH
	connections := 0
	connectComposeSSH = func(config.SSHConfig, string) (docker.RemoteRunner, error) {
		connections++
		return runner, nil
	}
	t.Cleanup(func() { connectComposeSSH = original })
	return &connections
}

func useFakeComposeSync(t *testing.T, changed bool, callback func(config.ComposeProjectConfig)) {
	t.Helper()
	original := syncComposeFile
	syncComposeFile = func(_ docker.RemoteRunner, project config.ComposeProjectConfig) (string, bool, error) {
		if callback != nil {
			callback(project)
		}
		return project.RemoteDir + "/compose.yml", changed, nil
	}
	t.Cleanup(func() { syncComposeFile = original })
}

func TestRunPreflightVerifiesComposeAndUsesRegistryStdin(t *testing.T) {
	const token = "  compose-secret  "
	t.Setenv("HARBOR_TOKEN", token)
	runner := &fakeRemoteRunner{}
	connections := useFakeComposeSSH(t, runner)

	var output bytes.Buffer
	restore := logger.SetOutput(&output)
	t.Cleanup(restore)

	if err := Run(composeConfig(), Options{Preflight: true}); err != nil {
		t.Fatal(err)
	}
	if *connections != 1 || !runner.closed {
		t.Fatalf("connections=%d closed=%t", *connections, runner.closed)
	}
	if len(runner.commands) != 3 || runner.commands[1] != composeVersionCommand {
		t.Fatalf("commands=%q", runner.commands)
	}
	if len(runner.stdinValues) != 1 || runner.stdinValues[0] != token+"\n" {
		t.Fatalf("stdin=%q", runner.stdinValues)
	}
	if strings.Contains(strings.Join(runner.commands, " "), "compose-secret") || strings.Contains(output.String(), "compose-secret") {
		t.Fatalf("registry token leaked: commands=%q output=%q", runner.commands, output.String())
	}
}

func TestRunPreflightRejectsMissingTokenBeforeSSH(t *testing.T) {
	t.Setenv("HARBOR_TOKEN", "   ")
	connections := useFakeComposeSSH(t, &fakeRemoteRunner{})
	err := Run(composeConfig(), Options{Preflight: true})
	if err == nil || !strings.Contains(err.Error(), "HARBOR_TOKEN") {
		t.Fatalf("error=%v", err)
	}
	if *connections != 0 {
		t.Fatalf("SSH must not run without a token, connections=%d", *connections)
	}
}

func TestRunPreflightStopsBeforeLoginWhenComposeIsUnavailable(t *testing.T) {
	t.Setenv("HARBOR_TOKEN", "secret")
	runner := &fakeRemoteRunner{composeErr: errors.New("not found")}
	useFakeComposeSSH(t, runner)
	err := Run(composeConfig(), Options{Preflight: true})
	if err == nil || !strings.Contains(err.Error(), "Compose v2 is unavailable") {
		t.Fatalf("error=%v", err)
	}
	if len(runner.commands) != 2 || len(runner.stdinValues) != 0 {
		t.Fatalf("registry login must not run after Compose failure: commands=%q stdin=%q", runner.commands, runner.stdinValues)
	}
}

func TestRunSyncTransfersAndValidatesWithoutApplying(t *testing.T) {
	t.Setenv("HARBOR_TOKEN", "secret")
	runner := &fakeRemoteRunner{}
	useFakeComposeSSH(t, runner)
	useFakeComposeSync(t, true, func(project config.ComposeProjectConfig) {
		if project.Name != "commerce" || project.RemoteDir != "/app/overpass/compose/commerce" {
			t.Fatalf("project=%#v", project)
		}
	})

	if err := Run(composeConfig(), Options{Sync: true}); err != nil {
		t.Fatal(err)
	}
	commands := strings.Join(runner.commands, "\n")
	if !strings.Contains(commands, "PODMAN_COMPOSE_WARNING_LOGS=false docker compose --project-name 'commerce' --file '/app/overpass/compose/commerce/compose.yml' config --quiet") {
		t.Fatalf("validation command missing: %q", commands)
	}
	if strings.Contains(commands, " compose up ") || strings.Contains(commands, " compose pull ") {
		t.Fatalf("sync must not apply or pull workloads: %q", commands)
	}
}

func TestRunSyncReportsChecksumSkip(t *testing.T) {
	t.Setenv("HARBOR_TOKEN", "secret")
	useFakeComposeSSH(t, &fakeRemoteRunner{})
	useFakeComposeSync(t, false, nil)
	var output bytes.Buffer
	restore := logger.SetOutput(&output)
	t.Cleanup(restore)

	if err := Run(composeConfig(), Options{Sync: true}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "skipped compose-dev-01") {
		t.Fatalf("output=%q", output.String())
	}
}

func TestRunRedactsRegistryLoginFailure(t *testing.T) {
	const token = "registry-secret"
	t.Setenv("HARBOR_TOKEN", token)
	useFakeComposeSSH(t, &fakeRemoteRunner{loginErr: errors.New("remote echoed " + token)})
	err := Run(composeConfig(), Options{Preflight: true})
	if err == nil || !strings.Contains(err.Error(), "registry login to harbor.example.com failed") {
		t.Fatalf("error=%v", err)
	}
	if strings.Contains(err.Error(), token) {
		t.Fatalf("token leaked into error: %v", err)
	}
}

func TestRunRejectsMutuallyExclusiveModes(t *testing.T) {
	err := Run(composeConfig(), Options{Preflight: true, Sync: true})
	if err == nil || !strings.Contains(err.Error(), "only one") {
		t.Fatalf("error=%v", err)
	}
}

func TestRunContinuesAfterOneServerPreflightFailure(t *testing.T) {
	t.Setenv("HARBOR_TOKEN", "secret")
	cfg := composeConfig()
	cfg.Servers = append(cfg.Servers, config.ServerConfig{
		Name: "compose-dev-02", Host: "192.0.2.21",
		ComposeProjects: []config.ComposeProjectConfig{{Name: "worker", LocalPath: "./compose/worker.yml", RemoteDir: "/app/overpass/compose/worker"}},
	})
	first := &fakeRemoteRunner{composeErr: errors.New("plugin missing")}
	second := &fakeRemoteRunner{}
	original := connectComposeSSH
	connections := 0
	connectComposeSSH = func(_ config.SSHConfig, host string) (docker.RemoteRunner, error) {
		connections++
		if host == "192.0.2.20" {
			return first, nil
		}
		return second, nil
	}
	t.Cleanup(func() { connectComposeSSH = original })

	err := Run(cfg, Options{Preflight: true})
	if err == nil || !strings.Contains(err.Error(), "compose-dev-01") {
		t.Fatalf("error=%v", err)
	}
	if connections != 2 || len(second.stdinValues) != 1 || !second.closed {
		t.Fatalf("connections=%d second commands=%q stdin=%q closed=%t", connections, second.commands, second.stdinValues, second.closed)
	}
}

func TestRunPreflightLimitsConcurrencyAndPreservesServerOrder(t *testing.T) {
	set := composeConfig()
	set.Servers = append(set.Servers,
		config.ServerConfig{Name: "second", Host: "192.0.2.21", ComposeProjects: []config.ComposeProjectConfig{{Name: "worker"}}},
		config.ServerConfig{Name: "third", Host: "192.0.2.22", ComposeProjects: []config.ComposeProjectConfig{{Name: "billing"}}},
	)
	var active, maximum atomic.Int32
	original := connectComposeSSH
	connectComposeSSH = func(_ config.SSHConfig, _ string) (docker.RemoteRunner, error) {
		return &fakeRemoteRunner{runHook: func(command string) (string, error) {
			if command == "docker version --format '{{.Server.Version}}'" {
				current := active.Add(1)
				for {
					previous := maximum.Load()
					if current <= previous || maximum.CompareAndSwap(previous, current) {
						break
					}
				}
				time.Sleep(15 * time.Millisecond)
				active.Add(-1)
				return "28.0", nil
			}
			if command == composeVersionCommand {
				return "2.29.7", nil
			}
			return "", nil
		}}, nil
	}
	t.Cleanup(func() { connectComposeSSH = original })
	summary := runServers(set, "secret", Options{Preflight: true, ParallelServers: 2})
	if maximum.Load() != 2 || len(summary.results) != 3 || summary.results[0].name != "compose-dev-01" || summary.results[1].name != "second" || summary.results[2].name != "third" || summary.err() != nil {
		t.Fatalf("maximum=%d results=%#v error=%v", maximum.Load(), summary.results, summary.err())
	}
}
