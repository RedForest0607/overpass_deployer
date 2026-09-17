package docker

import (
	"errors"
	"strconv"
	"strings"
	"testing"

	"go-deployer/internal/config"
)

func TestCanonicalSpecMatchesManagedDockerInspect(t *testing.T) {
	cfg := dockerConfig()
	container := cfg.Servers[0].Containers[0]
	desiredImage := cfg.Docker.Registry.ImageReference(container)
	imageID := "sha256:" + strings.Repeat("b", 64)
	runner := &functionalRemoteRunner{run: func(command string) (string, error) {
		if strings.HasPrefix(command, "docker container inspect '") {
			return managedInspectJSON(container, false), nil
		}
		return "", errors.New("unexpected command")
	}}

	actual, managed, err := inspectActualContainerSpec(runner, container.Name, desiredImage, containerImageState{
		Exists: true, ImageID: imageID, RepoDigests: []string{desiredImage},
	})
	if err != nil {
		t.Fatal(err)
	}
	same, err := sameCanonicalSpec(desiredContainerSpec(cfg.Docker.Registry, container), actual)
	if err != nil || !managed || !same {
		t.Fatalf("managed=%t same=%t err=%v actual=%#v", managed, same, err, actual)
	}
}

func TestCreateContainerPassesEnvironmentOnlyThroughStdin(t *testing.T) {
	cfg := dockerConfig()
	container := cfg.Servers[0].Containers[0]
	container.HealthTimeoutSec = 1
	desiredImage := cfg.Docker.Registry.ImageReference(container)
	runner := &functionalRemoteRunner{run: func(command string) (string, error) {
		switch {
		case strings.HasPrefix(command, "docker container start "):
			return "commerce", nil
		case strings.Contains(command, "{{json .State}}"):
			return `{"Running":true,"Status":"running"}`, nil
		}
		return "", errors.New("unexpected command")
	}}

	if err := createAndVerifyContainer(runner, container, desiredImage); err != nil {
		t.Fatal(err)
	}
	commands := strings.Join(runner.commands, " ")
	if strings.Contains(commands, "top-secret") || strings.Contains(commands, "SPRING_PROFILES_ACTIVE=prod") {
		t.Fatalf("environment value leaked into command: %q", commands)
	}
	if !strings.Contains(runner.stdinValue, "DB_PASSWORD=top-secret\n") || !strings.Contains(runner.stdinValue, "SPRING_PROFILES_ACTIVE=prod\n") {
		t.Fatalf("stdin=%q", runner.stdinValue)
	}
	if !strings.Contains(commands, "--env-file") || !strings.Contains(commands, managedLabel+"=true") {
		t.Fatalf("create command=%q", commands)
	}
}

func TestRunFullApplyPreparesAndCreatesMissingContainer(t *testing.T) {
	cfg := dockerConfig()
	container := cfg.Servers[0].Containers[0]
	container.HealthTimeoutSec = 1
	cfg.Servers[0].Containers[0] = container
	desiredImage := cfg.Docker.Registry.ImageReference(container)
	imageID := "sha256:" + strings.Repeat("c", 64)
	runner := &functionalRemoteRunner{run: func(command string) (string, error) {
		switch {
		case command == dockerVersionCommand:
			return "28.4.0", nil
		case strings.HasPrefix(command, "docker container ls "):
			return "", nil
		case strings.HasPrefix(command, "docker image ls "):
			return desiredImage + " " + imageID, nil
		case strings.HasPrefix(command, "docker image inspect "):
			return `["` + desiredImage + `"]`, nil
		case strings.HasPrefix(command, "docker container start "):
			return container.Name, nil
		case strings.Contains(command, "{{json .State}}"):
			return `{"Running":true,"Status":"running"}`, nil
		}
		return "", errors.New("unexpected command")
	}}
	useFakeDockerSSH(t, runner)
	t.Setenv("HARBOR_TOKEN", "registry-secret")

	if err := Run(cfg, Options{}); err != nil {
		t.Fatal(err)
	}
	commands := strings.Join(runner.commands, " ")
	if !strings.Contains(commands, "docker container create") || strings.Contains(commands, "docker pull ") {
		t.Fatalf("commands=%q", runner.commands)
	}
	if strings.Contains(commands, "top-secret") || !strings.Contains(runner.stdinValue, "DB_PASSWORD=top-secret") {
		t.Fatalf("unsafe environment handling commands=%q stdin=%q", runner.commands, runner.stdinValue)
	}
}

func TestReconcileSkipsHealthyMatchingContainer(t *testing.T) {
	cfg := dockerConfig()
	container := cfg.Servers[0].Containers[0]
	container.HealthTimeoutSec = 1
	desiredImage := cfg.Docker.Registry.ImageReference(container)
	imageID := "sha256:" + strings.Repeat("b", 64)
	runner := &functionalRemoteRunner{run: func(command string) (string, error) {
		switch {
		case strings.HasPrefix(command, "docker container ls "):
			return container.Name, nil
		case strings.Contains(command, "--format '{{.Image}}'"):
			return imageID, nil
		case strings.HasPrefix(command, "docker image inspect "):
			return `["` + desiredImage + `"]`, nil
		case command == "docker container inspect '"+container.Name+"'":
			return managedInspectJSON(container, false), nil
		case strings.Contains(command, "{{json .State}}"):
			return `{"Running":true,"Status":"running","Health":{"Status":"healthy"}}`, nil
		case strings.Contains(command, " create "), strings.Contains(command, " stop "), strings.Contains(command, " rename "):
			t.Fatalf("matching container must not be replaced: %s", command)
		}
		return "", errors.New("unexpected command")
	}}

	if err := reconcileContainer(runner, "app-1", cfg.Docker.Registry, container); err != nil {
		t.Fatal(err)
	}
	if runner.stdinValue != "" {
		t.Fatalf("create stdin must remain unused: %q", runner.stdinValue)
	}
}

func TestReconcileRestoresPreviousContainerWhenReplacementIsUnhealthy(t *testing.T) {
	cfg := dockerConfig()
	container := cfg.Servers[0].Containers[0]
	container.HealthTimeoutSec = 1
	imageID := "sha256:" + strings.Repeat("b", 64)
	stateCalls := 0
	runner := &functionalRemoteRunner{run: func(command string) (string, error) {
		switch {
		case strings.HasPrefix(command, "docker container ls "):
			if strings.Contains(command, backupContainerSuffix) {
				return "", nil
			}
			return container.Name, nil
		case strings.Contains(command, "--format '{{.Image}}'"):
			return imageID, nil
		case strings.HasPrefix(command, "docker image inspect "):
			return `["harbor.example.com/team/commerce@sha256:` + strings.Repeat("d", 64) + `"]`, nil
		case command == "docker container inspect '"+container.Name+"'":
			return managedInspectJSON(container, true), nil
		case strings.Contains(command, "{{json .State}}"):
			stateCalls++
			if stateCalls == 1 {
				return `{"Running":true,"Status":"running"}`, nil
			}
			return `{"Running":true,"Status":"running","Health":{"Status":"unhealthy"}}`, nil
		case strings.HasPrefix(command, "docker container stop "),
			strings.HasPrefix(command, "docker container rename "),
			strings.HasPrefix(command, "docker container start "),
			strings.HasPrefix(command, "docker container rm --force "):
			return "", nil
		}
		return "", errors.New("unexpected command")
	}}

	err := reconcileContainer(runner, "app-1", cfg.Docker.Registry, container)
	if err == nil || !strings.Contains(err.Error(), "previous container restored") {
		t.Fatalf("error=%v", err)
	}
	for _, expected := range []string{
		"docker container stop 'commerce'",
		"docker container rename 'commerce' 'commerce-crosswalk-backup'",
		"docker container rm --force 'commerce'",
		"docker container rename 'commerce-crosswalk-backup' 'commerce'",
		"docker container start 'commerce'",
	} {
		if !commandContains(runner.commands, expected) {
			t.Fatalf("missing command %q in %q", expected, runner.commands)
		}
	}
	if strings.Contains(strings.Join(runner.commands, " "), "top-secret") || !strings.Contains(runner.stdinValue, "DB_PASSWORD=top-secret") {
		t.Fatalf("unsafe secret handling commands=%q stdin=%q", runner.commands, runner.stdinValue)
	}
}

func TestReconcileRefusesExistingBackupBeforeStoppingCurrentContainer(t *testing.T) {
	cfg := dockerConfig()
	container := cfg.Servers[0].Containers[0]
	desiredImage := cfg.Docker.Registry.ImageReference(container)
	imageID := "sha256:" + strings.Repeat("b", 64)
	runner := &functionalRemoteRunner{run: func(command string) (string, error) {
		switch {
		case strings.HasPrefix(command, "docker container ls "):
			if strings.Contains(command, backupContainerSuffix) {
				return container.Name + backupContainerSuffix, nil
			}
			return container.Name, nil
		case strings.Contains(command, "--format '{{.Image}}'"):
			return imageID, nil
		case strings.HasPrefix(command, "docker image inspect "):
			return `["` + desiredImage + `"]`, nil
		case command == "docker container inspect '"+container.Name+"'":
			return managedInspectJSON(container, true), nil
		case strings.HasPrefix(command, "docker container stop "):
			t.Fatal("current container must not stop while a backup already exists")
		}
		return "", errors.New("unexpected command")
	}}

	err := reconcileContainer(runner, "app-1", cfg.Docker.Registry, container)
	if err == nil || !strings.Contains(err.Error(), "backup container") {
		t.Fatalf("error=%v", err)
	}
}

func managedInspectJSON(container config.ContainerConfig, differentPort bool) string {
	hostPort := container.Ports[0].Host
	if differentPort {
		hostPort++
	}
	return `[{"Config":{"Env":["DB_PASSWORD=top-secret","SPRING_PROFILES_ACTIVE=prod"],"Labels":{"` +
		managedLabel + `":"true","` + environmentKeysLabel + `":"DB_PASSWORD,SPRING_PROFILES_ACTIVE"}},` +
		`"HostConfig":{"PortBindings":{"8080/tcp":[{"HostIp":"0.0.0.0","HostPort":"` +
		fmtInt(hostPort) + `"},{"HostIp":"::","HostPort":"` + fmtInt(hostPort) + `"}]},` +
		`"RestartPolicy":{"Name":"unless-stopped"}}}]`
}

func fmtInt(value int) string {
	return strconv.Itoa(value)
}
