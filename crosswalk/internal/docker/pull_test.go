package docker

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"

	"go-deployer/pkg/logger"
)

type functionalRemoteRunner struct {
	run        func(string) (string, error)
	commands   []string
	stdinValue string
	closed     bool
}

func (f *functionalRemoteRunner) Run(command string) (string, error) {
	f.commands = append(f.commands, command)
	return f.run(command)
}

func (f *functionalRemoteRunner) RunWithStdin(command string, input io.Reader) (string, error) {
	f.commands = append(f.commands, command)
	value, err := io.ReadAll(input)
	if err != nil {
		return "", err
	}
	f.stdinValue = string(value)
	return "", nil
}

func (f *functionalRemoteRunner) Close() error {
	f.closed = true
	return nil
}

func TestRunPullSkipsImageAlreadyAvailableByDigest(t *testing.T) {
	cfg := dockerConfig()
	desired := cfg.Docker.Registry.ImageReference(cfg.Servers[0].Containers[0])
	imageID := "sha256:" + strings.Repeat("b", 64)
	runner := &functionalRemoteRunner{}
	runner.run = func(command string) (string, error) {
		switch {
		case command == dockerVersionCommand:
			return "28.4.0", nil
		case strings.HasPrefix(command, "docker container ls "):
			return "commerce\n", nil
		case strings.HasPrefix(command, "docker container inspect "):
			return imageID + "\n", nil
		case strings.HasPrefix(command, "docker image ls "):
			return desired + " " + imageID + "\n", nil
		case strings.HasPrefix(command, "docker image inspect "):
			return `["` + desired + `"]`, nil
		case strings.HasPrefix(command, "docker pull "):
			t.Fatal("pull must be skipped when desired digest is local")
		}
		return "", errors.New("unexpected command")
	}
	useFakeDockerSSH(t, runner)
	t.Setenv("HARBOR_TOKEN", "secret")

	var output bytes.Buffer
	restore := logger.SetOutput(&output)
	t.Cleanup(restore)

	if err := Run(cfg, Options{Pull: true}); err != nil {
		t.Fatal(err)
	}
	if !runner.closed || runner.stdinValue != "secret\n" {
		t.Fatalf("closed=%t stdin=%q", runner.closed, runner.stdinValue)
	}
	if !strings.Contains(output.String(), "skipping pull") || !strings.Contains(output.String(), "already uses") {
		t.Fatalf("output=%q", output.String())
	}
}

func TestRunPullFetchesMissingImageAndVerifiesDigest(t *testing.T) {
	cfg := dockerConfig()
	desired := cfg.Docker.Registry.ImageReference(cfg.Servers[0].Containers[0])
	imageID := "sha256:" + strings.Repeat("c", 64)
	pulled := false
	runner := &functionalRemoteRunner{}
	runner.run = func(command string) (string, error) {
		switch {
		case command == dockerVersionCommand:
			return "28.4.0", nil
		case strings.HasPrefix(command, "docker container ls "):
			return "", nil
		case strings.HasPrefix(command, "docker image ls "):
			if pulled {
				return desired + " " + imageID + "\n", nil
			}
			return "", nil
		case strings.HasPrefix(command, "docker image inspect "):
			return `["` + desired + `"]`, nil
		case strings.HasPrefix(command, "docker pull "):
			pulled = true
			return "pull output is intentionally ignored", nil
		}
		return "", errors.New("unexpected command")
	}
	useFakeDockerSSH(t, runner)
	t.Setenv("HARBOR_TOKEN", "secret")

	if err := Run(cfg, Options{Pull: true}); err != nil {
		t.Fatal(err)
	}
	if !pulled {
		t.Fatal("expected digest-pinned image pull")
	}
	if !commandContains(runner.commands, "docker pull '"+desired+"'") {
		t.Fatalf("commands=%q", runner.commands)
	}
}

func TestRunPullRejectsDigestMismatchAfterPull(t *testing.T) {
	cfg := dockerConfig()
	desired := cfg.Docker.Registry.ImageReference(cfg.Servers[0].Containers[0])
	wrong := "harbor.example.com/team/commerce@sha256:" + strings.Repeat("d", 64)
	imageID := "sha256:" + strings.Repeat("c", 64)
	pulled := false
	runner := &functionalRemoteRunner{}
	runner.run = func(command string) (string, error) {
		switch {
		case command == dockerVersionCommand:
			return "28.4.0", nil
		case strings.HasPrefix(command, "docker container ls "):
			return "", nil
		case strings.HasPrefix(command, "docker image ls "):
			if pulled {
				return desired + " " + imageID, nil
			}
			return "", nil
		case strings.HasPrefix(command, "docker image inspect "):
			return `["` + wrong + `"]`, nil
		case strings.HasPrefix(command, "docker pull "):
			pulled = true
			return "", nil
		}
		return "", errors.New("unexpected command")
	}
	useFakeDockerSSH(t, runner)
	t.Setenv("HARBOR_TOKEN", "secret")

	err := Run(cfg, Options{Pull: true})
	if err == nil || !strings.Contains(err.Error(), "does not match the requested digest") {
		t.Fatalf("error=%v", err)
	}
}

func TestRunPullRejectsInvalidContainerImageIDBeforePull(t *testing.T) {
	cfg := dockerConfig()
	runner := &functionalRemoteRunner{}
	runner.run = func(command string) (string, error) {
		switch {
		case command == dockerVersionCommand:
			return "28.4.0", nil
		case strings.HasPrefix(command, "docker container ls "):
			return "commerce", nil
		case strings.HasPrefix(command, "docker container inspect "):
			return "unexpected-image-id", nil
		case strings.HasPrefix(command, "docker image ls "), strings.HasPrefix(command, "docker pull "):
			t.Fatal("local image mutation must not run after invalid container state")
		}
		return "", errors.New("unexpected command")
	}
	useFakeDockerSSH(t, runner)
	t.Setenv("HARBOR_TOKEN", "secret")

	err := Run(cfg, Options{Pull: true})
	if err == nil || !strings.Contains(err.Error(), "invalid image ID") {
		t.Fatalf("error=%v", err)
	}
}

func TestRunPullRedactsRemotePullFailure(t *testing.T) {
	const token = "pull-secret"
	cfg := dockerConfig()
	runner := &functionalRemoteRunner{}
	runner.run = func(command string) (string, error) {
		switch {
		case command == dockerVersionCommand:
			return "28.4.0", nil
		case strings.HasPrefix(command, "docker container ls "), strings.HasPrefix(command, "docker image ls "):
			return "", nil
		case strings.HasPrefix(command, "docker pull "):
			return token, errors.New("registry echoed " + token)
		}
		return "", errors.New("unexpected command")
	}
	useFakeDockerSSH(t, runner)
	t.Setenv("HARBOR_TOKEN", token)

	err := Run(cfg, Options{Pull: true})
	if err == nil || !strings.Contains(err.Error(), "pulling image") {
		t.Fatalf("error=%v", err)
	}
	if strings.Contains(err.Error(), token) {
		t.Fatalf("token leaked into error: %v", err)
	}
}

func commandContains(commands []string, expected string) bool {
	for _, command := range commands {
		if command == expected {
			return true
		}
	}
	return false
}
