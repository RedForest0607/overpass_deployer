package vm

import (
	"strings"
	"testing"

	"go-deployer/internal/config"
)

func TestCollectRemoteFilePathsDeduplicatesServerAndAppFiles(t *testing.T) {
	t.Helper()

	paths := collectRemoteFilePaths(config.ServerConfig{
		ExtraFiles: []config.ExtraFile{
			{RemotePath: "/opt/shared/software.tar"},
			{
				RemotePath: "/app/elasticsearch/elasticsearch.tar.gz",
				Extract: config.ExtractConfig{
					Enabled:     true,
					Sudo:        true,
					StagingPath: "/tmp/elasticsearch.tar.gz",
				},
			},
		},
		App: config.AppConfig{
			BaseDir: "/app/sample",
			Jar: config.JarConfig{
				RemotePath: "/app/sample/lib/app.jar",
			},
			ConfigFiles: []config.ConfigFile{
				{RemotePath: "/app/sample/config/application.yml"},
			},
			ExtraFiles: []config.ExtraFile{
				{RemotePath: "/opt/shared/software.tar"},
			},
			Script: config.ScriptConfig{
				RemotePath: "/app/sample/bin/server.sh",
			},
		},
	})

	want := strings.Join([]string{
		"/opt/shared/software.tar",
		"/tmp/elasticsearch.tar.gz",
		"/app/sample/lib/app.jar",
		"/app/sample/config/application.yml",
		"/app/sample/bin/server.sh",
	}, "\n")
	if got := strings.Join(paths, "\n"); got != want {
		t.Fatalf("unexpected remote paths:\ngot:\n%s\nwant:\n%s", got, want)
	}
}

func TestFetchRemoteSHA256BatchInitializesMissingFiles(t *testing.T) {
	t.Helper()

	runner := &checksumRunner{
		output: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef  /app/sample/lib/app.jar\n",
	}

	hashes, err := fetchRemoteSHA256Batch(runner, []string{
		"/app/sample/lib/app.jar",
		"/app/sample/lib/missing.jar",
	})
	if err != nil {
		t.Fatalf("expected batch checksum to succeed, got %v", err)
	}
	if hashes["/app/sample/lib/app.jar"] != "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef" {
		t.Fatalf("expected existing hash to be parsed, got %q", hashes["/app/sample/lib/app.jar"])
	}
	if hashes["/app/sample/lib/missing.jar"] != "" {
		t.Fatalf("expected missing hash to be empty, got %q", hashes["/app/sample/lib/missing.jar"])
	}
	if !strings.Contains(runner.command, "sha256sum") || !strings.Contains(runner.command, "/app/sample/lib/app.jar") {
		t.Fatalf("expected batched sha256sum command, got %q", runner.command)
	}
}

type checksumRunner struct {
	command string
	output  string
}

func (r *checksumRunner) Run(cmd string) (string, error) {
	r.command = cmd
	return r.output, nil
}

func (r *checksumRunner) RunSudo(cmd string) (string, error) {
	return "", nil
}

func (r *checksumRunner) Host() string {
	return "checksum-host"
}

func (r *checksumRunner) Close() error {
	return nil
}
