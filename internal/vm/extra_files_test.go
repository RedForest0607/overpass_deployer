package vm

import (
	"strings"
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

func TestBuildRemoteArchiveExtractionCommand(t *testing.T) {
	cmd, err := buildRemoteArchiveExtractionCommand(config.ExtraFile{
		RemotePath: "/home/ec2-user/software/elasticsearch/elasticsearch.tar.gz",
		Extract: config.ExtractConfig{
			Enabled:         true,
			RemoteDir:       "/home/ec2-user/software",
			StripComponents: 1,
		},
	})
	if err != nil {
		t.Fatalf("expected extraction command to build, got %v", err)
	}

	want := "tar -xzf '/home/ec2-user/software/elasticsearch/elasticsearch.tar.gz' -C '/home/ec2-user/software' --strip-components=1"
	if cmd != want {
		t.Fatalf("unexpected extraction command: got %q want %q", cmd, want)
	}
}

func TestBuildRemoteArchiveExtractionCommandSupportsSudoStagingAndOwnership(t *testing.T) {
	cmd, err := buildRemoteArchiveExtractionCommand(config.ExtraFile{
		RemotePath: "/app/elasticsearch/elasticsearch.tar.gz",
		Extract: config.ExtractConfig{
			Enabled:         true,
			RemoteDir:       "/app/elasticsearch",
			StripComponents: 1,
			StagingPath:     "/tmp/elasticsearch.tar.gz",
			Sudo:            true,
			Owner:           "elasticsearch",
			Group:           "elasticsearch",
			ChownRecursive:  true,
		},
	})
	if err != nil {
		t.Fatalf("expected extraction command to build, got %v", err)
	}

	want := "tar -xzf '/tmp/elasticsearch.tar.gz' -C '/app/elasticsearch' --strip-components=1 && chown -R 'elasticsearch:elasticsearch' '/app/elasticsearch'"
	if cmd != want {
		t.Fatalf("unexpected extraction command: got %q want %q", cmd, want)
	}
}

func TestApplyRemoteArchiveExtractionPreparesExtractDirectory(t *testing.T) {
	runner := &extraFilesRunner{}

	err := applyRemoteArchiveExtraction(runner, config.ExtraFile{
		RemotePath: "/home/ec2-user/software/elasticsearch/elasticsearch.tar.gz",
		Extract: config.ExtractConfig{
			Enabled:   true,
			RemoteDir: "/home/ec2-user/software",
		},
	})
	if err != nil {
		t.Fatalf("expected archive extraction to succeed, got %v", err)
	}
	if len(runner.sudoCommands) != 1 {
		t.Fatalf("expected one sudo setup command, got %v", runner.sudoCommands)
	}
	if strings.Contains(runner.sudoCommands[0], "chown -R") {
		t.Fatalf("expected sudo setup command not to recursively chown extract directory, got %q", runner.sudoCommands[0])
	}
	if !strings.Contains(runner.sudoCommands[0], `chown "${owner_name}:${owner_group}" "${base_dir}"`) {
		t.Fatalf("expected sudo setup command to chown only extract directory, got %q", runner.sudoCommands[0])
	}
	if len(runner.commands) != 1 {
		t.Fatalf("expected one extraction command, got %v", runner.commands)
	}
	if want := "tar -xzf '/home/ec2-user/software/elasticsearch/elasticsearch.tar.gz' -C '/home/ec2-user/software'"; runner.commands[0] != want {
		t.Fatalf("unexpected extraction command: got %q want %q", runner.commands[0], want)
	}
}

func TestApplyRemoteArchiveExtractionUsesSudoForPrivilegedExtract(t *testing.T) {
	runner := &extraFilesRunner{}

	err := applyRemoteArchiveExtraction(runner, config.ExtraFile{
		RemotePath: "/app/elasticsearch/elasticsearch.tar.gz",
		Extract: config.ExtractConfig{
			Enabled:     true,
			RemoteDir:   "/app/elasticsearch",
			StagingPath: "/tmp/elasticsearch.tar.gz",
			Sudo:        true,
			Owner:       "elasticsearch",
			Group:       "elasticsearch",
		},
	})
	if err != nil {
		t.Fatalf("expected privileged archive extraction to succeed, got %v", err)
	}
	if len(runner.commands) != 0 {
		t.Fatalf("expected no non-sudo extraction commands, got %v", runner.commands)
	}
	if len(runner.sudoCommands) != 2 {
		t.Fatalf("expected setup and extraction sudo commands, got %v", runner.sudoCommands)
	}
	if strings.Contains(runner.sudoCommands[0], "chown") {
		t.Fatalf("expected privileged setup not to chown extract directory, got %q", runner.sudoCommands[0])
	}
	if !strings.Contains(runner.sudoCommands[1], "/tmp/elasticsearch.tar.gz") {
		t.Fatalf("expected extraction to use staging path, got %q", runner.sudoCommands[1])
	}
}

func TestBuildRemoteArchiveExtractionCommandRejectsUnsupportedArchive(t *testing.T) {
	_, err := buildRemoteArchiveExtractionCommand(config.ExtraFile{
		RemotePath: "/home/ec2-user/software/software.zip",
		Extract: config.ExtractConfig{
			Enabled:   true,
			RemoteDir: "/home/ec2-user/software",
		},
	})
	if err == nil {
		t.Fatal("expected unsupported archive error")
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
	commands     []string
	sudoCommands []string
}

func (r *extraFilesRunner) Run(cmd string) (string, error) {
	r.commands = append(r.commands, cmd)
	return "", nil
}

func (r *extraFilesRunner) RunSudo(cmd string) (string, error) {
	r.sudoCommands = append(r.sudoCommands, cmd)
	return "", nil
}

func (r *extraFilesRunner) Host() string {
	return "sample-host"
}

func (r *extraFilesRunner) Close() error {
	return nil
}
