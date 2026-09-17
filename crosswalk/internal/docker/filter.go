package docker

import (
	"errors"

	"go-deployer/internal/config"
)

// filterConfig는 server/container tag 중 하나라도 일치하는 Docker 대상만 복사한다.
func filterConfig(cfg *config.Config, opts Options) (*config.Config, error) {
	filtered := *cfg
	filtered.Servers = make([]config.ServerConfig, 0, len(cfg.Servers))
	for _, server := range cfg.Servers {
		if len(opts.ServerTags) > 0 && !matchesAnyTag(server.Tags, opts.ServerTags) {
			continue
		}
		serverCopy := server
		serverCopy.Containers = make([]config.ContainerConfig, 0, len(server.Containers))
		for _, container := range server.Containers {
			if matchesAnyTag(container.Tags, opts.ContainerTags) {
				serverCopy.Containers = append(serverCopy.Containers, container)
			}
		}
		if len(serverCopy.Containers) > 0 {
			filtered.Servers = append(filtered.Servers, serverCopy)
		}
	}
	if len(filtered.Servers) == 0 {
		return nil, errors.New("no Docker containers matched the requested tag filters")
	}
	return &filtered, nil
}

func matchesAnyTag(resourceTags, filterTags []string) bool {
	if len(filterTags) == 0 {
		return true
	}
	available := make(map[string]struct{}, len(resourceTags))
	for _, tag := range resourceTags {
		available[tag] = struct{}{}
	}
	for _, tag := range filterTags {
		if _, exists := available[tag]; exists {
			return true
		}
	}
	return false
}
