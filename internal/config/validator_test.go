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
	valuesPath := writeTempFile(t, "start.values.yml", "AppName: sample\n")

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
						{LocalPath: configPath, RemotePath: "/opt/sample/config/application.yml"},
					},
					Script: ScriptConfig{
						Template:   templatePath,
						ValuesFile: valuesPath,
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

func TestValidateAndApplyDefaultsMergesBootstrapSettings(t *testing.T) {
	keyPath := writeTempFile(t, "id_rsa", "key")
	jarPath := writeTempFile(t, "app.jar", "jar")
	headlessFalse := false

	cfg := &Config{
		SSH: SSHConfig{
			User:    "deploy",
			KeyPath: keyPath,
		},
		Bootstrap: BootstrapConfig{
			Packages: []string{"unzip", "procps-ng"},
			JDK: JDKConfig{
				Vendor: "corretto",
				Major:  21,
			},
		},
		Servers: []ServerConfig{
			{
				Host: "app.example.com",
				Bootstrap: BootstrapConfig{
					Packages: []string{"git", "unzip"},
					JDK: JDKConfig{
						Major:    17,
						Headless: &headlessFalse,
					},
				},
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

	gotPackages := cfg.Servers[0].Bootstrap.Packages
	wantPackages := []string{"unzip", "procps-ng", "git"}
	if strings.Join(gotPackages, ",") != strings.Join(wantPackages, ",") {
		t.Fatalf("unexpected merged packages: got %v want %v", gotPackages, wantPackages)
	}
	if cfg.Servers[0].Bootstrap.JDK.Vendor != "corretto" {
		t.Fatalf("expected merged jdk vendor corretto, got %q", cfg.Servers[0].Bootstrap.JDK.Vendor)
	}
	if cfg.Servers[0].Bootstrap.JDK.Major != 17 {
		t.Fatalf("expected merged jdk major 17, got %d", cfg.Servers[0].Bootstrap.JDK.Major)
	}
	if cfg.Servers[0].Bootstrap.JDK.Headless == nil || *cfg.Servers[0].Bootstrap.JDK.Headless {
		t.Fatalf("expected merged headless override false, got %#v", cfg.Servers[0].Bootstrap.JDK.Headless)
	}
	if cfg.Servers[0].Bootstrap.OSUpdate.Enabled == nil || *cfg.Servers[0].Bootstrap.OSUpdate.Enabled {
		t.Fatalf("expected os update default false, got %#v", cfg.Servers[0].Bootstrap.OSUpdate.Enabled)
	}
}

func TestValidateAndApplyDefaultsRejectsInvalidBootstrapSettings(t *testing.T) {
	keyPath := writeTempFile(t, "id_rsa", "key")
	jarPath := writeTempFile(t, "app.jar", "jar")

	cfg := &Config{
		SSH: SSHConfig{
			User:    "deploy",
			KeyPath: keyPath,
		},
		Bootstrap: BootstrapConfig{
			Packages: []string{"", "git"},
			JDK: JDKConfig{
				Vendor: "temurin",
				Major:  0,
			},
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
	if err == nil {
		t.Fatal("expected bootstrap validation error")
	}

	for _, fragment := range []string{
		"bootstrap.packages[0] must not be empty",
		`bootstrap.jdk.vendor must be "corretto"`,
		"bootstrap.jdk.major is required when bootstrap.jdk.vendor is set",
	} {
		if !strings.Contains(err.Error(), fragment) {
			t.Fatalf("expected error to contain %q, got %v", fragment, err)
		}
	}
}

func TestValidateAndApplyDefaultsAcceptsEmbeddedTemplateReference(t *testing.T) {
	keyPath := writeTempFile(t, "id_rsa", "key")
	jarPath := writeTempFile(t, "app.jar", "jar")
	valuesPath := writeTempFile(t, "server.values.yml", "ActiveProfile: prod\n")

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
					Script: ScriptConfig{
						Template:   "embedded:server.sh.tmpl",
						ValuesFile: valuesPath,
					},
				},
			},
		},
	}

	if err := ValidateAndApplyDefaults(cfg); err != nil {
		t.Fatalf("unexpected validation error: %v", err)
	}
}

func TestValidateAndApplyDefaultsRejectsUnknownEmbeddedTemplateReference(t *testing.T) {
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
					Script: ScriptConfig{
						Template: "embedded:missing-template.tmpl",
					},
				},
			},
		},
	}

	err := ValidateAndApplyDefaults(cfg)
	if err == nil || !strings.Contains(err.Error(), "servers[0].app.script.template is invalid") {
		t.Fatalf("expected invalid embedded template error, got %v", err)
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
	if cfg.Servers[0].App.Script.RemotePath != "/opt/sample/scripts/server.sh" {
		t.Fatalf("unexpected remote path: %q", cfg.Servers[0].App.Script.RemotePath)
	}
	if cfg.Servers[0].Name != "app.example.com" {
		t.Fatalf("expected server name to default to host, got %q", cfg.Servers[0].Name)
	}
	if cfg.Servers[0].SSHPort != DefaultSSHPort {
		t.Fatalf("expected server ssh port to default to %d, got %d", DefaultSSHPort, cfg.Servers[0].SSHPort)
	}
	if cfg.Servers[0].BastionSSHPort != DefaultSSHPort {
		t.Fatalf("expected server bastion ssh port to default to %d, got %d", DefaultSSHPort, cfg.Servers[0].BastionSSHPort)
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

func TestValidateAndApplyDefaultsAppliesBastionDefaults(t *testing.T) {
	keyPath := writeTempFile(t, "id_rsa", "key")
	jarPath := writeTempFile(t, "app.jar", "jar")

	cfg := &Config{
		SSH: SSHConfig{
			User:    "ec2-user",
			KeyPath: keyPath,
		},
		Bastion: BastionConfig{
			Host: "bastion.example.com",
		},
		Servers: []ServerConfig{
			{
				Host: "10.0.0.10",
				Name: "app-a",
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

	if cfg.Bastion.User != "ec2-user" {
		t.Fatalf("expected bastion user to default from ssh.user, got %q", cfg.Bastion.User)
	}
	if cfg.Bastion.AliasUser != "ec2-user" {
		t.Fatalf("expected bastion alias user to default from ssh.user, got %q", cfg.Bastion.AliasUser)
	}
	if cfg.Bastion.SSHConfigPath == "" {
		t.Fatal("expected bastion ssh config path to be defaulted")
	}
	if cfg.Bastion.TargetKnownHosts == "" {
		t.Fatal("expected bastion target known_hosts path to be defaulted")
	}
}

func TestValidateAndApplyDefaultsRejectsInvalidBastionAliasName(t *testing.T) {
	keyPath := writeTempFile(t, "id_rsa", "key")
	jarPath := writeTempFile(t, "app.jar", "jar")

	cfg := &Config{
		SSH: SSHConfig{
			User:    "deploy",
			KeyPath: keyPath,
		},
		Bastion: BastionConfig{
			Host: "bastion.example.com",
		},
		Servers: []ServerConfig{
			{
				Host: "app.example.com",
				Name: "bad alias",
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
	if err == nil || !strings.Contains(err.Error(), "servers[0].name must contain only letters, numbers, dots, hyphens, or underscores") {
		t.Fatalf("expected bastion alias validation error, got %v", err)
	}
}

func TestValidateAndApplyDefaultsRejectsDuplicateServerNames(t *testing.T) {
	keyPath := writeTempFile(t, "id_rsa", "key")
	jarPath := writeTempFile(t, "app.jar", "jar")

	cfg := &Config{
		SSH: SSHConfig{
			User:    "deploy",
			KeyPath: keyPath,
		},
		Servers: []ServerConfig{
			{
				Host: "app-a.example.com",
				Name: "app",
				App: AppConfig{
					Name:    "sample-a",
					BaseDir: "/opt/sample-a",
					Port:    8080,
					Jar: JarConfig{
						LocalPath:  jarPath,
						RemotePath: "/opt/sample-a/bin/app.jar",
					},
				},
			},
			{
				Host: "app-b.example.com",
				Name: "app",
				App: AppConfig{
					Name:    "sample-b",
					BaseDir: "/opt/sample-b",
					Port:    8081,
					Jar: JarConfig{
						LocalPath:  jarPath,
						RemotePath: "/opt/sample-b/bin/app.jar",
					},
				},
			},
		},
	}

	err := ValidateAndApplyDefaults(cfg)
	if err == nil || !strings.Contains(err.Error(), `server name "app" must be unique`) {
		t.Fatalf("expected duplicate server name validation error, got %v", err)
	}
}

func TestValidateAndApplyDefaultsRejectsInvalidServerSSHPort(t *testing.T) {
	keyPath := writeTempFile(t, "id_rsa", "key")
	jarPath := writeTempFile(t, "app.jar", "jar")

	cfg := &Config{
		SSH: SSHConfig{
			User:    "deploy",
			KeyPath: keyPath,
		},
		Servers: []ServerConfig{
			{
				Host:    "app.example.com",
				SSHPort: 70000,
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
	if err == nil || !strings.Contains(err.Error(), "servers[0].ssh_port must be between 1 and 65535") {
		t.Fatalf("expected invalid server ssh port error, got %v", err)
	}
}

func TestValidateAndApplyDefaultsRejectsInvalidServerBastionSSHPort(t *testing.T) {
	keyPath := writeTempFile(t, "id_rsa", "key")
	jarPath := writeTempFile(t, "app.jar", "jar")

	cfg := &Config{
		SSH: SSHConfig{
			User:    "deploy",
			KeyPath: keyPath,
		},
		Servers: []ServerConfig{
			{
				Host:           "app.example.com",
				SSHPort:        2222,
				BastionHost:    "app-vm",
				BastionSSHPort: 70000,
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
	if err == nil || !strings.Contains(err.Error(), "servers[0].bastion_ssh_port must be between 1 and 65535") {
		t.Fatalf("expected invalid server bastion ssh port error, got %v", err)
	}
}

func TestValidateAndApplyDefaultsSupportsAppsList(t *testing.T) {
	keyPath := writeTempFile(t, "id_rsa", "key")
	jarA := writeTempFile(t, "app-a.jar", "jar")
	jarB := writeTempFile(t, "app-b.jar", "jar")

	cfg := &Config{
		SSH: SSHConfig{
			User:    "deploy",
			KeyPath: keyPath,
		},
		Servers: []ServerConfig{
			{
				Host: "app.example.com",
				Apps: []AppConfig{
					{
						Name:    "sample-a",
						BaseDir: "/opt/sample-a",
						Port:    8081,
						Jar: JarConfig{
							LocalPath:  jarA,
							RemotePath: "/opt/sample-a/lib/app.jar",
						},
					},
					{
						Name:    "sample-b",
						BaseDir: "/opt/sample-b",
						Port:    8082,
						Jar: JarConfig{
							LocalPath:  jarB,
							RemotePath: "/opt/sample-b/lib/app.jar",
						},
					},
				},
			},
		},
	}

	if err := ValidateAndApplyDefaults(cfg); err != nil {
		t.Fatalf("unexpected validation error: %v", err)
	}

	if cfg.Servers[0].Apps[0].Jvm.MinHeap != DefaultJvmMin {
		t.Fatalf("expected default min heap for first app, got %q", cfg.Servers[0].Apps[0].Jvm.MinHeap)
	}
	if cfg.Servers[0].Apps[1].Script.RemotePath != "/opt/sample-b/scripts/server.sh" {
		t.Fatalf("expected default remote path for second app, got %q", cfg.Servers[0].Apps[1].Script.RemotePath)
	}
}

func TestValidateAndApplyDefaultsRejectsMixedAppAndApps(t *testing.T) {
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
					Name:    "legacy",
					BaseDir: "/opt/legacy",
					Port:    8080,
					Jar: JarConfig{
						LocalPath:  jarPath,
						RemotePath: "/opt/legacy/lib/app.jar",
					},
				},
				Apps: []AppConfig{
					{
						Name:    "sample-a",
						BaseDir: "/opt/sample-a",
						Port:    8081,
						Jar: JarConfig{
							LocalPath:  jarPath,
							RemotePath: "/opt/sample-a/lib/app.jar",
						},
					},
				},
			},
		},
	}

	err := ValidateAndApplyDefaults(cfg)
	if err == nil || !strings.Contains(err.Error(), "servers[0] cannot define both app and apps") {
		t.Fatalf("expected mixed app/apps validation error, got %v", err)
	}
}

func TestValidateAndApplyDefaultsSupportsLocalFileScriptMode(t *testing.T) {
	keyPath := writeTempFile(t, "id_rsa", "key")
	jarPath := writeTempFile(t, "app.jar", "jar")
	scriptPath := writeTempFile(t, "server.sh", "#!/bin/sh\nexit 0\n")

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
						RemotePath: "/opt/sample/lib/app.jar",
					},
					Script: ScriptConfig{
						Mode:       ScriptModeLocalFile,
						LocalPath:  scriptPath,
						RemotePath: "/opt/sample/bin/server.sh",
					},
				},
			},
		},
	}

	if err := ValidateAndApplyDefaults(cfg); err != nil {
		t.Fatalf("unexpected validation error: %v", err)
	}
	if cfg.Servers[0].App.Script.Mode != ScriptModeLocalFile {
		t.Fatalf("expected local-file script mode, got %q", cfg.Servers[0].App.Script.Mode)
	}
}

func TestValidateAndApplyDefaultsRejectsLocalFileScriptWithoutPath(t *testing.T) {
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
						RemotePath: "/opt/sample/lib/app.jar",
					},
					Script: ScriptConfig{
						Mode: ScriptModeLocalFile,
					},
				},
			},
		},
	}

	err := ValidateAndApplyDefaults(cfg)
	if err == nil || !strings.Contains(err.Error(), "servers[0].app.script.local_path is required when script.mode is local-file") {
		t.Fatalf("expected missing local script path error, got %v", err)
	}
}

func TestValidateAndApplyDefaultsRejectsTemplateFieldsInLocalFileMode(t *testing.T) {
	keyPath := writeTempFile(t, "id_rsa", "key")
	jarPath := writeTempFile(t, "app.jar", "jar")
	scriptPath := writeTempFile(t, "server.sh", "#!/bin/sh\nexit 0\n")

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
						RemotePath: "/opt/sample/lib/app.jar",
					},
					Script: ScriptConfig{
						Mode:       ScriptModeLocalFile,
						LocalPath:  scriptPath,
						Template:   "embedded:server.sh.tmpl",
						ValuesFile: writeTempFile(t, "server.values.yml", "AppName: sample\n"),
					},
				},
			},
		},
	}

	err := ValidateAndApplyDefaults(cfg)
	if err == nil {
		t.Fatal("expected local-file/template conflict error")
	}
	for _, fragment := range []string{
		"servers[0].app.script.template cannot be used when script.mode is local-file",
		"servers[0].app.script.values_file cannot be used when script.mode is local-file",
	} {
		if !strings.Contains(err.Error(), fragment) {
			t.Fatalf("expected error to contain %q, got %v", fragment, err)
		}
	}
}

func TestValidateAndApplyDefaultsAllowsBootstrapOnlyServerWithDirectories(t *testing.T) {
	keyPath := writeTempFile(t, "id_rsa", "key")

	cfg := &Config{
		SSH: SSHConfig{
			User:    "deploy",
			KeyPath: keyPath,
		},
		Servers: []ServerConfig{
			{
				Host:        "infra.example.com",
				Directories: []string{"/app/elasticsearch"},
				Bootstrap: BootstrapConfig{
					Packages: []string{"docker"},
				},
			},
		},
	}

	if err := ValidateAndApplyDefaults(cfg); err != nil {
		t.Fatalf("unexpected validation error: %v", err)
	}
	if cfg.Servers[0].Directories[0] != "/app/elasticsearch" {
		t.Fatalf("unexpected normalized directory: %q", cfg.Servers[0].Directories[0])
	}
}

func TestValidateAndApplyDefaultsAllowsBootstrapOnlyServerWithServerExtraFiles(t *testing.T) {
	keyPath := writeTempFile(t, "id_rsa", "key")
	tgzPath := writeTempFile(t, "hazelcast.tgz", "archive")

	cfg := &Config{
		SSH: SSHConfig{
			User:    "deploy",
			KeyPath: keyPath,
		},
		Servers: []ServerConfig{
			{
				Host: "infra.example.com",
				ExtraFiles: []ExtraFile{
					{
						LocalPath:  tgzPath,
						RemotePath: "/home/ec2-user/software/hazelcast/hazelcast.tgz",
					},
				},
			},
		},
	}

	if err := ValidateAndApplyDefaults(cfg); err != nil {
		t.Fatalf("unexpected validation error: %v", err)
	}
}

func TestValidateAndApplyDefaultsSupportsExtraFiles(t *testing.T) {
	keyPath := writeTempFile(t, "id_rsa", "key")
	jarPath := writeTempFile(t, "app.jar", "jar")
	extraPath := writeTempFile(t, "setEnv.sh", "#!/bin/sh\n")

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
						RemotePath: "/opt/sample/lib/app.jar",
					},
					ExtraFiles: []ExtraFile{
						{
							LocalPath:  extraPath,
							RemotePath: "/opt/sample/bin/setEnv.sh",
							Chmod:      "774",
						},
					},
				},
			},
		},
	}

	if err := ValidateAndApplyDefaults(cfg); err != nil {
		t.Fatalf("unexpected validation error: %v", err)
	}
}

func TestValidateAndApplyDefaultsRejectsInvalidExtraFileMode(t *testing.T) {
	keyPath := writeTempFile(t, "id_rsa", "key")
	jarPath := writeTempFile(t, "app.jar", "jar")
	extraPath := writeTempFile(t, "setEnv.sh", "#!/bin/sh\n")

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
						RemotePath: "/opt/sample/lib/app.jar",
					},
					ExtraFiles: []ExtraFile{
						{
							LocalPath:  extraPath,
							RemotePath: "/opt/sample/bin/setEnv.sh",
							Chmod:      "not-a-mode",
						},
					},
				},
			},
		},
	}

	err := ValidateAndApplyDefaults(cfg)
	if err == nil || !strings.Contains(err.Error(), "servers[0].app.extra_files[0].chmod must be a 3 or 4 digit octal mode") {
		t.Fatalf("expected invalid chmod error, got %v", err)
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
