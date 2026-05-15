package vm

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"go-deployer/internal/config"
)

func TestEstimateConfigDeploymentIncludesBootstrapAndFiles(t *testing.T) {
	t.Helper()

	jarPath := writeEstimateFile(t, "app.jar", 1024)
	extraPath := writeEstimateFile(t, "software.tar", estimatedTransferBytesPerSecond+1)
	enabled := true

	estimate, err := estimateConfigDeployment(&config.Config{
		Bastion: config.BastionConfig{Host: "bastion.example.com"},
		Servers: []config.ServerConfig{
			{
				Host: "app.example.com",
				Name: "app",
				Bootstrap: config.BootstrapConfig{
					Packages: []string{"nc"},
					OSUpdate: config.OSUpdateConfig{
						Enabled: &enabled,
					},
					Timezone: config.TimezoneConfig{Name: "Asia/Seoul"},
					Swap: config.SwapConfig{
						Enabled: &enabled,
						Path:    "/swapfile",
						Size:    "4G",
					},
				},
				App: config.AppConfig{
					Name:    "sample",
					BaseDir: "/app/sample",
					Jar: config.JarConfig{
						LocalPath:  jarPath,
						RemotePath: "/app/sample/bin/app.jar",
					},
					ExtraFiles: []config.ExtraFile{
						{
							LocalPath:  extraPath,
							RemotePath: "/app/sample/software.tar",
							Extract: config.ExtractConfig{
								Enabled:   true,
								RemoteDir: "/app/sample",
							},
						},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("expected estimate to succeed, got %v", err)
	}

	if estimate.ServerCount != 1 {
		t.Fatalf("expected one server, got %d", estimate.ServerCount)
	}
	if estimate.FileCount != 2 {
		t.Fatalf("expected two files, got %d", estimate.FileCount)
	}
	if estimate.TransferBytes <= estimatedTransferBytesPerSecond {
		t.Fatalf("expected transfer bytes to include both files, got %d", estimate.TransferBytes)
	}
	if estimate.Duration <= 3*time.Minute {
		t.Fatalf("expected bootstrap-heavy estimate above 3m, got %s", estimate.Duration)
	}
}

func TestFormatEstimateDuration(t *testing.T) {
	t.Helper()

	if got := formatEstimateDuration(45 * time.Second); got != "45s" {
		t.Fatalf("unexpected seconds duration: %q", got)
	}
	if got := formatEstimateDuration(90 * time.Minute); got != "1h 30m" {
		t.Fatalf("unexpected hour duration: %q", got)
	}
}

func TestEstimateConfigDeploymentReturnsFileErrors(t *testing.T) {
	t.Helper()

	_, err := estimateConfigDeployment(&config.Config{
		Servers: []config.ServerConfig{
			{
				Host: "app.example.com",
				Name: "app",
				App: config.AppConfig{
					Name: "sample",
					Jar: config.JarConfig{
						LocalPath:  filepath.Join(t.TempDir(), "missing.jar"),
						RemotePath: "/app/sample/bin/app.jar",
					},
				},
			},
		},
	})
	if err == nil || !strings.Contains(err.Error(), "missing.jar") {
		t.Fatalf("expected missing file error, got %v", err)
	}
}

func writeEstimateFile(t *testing.T, name string, size int64) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), name)
	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("creating estimate file: %v", err)
	}
	defer file.Close()
	if err := file.Truncate(size); err != nil {
		t.Fatalf("sizing estimate file: %v", err)
	}
	return path
}
