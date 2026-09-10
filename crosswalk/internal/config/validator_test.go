package config

import (
	"os"
	"reflect"
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
						RemotePath: "/opt/sample/lib/app.jar",
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
			Services: []ServiceConfig{
				{Name: "nginx"},
			},
			JDK: JDKConfig{
				Vendor: "corretto",
				Major:  21,
			},
			Timezone: TimezoneConfig{
				Name: "Asia/Seoul",
			},
			Swap: SwapConfig{
				Enabled: boolPtr(true),
				Size:    "4G",
			},
			DockerCompose: DockerComposeConfig{
				Enabled:     boolPtr(true),
				Version:     "v2.29.7",
				SourceURL:   "https://github.com/docker/compose/releases/download/v2.29.7/docker-compose-linux-x86_64",
				InstallPath: "/opt/docker-compose",
			},
		},
		Servers: []ServerConfig{
			{
				Host: "app.example.com",
				Bootstrap: BootstrapConfig{
					Packages: []string{"git", "unzip"},
					Services: []ServiceConfig{
						{Name: "nginx", State: "restarted"},
					},
					JDK: JDKConfig{
						Major:    17,
						Headless: &headlessFalse,
					},
					DockerCompose: DockerComposeConfig{
						InstallPath: "/usr/local/lib/docker/cli-plugins/docker-compose",
					},
				},
				App: AppConfig{
					Name:    "sample",
					BaseDir: "/opt/sample",
					Port:    8080,
					Jar: JarConfig{
						LocalPath:  jarPath,
						RemotePath: "/opt/sample/lib/app.jar",
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
	if cfg.Servers[0].Bootstrap.Timezone.Name != "Asia/Seoul" {
		t.Fatalf("expected merged timezone Asia/Seoul, got %q", cfg.Servers[0].Bootstrap.Timezone.Name)
	}
	if cfg.Servers[0].Bootstrap.Swap.Enabled == nil || !*cfg.Servers[0].Bootstrap.Swap.Enabled {
		t.Fatalf("expected merged swap enabled, got %#v", cfg.Servers[0].Bootstrap.Swap.Enabled)
	}
	if cfg.Servers[0].Bootstrap.Swap.Path != "/swapfile" {
		t.Fatalf("expected default swap path /swapfile, got %q", cfg.Servers[0].Bootstrap.Swap.Path)
	}
	if cfg.Servers[0].Bootstrap.Swap.Size != "4G" {
		t.Fatalf("expected merged swap size 4G, got %q", cfg.Servers[0].Bootstrap.Swap.Size)
	}
	gotServices := cfg.Servers[0].Bootstrap.Services
	if len(gotServices) != 1 || gotServices[0].Name != "nginx" || gotServices[0].State != ServiceStateRestarted || gotServices[0].Enabled == nil || !*gotServices[0].Enabled {
		t.Fatalf("unexpected merged services: %#v", gotServices)
	}
	gotCompose := cfg.Servers[0].Bootstrap.DockerCompose
	if gotCompose.Enabled == nil || !*gotCompose.Enabled {
		t.Fatalf("expected merged docker compose enabled, got %#v", gotCompose.Enabled)
	}
	if gotCompose.Version != "v2.29.7" {
		t.Fatalf("expected merged docker compose version, got %q", gotCompose.Version)
	}
	if gotCompose.SourceURL != "https://github.com/docker/compose/releases/download/v2.29.7/docker-compose-linux-x86_64" {
		t.Fatalf("expected merged docker compose source URL, got %q", gotCompose.SourceURL)
	}
	if gotCompose.InstallPath != "/usr/local/lib/docker/cli-plugins/docker-compose" {
		t.Fatalf("expected overridden docker compose install path, got %q", gotCompose.InstallPath)
	}
}

func TestValidateAndApplyDefaultsMergesBootstrapUsersAndSysctls(t *testing.T) {
	keyPath := writeTempFile(t, "id_rsa", "key")

	cfg := &Config{
		SSH: SSHConfig{
			User:    "deploy",
			KeyPath: keyPath,
		},
		Bootstrap: BootstrapConfig{
			Users: []UserConfig{
				{Name: "elasticsearch", Group: "elastic", Shell: "/sbin/nologin"},
			},
			Sysctls: []SysctlConfig{
				{Name: "vm.max_map_count", Value: "65530"},
			},
		},
		Servers: []ServerConfig{
			{
				Host: "search.example.com",
				Bootstrap: BootstrapConfig{
					Users: []UserConfig{
						{Name: "elasticsearch", Group: "elasticsearch", Home: "/home/elasticsearch"},
					},
					Sysctls: []SysctlConfig{
						{Name: "vm.max_map_count", Value: "262144", PersistPath: "/etc/sysctl.d/99-elasticsearch.conf"},
					},
				},
			},
		},
	}

	if err := ValidateAndApplyDefaults(cfg); err != nil {
		t.Fatalf("unexpected validation error: %v", err)
	}

	users := cfg.Servers[0].Bootstrap.Users
	if len(users) != 1 {
		t.Fatalf("expected one merged user, got %#v", users)
	}
	if users[0].Group != "elasticsearch" || users[0].Home != "/home/elasticsearch" {
		t.Fatalf("unexpected merged user: %#v", users[0])
	}
	if users[0].System == nil || !*users[0].System {
		t.Fatalf("expected system user default true, got %#v", users[0].System)
	}
	if users[0].CreateHome == nil || *users[0].CreateHome {
		t.Fatalf("expected create_home default false, got %#v", users[0].CreateHome)
	}

	sysctls := cfg.Servers[0].Bootstrap.Sysctls
	if len(sysctls) != 1 {
		t.Fatalf("expected one merged sysctl, got %#v", sysctls)
	}
	if sysctls[0].Value != "262144" || sysctls[0].PersistPath != "/etc/sysctl.d/99-elasticsearch.conf" {
		t.Fatalf("unexpected merged sysctl: %#v", sysctls[0])
	}
}

func TestValidateAndApplyDefaultsAllowsServerToDisableMergedJDK(t *testing.T) {
	keyPath := writeTempFile(t, "id_rsa", "key")
	disabled := false

	cfg := &Config{
		SSH: SSHConfig{
			User:    "deploy",
			KeyPath: keyPath,
		},
		Bootstrap: BootstrapConfig{
			JDK: JDKConfig{
				Vendor: "corretto",
				Major:  25,
			},
		},
		Servers: []ServerConfig{
			{
				Host: "postgresql.example.com",
				Bootstrap: BootstrapConfig{
					JDK: JDKConfig{
						Enabled: &disabled,
					},
				},
				Directories: []string{"/home/ec2-user/software/postgresql"},
			},
		},
	}

	if err := ValidateAndApplyDefaults(cfg); err != nil {
		t.Fatalf("unexpected validation error: %v", err)
	}

	got := cfg.Servers[0].Bootstrap.JDK
	if got.Enabled == nil || *got.Enabled {
		t.Fatalf("expected merged jdk enabled false, got %#v", got.Enabled)
	}
	if got.Vendor != "" || got.Major != 0 || got.Headless != nil {
		t.Fatalf("expected disabled jdk to clear package fields, got %#v", got)
	}
}

func TestValidateAndApplyDefaultsTreatsDisabledServiceAsStoppedByDefault(t *testing.T) {
	keyPath := writeTempFile(t, "id_rsa", "key")
	disabled := false

	cfg := &Config{
		SSH: SSHConfig{
			User:    "deploy",
			KeyPath: keyPath,
		},
		Servers: []ServerConfig{
			{
				Host: "web.example.com",
				Bootstrap: BootstrapConfig{
					Services: []ServiceConfig{
						{Name: "nginx", Enabled: &disabled},
					},
				},
			},
		},
	}

	if err := ValidateAndApplyDefaults(cfg); err != nil {
		t.Fatalf("unexpected validation error: %v", err)
	}

	got := cfg.Servers[0].Bootstrap.Services[0]
	if got.State != ServiceStateStopped {
		t.Fatalf("expected disabled service to default to stopped, got %q", got.State)
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
			Swap: SwapConfig{
				Enabled: boolPtr(true),
				Path:    "swapfile",
				Size:    "0G",
			},
			Services: []ServiceConfig{
				{Name: "", State: "booted"},
			},
			DockerCompose: DockerComposeConfig{
				Enabled:     boolPtr(true),
				SourceURL:   "http://example.com/docker-compose",
				InstallPath: "usr/local/bin/docker-compose",
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
						RemotePath: "/opt/sample/lib/app.jar",
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
		"bootstrap.swap.path must be an absolute path",
		"bootstrap.swap.size must be a positive size like 4G, 512M, or 1024K",
		"bootstrap.services[0].name must not be empty",
		`bootstrap.services[0].state must be one of "started", "stopped", or "restarted"`,
		"bootstrap.docker_compose.version is required when enabled is true",
		"bootstrap.docker_compose.source_url must be an https URL",
		"bootstrap.docker_compose.install_path must be an absolute path",
	} {
		if !strings.Contains(err.Error(), fragment) {
			t.Fatalf("expected error to contain %q, got %v", fragment, err)
		}
	}
}

func TestValidateAndApplyDefaultsRejectsInvalidBootstrapUsersAndSysctls(t *testing.T) {
	keyPath := writeTempFile(t, "id_rsa", "key")

	cfg := &Config{
		SSH: SSHConfig{
			User:    "deploy",
			KeyPath: keyPath,
		},
		Bootstrap: BootstrapConfig{
			Users: []UserConfig{
				{Name: "bad user", UID: -1, GID: -1, Home: "home/elasticsearch", Shell: "nologin"},
			},
			Sysctls: []SysctlConfig{
				{Name: "vm max", Value: "", PersistPath: "etc/sysctl.conf"},
			},
		},
		Servers: []ServerConfig{
			{Host: "search.example.com"},
		},
	}

	err := ValidateAndApplyDefaults(cfg)
	if err == nil {
		t.Fatal("expected bootstrap users/sysctl validation error")
	}

	for _, fragment := range []string{
		"bootstrap.users[0].name contains unsupported characters",
		"bootstrap.users[0].uid must be greater than or equal to zero",
		"bootstrap.users[0].gid must be greater than or equal to zero",
		"bootstrap.users[0].home must be an absolute path",
		"bootstrap.users[0].shell must be an absolute path",
		"bootstrap.sysctl[0].name contains unsupported characters",
		"bootstrap.sysctl[0].value is required",
		"bootstrap.sysctl[0].persist_path must be an absolute path",
	} {
		if !strings.Contains(err.Error(), fragment) {
			t.Fatalf("expected error to contain %q, got %v", fragment, err)
		}
	}
}

func TestValidateUtilityConfigsRejectsDuplicateAndEmptyDeclarations(t *testing.T) {
	utilities := []UtilityConfig{
		{Name: "redis", Directories: []string{"/app/redis"}, Services: []ServiceConfig{{Name: "redis-server"}}},
		{Name: "redis"},
		{Name: ""},
	}
	var errs []string
	validateUtilityConfigs(utilities, "servers[0].utils", &errs, func(string, string) {}, false)

	joined := strings.Join(errs, "\n")
	for _, fragment := range []string{
		`servers[0].utils[1].name "redis" must be unique per server`,
		"servers[0].utils[1] must define directories, files, or services",
		"servers[0].utils[2].name must not be empty",
	} {
		if !strings.Contains(joined, fragment) {
			t.Fatalf("expected error to contain %q, got %s", fragment, joined)
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
						RemotePath: "/opt/sample/lib/app.jar",
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
						RemotePath: "/opt/sample/lib/app.jar",
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
						RemotePath: "/opt/sample/lib/app.jar",
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
	if cfg.Servers[0].App.Script.RemotePath != "/opt/sample/bin/server.sh" {
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
						RemotePath: "/opt/sample/lib/app.jar",
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
						RemotePath: "/opt/sample/lib/app.jar",
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
	if cfg.Bastion.ShellAliasesPath == "" {
		t.Fatal("expected bastion shell aliases path to be defaulted")
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
						RemotePath: "/opt/sample/lib/app.jar",
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
						RemotePath: "/opt/sample-a/lib/app.jar",
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
						RemotePath: "/opt/sample-b/lib/app.jar",
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
						RemotePath: "/opt/sample/lib/app.jar",
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
						RemotePath: "/opt/sample/lib/app.jar",
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
	if cfg.Servers[0].Apps[1].Script.RemotePath != "/opt/sample-b/bin/server.sh" {
		t.Fatalf("expected default remote path for second app, got %q", cfg.Servers[0].Apps[1].Script.RemotePath)
	}
}

func TestValidateAndApplyDefaultsNormalizesTags(t *testing.T) {
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
				Tags: []string{" Wave1 ", "apm", "wave1"},
				App: AppConfig{
					Name:    "sample",
					Tags:    []string{" API ", "api"},
					BaseDir: "/opt/sample",
					Port:    8080,
					Jar: JarConfig{
						LocalPath:  jarPath,
						RemotePath: "/opt/sample/lib/app.jar",
					},
				},
			},
		},
	}

	if err := ValidateAndApplyDefaults(cfg); err != nil {
		t.Fatalf("unexpected validation error: %v", err)
	}
	if !reflect.DeepEqual(cfg.Servers[0].Tags, []string{"wave1", "apm"}) {
		t.Fatalf("expected normalized server tags, got %v", cfg.Servers[0].Tags)
	}
	if !reflect.DeepEqual(cfg.Servers[0].App.Tags, []string{"api"}) {
		t.Fatalf("expected normalized app tags, got %v", cfg.Servers[0].App.Tags)
	}
}

func TestValidateAndApplyDefaultsRejectsInvalidTags(t *testing.T) {
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
				Tags: []string{"bad tag"},
				App: AppConfig{
					Name:    "sample",
					Tags:    []string{"ok", ""},
					BaseDir: "/opt/sample",
					Port:    8080,
					Jar: JarConfig{
						LocalPath:  jarPath,
						RemotePath: "/opt/sample/lib/app.jar",
					},
				},
			},
		},
	}

	err := ValidateAndApplyDefaults(cfg)
	if err == nil {
		t.Fatal("expected invalid tag validation error")
	}
	for _, fragment := range []string{
		"servers[0].tags[0] must contain only letters, numbers, dots, hyphens, or underscores",
		"servers[0].app.tags[1] must not be empty",
	} {
		if !strings.Contains(err.Error(), fragment) {
			t.Fatalf("expected error to contain %q, got %v", fragment, err)
		}
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
						Extract: ExtractConfig{
							Enabled: true,
						},
					},
				},
			},
		},
	}

	if err := ValidateAndApplyDefaults(cfg); err != nil {
		t.Fatalf("unexpected validation error: %v", err)
	}
	if got := cfg.Servers[0].ExtraFiles[0].Extract.RemoteDir; got != "/home/ec2-user/software/hazelcast" {
		t.Fatalf("unexpected default extract directory: %q", got)
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

func TestValidateAndApplyDefaultsRejectsProdSampleEnvPaths(t *testing.T) {
	keyPath := writeTempFile(t, "id_rsa", "key")
	jarPath := writeTempFile(t, "app.jar", "jar")
	envSamplePath := writeTempFile(t, ".env.sample", "APP_ENV=sample\n")

	cfg := &Config{
		SSH: SSHConfig{
			User:    "deploy",
			KeyPath: keyPath,
		},
		Servers: []ServerConfig{
			{
				Host: "prd-app.example.com",
				Tags: []string{"prd"},
				App: AppConfig{
					Name:    "sample",
					BaseDir: "/opt/sample",
					Port:    8080,
					Jar: JarConfig{
						LocalPath:  jarPath,
						RemotePath: "/opt/sample/lib/app.jar",
					},
					ConfigFiles: []ConfigFile{
						{LocalPath: envSamplePath, RemotePath: "/opt/sample/.env"},
					},
					ExtraFiles: []ExtraFile{
						{LocalPath: envSamplePath, RemotePath: "/opt/sample/.env.backup"},
					},
				},
			},
		},
	}

	err := ValidateAndApplyDefaults(cfg)
	if err == nil {
		t.Fatal("expected prod sample env validation error")
	}
	for _, fragment := range []string{
		"servers[0].app.config_files[0].local_path cannot reference .env.sample for prod/prd servers",
		"servers[0].app.extra_files[0].local_path cannot reference .env.sample for prod/prd servers",
	} {
		if !strings.Contains(err.Error(), fragment) {
			t.Fatalf("expected error to contain %q, got %v", fragment, err)
		}
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

func TestValidateAndApplyDefaultsRejectsUnsupportedExtractArchive(t *testing.T) {
	keyPath := writeTempFile(t, "id_rsa", "key")
	zipPath := writeTempFile(t, "software.zip", "archive")

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
						LocalPath:  zipPath,
						RemotePath: "/home/ec2-user/software/software.zip",
						Extract: ExtractConfig{
							Enabled: true,
						},
					},
				},
			},
		},
	}

	err := ValidateAndApplyDefaults(cfg)
	if err == nil || !strings.Contains(err.Error(), "servers[0].extra_files[0].extract.enabled supports only .tar, .tar.gz, or .tgz archives") {
		t.Fatalf("expected unsupported archive error, got %v", err)
	}
}

func TestValidateAndApplyDefaultsNormalizesDirectoryDefaultRules(t *testing.T) {
	keyPath := writeTempFile(t, "id_rsa", "key")

	cfg := &Config{
		SSH: SSHConfig{
			User:    "deploy",
			KeyPath: keyPath,
		},
		DirectoryDefaults: DirectoryDefaultsConfig{
			Rules: []DirectoryOwnershipRule{
				{Prefix: "/app/overpass/..", Owner: "ec2-user"},
			},
		},
		Servers: []ServerConfig{
			{
				Host:        "app.example.com",
				Directories: []string{"/app/overpass/sample"},
			},
		},
	}

	if err := ValidateAndApplyDefaults(cfg); err != nil {
		t.Fatalf("unexpected validation error: %v", err)
	}

	got := cfg.DirectoryDefaults.Rules[0]
	if got.Prefix != "/app" {
		t.Fatalf("expected normalized prefix /app, got %q", got.Prefix)
	}
	if got.Group != "ec2-user" {
		t.Fatalf("expected group to default to owner, got %q", got.Group)
	}
}

func TestValidateAndApplyDefaultsRejectsInvalidDirectoryDefaultRules(t *testing.T) {
	keyPath := writeTempFile(t, "id_rsa", "key")

	cfg := &Config{
		SSH: SSHConfig{
			User:    "deploy",
			KeyPath: keyPath,
		},
		DirectoryDefaults: DirectoryDefaultsConfig{
			Rules: []DirectoryOwnershipRule{
				{Prefix: "app", Owner: "ec2-user"},
				{Prefix: "/app", Owner: "bad user"},
				{Prefix: "/app", Owner: "ec2-user"},
			},
		},
		Servers: []ServerConfig{
			{
				Host:        "app.example.com",
				Directories: []string{"/app/overpass/sample"},
			},
		},
	}

	err := ValidateAndApplyDefaults(cfg)
	if err == nil {
		t.Fatal("expected directory defaults validation error")
	}

	for _, fragment := range []string{
		"directory_defaults.rules[0].prefix must be an absolute path",
		"directory_defaults.rules[1].owner contains unsupported characters",
		"directory_defaults.rules[2].prefix must be unique: /app",
	} {
		if !strings.Contains(err.Error(), fragment) {
			t.Fatalf("expected error to contain %q, got %v", fragment, err)
		}
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
