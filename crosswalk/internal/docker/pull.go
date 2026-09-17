package docker

import (
	"errors"
	"fmt"

	"go-deployer/internal/config"
	"go-deployer/internal/ssh"
	"go-deployer/pkg/logger"
)

// prepareContainerImage는 현재 container 상태를 기록하고 desired digest image를 로컬에 준비한다.
// container 생성·교체는 수행하지 않는다.
func prepareContainerImage(runner remoteRunner, serverName string, registry config.DockerRegistryConfig, container config.ContainerConfig) error {
	_, err := prepareContainerImageWithOutcome(runner, serverName, registry, container)
	return err
}

func prepareContainerImageWithOutcome(runner remoteRunner, serverName string, registry config.DockerRegistryConfig, container config.ContainerConfig) (bool, error) {
	desired := registry.ImageReference(container)
	current, err := inspectContainerImage(runner, container.Name)
	if err != nil {
		return false, err
	}
	if !current.Exists {
		logger.Info(serverName, "Container %s is not present", container.Name)
	} else if containsDigest(current.RepoDigests, desired) {
		logger.Info(serverName, "Container %s already uses the desired image digest", container.Name)
	} else {
		logger.Info(serverName, "Container %s uses a different image digest", container.Name)
	}

	local, err := inspectLocalImage(runner, desired)
	if err != nil {
		return false, err
	}
	if local.Exists && containsDigest(local.RepoDigests, desired) {
		logger.Info(serverName, "Image %s is already available locally; skipping pull", desired)
		return false, nil
	}

	command := "docker pull " + ssh.ShellQuote(desired)
	if _, err := runner.Run(command); err != nil {
		return false, fmt.Errorf("pulling image %s failed", desired)
	}

	local, err = inspectLocalImage(runner, desired)
	if err != nil {
		return false, errors.New("pulled image digest verification failed")
	}
	if !local.Exists || !containsDigest(local.RepoDigests, desired) {
		return false, errors.New("pulled image does not match the requested digest")
	}
	logger.Ok(serverName, "Pulled and verified image %s", desired)
	return true, nil
}
