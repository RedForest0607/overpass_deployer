package compose

import (
	"errors"
	"strings"

	"go-deployer/internal/docker"
)

const composeVersionCommand = "PODMAN_COMPOSE_WARNING_LOGS=false docker compose version --short"

func verifyComposePlugin(runner docker.RemoteRunner) error {
	version, err := runner.Run(composeVersionCommand)
	if err != nil || strings.TrimSpace(version) == "" {
		return errors.New("Docker Compose v2 is unavailable for the SSH user; install the Compose plugin and verify docker compose access")
	}
	return nil
}

func validateComposeProject(runner docker.RemoteRunner, projectName, remoteFile string) error {
	command := composeCommand(projectName, remoteFile) + " config --quiet"
	if _, err := runner.Run(command); err != nil {
		return errors.New("remote Compose config validation failed")
	}
	return nil
}
