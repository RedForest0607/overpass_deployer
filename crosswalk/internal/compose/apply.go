package compose

import (
	"encoding/json"
	"errors"
	"fmt"
	"path"
	"sort"
	"strings"
	"time"

	"go-deployer/internal/config"
	"go-deployer/internal/docker"
	"go-deployer/internal/ssh"
	"go-deployer/pkg/logger"
)

var composeHealthInterval = time.Second
var composeHealthTimeout = 60 * time.Second

type composeContainer struct {
	Service string `json:"Service"`
	State   string `json:"State"`
	Health  string `json:"Health"`
}

func composeCommand(projectName, file string) string {
	return "PODMAN_COMPOSE_WARNING_LOGS=false docker compose --project-name " + ssh.ShellQuote(projectName) + " --file " + ssh.ShellQuote(file)
}

func verifyResolvedCompose(runner docker.RemoteRunner, project config.ComposeProjectConfig, file string, desired desiredCompose) error {
	if err := validateComposeProject(runner, project.Name, file); err != nil {
		return err
	}
	base := composeCommand(project.Name, file)
	services, err := runner.Run(base + " config --services")
	if err != nil || !sameLines(services, mapKeys(desired.services)) {
		return errors.New("remote Compose service list does not match the requested revision")
	}
	images, err := runner.Run(base + " config --images")
	if err != nil || !sameLines(images, desired.resolvedImages) {
		return errors.New("remote Compose images do not match the digest-pinned revision")
	}
	return nil
}

func sameLines(actual string, expected []string) bool {
	lines := strings.Fields(actual)
	if len(lines) != len(expected) {
		return false
	}
	sort.Strings(lines)
	want := append([]string(nil), expected...)
	sort.Strings(want)
	for index := range want {
		if lines[index] != want[index] {
			return false
		}
	}
	return true
}

func mapKeys(values map[string]struct{}) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	return keys
}

func readComposeContainers(output string) ([]composeContainer, error) {
	trimmed := strings.TrimSpace(output)
	if trimmed == "" {
		return nil, nil
	}
	var values []composeContainer
	if strings.HasPrefix(trimmed, "[") {
		if err := json.Unmarshal([]byte(trimmed), &values); err != nil {
			return nil, errors.New("invalid Compose container status")
		}
		return values, nil
	}
	for _, line := range strings.Split(trimmed, "\n") {
		var value composeContainer
		if err := json.Unmarshal([]byte(line), &value); err != nil {
			return nil, errors.New("invalid Compose container status")
		}
		values = append(values, value)
	}
	return values, nil
}

func waitComposeReady(runner docker.RemoteRunner, projectName, file string, services map[string]struct{}) error {
	base := composeCommand(projectName, file)
	deadline := time.Now().Add(composeHealthTimeout)
	for {
		output, err := runner.Run(base + " ps --all --format json")
		if err != nil {
			return errors.New("failed to inspect Compose services")
		}
		containers, err := readComposeContainers(output)
		if err != nil {
			return err
		}
		seen := make(map[string]int, len(services))
		ready := len(containers) > 0
		for _, container := range containers {
			if _, exists := services[container.Service]; !exists {
				return fmt.Errorf("unexpected Compose service %q", container.Service)
			}
			seen[container.Service]++
			if container.State == "exited" || container.State == "dead" || container.Health == "unhealthy" {
				return fmt.Errorf("Compose service %s is not healthy", container.Service)
			}
			ready = ready && container.State == "running" && (container.Health == "" || container.Health == "healthy")
		}
		for service := range services {
			if seen[service] != 1 {
				ready = false
			}
		}
		if ready {
			return nil
		}
		if !time.Now().Before(deadline) {
			return errors.New("Compose service readiness timed out")
		}
		time.Sleep(composeHealthInterval)
	}
}

func rollbackCompose(runner docker.RemoteRunner, project config.ComposeProjectConfig, paths revisionPaths, previous string) error {
	if previous == "" {
		if _, err := runner.Run(composeCommand(project.Name, paths.file) + " down --remove-orphans"); err != nil {
			return errors.New("failed to stop the first Compose revision; active link retained for recovery")
		}
		if _, err := runner.Run("unlink " + ssh.ShellQuote(paths.active)); err != nil {
			return errors.New("failed to remove the first Compose active link")
		}
		return nil
	}
	if err := switchRevision(runner, paths, previous); err != nil {
		return err
	}
	previousFile := path.Join(paths.root, previous, "compose.yml")
	previousBase := composeCommand(project.Name, previousFile)
	if _, err := runner.Run(previousBase + " up -d --remove-orphans --pull never --no-build"); err != nil {
		return errors.New("failed to restore the previous Compose revision")
	}
	output, err := runner.Run(previousBase + " config --services")
	if err != nil || strings.TrimSpace(output) == "" {
		return errors.New("failed to inspect restored Compose services")
	}
	services := make(map[string]struct{})
	for _, service := range strings.Fields(output) {
		services[service] = struct{}{}
	}
	if err := waitComposeReady(runner, project.Name, previousFile, services); err != nil {
		return fmt.Errorf("restored Compose revision is unhealthy: %w", err)
	}
	return nil
}

func applyProject(runner docker.RemoteRunner, project config.ComposeProjectConfig, desired desiredCompose) (changed bool, err error) {
	paths := pathsFor(project, desired.revision)
	release, err := acquireRevisionLock(runner, project.RemoteDir)
	if err != nil {
		return false, err
	}
	defer func() { err = errors.Join(err, release()) }()

	previous, err := currentRevision(runner, paths)
	if err != nil {
		return false, err
	}
	if previous == "" {
		output, err := runner.Run("docker ps --all --filter " + ssh.ShellQuote("label=com.docker.compose.project="+project.Name) + " --format '{{.ID}}'")
		if err != nil || strings.TrimSpace(output) != "" {
			return false, errors.New("Compose project already has unmanaged containers; refusing first managed apply")
		}
	}
	if previous == paths.target {
		if err := verifyRevisionChecksum(runner, paths.file, desired.revision); err != nil {
			return false, fmt.Errorf("active revision: %w", err)
		}
		if err := verifyResolvedCompose(runner, project, paths.file, desired); err != nil {
			return false, err
		}
		if err := waitComposeReady(runner, project.Name, paths.file, desired.services); err != nil {
			return false, fmt.Errorf("active Compose revision is unhealthy: %w", err)
		}
		return false, nil
	}
	if previous != "" {
		previousFile := path.Join(paths.root, previous, "compose.yml")
		if err := verifyRevisionChecksum(runner, previousFile, strings.TrimPrefix(previous, ".revisions/")); err != nil {
			return false, fmt.Errorf("previous revision unavailable for rollback: %w", err)
		}
		if err := validateComposeProject(runner, project.Name, previousFile); err != nil {
			return false, errors.New("previous revision is unavailable for rollback")
		}
	}
	staged, err := stageRevision(runner, project, paths, desired.revision)
	if err != nil {
		return false, err
	}
	if err := verifyResolvedCompose(runner, project, paths.file, desired); err != nil {
		return false, err
	}
	if _, err := pullImages(runner, desired.images); err != nil {
		return false, err
	}
	if err := switchRevision(runner, paths, paths.target); err != nil {
		return false, err
	}
	base := composeCommand(project.Name, paths.active+"/compose.yml")
	_, applyErr := runner.Run(base + " up -d --remove-orphans --pull never --no-build")
	if applyErr == nil {
		applyErr = waitComposeReady(runner, project.Name, paths.active+"/compose.yml", desired.services)
	}
	if applyErr != nil {
		rollbackErr := rollbackCompose(runner, project, paths, previous)
		if rollbackErr != nil {
			return false, fmt.Errorf("Compose apply failed and rollback failed: %w", rollbackErr)
		}
		if previous == "" {
			return false, fmt.Errorf("Compose apply failed: %w; first revision stopped and deactivated", applyErr)
		}
		return false, fmt.Errorf("Compose apply failed: %w; previous revision restored", applyErr)
	}
	logger.Ok(project.Name, "Compose revision %s active (new file: %t)", desired.revision[:12], staged)
	return true, nil
}
