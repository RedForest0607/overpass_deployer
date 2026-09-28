package compose

import (
	"strings"
	"testing"

	"go-deployer/internal/config"
)

func composeConfig() *config.Config {
	return &config.Config{
		Docker: config.DockerDeploymentConfig{Registry: config.DockerRegistryConfig{Host: "harbor.example.com", Username: "robot-studio", TokenEnv: "HARBOR_TOKEN"}},
		Servers: []config.ServerConfig{{
			Name:            "compose-dev-01",
			Host:            "192.0.2.20",
			ComposeProjects: []config.ComposeProjectConfig{{Name: "commerce", LocalPath: "./compose/compose-dev-01/compose.yml", RemoteDir: "/app/overpass/compose/commerce"}},
		}},
	}
}

func TestBuildPlanKeepsCredentialsOutAndOrdersActions(t *testing.T) {
	plan, err := BuildPlan(composeConfig())
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Actions) != 10 {
		t.Fatalf("actions=%#v", plan.Actions)
	}
	got := make([]string, 0, len(plan.Actions))
	for _, action := range plan.Actions {
		got = append(got, string(action.Kind))
		if strings.Contains(action.Summary, "actual-token") {
			t.Fatalf("plan leaked token: %#v", action)
		}
	}
	want := "verify-engine,verify-compose,registry-login,sync-compose-file,validate-compose,pull-compose-images,activate-compose-revision,apply-compose-project,verify-compose-health,rollback-compose-on-failure"
	if strings.Join(got, ",") != want {
		t.Fatalf("actions=%s", strings.Join(got, ","))
	}
}

func TestRunRequiresExplicitApply(t *testing.T) {
	if err := Run(composeConfig(), Options{}); err == nil || !strings.Contains(err.Error(), "--apply") {
		t.Fatalf("expected explicit apply error, got %v", err)
	}
	if err := Run(composeConfig(), Options{DryRun: true}); err != nil {
		t.Fatalf("dry-run failed: %v", err)
	}
}
