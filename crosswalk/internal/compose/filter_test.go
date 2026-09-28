package compose

import (
	"strings"
	"testing"

	"go-deployer/internal/config"
)

func TestFilterConfigSelectsProjectsWithoutMutatingInput(t *testing.T) {
	cfg := composeConfig()
	cfg.Servers[0].Tags = []string{"dev", "compose"}
	cfg.Servers[0].ComposeProjects = append(cfg.Servers[0].ComposeProjects,
		config.ComposeProjectConfig{Name: "worker", LocalPath: "worker.yml", RemoteDir: "/app/compose/worker"})
	cfg.Servers = append(cfg.Servers, config.ServerConfig{Name: "prod", Tags: []string{"prod"}, ComposeProjects: []config.ComposeProjectConfig{{Name: "commerce"}}})
	filtered, err := filterConfig(cfg, Options{ServerTags: []string{"dev"}, Projects: []string{"worker"}})
	if err != nil || len(filtered.Servers) != 1 || len(filtered.Servers[0].ComposeProjects) != 1 || filtered.Servers[0].ComposeProjects[0].Name != "worker" {
		t.Fatalf("filtered=%#v err=%v", filtered, err)
	}
	if len(cfg.Servers[0].ComposeProjects) != 2 {
		t.Fatal("filter changed original configuration")
	}
	if _, err := filterConfig(cfg, Options{Projects: []string{"missing"}}); err == nil || !strings.Contains(err.Error(), "no Compose projects") {
		t.Fatalf("missing project error=%v", err)
	}
}
