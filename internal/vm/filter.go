package vm

import (
	"fmt"

	"go-deployer/internal/config"
)

func filterConfig(cfg *config.Config, opts RunOptions) (*config.Config, error) {
	if len(opts.ServerTags) == 0 && len(opts.AppTags) == 0 {
		return cfg, nil
	}

	filtered := *cfg
	filtered.Servers = make([]config.ServerConfig, 0, len(cfg.Servers))

	for _, server := range cfg.Servers {
		if len(opts.ServerTags) > 0 && !matchesAnyTag(server.Tags, opts.ServerTags) {
			continue
		}

		serverCopy, include := filterServer(server, opts.AppTags)
		if !include {
			continue
		}
		filtered.Servers = append(filtered.Servers, serverCopy)
	}

	if len(filtered.Servers) == 0 {
		return nil, fmt.Errorf("no servers matched the requested tag filters")
	}

	return &filtered, nil
}

func filterServer(server config.ServerConfig, appTags []string) (config.ServerConfig, bool) {
	if len(appTags) == 0 {
		return server, true
	}

	apps := server.EffectiveApps()
	if len(apps) == 0 {
		return config.ServerConfig{}, false
	}

	selectedApps := make([]config.AppConfig, 0, len(apps))
	for _, app := range apps {
		if matchesAnyTag(app.Tags, appTags) {
			selectedApps = append(selectedApps, app)
		}
	}
	if len(selectedApps) == 0 {
		return config.ServerConfig{}, false
	}

	serverCopy := server
	if server.UsesLegacyApp() && len(selectedApps) == 1 {
		serverCopy.App = selectedApps[0]
		serverCopy.Apps = nil
		return serverCopy, true
	}

	serverCopy.App = config.AppConfig{}
	serverCopy.Apps = selectedApps
	return serverCopy, true
}

func matchesAnyTag(resourceTags []string, filterTags []string) bool {
	if len(filterTags) == 0 {
		return true
	}

	selected := make(map[string]struct{}, len(resourceTags))
	for _, tag := range resourceTags {
		selected[tag] = struct{}{}
	}

	for _, filterTag := range filterTags {
		if _, ok := selected[filterTag]; ok {
			return true
		}
	}

	return false
}
