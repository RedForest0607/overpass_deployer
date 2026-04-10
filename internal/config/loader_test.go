package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadSubstitutesEnvAndAppliesDefaults(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	t.Setenv("DEPLOY_USER", "deploy")
	t.Setenv("APP_NAME", "sample-app")

	keyPath := filepath.Join(homeDir, "keys", "id_rsa")
	jarPath := writeTempFile(t, "app.jar", "jar")
	configPath := writeTempFile(t, "application.yml", "server:\n  port: 8080\n")

	writeFile(t, keyPath, "key")

	configYAML := strings.Join([]string{
		"ssh:",
		"  user: ${DEPLOY_USER}",
		"  key_path: ~/keys/id_rsa",
		"servers:",
		"  - host: app.example.com",
		"    ssh_port: 2223",
		"    bastion_host: sample-vm",
		"    bastion_ssh_port: 22",
		"    app:",
		"      name: ${APP_NAME}",
		"      base_dir: /opt/sample",
		"      port: 8080",
		"      jar:",
		"        local_path: " + jarPath,
		"        remote_path: /opt/sample/bin/app.jar",
		"      config_files:",
		"        - local_path: " + configPath,
		"          remote_path: /opt/sample/config/application.yml",
	}, "\n")

	path := writeTempFile(t, "deploy.yml", configYAML)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}

	if cfg.SSH.User != "deploy" {
		t.Fatalf("expected ssh.user to be substituted, got %q", cfg.SSH.User)
	}
	if cfg.SSH.KeyPath != keyPath {
		t.Fatalf("expected expanded key path %q, got %q", keyPath, cfg.SSH.KeyPath)
	}
	if cfg.SSH.Port != DefaultSSHPort {
		t.Fatalf("expected default ssh port %d, got %d", DefaultSSHPort, cfg.SSH.Port)
	}
	if cfg.SSH.TimeoutSec != DefaultSSHTimeout {
		t.Fatalf("expected default timeout %d, got %d", DefaultSSHTimeout, cfg.SSH.TimeoutSec)
	}
	if cfg.Servers[0].App.Name != "sample-app" {
		t.Fatalf("expected app name substitution, got %q", cfg.Servers[0].App.Name)
	}
	if cfg.Servers[0].SSHPort != 2223 {
		t.Fatalf("expected server ssh port override 2223, got %d", cfg.Servers[0].SSHPort)
	}
	if cfg.Servers[0].BastionHost != "sample-vm" {
		t.Fatalf("expected bastion host override sample-vm, got %q", cfg.Servers[0].BastionHost)
	}
	if cfg.Servers[0].BastionSSHPort != 22 {
		t.Fatalf("expected bastion ssh port override 22, got %d", cfg.Servers[0].BastionSSHPort)
	}
	if cfg.Servers[0].App.Jvm.MinHeap != DefaultJvmMin {
		t.Fatalf("expected default min heap %q, got %q", DefaultJvmMin, cfg.Servers[0].App.Jvm.MinHeap)
	}
	if cfg.Servers[0].App.Jvm.MaxHeap != DefaultJvmMax {
		t.Fatalf("expected default max heap %q, got %q", DefaultJvmMax, cfg.Servers[0].App.Jvm.MaxHeap)
	}
}

func TestLoadReportsUnresolvedEnvironmentVariables(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	keyPath := filepath.Join(homeDir, "keys", "id_rsa")
	jarPath := writeTempFile(t, "app.jar", "jar")
	writeFile(t, keyPath, "key")

	configYAML := strings.Join([]string{
		"ssh:",
		"  user: deploy",
		"  key_path: ~/keys/id_rsa",
		"servers:",
		"  - host: app.example.com",
		"    app:",
		"      name: ${APP_NAME}",
		"      base_dir: /opt/sample",
		"      port: 8080",
		"      jar:",
		"        local_path: " + jarPath,
		"        remote_path: /opt/sample/bin/app.jar",
	}, "\n")

	path := writeTempFile(t, "deploy.yml", configYAML)

	_, err := Load(path)
	if err == nil {
		t.Fatal("expected unresolved environment variable error")
	}
	if !strings.Contains(err.Error(), "contains unresolved environment variable") {
		t.Fatalf("expected unresolved env error, got %v", err)
	}
}

func TestLoadSupportsMultipleAppsPerServer(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	keyPath := filepath.Join(homeDir, "keys", "id_rsa")
	writeFile(t, keyPath, "key")

	jarA := writeTempFile(t, "app-a.jar", "jar-a")
	jarB := writeTempFile(t, "app-b.jar", "jar-b")
	configA := writeTempFile(t, "application-a.yml", "server:\n  port: 8081\n")
	configB := writeTempFile(t, "application-b.yml", "server:\n  port: 8082\n")

	configYAML := strings.Join([]string{
		"ssh:",
		"  user: deploy",
		"  key_path: ~/keys/id_rsa",
		"servers:",
		"  - host: app.example.com",
		"    name: devwas",
		"    apps:",
		"      - name: sample-a",
		"        base_dir: /opt/sample-a",
		"        port: 8081",
		"        jar:",
		"          local_path: " + jarA,
		"          remote_path: /opt/sample-a/lib/app.jar",
		"        config_files:",
		"          - local_path: " + configA,
		"            remote_path: /opt/sample-a/conf/application.yml",
		"      - name: sample-b",
		"        base_dir: /opt/sample-b",
		"        port: 8082",
		"        jar:",
		"          local_path: " + jarB,
		"          remote_path: /opt/sample-b/lib/app.jar",
		"        config_files:",
		"          - local_path: " + configB,
		"            remote_path: /opt/sample-b/conf/application.yml",
	}, "\n")

	path := writeTempFile(t, "deploy-apps.yml", configYAML)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}

	if len(cfg.Servers) != 1 {
		t.Fatalf("expected 1 server, got %d", len(cfg.Servers))
	}
	if len(cfg.Servers[0].Apps) != 2 {
		t.Fatalf("expected 2 apps, got %d", len(cfg.Servers[0].Apps))
	}
	if cfg.Servers[0].Apps[0].Name != "sample-a" {
		t.Fatalf("expected first app name sample-a, got %q", cfg.Servers[0].Apps[0].Name)
	}
	if cfg.Servers[0].Apps[1].Script.RemotePath != "/opt/sample-b/scripts/server.sh" {
		t.Fatalf("expected default script remote path for second app, got %q", cfg.Servers[0].Apps[1].Script.RemotePath)
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("creating directory %s: %v", path, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("writing file %s: %v", path, err)
	}
}
