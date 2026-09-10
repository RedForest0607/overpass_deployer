package dashboard

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rivo/tview"

	"go-deployer/internal/config"
	"go-deployer/internal/monitor"
)

func TestUIErrorPathsDoNotWaitForUpdateQueue(t *testing.T) {
	for _, action := range []string{"control", "deploy", "busy", "invalid-target"} {
		t.Run(action, func(t *testing.T) {
			a := &application{ui: tview.NewApplication(), activity: tview.NewTextView(), cfg: &config.Config{}}
			done := make(chan struct{})
			go func() {
				defer close(done)
				switch action {
				case "control":
					a.confirmControl(monitor.ActionStart)
				case "deploy":
					a.confirmDeployment(false)
				case "busy":
					a.deploying = true
					a.confirmDeployment(false)
				case "invalid-target":
					a.runDeployment(monitor.Target{}, false)
				}
			}()
			select {
			case <-done:
				if a.activity.GetText(true) == "" {
					t.Fatal("expected error message")
				}
			case <-time.After(time.Second):
				t.Fatal("UI callback blocked on its own update queue")
			}
		})
	}
}

func TestSelectedDeploymentConfigScopesToOneAppAndExcludesServerWideWork(t *testing.T) {
	cfg := &config.Config{
		Bastion:   config.BastionConfig{Host: "bastion.internal"},
		Bootstrap: config.BootstrapConfig{Packages: []string{"curl"}},
		Servers: []config.ServerConfig{{
			Name:        "app-server",
			Host:        "10.0.0.10",
			SSHPort:     2222,
			BastionHost: "app-server.internal",
			Bootstrap:   config.BootstrapConfig{Packages: []string{"jq"}},
			Directories: []string{"/srv/shared"},
			ExtraFiles:  []config.ExtraFile{{RemotePath: "/srv/shared/env"}},
			Utils:       []config.UtilityConfig{{Name: "redis"}},
			Apps:        []config.AppConfig{{Name: "first"}, {Name: "second"}},
		}},
	}
	actual, err := selectedDeploymentConfig(cfg, monitor.Target{ServerIndex: 0, AppIndex: 1})
	if err != nil {
		t.Fatalf("selectedDeploymentConfig() returned error: %v", err)
	}
	if len(actual.Servers) != 1 || len(actual.Servers[0].Apps) != 1 || actual.Servers[0].Apps[0].Name != "second" {
		t.Fatalf("selected deployment should contain only the selected app: %#v", actual.Servers)
	}
	server := actual.Servers[0]
	if actual.Bastion.Enabled() || len(actual.Bootstrap.Packages) > 0 || len(server.Bootstrap.Packages) > 0 || len(server.Directories) > 0 || len(server.ExtraFiles) > 0 || len(server.Utils) > 0 || server.BastionHost != "" {
		t.Fatalf("selected dashboard deployment must exclude server-wide work: %#v", actual)
	}
	if len(cfg.Servers[0].Apps) != 2 || cfg.Bastion.Host == "" {
		t.Fatal("selectedDeploymentConfig() must not mutate the original config")
	}
}

func TestSelectedDeploymentConfigRequiresFullValidationBeforeDeployment(t *testing.T) {
	dir := t.TempDir()
	keyPath := filepath.Join(dir, "id_rsa")
	jarPath := filepath.Join(dir, "app.jar")
	for path, contents := range map[string]string{keyPath: "key", jarPath: "jar"} {
		if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
			t.Fatalf("write fixture %s: %v", path, err)
		}
	}

	cfg := &config.Config{
		SSH: config.SSHConfig{User: "deploy", KeyPath: keyPath, HostKeyChecking: config.HostKeyInsecure},
		Servers: []config.ServerConfig{{
			Name: "app-server",
			Host: "10.0.0.10",
			Apps: []config.AppConfig{{
				Name:    "sample-api",
				BaseDir: "/srv/sample-api",
				Port:    8080,
				Jar:     config.JarConfig{LocalPath: jarPath, RemotePath: "/srv/sample-api/lib/app.jar"},
			}},
		}},
	}

	selected, err := selectedDeploymentConfig(cfg, monitor.Target{})
	if err != nil {
		t.Fatalf("selectedDeploymentConfig() returned error: %v", err)
	}
	if err := config.ValidateAndApplyDefaults(selected); err != nil {
		t.Fatalf("selected deployment config should pass full validation: %v", err)
	}

	selected.Servers[0].Apps[0].Jar.LocalPath = filepath.Join(dir, "missing.jar")
	err = config.ValidateAndApplyDefaults(selected)
	if err == nil || !strings.Contains(err.Error(), "jar.local_path does not exist") {
		t.Fatalf("expected full deployment validation to reject a missing JAR, got %v", err)
	}
}

func TestSelectedDeploymentConfigRejectsInvalidTarget(t *testing.T) {
	cfg := &config.Config{Servers: []config.ServerConfig{{Apps: []config.AppConfig{{Name: "app"}}}}}
	if _, err := selectedDeploymentConfig(cfg, monitor.Target{ServerIndex: 0, AppIndex: 1}); err == nil {
		t.Fatal("expected invalid application selection to fail")
	}
}
