package docker

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"go-deployer/internal/config"
	"go-deployer/internal/ssh"
	"go-deployer/pkg/logger"
)

const backupContainerSuffix = config.DockerBackupContainerSuffix

// reconcileContainer는 desired spec이 다를 때만 기존 container를 backup 이름으로 보존해 교체한다.
func reconcileContainer(runner remoteRunner, serverName string, registry config.DockerRegistryConfig, container config.ContainerConfig) error {
	_, err := reconcileContainerWithOutcome(runner, serverName, registry, container)
	return err
}

func reconcileContainerWithOutcome(runner remoteRunner, serverName string, registry config.DockerRegistryConfig, container config.ContainerConfig) (bool, error) {
	desiredImage := registry.ImageReference(container)
	currentImage, err := inspectContainerImage(runner, container.Name)
	if err != nil {
		return false, err
	}
	if !currentImage.Exists {
		if err := createAndVerifyContainer(runner, container, desiredImage); err != nil {
			return false, err
		}
		logger.Ok(serverName, "Created container %s", container.Name)
		return true, nil
	}

	desiredSpec := desiredContainerSpec(registry, container)
	actualSpec, managed, err := inspectActualContainerSpec(runner, container.Name, desiredImage, currentImage)
	if err != nil {
		return false, err
	}
	sameSpec, err := sameCanonicalSpec(desiredSpec, actualSpec)
	if err != nil {
		return false, errors.New("failed to compare canonical container specification")
	}
	if managed && sameSpec {
		recovered, err := ensureExistingContainerReady(runner, container)
		if err != nil {
			return false, err
		}
		if recovered {
			logger.Ok(serverName, "Recovered container %s without replacement", container.Name)
			return true, nil
		}
		logger.Skip(serverName, "Container %s already matches desired state", container.Name)
		return false, nil
	}

	if err := replaceContainer(runner, container, desiredImage); err != nil {
		return false, err
	}
	logger.Ok(serverName, "Replaced container %s and removed its backup", container.Name)
	return true, nil
}

func createAndVerifyContainer(runner remoteRunner, container config.ContainerConfig, desiredImage string) error {
	if err := createContainer(runner, container, desiredImage); err != nil {
		return err
	}
	if err := runContainerCommand(runner, "docker container start "+ssh.ShellQuote(container.Name), "starting new container failed"); err != nil {
		if cleanupErr := cleanupCreatedContainer(runner, container.Name); cleanupErr != nil {
			return errors.New("starting new container failed and cleanup was incomplete")
		}
		return err
	}
	if err := waitContainerReady(runner, container.Name, time.Duration(container.HealthTimeoutSec)*time.Second, healthPollInterval); err != nil {
		if cleanupErr := cleanupCreatedContainer(runner, container.Name); cleanupErr != nil {
			return errors.New("new container readiness failed and cleanup was incomplete")
		}
		return fmt.Errorf("new container readiness failed: %w", err)
	}
	return nil
}

func ensureExistingContainerReady(runner remoteRunner, container config.ContainerConfig) (bool, error) {
	state, err := inspectContainerRuntime(runner, container.Name)
	if err != nil {
		return false, err
	}
	command := ""
	if !state.Running {
		command = "docker container start " + ssh.ShellQuote(container.Name)
	} else if state.Health != nil && state.Health.Status == "unhealthy" {
		command = "docker container restart " + ssh.ShellQuote(container.Name)
	}
	if command != "" {
		if err := runContainerCommand(runner, command, "recovering existing container failed"); err != nil {
			return false, err
		}
	}
	if err := waitContainerReady(runner, container.Name, time.Duration(container.HealthTimeoutSec)*time.Second, healthPollInterval); err != nil {
		return false, err
	}
	return command != "", nil
}

func replaceContainer(runner remoteRunner, container config.ContainerConfig, desiredImage string) error {
	backupName := container.Name + backupContainerSuffix
	backupExists, err := containerExists(runner, backupName)
	if err != nil {
		return err
	}
	if backupExists {
		return fmt.Errorf("backup container %s already exists; inspect it before retrying", backupName)
	}

	previousState, err := inspectContainerRuntime(runner, container.Name)
	if err != nil {
		return err
	}
	if previousState.Running {
		if err := runContainerCommand(runner, "docker container stop "+ssh.ShellQuote(container.Name), "stopping previous container failed"); err != nil {
			return err
		}
	}
	if err := runContainerCommand(
		runner,
		"docker container rename "+ssh.ShellQuote(container.Name)+" "+ssh.ShellQuote(backupName),
		"preserving previous container failed",
	); err != nil {
		if previousState.Running {
			if restartErr := runContainerCommand(runner, "docker container start "+ssh.ShellQuote(container.Name), "restarting previous container failed"); restartErr != nil {
				return errors.New("preserving previous container failed and rollback could not restart it")
			}
		}
		return err
	}

	if err := createContainer(runner, container, desiredImage); err != nil {
		return rollbackReplacement(runner, container.Name, backupName, previousState.Running, false, err)
	}
	if err := runContainerCommand(runner, "docker container start "+ssh.ShellQuote(container.Name), "starting replacement container failed"); err != nil {
		return rollbackReplacement(runner, container.Name, backupName, previousState.Running, true, err)
	}
	if err := waitContainerReady(runner, container.Name, time.Duration(container.HealthTimeoutSec)*time.Second, healthPollInterval); err != nil {
		return rollbackReplacement(runner, container.Name, backupName, previousState.Running, true, fmt.Errorf("replacement readiness failed: %w", err))
	}

	if err := runContainerCommand(runner, "docker container rm "+ssh.ShellQuote(backupName), "removing healthy replacement backup failed"); err != nil {
		return fmt.Errorf("replacement is healthy but backup cleanup failed: %s", backupName)
	}
	return nil
}

func rollbackReplacement(runner remoteRunner, name, backupName string, wasRunning, newExists bool, cause error) error {
	if newExists {
		if err := runContainerCommand(runner, "docker container rm --force "+ssh.ShellQuote(name), "removing failed replacement during rollback failed"); err != nil {
			return fmt.Errorf("%v; rollback incomplete and previous container remains at %s", cause, backupName)
		}
	}
	if err := runContainerCommand(
		runner,
		"docker container rename "+ssh.ShellQuote(backupName)+" "+ssh.ShellQuote(name),
		"restoring previous container name failed",
	); err != nil {
		return fmt.Errorf("%v; rollback incomplete and previous container remains at %s", cause, backupName)
	}
	if wasRunning {
		if err := runContainerCommand(runner, "docker container start "+ssh.ShellQuote(name), "restarting previous container during rollback failed"); err != nil {
			return fmt.Errorf("%v; previous container name was restored but restart failed", cause)
		}
	}
	return fmt.Errorf("%v; previous container restored", cause)
}

func createContainer(runner remoteRunner, container config.ContainerConfig, desiredImage string) error {
	keys := sortedEnvironmentKeys(container.Environment)
	arguments := []string{
		"docker container create",
		"--name " + ssh.ShellQuote(container.Name),
		"--restart " + ssh.ShellQuote(container.RestartPolicy),
		"--label " + ssh.ShellQuote(managedLabel+"=true"),
		"--label " + ssh.ShellQuote(environmentKeysLabel+"="+strings.Join(keys, ",")),
		`--env-file "$env_file"`,
	}
	ports := append([]config.ContainerPort(nil), container.Ports...)
	sort.Slice(ports, func(i, j int) bool {
		if ports[i].Host != ports[j].Host {
			return ports[i].Host < ports[j].Host
		}
		if ports[i].Container != ports[j].Container {
			return ports[i].Container < ports[j].Container
		}
		return ports[i].Protocol < ports[j].Protocol
	})
	for _, port := range ports {
		binding := fmt.Sprintf("%d:%d/%s", port.Host, port.Container, port.Protocol)
		arguments = append(arguments, "--publish "+ssh.ShellQuote(binding))
	}
	arguments = append(arguments, ssh.ShellQuote(desiredImage))

	command := `set -eu; env_file=$(mktemp); trap 'rm -f "$env_file"' EXIT HUP INT TERM; chmod 600 "$env_file"; cat > "$env_file"; ` + strings.Join(arguments, " ")
	if _, err := runner.RunWithStdin(command, strings.NewReader(environmentFile(container.Environment))); err != nil {
		return errors.New("creating container failed")
	}
	return nil
}

func environmentFile(environment map[string]string) string {
	keys := sortedEnvironmentKeys(environment)
	var builder strings.Builder
	for _, name := range keys {
		builder.WriteString(name)
		builder.WriteByte('=')
		builder.WriteString(environment[name])
		builder.WriteByte('\n')
	}
	return builder.String()
}

func sortedEnvironmentKeys(environment map[string]string) []string {
	keys := make([]string, 0, len(environment))
	for name := range environment {
		keys = append(keys, name)
	}
	sort.Strings(keys)
	return keys
}

func cleanupCreatedContainer(runner remoteRunner, name string) error {
	return runContainerCommand(runner, "docker container rm --force "+ssh.ShellQuote(name), "cleaning up failed container failed")
}

func runContainerCommand(runner remoteRunner, command, message string) error {
	if _, err := runner.Run(command); err != nil {
		return errors.New(message)
	}
	return nil
}
