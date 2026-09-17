package docker

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"go-deployer/internal/config"
	"go-deployer/pkg/logger"
)

func dockerConfig() *config.Config {
	return &config.Config{
		Docker: config.DockerDeploymentConfig{Registry: config.DockerRegistryConfig{
			Host: "harbor.example.com", Username: "robot$deploy", TokenEnv: "HARBOR_TOKEN",
		}},
		Servers: []config.ServerConfig{{
			Name: "app-1",
			Containers: []config.ContainerConfig{{
				Name: "commerce", Image: "team/commerce", Digest: "sha256:" + strings.Repeat("a", 64),
				Ports:         []config.ContainerPort{{Host: 8080, Container: 8080, Protocol: "tcp"}},
				Environment:   map[string]string{"SPRING_PROFILES_ACTIVE": "prod", "DB_PASSWORD": "top-secret"},
				RestartPolicy: "unless-stopped",
			}},
		}},
	}
}

func TestBuildPlanIsDeterministicAndDoesNotExposeEnvironmentValues(t *testing.T) {
	plan, err := BuildPlan(dockerConfig())
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Actions) != 5 {
		t.Fatalf("actions=%d", len(plan.Actions))
	}
	if plan.Actions[0].Kind != ActionVerifyEngine || plan.Actions[4].Kind != ActionReconcile {
		t.Fatalf("actions=%#v", plan.Actions)
	}
	got := plan.Actions[4].Summary
	if !strings.Contains(got, "DB_PASSWORD, SPRING_PROFILES_ACTIVE") || strings.Contains(got, "top-secret") {
		t.Fatalf("unsafe or unstable summary: %q", got)
	}
	if !strings.Contains(got, "harbor.example.com/team/commerce@sha256:") {
		t.Fatalf("digest-pinned image missing: %q", got)
	}
}

func TestRunSupportsDryRunAndRefusesUnimplementedApply(t *testing.T) {
	t.Setenv("HARBOR_TOKEN", "")
	var output bytes.Buffer
	restore := logger.SetOutput(&output)
	defer restore()
	if err := Run(dockerConfig(), Options{DryRun: true}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "DRY-RUN [pull-image]") || strings.Contains(output.String(), "top-secret") {
		t.Fatalf("output=%q", output.String())
	}
	if err := Run(dockerConfig(), Options{}); !errors.Is(err, ErrApplyNotImplemented) {
		t.Fatalf("error=%v", err)
	}
}

func TestBuildPlanRequiresContainers(t *testing.T) {
	_, err := BuildPlan(&config.Config{})
	if err == nil || !strings.Contains(err.Error(), "at least one") {
		t.Fatalf("error=%v", err)
	}
}
