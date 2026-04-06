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
		"    app:",
		"      name: ${APP_NAME}",
		"      base_dir: /opt/sample",
		"      port: 8080",
		"      jar:",
		"        local_path: " + jarPath,
		"        remote_path: /opt/sample/bin/app.jar",
		"      config_files:",
		"        - local: " + configPath,
		"          remote: /opt/sample/config/application.yml",
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

func writeFile(t *testing.T, path, content string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("creating directory %s: %v", path, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("writing file %s: %v", path, err)
	}
}
