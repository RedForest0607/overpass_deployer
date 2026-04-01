package config

import (
	"os"
	"strings"
	"testing"
)

func TestValidateAndApplyDefaultsRejectsInvalidInputs(t *testing.T) {
	keyPath := writeTempFile(t, "id_rsa", "key")
	knownHostsPath := writeTempFile(t, "known_hosts", "example ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQC")
	jarPath := writeTempFile(t, "app.jar", "jar")
	configPath := writeTempFile(t, "application.yml", "server:\n  port: 8080\n")
	templatePath := writeTempFile(t, "start.sh.tmpl", "#!/bin/bash\n")

	cfg := &Config{
		SSH: SSHConfig{
			User:            "deploy",
			KeyPath:         keyPath,
			KnownHosts:      knownHostsPath,
			HostKeyChecking: HostKeyStrict,
			Port:            70000,
		},
		Servers: []ServerConfig{
			{
				Host: "app.example.com",
				App: AppConfig{
					Name:      "sample",
					BaseDir:   "/opt/sample",
					Port:      0,
					ExtraOpts: StringList{"", "--debug"},
					Jar: JarConfig{
						LocalPath:  jarPath,
						RemotePath: "/opt/sample/bin/app.jar",
					},
					ConfigFiles: []ConfigFile{
						{Local: configPath, Remote: "/opt/sample/config/application.yml"},
					},
					Script: ScriptConfig{
						Template: templatePath,
					},
				},
			},
		},
	}

	err := ValidateAndApplyDefaults(cfg)
	if err == nil {
		t.Fatal("expected validation error")
	}

	for _, fragment := range []string{
		"ssh.port must be between 1 and 65535",
		"servers[0].app.port is required",
		"servers[0].app.extra_opts[0] must not be empty",
	} {
		if !strings.Contains(err.Error(), fragment) {
			t.Fatalf("expected error to contain %q, got %v", fragment, err)
		}
	}
}

func TestValidateAndApplyDefaultsAppliesDefaults(t *testing.T) {
	keyPath := writeTempFile(t, "id_rsa", "key")
	jarPath := writeTempFile(t, "app.jar", "jar")

	cfg := &Config{
		SSH: SSHConfig{
			User:    "deploy",
			KeyPath: keyPath,
		},
		Servers: []ServerConfig{
			{
				Host: "app.example.com",
				App: AppConfig{
					Name:    "sample",
					BaseDir: "/opt/sample",
					Port:    8080,
					Jar: JarConfig{
						LocalPath:  jarPath,
						RemotePath: "/opt/sample/bin/app.jar",
					},
				},
			},
		},
	}

	if err := ValidateAndApplyDefaults(cfg); err != nil {
		t.Fatalf("unexpected validation error: %v", err)
	}

	if cfg.SSH.Port != DefaultSSHPort {
		t.Fatalf("expected default ssh port %d, got %d", DefaultSSHPort, cfg.SSH.Port)
	}
	if cfg.SSH.TimeoutSec != DefaultSSHTimeout {
		t.Fatalf("expected default timeout %d, got %d", DefaultSSHTimeout, cfg.SSH.TimeoutSec)
	}
	if cfg.SSH.HostKeyChecking != HostKeyAcceptNew {
		t.Fatalf("expected default host key checking %q, got %q", HostKeyAcceptNew, cfg.SSH.HostKeyChecking)
	}
	if cfg.SSH.KnownHosts == "" {
		t.Fatal("expected known_hosts path to be defaulted for accept-new mode")
	}
	if cfg.Servers[0].App.Jvm.MinHeap != DefaultJvmMin {
		t.Fatalf("expected default min heap %q, got %q", DefaultJvmMin, cfg.Servers[0].App.Jvm.MinHeap)
	}
	if cfg.Servers[0].App.Jvm.MaxHeap != DefaultJvmMax {
		t.Fatalf("expected default max heap %q, got %q", DefaultJvmMax, cfg.Servers[0].App.Jvm.MaxHeap)
	}
	if cfg.Servers[0].App.Script.RemoteDir != "/opt/sample/scripts" {
		t.Fatalf("unexpected remote dir: %q", cfg.Servers[0].App.Script.RemoteDir)
	}
	if cfg.Servers[0].Name != "app.example.com" {
		t.Fatalf("expected server name to default to host, got %q", cfg.Servers[0].Name)
	}
}

func TestValidateAndApplyDefaultsStrictRequiresKnownHostsFile(t *testing.T) {
	keyPath := writeTempFile(t, "id_rsa", "key")
	jarPath := writeTempFile(t, "app.jar", "jar")

	cfg := &Config{
		SSH: SSHConfig{
			User:            "deploy",
			KeyPath:         keyPath,
			KnownHosts:      "/tmp/does-not-exist-known-hosts",
			HostKeyChecking: HostKeyStrict,
		},
		Servers: []ServerConfig{
			{
				Host: "app.example.com",
				App: AppConfig{
					Name:    "sample",
					BaseDir: "/opt/sample",
					Port:    8080,
					Jar: JarConfig{
						LocalPath:  jarPath,
						RemotePath: "/opt/sample/bin/app.jar",
					},
				},
			},
		},
	}

	err := ValidateAndApplyDefaults(cfg)
	if err == nil || !strings.Contains(err.Error(), "ssh.known_hosts_path does not exist") {
		t.Fatalf("expected strict mode known_hosts validation error, got %v", err)
	}
}

func writeTempFile(t *testing.T, pattern, content string) string {
	t.Helper()

	dir := t.TempDir()
	path := dir + "/" + pattern
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write temp file: %v", err)
	}
	return path
}
