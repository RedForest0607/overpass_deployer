package config

import (
	"fmt"
	"path"
	"strconv"
	"strings"
)

func validateDockerDeploymentConfig(cfg *Config, errs *[]string, checkUnresolvedEnv func(string, string)) {
	hasDockerWorkloads := false
	for _, server := range cfg.Servers {
		if len(server.Containers) > 0 || len(server.ComposeProjects) > 0 {
			hasDockerWorkloads = true
			break
		}
	}
	if !hasDockerWorkloads {
		return
	}

	registry := &cfg.Docker.Registry
	registry.Host = strings.ToLower(strings.TrimSpace(registry.Host))
	registry.Username = strings.TrimSpace(registry.Username)
	registry.TokenEnv = strings.TrimSpace(registry.TokenEnv)
	checkUnresolvedEnv(registry.Host, "docker.registry.host")
	checkUnresolvedEnv(registry.Username, "docker.registry.username")
	if registry.Host == "" {
		*errs = append(*errs, "docker.registry.host is required when Docker workloads are configured")
	} else if !isValidDockerRegistryHost(registry.Host) {
		*errs = append(*errs, "docker.registry.host must be a hostname with an optional port")
	}
	if registry.Username == "" {
		*errs = append(*errs, "docker.registry.username is required when Docker workloads are configured")
	} else if !dockerRegistryUsernamePattern.MatchString(registry.Username) {
		*errs = append(*errs, "docker.registry.username contains unsupported characters")
	}
	if !environmentNamePattern.MatchString(registry.TokenEnv) {
		*errs = append(*errs, "docker.registry.token_env must be an environment variable name")
	}
}

func validateComposeProjects(projects []ComposeProjectConfig, prefix string, errs *[]string, checkUnresolvedEnv func(string, string)) {
	names := make(map[string]struct{}, len(projects))
	remoteDirs := make(map[string]struct{}, len(projects))
	for index := range projects {
		project := &projects[index]
		fieldPrefix := fmt.Sprintf("%s[%d]", prefix, index)
		project.Name = strings.ToLower(strings.TrimSpace(project.Name))
		project.LocalPath = strings.TrimSpace(project.LocalPath)
		project.RemoteDir = strings.TrimSpace(project.RemoteDir)
		checkUnresolvedEnv(project.Name, fieldPrefix+".name")
		checkUnresolvedEnv(project.LocalPath, fieldPrefix+".local_path")
		checkUnresolvedEnv(project.RemoteDir, fieldPrefix+".remote_dir")

		if !composeProjectNamePattern.MatchString(project.Name) {
			*errs = append(*errs, fieldPrefix+".name must start with a lowercase letter or number and contain only lowercase letters, numbers, hyphens, or underscores")
		} else if _, exists := names[project.Name]; exists {
			*errs = append(*errs, fmt.Sprintf("%s.name %q must be unique on the server", fieldPrefix, project.Name))
		}
		names[project.Name] = struct{}{}

		if project.LocalPath == "" {
			*errs = append(*errs, fieldPrefix+".local_path is required")
		} else {
			validateExistingFile(project.LocalPath, fieldPrefix+".local_path", errs)
		}
		if project.RemoteDir == "" || !path.IsAbs(project.RemoteDir) {
			*errs = append(*errs, fieldPrefix+".remote_dir must be an absolute path")
		} else if path.Clean(project.RemoteDir) != project.RemoteDir || !composeRemoteDirPattern.MatchString(project.RemoteDir) {
			*errs = append(*errs, fieldPrefix+".remote_dir must be a clean absolute path containing only letters, numbers, dots, underscores, hyphens, and slashes")
		} else if _, exists := remoteDirs[project.RemoteDir]; exists {
			*errs = append(*errs, fieldPrefix+".remote_dir must be unique on the server")
		}
		remoteDirs[project.RemoteDir] = struct{}{}
	}
}

func validateContainerConfigs(containers []ContainerConfig, prefix string, errs *[]string, checkUnresolvedEnv func(string, string)) {
	names := make(map[string]struct{}, len(containers))
	hostPorts := make(map[string]string)
	for index := range containers {
		container := &containers[index]
		fieldPrefix := fmt.Sprintf("%s[%d]", prefix, index)
		container.Name = strings.TrimSpace(container.Name)
		container.Image = strings.ToLower(strings.TrimSpace(container.Image))
		container.Digest = strings.ToLower(strings.TrimSpace(container.Digest))
		container.RestartPolicy = strings.ToLower(strings.TrimSpace(container.RestartPolicy))
		validateTags(&container.Tags, fieldPrefix+".tags", errs, checkUnresolvedEnv)
		checkUnresolvedEnv(container.Name, fieldPrefix+".name")
		checkUnresolvedEnv(container.Image, fieldPrefix+".image")
		checkUnresolvedEnv(container.Digest, fieldPrefix+".digest")

		if !containerNamePattern.MatchString(container.Name) {
			*errs = append(*errs, fieldPrefix+".name must be a valid Docker container name")
		} else if strings.HasSuffix(container.Name, DockerBackupContainerSuffix) {
			*errs = append(*errs, fieldPrefix+".name uses the reserved Crosswalk backup suffix")
		} else if _, exists := names[container.Name]; exists {
			*errs = append(*errs, fmt.Sprintf("%s.name %q must be unique on the server", fieldPrefix, container.Name))
		}
		names[container.Name] = struct{}{}
		if !dockerRepositoryPattern.MatchString(container.Image) {
			*errs = append(*errs, fieldPrefix+".image must be a lowercase repository path without a tag or digest")
		}
		if !dockerDigestPattern.MatchString(container.Digest) {
			*errs = append(*errs, fieldPrefix+".digest must be a sha256 digest")
		}
		if container.RestartPolicy == "" {
			container.RestartPolicy = "unless-stopped"
		} else if !allowedRestartPolicies[container.RestartPolicy] {
			*errs = append(*errs, fieldPrefix+".restart must be one of no, always, on-failure, or unless-stopped")
		}
		if container.HealthTimeoutSec == 0 {
			container.HealthTimeoutSec = DefaultDockerHealthTimeoutSec
		} else if container.HealthTimeoutSec < 1 || container.HealthTimeoutSec > 3600 {
			*errs = append(*errs, fieldPrefix+".health_timeout_sec must be between 1 and 3600")
		}

		for portIndex := range container.Ports {
			port := &container.Ports[portIndex]
			portPrefix := fmt.Sprintf("%s.ports[%d]", fieldPrefix, portIndex)
			validatePort(port.Host, portPrefix+".host", errs)
			validatePort(port.Container, portPrefix+".container", errs)
			port.Protocol = strings.ToLower(strings.TrimSpace(port.Protocol))
			if port.Protocol == "" {
				port.Protocol = "tcp"
			} else if port.Protocol != "tcp" && port.Protocol != "udp" {
				*errs = append(*errs, portPrefix+".protocol must be tcp or udp")
			}
			key := fmt.Sprintf("%d/%s", port.Host, port.Protocol)
			if previous, exists := hostPorts[key]; exists {
				*errs = append(*errs, fmt.Sprintf("%s.host conflicts with %s on %s", portPrefix, previous, key))
			} else {
				hostPorts[key] = portPrefix
			}
		}

		for name, value := range container.Environment {
			if !environmentNamePattern.MatchString(name) {
				*errs = append(*errs, fmt.Sprintf("%s.environment key %q is invalid", fieldPrefix, name))
			}
			checkUnresolvedEnv(value, fmt.Sprintf("%s.environment.%s", fieldPrefix, name))
			if strings.ContainsAny(value, "\r\n\x00") {
				*errs = append(*errs, fmt.Sprintf("%s.environment.%s must not contain newlines or NUL bytes", fieldPrefix, name))
			}
		}
	}
}

func isValidDockerRegistryHost(value string) bool {
	host := value
	if strings.Count(value, ":") > 1 {
		return false
	}
	if before, port, found := strings.Cut(value, ":"); found {
		host = before
		parsed, err := strconv.Atoi(port)
		if err != nil || parsed < 1 || parsed > 65535 {
			return false
		}
	}
	if !dockerRegistryHostPattern.MatchString(host) || strings.Contains(host, "..") {
		return false
	}
	for _, label := range strings.Split(host, ".") {
		if label == "" || strings.HasPrefix(label, "-") || strings.HasSuffix(label, "-") {
			return false
		}
	}
	return true
}
