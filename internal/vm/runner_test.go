package vm

import (
	"fmt"
	"os"
	"reflect"
	"testing"

	"go-deployer/internal/config"
	"go-deployer/internal/ssh"
)

func TestRunWithOptionsDryRunSkipsSSHConnect(t *testing.T) {
	t.Helper()

	originalConnect := connectSSH
	originalBootstrap := bootstrapHostStep
	originalCreateServerDirs := createServerDirsStep
	originalDeployServerFiles := deployServerFilesStep
	originalCreateDirectories := createDirectoriesStep
	originalDeployJar := deployJarStep
	originalDeployConfigs := deployConfigFilesStep
	originalDeployExtraFiles := deployExtraFilesStep
	originalDeployScripts := deployScriptsStep
	t.Cleanup(func() {
		connectSSH = originalConnect
		bootstrapHostStep = originalBootstrap
		createServerDirsStep = originalCreateServerDirs
		deployServerFilesStep = originalDeployServerFiles
		createDirectoriesStep = originalCreateDirectories
		deployJarStep = originalDeployJar
		deployConfigFilesStep = originalDeployConfigs
		deployExtraFilesStep = originalDeployExtraFiles
		deployScriptsStep = originalDeployScripts
	})

	connectCalled := false
	connectSSH = func(sshCfg config.SSHConfig, host string) (*ssh.Client, error) {
		connectCalled = true
		return nil, nil
	}
	bootstrapHostStep = func(runner ssh.Runner, bootstrap config.BootstrapConfig, opts RunOptions, host string) error {
		return nil
	}
	createServerDirsStep = func(runner ssh.Runner, directories []string, opts RunOptions, host string) error {
		return nil
	}
	deployServerFilesStep = func(client *ssh.Client, extraFiles []config.ExtraFile, opts RunOptions, host string) error {
		return nil
	}
	createDirectoriesStep = func(runner ssh.Runner, app *config.AppConfig, opts RunOptions, host string) error {
		return nil
	}
	deployJarStep = func(client *ssh.Client, app *config.AppConfig, opts RunOptions, host string) error {
		return nil
	}
	deployConfigFilesStep = func(client *ssh.Client, app *config.AppConfig, opts RunOptions, host string) error {
		return nil
	}
	deployExtraFilesStep = func(client *ssh.Client, app *config.AppConfig, opts RunOptions, host string) error {
		return nil
	}
	deployScriptsStep = func(client *ssh.Client, app *config.AppConfig, opts RunOptions, host string) error {
		return nil
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
						{LocalPath: tempFile(t, "application.yml"), RemotePath: "/srv/app/config/application.yml"},
					},
					Script: config.ScriptConfig{
						RemotePath: "/srv/app/scripts/server.sh",
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

func TestRunWithOptionsDryRunBootstrapsBeforeOtherSteps(t *testing.T) {
	t.Helper()

	originalBootstrap := bootstrapHostStep
	originalCreateServerDirs := createServerDirsStep
	originalDeployServerFiles := deployServerFilesStep
	originalCreateDirectories := createDirectoriesStep
	originalDeployJar := deployJarStep
	originalDeployConfigs := deployConfigFilesStep
	originalDeployExtraFiles := deployExtraFilesStep
	originalDeployScripts := deployScriptsStep
	t.Cleanup(func() {
		bootstrapHostStep = originalBootstrap
		createServerDirsStep = originalCreateServerDirs
		deployServerFilesStep = originalDeployServerFiles
		createDirectoriesStep = originalCreateDirectories
		deployJarStep = originalDeployJar
		deployConfigFilesStep = originalDeployConfigs
		deployExtraFilesStep = originalDeployExtraFiles
		deployScriptsStep = originalDeployScripts
	})

	var order []string
	bootstrapHostStep = func(runner ssh.Runner, bootstrap config.BootstrapConfig, opts RunOptions, host string) error {
		order = append(order, "bootstrap")
		return nil
	}
	createServerDirsStep = func(runner ssh.Runner, directories []string, opts RunOptions, host string) error {
		order = append(order, "server-dirs")
		return nil
	}
	deployServerFilesStep = func(client *ssh.Client, extraFiles []config.ExtraFile, opts RunOptions, host string) error {
		order = append(order, "server-extra-files")
		return nil
	}
	createDirectoriesStep = func(runner ssh.Runner, app *config.AppConfig, opts RunOptions, host string) error {
		order = append(order, "dirs")
		return nil
	}
	deployJarStep = func(client *ssh.Client, app *config.AppConfig, opts RunOptions, host string) error {
		order = append(order, "jar")
		return nil
	}
	deployConfigFilesStep = func(client *ssh.Client, app *config.AppConfig, opts RunOptions, host string) error {
		order = append(order, "configs")
		return nil
	}
	deployExtraFilesStep = func(client *ssh.Client, app *config.AppConfig, opts RunOptions, host string) error {
		order = append(order, "extra-files")
		return nil
	}
	deployScriptsStep = func(client *ssh.Client, app *config.AppConfig, opts RunOptions, host string) error {
		order = append(order, "scripts")
		return nil
	}

	cfg := &config.Config{
		SSH: config.SSHConfig{
			User:    "ubuntu",
			KeyPath: "/tmp/id_rsa",
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
				},
			},
		},
	}

	if err := RunWithOptions(cfg, RunOptions{DryRun: true}); err != nil {
		t.Fatalf("expected dry-run to succeed, got %v", err)
	}

	want := []string{"bootstrap", "dirs", "jar", "configs", "extra-files", "scripts"}
	if !reflect.DeepEqual(order, want) {
		t.Fatalf("unexpected step order: got %v want %v", order, want)
	}
}

func TestRunWithOptionsStopsWhenBootstrapFails(t *testing.T) {
	t.Helper()

	originalBootstrap := bootstrapHostStep
	originalCreateServerDirs := createServerDirsStep
	originalDeployServerFiles := deployServerFilesStep
	originalCreateDirectories := createDirectoriesStep
	t.Cleanup(func() {
		bootstrapHostStep = originalBootstrap
		createServerDirsStep = originalCreateServerDirs
		deployServerFilesStep = originalDeployServerFiles
		createDirectoriesStep = originalCreateDirectories
	})

	bootstrapHostStep = func(runner ssh.Runner, bootstrap config.BootstrapConfig, opts RunOptions, host string) error {
		return fmt.Errorf("bootstrap failed")
	}
	createServerDirsStep = func(runner ssh.Runner, directories []string, opts RunOptions, host string) error {
		return nil
	}
	deployServerFilesStep = func(client *ssh.Client, extraFiles []config.ExtraFile, opts RunOptions, host string) error {
		return nil
	}
	createCalled := false
	createDirectoriesStep = func(runner ssh.Runner, app *config.AppConfig, opts RunOptions, host string) error {
		createCalled = true
		return nil
	}

	cfg := &config.Config{
		SSH: config.SSHConfig{
			User:    "ubuntu",
			KeyPath: "/tmp/id_rsa",
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
				},
			},
		},
	}

	err := RunWithOptions(cfg, RunOptions{DryRun: true})
	if err == nil || err.Error() != "plan bootstrap: bootstrap failed" {
		t.Fatalf("expected bootstrap failure to bubble up, got %v", err)
	}
	if createCalled {
		t.Fatalf("expected create directories step to be skipped after bootstrap failure")
	}
}

func TestRunWithOptionsDryRunDeploysEachAppOnServer(t *testing.T) {
	t.Helper()

	originalBootstrap := bootstrapHostStep
	originalCreateServerDirs := createServerDirsStep
	originalDeployServerFiles := deployServerFilesStep
	originalCreateDirectories := createDirectoriesStep
	originalDeployJar := deployJarStep
	originalDeployConfigs := deployConfigFilesStep
	originalDeployExtraFiles := deployExtraFilesStep
	originalDeployScripts := deployScriptsStep
	t.Cleanup(func() {
		bootstrapHostStep = originalBootstrap
		createServerDirsStep = originalCreateServerDirs
		deployServerFilesStep = originalDeployServerFiles
		createDirectoriesStep = originalCreateDirectories
		deployJarStep = originalDeployJar
		deployConfigFilesStep = originalDeployConfigs
		deployExtraFilesStep = originalDeployExtraFiles
		deployScriptsStep = originalDeployScripts
	})

	var order []string
	bootstrapHostStep = func(runner ssh.Runner, bootstrap config.BootstrapConfig, opts RunOptions, host string) error {
		order = append(order, "bootstrap")
		return nil
	}
	createServerDirsStep = func(runner ssh.Runner, directories []string, opts RunOptions, host string) error {
		order = append(order, "server-dirs")
		return nil
	}
	deployServerFilesStep = func(client *ssh.Client, extraFiles []config.ExtraFile, opts RunOptions, host string) error {
		order = append(order, "server-extra-files")
		return nil
	}
	createDirectoriesStep = func(runner ssh.Runner, app *config.AppConfig, opts RunOptions, host string) error {
		order = append(order, "dirs:"+app.Name)
		return nil
	}
	deployJarStep = func(client *ssh.Client, app *config.AppConfig, opts RunOptions, host string) error {
		order = append(order, "jar:"+app.Name)
		return nil
	}
	deployConfigFilesStep = func(client *ssh.Client, app *config.AppConfig, opts RunOptions, host string) error {
		order = append(order, "configs:"+app.Name)
		return nil
	}
	deployExtraFilesStep = func(client *ssh.Client, app *config.AppConfig, opts RunOptions, host string) error {
		order = append(order, "extra-files:"+app.Name)
		return nil
	}
	deployScriptsStep = func(client *ssh.Client, app *config.AppConfig, opts RunOptions, host string) error {
		order = append(order, "scripts:"+app.Name)
		return nil
	}

	cfg := &config.Config{
		SSH: config.SSHConfig{
			User:    "ubuntu",
			KeyPath: "/tmp/id_rsa",
		},
		Servers: []config.ServerConfig{
			{
				Name: "devwas",
				Host: "10.0.0.10",
				Apps: []config.AppConfig{
					{
						Name:    "app-a",
						BaseDir: "/srv/app-a",
						Jar: config.JarConfig{
							LocalPath:  tempFile(t, "app-a.jar"),
							RemotePath: "/srv/app-a/bin/app.jar",
						},
					},
					{
						Name:    "app-b",
						BaseDir: "/srv/app-b",
						Jar: config.JarConfig{
							LocalPath:  tempFile(t, "app-b.jar"),
							RemotePath: "/srv/app-b/bin/app.jar",
						},
					},
				},
			},
		},
	}

	if err := RunWithOptions(cfg, RunOptions{DryRun: true}); err != nil {
		t.Fatalf("expected dry-run to succeed, got %v", err)
	}

	want := []string{
		"bootstrap",
		"dirs:app-a", "jar:app-a", "configs:app-a", "extra-files:app-a", "scripts:app-a",
		"dirs:app-b", "jar:app-b", "configs:app-b", "extra-files:app-b", "scripts:app-b",
	}
	if !reflect.DeepEqual(order, want) {
		t.Fatalf("unexpected step order: got %v want %v", order, want)
	}
}

func TestRunWithOptionsDryRunSupportsBootstrapOnlyServer(t *testing.T) {
	t.Helper()

	originalBootstrap := bootstrapHostStep
	originalCreateServerDirs := createServerDirsStep
	originalDeployServerFiles := deployServerFilesStep
	t.Cleanup(func() {
		bootstrapHostStep = originalBootstrap
		createServerDirsStep = originalCreateServerDirs
		deployServerFilesStep = originalDeployServerFiles
	})

	var order []string
	bootstrapHostStep = func(runner ssh.Runner, bootstrap config.BootstrapConfig, opts RunOptions, host string) error {
		order = append(order, "bootstrap")
		return nil
	}
	createServerDirsStep = func(runner ssh.Runner, directories []string, opts RunOptions, host string) error {
		order = append(order, "server-dirs:"+directories[0])
		return nil
	}
	deployServerFilesStep = func(client *ssh.Client, extraFiles []config.ExtraFile, opts RunOptions, host string) error {
		order = append(order, "server-extra-files")
		return nil
	}

	cfg := &config.Config{
		SSH: config.SSHConfig{
			User:    "ubuntu",
			KeyPath: "/tmp/id_rsa",
		},
		Servers: []config.ServerConfig{
			{
				Name:        "devapp1",
				Host:        "10.0.0.20",
				Directories: []string{"/app/elasticsearch"},
				Bootstrap: config.BootstrapConfig{
					Packages: []string{"docker"},
				},
			},
		},
	}

	if err := RunWithOptions(cfg, RunOptions{DryRun: true}); err != nil {
		t.Fatalf("expected bootstrap-only dry-run to succeed, got %v", err)
	}

	want := []string{"bootstrap", "server-dirs:/app/elasticsearch"}
	if !reflect.DeepEqual(order, want) {
		t.Fatalf("unexpected bootstrap-only step order: got %v want %v", order, want)
	}
}

func TestRunWithOptionsDryRunSupportsServerExtraFiles(t *testing.T) {
	t.Helper()

	originalBootstrap := bootstrapHostStep
	originalCreateServerDirs := createServerDirsStep
	originalDeployServerFiles := deployServerFilesStep
	t.Cleanup(func() {
		bootstrapHostStep = originalBootstrap
		createServerDirsStep = originalCreateServerDirs
		deployServerFilesStep = originalDeployServerFiles
	})

	var order []string
	bootstrapHostStep = func(runner ssh.Runner, bootstrap config.BootstrapConfig, opts RunOptions, host string) error {
		order = append(order, "bootstrap")
		return nil
	}
	createServerDirsStep = func(runner ssh.Runner, directories []string, opts RunOptions, host string) error {
		order = append(order, "server-dirs")
		return nil
	}
	deployServerFilesStep = func(client *ssh.Client, extraFiles []config.ExtraFile, opts RunOptions, host string) error {
		order = append(order, "server-extra-files:"+extraFiles[0].RemotePath)
		return nil
	}

	cfg := &config.Config{
		SSH: config.SSHConfig{
			User:    "ubuntu",
			KeyPath: "/tmp/id_rsa",
		},
		Servers: []config.ServerConfig{
			{
				Name: "devapm1",
				Host: "10.0.0.30",
				ExtraFiles: []config.ExtraFile{
					{
						LocalPath:  tempFile(t, "hazelcast.tgz"),
						RemotePath: "/home/ec2-user/software/hazelcast/hazelcast.tgz",
					},
				},
			},
		},
	}

	if err := RunWithOptions(cfg, RunOptions{DryRun: true}); err != nil {
		t.Fatalf("expected server extra files dry-run to succeed, got %v", err)
	}

	want := []string{"bootstrap", "server-extra-files:/home/ec2-user/software/hazelcast/hazelcast.tgz"}
	if !reflect.DeepEqual(order, want) {
		t.Fatalf("unexpected server extra file order: got %v want %v", order, want)
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
