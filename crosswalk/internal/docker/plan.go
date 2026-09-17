package docker

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"go-deployer/internal/config"
)

type ActionKind string

const (
	ActionVerifyEngine  ActionKind = "verify-engine"
	ActionRegistryLogin ActionKind = "registry-login"
	ActionInspect       ActionKind = "inspect-container"
	ActionPull          ActionKind = "pull-image"
	ActionReconcile     ActionKind = "reconcile-container"
)

type Action struct {
	Server  string
	Kind    ActionKind
	Summary string
}

type Plan struct {
	Actions []Action
}

// BuildPlan은 변경 명령을 실행하지 않고 서버별 Docker 작업 순서를 결정한다.
// 환경변수 값은 계획에 포함하지 않아 dry-run 로그에 비밀이 노출되지 않는다.
func BuildPlan(cfg *config.Config) (Plan, error) {
	plan := Plan{}
	for _, server := range cfg.Servers {
		if len(server.Containers) == 0 {
			continue
		}
		plan.Actions = append(plan.Actions,
			Action{Server: server.Name, Kind: ActionVerifyEngine, Summary: "verify Docker engine availability"},
			Action{
				Server: server.Name,
				Kind:   ActionRegistryLogin,
				Summary: fmt.Sprintf(
					"authenticate %s to %s using token from %s",
					cfg.Docker.Registry.Username,
					cfg.Docker.Registry.Host,
					cfg.Docker.Registry.TokenEnv,
				),
			},
		)

		for _, container := range server.Containers {
			image := cfg.Docker.Registry.ImageReference(container)
			plan.Actions = append(plan.Actions,
				Action{Server: server.Name, Kind: ActionInspect, Summary: fmt.Sprintf("inspect container %s and its configured image digest", container.Name)},
				Action{Server: server.Name, Kind: ActionPull, Summary: fmt.Sprintf("pull %s only when the local digest differs", image)},
				Action{Server: server.Name, Kind: ActionReconcile, Summary: containerSummary(container, image)},
			)
		}
	}
	if len(plan.Actions) == 0 {
		return Plan{}, errors.New("docker mode requires at least one server container")
	}
	return plan, nil
}

func containerSummary(container config.ContainerConfig, image string) string {
	ports := make([]string, 0, len(container.Ports))
	for _, port := range container.Ports {
		ports = append(ports, fmt.Sprintf("%d:%d/%s", port.Host, port.Container, port.Protocol))
	}
	environmentKeys := make([]string, 0, len(container.Environment))
	for name := range container.Environment {
		environmentKeys = append(environmentKeys, name)
	}
	sort.Strings(environmentKeys)
	return fmt.Sprintf(
		"reconcile %s with image %s, ports [%s], environment keys [%s], restart %s, health timeout %ds",
		container.Name,
		image,
		strings.Join(ports, ", "),
		strings.Join(environmentKeys, ", "),
		container.RestartPolicy,
		container.HealthTimeoutSec,
	)
}
