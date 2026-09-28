package compose

import (
	"errors"
	"strings"

	"go-deployer/internal/config"
)

func filterConfig(cfg *config.Config, options Options) (*config.Config, error) {
	filtered := *cfg
	filtered.Servers = nil
	for _, server := range cfg.Servers {
		if len(options.ServerTags) > 0 && !matchesTags(server.Tags, options.ServerTags) {
			continue
		}
		selected := server
		selected.ComposeProjects = nil
		for _, project := range server.ComposeProjects {
			if len(options.Projects) == 0 || matchesName(project.Name, options.Projects) {
				selected.ComposeProjects = append(selected.ComposeProjects, project)
			}
		}
		if len(selected.ComposeProjects) > 0 {
			filtered.Servers = append(filtered.Servers, selected)
		}
	}
	if len(filtered.Servers) == 0 {
		return nil, errors.New("no Compose projects matched the requested filters")
	}
	return &filtered, nil
}

func matchesTags(available, filters []string) bool {
	for _, filter := range filters {
		for _, tag := range available {
			if strings.EqualFold(strings.TrimSpace(tag), strings.TrimSpace(filter)) {
				return true
			}
		}
	}
	return false
}

func matchesName(name string, filters []string) bool {
	for _, filter := range filters {
		if name == strings.TrimSpace(filter) {
			return true
		}
	}
	return false
}
