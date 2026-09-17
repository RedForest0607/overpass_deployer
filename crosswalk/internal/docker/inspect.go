package docker

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"go-deployer/internal/ssh"
)

var dockerImageIDPattern = regexp.MustCompile(`^sha256:[a-f0-9]{64}$`)

type containerImageState struct {
	Exists      bool
	ImageID     string
	RepoDigests []string
}

type localImageState struct {
	Exists      bool
	ImageID     string
	RepoDigests []string
}

// inspectContainerImage는 이름이 정확히 일치하는 container와 그 image digest 상태를 조회한다.
func inspectContainerImage(runner remoteRunner, name string) (containerImageState, error) {
	found, err := containerExists(runner, name)
	if err != nil {
		return containerImageState{}, err
	}
	if !found {
		return containerImageState{}, nil
	}

	inspectCommand := "docker container inspect --format '{{.Image}}' " + ssh.ShellQuote(name)
	output, err := runner.Run(inspectCommand)
	if err != nil {
		return containerImageState{}, fmt.Errorf("failed to inspect container %s", name)
	}
	imageID := strings.TrimSpace(output)
	if !dockerImageIDPattern.MatchString(imageID) {
		return containerImageState{}, fmt.Errorf("container %s returned an invalid image ID", name)
	}

	digests, err := inspectRepoDigests(runner, imageID)
	if err != nil {
		return containerImageState{}, fmt.Errorf("container %s image metadata is unavailable", name)
	}
	return containerImageState{Exists: true, ImageID: imageID, RepoDigests: digests}, nil
}

func containerExists(runner remoteRunner, name string) (bool, error) {
	listCommand := "docker container ls --all --filter " + ssh.ShellQuote("name="+name) + " --format '{{.Names}}'"
	output, err := runner.Run(listCommand)
	if err != nil {
		return false, errors.New("failed to list Docker containers")
	}
	for _, candidate := range strings.Fields(output) {
		if candidate == name {
			return true, nil
		}
	}
	return false, nil
}

// inspectLocalImage는 exact digest reference가 로컬에 존재하는지 확인하고 RepoDigests를 재검증한다.
func inspectLocalImage(runner remoteRunner, reference string) (localImageState, error) {
	listCommand := "docker image ls --digests --no-trunc --format '{{.Repository}}@{{.Digest}} {{.ID}}'"
	output, err := runner.Run(listCommand)
	if err != nil {
		return localImageState{}, errors.New("failed to inspect local Docker images")
	}
	var ids []string
	for _, line := range nonEmptyLines(output) {
		fields := strings.Fields(line)
		if len(fields) == 2 && fields[0] == reference {
			ids = append(ids, fields[1])
		}
	}
	if len(ids) == 0 {
		return localImageState{}, nil
	}
	if len(ids) != 1 || !dockerImageIDPattern.MatchString(ids[0]) {
		return localImageState{}, errors.New("local Docker image query returned an invalid image ID")
	}

	digests, err := inspectRepoDigests(runner, ids[0])
	if err != nil {
		return localImageState{}, errors.New("local Docker image metadata is unavailable")
	}
	return localImageState{Exists: true, ImageID: ids[0], RepoDigests: digests}, nil
}

func inspectRepoDigests(runner remoteRunner, imageReference string) ([]string, error) {
	command := "docker image inspect --format '{{json .RepoDigests}}' " + ssh.ShellQuote(imageReference)
	output, err := runner.Run(command)
	if err != nil {
		return nil, err
	}
	trimmed := strings.TrimSpace(output)
	if trimmed == "null" {
		return nil, nil
	}
	var digests []string
	if err := json.Unmarshal([]byte(trimmed), &digests); err != nil {
		return nil, err
	}
	return digests, nil
}

func nonEmptyLines(value string) []string {
	var lines []string
	for _, line := range strings.Split(value, "\n") {
		if trimmed := strings.TrimSpace(line); trimmed != "" {
			lines = append(lines, trimmed)
		}
	}
	return lines
}

func containsDigest(digests []string, desired string) bool {
	for _, digest := range digests {
		if digest == desired {
			return true
		}
	}
	return false
}
