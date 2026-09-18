package compose

import (
	"errors"
	"fmt"

	"go-deployer/internal/config"
)

type ActionKind string

const (
	ActionVerifyEngine  ActionKind = "verify-engine"
	ActionVerifyCompose ActionKind = "verify-compose"
	ActionRegistryLogin ActionKind = "registry-login"
	ActionSyncFile      ActionKind = "sync-compose-file"
	ActionValidate      ActionKind = "validate-compose"
	ActionPull          ActionKind = "pull-compose-images"
	ActionApply         ActionKind = "apply-compose-project"
)

type Action struct {
	Server  string
	Project string
	Kind    ActionKind
	Summary string
}

type Plan struct {
	Actions []Action
}

// BuildPlan creates a deterministic, credential-free Compose deployment plan.
// Phase 1 does not open SSH connections or execute these actions.
func BuildPlan(cfg *config.Config) (Plan, error) {
	plan := Plan{}
	for _, server := range cfg.Servers {
		if len(server.ComposeProjects) == 0 {
			continue
		}
		plan.Actions = append(plan.Actions,
			Action{Server: server.Name, Kind: ActionVerifyEngine, Summary: "verify Docker engine availability"},
			Action{Server: server.Name, Kind: ActionVerifyCompose, Summary: "verify Docker Compose v2 plugin availability"},
			Action{Server: server.Name, Kind: ActionRegistryLogin, Summary: fmt.Sprintf("authenticate %s to %s using token from %s", cfg.Docker.Registry.Username, cfg.Docker.Registry.Host, cfg.Docker.Registry.TokenEnv)},
		)
		for _, project := range server.ComposeProjects {
			remoteFile := project.RemoteDir + "/compose.yml"
			plan.Actions = append(plan.Actions,
				Action{Server: server.Name, Project: project.Name, Kind: ActionSyncFile, Summary: fmt.Sprintf("sync %s to %s", project.LocalPath, remoteFile)},
				Action{Server: server.Name, Project: project.Name, Kind: ActionValidate, Summary: fmt.Sprintf("run docker compose --project-name %s config --quiet", project.Name)},
				Action{Server: server.Name, Project: project.Name, Kind: ActionPull, Summary: "pull digest-pinned Compose service images"},
				Action{Server: server.Name, Project: project.Name, Kind: ActionApply, Summary: "apply the Compose project with orphan removal"},
			)
		}
	}
	if len(plan.Actions) == 0 {
		return Plan{}, errors.New("compose mode requires at least one compose project")
	}
	return plan, nil
}
