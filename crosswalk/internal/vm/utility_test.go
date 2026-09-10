package vm

import (
	"testing"

	"go-deployer/internal/config"
)

func TestDeployUtilitiesDryRunPlansFilesBeforeServices(t *testing.T) {
	filePath := tempFile(t, "redis.conf")
	utilities := []config.UtilityConfig{
		{
			Name:        "redis",
			Directories: []string{"/app/redis"},
			Files: []config.ExtraFile{
				{LocalPath: filePath, RemotePath: "/app/redis/redis.conf"},
			},
			Services: []config.ServiceConfig{{Name: "redis-server"}},
		},
	}

	if err := DeployUtilities(nil, utilities, RunOptions{DryRun: true}, "cache-01"); err != nil {
		t.Fatalf("expected utility dry-run to succeed, got %v", err)
	}
}
