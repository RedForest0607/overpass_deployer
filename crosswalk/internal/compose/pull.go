package compose

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"

	"go-deployer/internal/docker"
	"go-deployer/internal/ssh"
	"gopkg.in/yaml.v3"
)

var pinnedImage = regexp.MustCompile(`^[a-z0-9.-]+(?::[0-9]+)?(?:/[a-z0-9._/-]+)+@sha256:[a-f0-9]{64}$`)

type composeSpec struct {
	Name     string `yaml:"name"`
	Services map[string]struct {
		Image       string            `yaml:"image"`
		Ports       []string          `yaml:"ports"`
		Environment map[string]string `yaml:"environment"`
		Restart     string            `yaml:"restart"`
	} `yaml:"services"`
}

type desiredCompose struct {
	revision       string
	images         []string
	resolvedImages []string
	services       map[string]struct{}
}

func readDesiredCompose(localPath, registryHost string) (desiredCompose, error) {
	content, err := os.ReadFile(localPath)
	if err != nil {
		return desiredCompose{}, fmt.Errorf("read Compose document: %w", err)
	}
	var spec composeSpec
	decoder := yaml.NewDecoder(bytes.NewReader(content))
	decoder.KnownFields(true)
	if err := decoder.Decode(&spec); err != nil || len(spec.Services) == 0 {
		return desiredCompose{}, errors.New("Compose document must use the generated image, ports, environment, and restart fields")
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF || bytes.Contains(content, []byte("$")) {
		return desiredCompose{}, errors.New("Compose revision cannot contain multiple documents or environment interpolation")
	}
	sum := sha256.Sum256(content)
	result := desiredCompose{
		revision: hex.EncodeToString(sum[:]),
		services: make(map[string]struct{}, len(spec.Services)),
	}
	seen := map[string]bool{}
	for name, service := range spec.Services {
		if name == "" || !pinnedImage.MatchString(service.Image) || !strings.HasPrefix(service.Image, registryHost+"/") {
			return desiredCompose{}, fmt.Errorf("service %q requires a digest-pinned image from %s", name, registryHost)
		}
		result.services[name] = struct{}{}
		result.resolvedImages = append(result.resolvedImages, service.Image)
		if !seen[service.Image] {
			result.images = append(result.images, service.Image)
			seen[service.Image] = true
		}
	}
	return result, nil
}

func pullImages(runner docker.RemoteRunner, images []string) (bool, error) {
	changed := false
	for _, image := range images {
		if !pinnedImage.MatchString(image) {
			return false, errors.New("refusing an image without a pinned SHA-256 digest")
		}
		present, err := imageHasDigest(runner, image)
		if err != nil {
			return false, err
		}
		if present {
			continue
		}
		if _, err := runner.Run("docker pull " + ssh.ShellQuote(image)); err != nil {
			return false, fmt.Errorf("pulling image %s failed", image)
		}
		verified, err := imageHasDigest(runner, image)
		if err != nil || !verified {
			return false, fmt.Errorf("pulled image %s failed digest verification", image)
		}
		changed = true
	}
	return changed, nil
}

func imageHasDigest(runner docker.RemoteRunner, image string) (bool, error) {
	list, err := runner.Run("docker image ls --digests --no-trunc --format '{{.Repository}}@{{.Digest}}'")
	if err != nil {
		return false, errors.New("failed to list local Docker images")
	}
	found := false
	for _, reference := range strings.Fields(list) {
		if reference == image {
			found = true
			break
		}
	}
	if !found {
		return false, nil
	}
	output, err := runner.Run("docker image inspect --format '{{json .RepoDigests}}' " + ssh.ShellQuote(image))
	if err != nil {
		return false, errors.New("failed to inspect local Docker image metadata")
	}
	var digests []string
	if err := json.Unmarshal([]byte(strings.TrimSpace(output)), &digests); err != nil {
		return false, errors.New("invalid Docker image digest metadata")
	}
	for _, digest := range digests {
		if digest == image {
			return true, nil
		}
	}
	return false, nil
}
