package vm

import (
	"os"
	"testing"

	"go-deployer/internal/config"
	"go-deployer/internal/ssh"
)

func TestRunWithOptionsDryRunSkipsSSHConnect(t *testing.T) {
	t.Helper()

	originalConnect := connectSSH
	t.Cleanup(func() {
		connectSSH = originalConnect
	})

	connectCalled := false
	connectSSH = func(sshCfg config.SSHConfig, host string) (*ssh.Client, error) {
		connectCalled = true
		return nil, nil
	}

	cfg := &config.Config{
		SSH: config.SSHConfig{
			User:            "ubuntu",
			KeyPath:         "/tmp/id_rsa",
			KnownHosts:      "/tmp/known_hosts",
			HostKeyChecking: "strict",
			Port:            22,
			TimeoutSec:      30,
		},
		Bastion: config.BastionConfig{
			Host: "bastion.example.internal",
			Port: 22,
		},
		Servers: []config.ServerConfig{
			{
				Name: "app-01",
				Host: "10.0.0.10",
				App: config.AppConfig{
					BaseDir: "/srv/app",
					Jar: config.JarConfig{
						LocalPath:  tempFile(t, "app.jar"),
						RemotePath: "/srv/app/bin/app.jar",
					},
					ConfigFiles: []config.ConfigFile{
						{Local: tempFile(t, "application.yml"), Remote: "/srv/app/config/application.yml"},
					},
					Script: config.ScriptConfig{
						RemoteDir: "/srv/app/scripts",
					},
				},
			},
		},
	}

	if err := RunWithOptions(cfg, RunOptions{DryRun: true}); err != nil {
		t.Fatalf("expected dry-run to succeed, got %v", err)
	}
	if connectCalled {
		t.Fatalf("expected dry-run to skip ssh connect")
	}
}

func tempFile(t *testing.T, name string) string {
	t.Helper()

	path := t.TempDir() + "/" + name
	if err := os.WriteFile(path, []byte("test"), 0o644); err != nil {
		t.Fatalf("write temp file: %v", err)
	}
	return path
}
