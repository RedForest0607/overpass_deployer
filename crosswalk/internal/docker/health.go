package docker

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"go-deployer/internal/ssh"
)

const healthPollInterval = time.Second

type containerRuntimeState struct {
	Running bool   `json:"Running"`
	Status  string `json:"Status"`
	Health  *struct {
		Status string `json:"Status"`
	} `json:"Health"`
}

func inspectContainerRuntime(runner remoteRunner, name string) (containerRuntimeState, error) {
	command := "docker container inspect --format '{{json .State}}' " + ssh.ShellQuote(name)
	output, err := runner.Run(command)
	if err != nil {
		return containerRuntimeState{}, fmt.Errorf("container %s runtime state is unavailable", name)
	}
	var state containerRuntimeState
	if err := json.Unmarshal([]byte(strings.TrimSpace(output)), &state); err != nil {
		return containerRuntimeState{}, fmt.Errorf("container %s returned invalid runtime state", name)
	}
	return state, nil
}

// waitContainerReady는 Docker HEALTHCHECK가 있으면 healthy까지, 없으면 running 상태까지 기다린다.
func waitContainerReady(runner remoteRunner, name string, timeout, interval time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		state, err := inspectContainerRuntime(runner, name)
		if err != nil {
			return err
		}
		if !state.Running {
			return fmt.Errorf("container %s is not running (state %s)", name, state.Status)
		}
		if state.Health == nil || state.Health.Status == "healthy" {
			return nil
		}
		if state.Health.Status == "unhealthy" {
			return fmt.Errorf("container %s health check is unhealthy", name)
		}
		if time.Now().Add(interval).After(deadline) {
			return errors.New("container health check timed out")
		}
		time.Sleep(interval)
	}
}
