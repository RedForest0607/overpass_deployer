package config

import (
	"strings"
	"testing"
)

func validDockerConfig(t *testing.T) Config {
	t.Helper()
	keyPath := writeTempFile(t, "docker_id_rsa", "test-key")
	return Config{
		SSH: SSHConfig{User: "deploy", KeyPath: keyPath, HostKeyChecking: HostKeyInsecure},
		Docker: DockerDeploymentConfig{Registry: DockerRegistryConfig{
			Host: "Harbor.Example.com:5443", Username: "robot$deployer", TokenEnv: "HARBOR_TOKEN",
		}},
		Servers: []ServerConfig{{
			Host: "10.0.0.10",
			Name: "docker-app-1",
			Containers: []ContainerConfig{{
				Name:        "commerce-api",
				Image:       "TEAM/Commerce-API",
				Digest:      "sha256:" + strings.Repeat("a", 64),
				Ports:       []ContainerPort{{Host: 8080, Container: 8080}},
				Environment: map[string]string{"SPRING_PROFILES_ACTIVE": "prod"},
			}},
		}},
	}
}

func TestValidateDockerConfigAppliesSafeDefaults(t *testing.T) {
	cfg := validDockerConfig(t)
	if err := ValidateAndApplyDefaults(&cfg); err != nil {
		t.Fatal(err)
	}
	container := cfg.Servers[0].Containers[0]
	if cfg.Docker.Registry.Host != "harbor.example.com:5443" {
		t.Fatalf("registry host=%q", cfg.Docker.Registry.Host)
	}
	if container.Image != "team/commerce-api" || container.RestartPolicy != "unless-stopped" {
		t.Fatalf("container=%#v", container)
	}
	if container.Ports[0].Protocol != "tcp" {
		t.Fatalf("protocol=%q", container.Ports[0].Protocol)
	}
}

func TestLoadSupportsDockerOnlyServers(t *testing.T) {
	keyPath := writeTempFile(t, "docker_loader_id_rsa", "test-key")
	path := writeTempFile(t, "deploy-docker.yml", strings.Join([]string{
		"format: crosswalk/v1",
		"ssh:",
		"  user: deploy",
		"  key_path: " + keyPath,
		"  host_key_checking: insecure",
		"docker:",
		"  registry:",
		"    host: harbor.example.com",
		"    username: robot$deployer",
		"    token_env: HARBOR_TOKEN",
		"servers:",
		"  - name: docker-app-1",
		"    host: 10.0.0.10",
		"    containers:",
		"      - name: commerce-api",
		"        image: team/commerce-api",
		"        digest: sha256:" + strings.Repeat("b", 64),
		"        ports:",
		"          - host: 8080",
		"            container: 8080",
	}, "\n"))

	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Servers[0].Containers) != 1 || cfg.Servers[0].Containers[0].RestartPolicy != "unless-stopped" {
		t.Fatalf("containers=%#v", cfg.Servers[0].Containers)
	}
}

func TestValidateDockerConfigRejectsAmbiguousOrUnsafeInputs(t *testing.T) {
	cfg := validDockerConfig(t)
	cfg.Docker.Registry.Host = "harbor.example.com:99999"
	cfg.Docker.Registry.Username = "robot deployer"
	cfg.Docker.Registry.TokenEnv = "${HARBOR_TOKEN}"
	cfg.Servers[0].Containers = append(cfg.Servers[0].Containers, ContainerConfig{
		Name:          "commerce-api",
		Image:         "team/commerce:latest",
		Digest:        "sha256:short",
		RestartPolicy: "sometimes",
		Ports: []ContainerPort{
			{Host: 8080, Container: 8081},
			{Host: 8082, Container: 70000, Protocol: "sctp"},
		},
		Environment: map[string]string{"BAD-KEY": "value", "SECRET": "${MISSING_SECRET}"},
	})
	err := ValidateAndApplyDefaults(&cfg)
	if err == nil {
		t.Fatal("expected validation error")
	}
	for _, expected := range []string{
		"docker.registry.host",
		"docker.registry.username",
		"docker.registry.token_env",
		"must be unique",
		"without a tag or digest",
		"must be a sha256 digest",
		"restart must be one of",
		"conflicts with",
		"must be between 1 and 65535",
		"protocol must be tcp or udp",
		"environment key",
		"contains unresolved environment variable",
	} {
		if !strings.Contains(err.Error(), expected) {
			t.Fatalf("missing %q in %v", expected, err)
		}
	}
}
