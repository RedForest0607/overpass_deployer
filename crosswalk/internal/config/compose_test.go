package config

import (
	"strings"
	"testing"
)

func TestLoadSupportsComposeProjectServers(t *testing.T) {
	keyPath := writeTempFile(t, "compose_id_rsa", "test-key")
	composePath := writeTempFile(t, "compose.yml", "services:\n  api:\n    image: harbor.example.com/demo/api@sha256:"+strings.Repeat("a", 64)+"\n")
	path := writeTempFile(t, "deploy-compose.yml", strings.Join([]string{
		"format: crosswalk/v1",
		"ssh:",
		"  user: deploy",
		"  key_path: " + keyPath,
		"  host_key_checking: insecure",
		"docker:",
		"  registry:",
		"    host: harbor.example.com",
		"    username: robot-studio",
		"    token_env: HARBOR_TOKEN",
		"servers:",
		"  - name: compose-dev-01",
		"    host: 192.0.2.20",
		"    compose_projects:",
		"      - name: commerce",
		"        local_path: " + composePath,
		"        remote_dir: /app/overpass/compose/commerce",
	}, "\n"))

	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Servers[0].ComposeProjects) != 1 || cfg.Servers[0].ComposeProjects[0].Name != "commerce" {
		t.Fatalf("compose projects=%#v", cfg.Servers[0].ComposeProjects)
	}
}

func TestValidateComposeProjectsRejectsUnsafeValues(t *testing.T) {
	cfg := validDockerConfig(t)
	cfg.Servers[0].Containers = nil
	cfg.Servers[0].ComposeProjects = []ComposeProjectConfig{
		{Name: "Invalid.Project", LocalPath: "/missing/compose.yml", RemoteDir: "relative/path"},
		{Name: "invalid.project", LocalPath: "/missing/other.yml", RemoteDir: "/app/compose/../unsafe;command"},
	}
	err := ValidateAndApplyDefaults(&cfg)
	if err == nil {
		t.Fatal("expected Compose validation error")
	}
	for _, expected := range []string{"compose_projects", "local_path", "remote_dir", "clean absolute path", "lowercase"} {
		if !strings.Contains(err.Error(), expected) {
			t.Fatalf("missing %q in %v", expected, err)
		}
	}
}

func TestValidateComposeProjectsRejectsSharedRemoteDirectory(t *testing.T) {
	cfg := validDockerConfig(t)
	cfg.Servers[0].Containers = nil
	local := writeTempFile(t, "compose.yml", "services:\n  api:\n    image: harbor.example.com/demo/api@sha256:"+strings.Repeat("a", 64)+"\n")
	cfg.Servers[0].ComposeProjects = []ComposeProjectConfig{
		{Name: "api", LocalPath: local, RemoteDir: "/app/compose/shared"},
		{Name: "worker", LocalPath: local, RemoteDir: "/app/compose/shared"},
	}
	if err := ValidateAndApplyDefaults(&cfg); err == nil || !strings.Contains(err.Error(), "remote_dir must be unique") {
		t.Fatalf("expected shared directory error, got %v", err)
	}
}
