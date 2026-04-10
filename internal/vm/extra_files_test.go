package vm

import (
	"fmt"
	"testing"

	"go-deployer/internal/config"
)

func TestDeployExtraFilesDryRunSupportsChmod(t *testing.T) {
	localPath := tempFile(t, "server.sh")
	app := &config.AppConfig{
		Name: "sample",
		ExtraFiles: []config.ExtraFile{
			{
				LocalPath:  localPath,
				RemotePath: "/opt/sample/bin/server.sh",
				Chmod:      "774",
			},
		},
	}

	if err := DeployExtraFiles(nil, app, RunOptions{DryRun: true}, "sample-host"); err != nil {
		t.Fatalf("expected dry-run extra file deployment to succeed, got %v", err)
	}
}

func TestApplyRemoteFileModeBuildsExpectedCommand(t *testing.T) {
	runner := &extraFilesRunner{}

	if err := applyRemoteFileMode(runner, "/opt/sample/bin/setEnv.sh", "774"); err != nil {
		t.Fatalf("expected chmod helper to succeed, got %v", err)
	}
	if len(runner.commands) != 1 {
		t.Fatalf("expected one chmod command, got %v", runner.commands)
	}
	if want := "chmod '774' '/opt/sample/bin/setEnv.sh'"; runner.commands[0] != want {
		t.Fatalf("unexpected chmod command: got %q want %q", runner.commands[0], want)
	}
}

type extraFilesRunner struct {
	commands []string
}

func (r *extraFilesRunner) Run(cmd string) (string, error) {
	r.commands = append(r.commands, cmd)
	return "", nil
}

func (r *extraFilesRunner) RunSudo(cmd string) (string, error) {
	return "", fmt.Errorf("unexpected sudo command: %s", cmd)
}

func (r *extraFilesRunner) Host() string {
	return "sample-host"
}

func (r *extraFilesRunner) Close() error {
	return nil
}
